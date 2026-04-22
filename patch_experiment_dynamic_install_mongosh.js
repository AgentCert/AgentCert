const d = db.getSiblingDB('litmus');

// Dynamic: Accept experiment ID/name from environment or command line
const targetExperimentID = typeof process !== 'undefined' && process.env && process.env.EXPERIMENT_ID ? process.env.EXPERIMENT_ID : null;
const targetExperimentName = typeof process !== 'undefined' && process.env && process.env.EXPERIMENT_NAME ? process.env.EXPERIMENT_NAME : null;

function extractNamespace(args) {
  if (!Array.isArray(args)) return '';
  for (const arg of args) {
    if (typeof arg === 'string' && arg.startsWith('-namespace=')) {
      return arg.substring('-namespace='.length);
    }
  }
  return '';
}

function ensureInstallTimeoutParameter(wf) {
  if (!wf.spec) wf.spec = {};
  if (!wf.spec.arguments) wf.spec.arguments = {};
  if (!Array.isArray(wf.spec.arguments.parameters)) wf.spec.arguments.parameters = [];

  const params = wf.spec.arguments.parameters;
  const existing = params.find((p) => p && p.name === 'installTimeout');
  if (!existing) {
    params.push({ name: 'installTimeout', value: '15m' });
    return true;
  }
  if (!existing.value) {
    existing.value = '15m';
    return true;
  }
  return false;
}

function buildNormalizeTemplate(templateName, namespaceExpr) {
  const script = [
    'set -eu',
    `NS="${namespaceExpr}"`,
    'MAX_DELAY="${READINESS_MAX_INITIAL_DELAY:-120}"',
    'TARGET_DELAY="${READINESS_TARGET_INITIAL_DELAY:-45}"',
    'PERIOD="${READINESS_PERIOD_SECONDS:-5}"',
    'TIMEOUT="${READINESS_TIMEOUT_SECONDS:-2}"',
    'DEADLINE=$(($(date +%s) + 900))',
    'while [ "$(date +%s)" -lt "$DEADLINE" ]; do',
    '  if kubectl get ns "$NS" >/dev/null 2>&1; then',
    '    DEPLOYS=$(kubectl get deploy -n "$NS" -o jsonpath="{range .items[*]}{.metadata.name}{\"\\n\"}{end}" 2>/dev/null || true)',
    '    if [ -n "$DEPLOYS" ]; then',
    '      echo "$DEPLOYS" | while IFS= read -r DEPLOY; do',
    '        [ -z "$DEPLOY" ] && continue',
    '        CONTAINER=$(kubectl get deploy -n "$NS" "$DEPLOY" -o jsonpath="{.spec.template.spec.containers[0].name}" 2>/dev/null || true)',
    '        DELAY=$(kubectl get deploy -n "$NS" "$DEPLOY" -o jsonpath="{.spec.template.spec.containers[0].readinessProbe.initialDelaySeconds}" 2>/dev/null || true)',
    '        if echo "$DELAY" | grep -Eq "^[0-9]+$" && [ "$DELAY" -gt "$MAX_DELAY" ]; then',
    '          kubectl patch deployment "$DEPLOY" -n "$NS" --type=merge -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"$CONTAINER\",\"readinessProbe\":{\"initialDelaySeconds\":$TARGET_DELAY,\"periodSeconds\":$PERIOD,\"timeoutSeconds\":$TIMEOUT}}]}}}}" >/dev/null || true',
    '          echo "patched $DEPLOY readiness delay $DELAY -> $TARGET_DELAY"',
    '        fi',
    '      done',
    '      exit 0',
    '    fi',
    '  fi',
    '  sleep 5',
    'done',
    'exit 0'
  ].join('\n');

  return {
    name: templateName,
    inputs: {},
    outputs: {},
    metadata: {},
    container: {
      name: '',
      image: 'litmuschaos/k8s:latest',
      command: ['sh', '-c'],
      args: [script],
      resources: {}
    }
  };
}

function patchWorkflowObject(wf) {
  if (!wf || !wf.spec || !Array.isArray(wf.spec.templates)) return false;

  let changed = false;
  const templates = wf.spec.templates;
  const tplMap = {};
  templates.forEach((t) => { if (t && t.name) tplMap[t.name] = t; });

  // Ensure installTimeout parameter exists.
  if (ensureInstallTimeoutParameter(wf)) changed = true;

  // Patch install-application timeout args to use parameter.
  for (const t of templates) {
    if (!t || t.name !== 'install-application') continue;
    if (!t.container || !Array.isArray(t.container.args)) continue;

    let hasTimeout = false;
    t.container.args = t.container.args.map((arg) => {
      if (typeof arg !== 'string') return arg;
      if (arg.startsWith('-timeout=')) {
        hasTimeout = true;
        const desired = '-timeout={{workflow.parameters.installTimeout}}';
        if (arg !== desired) {
          changed = true;
          return desired;
        }
      }
      return arg;
    });

    if (!hasTimeout) {
      t.container.args.push('-timeout={{workflow.parameters.installTimeout}}');
      changed = true;
    }
  }

  // Add readiness normalization helper AFTER install steps (sequential, not parallel).
  const entrypoint = wf.spec.entrypoint || 'argowf-chaos';
  const root = templates.find((t) => t && t.name === entrypoint);
  if (root && Array.isArray(root.steps)) {
    // Find groups with install-application or install-agent and insert readiness checks after
    let groupIndex = 0;
    while (groupIndex < root.steps.length) {
      const group = root.steps[groupIndex];
      if (!Array.isArray(group)) {
        groupIndex++;
        continue;
      }

      const hasInstallApp = group.some((s) => s && s.name === 'install-application');
      const hasInstallAgent = group.some((s) => s && s.name === 'install-agent');

      let needsReadinessCheck = false;
      let readinessStepName = null;
      let readinessNamespace = null;

      if (hasInstallApp) {
        needsReadinessCheck = true;
        readinessStepName = 'normalize-install-application-readiness';
        const appTpl = tplMap['install-application'];
        readinessNamespace = extractNamespace(appTpl?.container?.args) || '{{workflow.parameters.appNamespace}}';
      } else if (hasInstallAgent) {
        needsReadinessCheck = true;
        readinessStepName = 'normalize-install-agent-readiness';
        const agentTpl = tplMap['install-agent'];
        readinessNamespace = extractNamespace(agentTpl?.container?.args) || '{{workflow.parameters.appNamespace}}';
      }

      if (needsReadinessCheck) {
        // Insert readiness check in a NEW step group AFTER current group (sequential)
        const newGroup = [{ name: readinessStepName, template: readinessStepName, arguments: {} }];
        root.steps.splice(groupIndex + 1, 0, newGroup);
        
        // Create the readiness template if not exists
        if (!tplMap[readinessStepName]) {
          const helperTpl = buildNormalizeTemplate(readinessStepName, readinessNamespace);
          templates.push(helperTpl);
          tplMap[readinessStepName] = helperTpl;
        }
        changed = true;
        groupIndex += 2; // Skip both the original group and the new readiness group
        continue;
      }

      groupIndex++;
    }
  }

  return changed;
}

let patched = 0;

// Build dynamic query
let query = {};
if (targetExperimentID) {
  query.experimentID = targetExperimentID;
} else if (targetExperimentName) {
  query.name = targetExperimentName;
}
// If no filter, query will match all documents

print('[PATCH INFO] Query: ' + JSON.stringify(query));
print('[PATCH INFO] Will patch all experiments matching query...');
print('');

const docs = d.chaosExperiments.find(query).toArray();
if (!docs || docs.length === 0) {
  print('No chaosExperiments found matching query: ' + JSON.stringify(query));
  quit(1);
}

// Iterate through all matching experiments and patch each
for (let docIdx = 0; docIdx < docs.length; docIdx++) {
  const doc = docs[docIdx];
  const docPatched = { count: 0, name: doc.name, id: doc.experimentID };
  
  const revs = Array.isArray(doc.revision) ? doc.revision : [];
  for (let i = 0; i < revs.length; i++) {
    const rev = revs[i] || {};
    const hasExperiment = typeof rev.experiment_manifest === 'string';
    const hasWorkflow = typeof rev.workflow_manifest === 'string';
    if (!hasExperiment && !hasWorkflow) continue;

    const field = hasExperiment ? `revision.${i}.experiment_manifest` : `revision.${i}.workflow_manifest`;
    const raw = hasExperiment ? rev.experiment_manifest : rev.workflow_manifest;

    let wf;
    try {
      wf = JSON.parse(raw);
    } catch (e) {
      continue;
    }

    if (!patchWorkflowObject(wf)) continue;

    d.chaosExperiments.updateOne(
      { _id: doc._id },
      { $set: { [field]: JSON.stringify(wf) } }
    );
    docPatched.count += 1;
    patched += 1;
  }
  
  if (docPatched.count > 0) {
    print(`✓ Patched experiment "${docPatched.name}" (ID: ${docPatched.id}): ${docPatched.count} revision(s)`);
  }
}

print('');
print('=== SUMMARY ===');
print('Total experiments processed: ' + docs.length);
print('Total revisions patched: ' + patched);
if (patched > 0) {
  print('✓ All patches applied successfully!');
} else {
  print('⚠ No revisions were patched (all workflows may already be patched)');
}

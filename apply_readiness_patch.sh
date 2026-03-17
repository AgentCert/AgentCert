#!/bin/bash
# Apply readiness patch to current running sock-shop experiment

EXPERIMENT_NAME="${1:-sock-shop}"

echo "[PATCHING] Applying readiness normalization patch to: $EXPERIMENT_NAME"
echo

# Create mongosh script with dynamic experiment name
wsl -d Ubuntu -- bash -c "cat > /tmp/patch_readiness_dynamic.js << 'ENDSCRIPT'
const d = db.getSiblingDB('litmus');

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
    \`NS=\"\${namespaceExpr}\"\`,
    'MAX_DELAY=\"\${READINESS_MAX_INITIAL_DELAY:-120}\"',
    'TARGET_DELAY=\"\${READINESS_TARGET_INITIAL_DELAY:-45}\"',
    'PERIOD=\"\${READINESS_PERIOD_SECONDS:-5}\"',
    'TIMEOUT=\"\${READINESS_TIMEOUT_SECONDS:-2}\"',
    'DEADLINE=\$((\\$(date +%s) + 900))',
    'while [ \"\\$(date +%s)\" -lt \"\$DEADLINE\" ]; do',
    '  if kubectl get ns \"\$NS\" >/dev/null 2>&1; then',
    '    DEPLOYS=\$(kubectl get deploy -n \"\$NS\" -o jsonpath=\"{range .items[*]}{.metadata.name}{\\\\n}{end}\" 2>/dev/null || true)',
    '    if [ -n \"\$DEPLOYS\" ]; then',
    '      echo \"\$DEPLOYS\" | while IFS= read -r DEPLOY; do',
    '        [ -z \"\$DEPLOY\" ] && continue',
    '        CONTAINER=\$(kubectl get deploy -n \"\$NS\" \"\$DEPLOY\" -o jsonpath=\"{.spec.template.spec.containers[0].name}\" 2>/dev/null || true)',
    '        DELAY=\$(kubectl get deploy -n \"\$NS\" \"\$DEPLOY\" -o jsonpath=\"{.spec.template.spec.containers[0].readinessProbe.initialDelaySeconds}\" 2>/dev/null || true)',
    '        if echo \"\$DELAY\" | grep -Eq \"^[0-9]+\$\" && [ \"\$DELAY\" -gt \"\$MAX_DELAY\" ]; then',
    '          kubectl patch deployment \"\$DEPLOY\" -n \"\$NS\" --type=merge -p \"{\\\\\"spec\\\\\":{\\\\\"template\\\\\":{\\\\\"spec\\\\\":{\\\\\"containers\\\\\":[{\\\\\"name\\\\\":\\\\\"\$CONTAINER\\\\\",\\\\\"readinessProbe\\\\\":{\\\\\"initialDelaySeconds\\\\\":\$TARGET_DELAY,\\\\\"periodSeconds\\\\\":\$PERIOD,\\\\\"timeoutSeconds\\\\\":\$TIMEOUT}}]}}}}\" >/dev/null || true',
    '        fi',
    '      done',
    '    fi',
    '    kubectl rollout status deploy -n \"\$NS\" --timeout=10m 2>/dev/null && break',
    '  fi',
    '  sleep 5',
    'done',
  ].join('\\n');

  return {
    name: templateName,
    container: {
      image: 'litmuschaos/k8s:latest',
      command: ['sh', '-c'],
      args: [script],
    },
    inputs: {},
    outputs: {},
    metadata: {},
  };
}

function patchWorkflowObject(wf) {
  if (!wf || !wf.spec || !Array.isArray(wf.spec.templates)) return false;

  let changed = false;
  const templates = wf.spec.templates;
  const tplMap = {};

  for (const t of templates) {
    if (t && t.name) tplMap[t.name] = t;
  }

  if (!('install-application' in tplMap)) return false;

  changed |= ensureInstallTimeoutParameter(wf);

  const appTpl = tplMap['install-application'];
  if (appTpl && appTpl.container && Array.isArray(appTpl.container.args)) {
    let hasTimeout = false;
    appTpl.container.args = appTpl.container.args.map((arg) => {
      if (typeof arg === 'string' && arg.startsWith('-timeout=')) {
        hasTimeout = true;
        return '-timeout={{workflow.parameters.installTimeout}}';
      }
      return arg;
    });

    if (!hasTimeout) {
      appTpl.container.args.push('-timeout={{workflow.parameters.installTimeout}}');
      changed = true;
    }
  }

  // Add readiness normalization helper AFTER install steps (sequential, not parallel).
  const entrypoint = wf.spec.entrypoint || 'argowf-chaos';
  const root = templates.find((t) => t && t.name === entrypoint);
  if (root && Array.isArray(root.steps)) {
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
        const newGroup = [{ name: readinessStepName, template: readinessStepName, arguments: {} }];
        root.steps.splice(groupIndex + 1, 0, newGroup);
        
        if (!tplMap[readinessStepName]) {
          const helperTpl = buildNormalizeTemplate(readinessStepName, readinessNamespace);
          templates.push(helperTpl);
          tplMap[readinessStepName] = helperTpl;
        }
        changed = true;
        groupIndex += 2;
        continue;
      }

      groupIndex++;
    }
  }

  return changed;
}

let patched = 0;
const query = { name: '${EXPERIMENT_NAME}' };

print('[PATCH INFO] Querying for experiments: ' + JSON.stringify(query));
const docs = d.chaosExperiments.find(query).toArray();

if (!docs || docs.length === 0) {
  print('✗ No experiments found with name: ${EXPERIMENT_NAME}');
  quit(1);
}

print('✓ Found ' + docs.length + ' experiment(s)');
print('');

for (let docIdx = 0; docIdx < docs.length; docIdx++) {
  const doc = docs[docIdx];
  const docPatched = { count: 0, name: doc.name, id: doc.experimentID };
  
  const revs = Array.isArray(doc.revision) ? doc.revision : [];
  for (let i = 0; i < revs.length; i++) {
    const rev = revs[i] || {};
    const hasExperiment = typeof rev.experiment_manifest === 'string';
    const hasWorkflow = typeof rev.workflow_manifest === 'string';
    if (!hasExperiment && !hasWorkflow) continue;

    const field = hasExperiment ? \`revision.\${i}.experiment_manifest\` : \`revision.\${i}.workflow_manifest\`;
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
      { \$set: { [field]: JSON.stringify(wf) } }
    );
    docPatched.count += 1;
    patched += 1;
  }
  
  if (docPatched.count > 0) {
    print(\`✓ Patched experiment \"\${docPatched.name}\" (ID: \${docPatched.id}): \${docPatched.count} revision(s)\`);
  } else {
    print(\`⚠ Experiment \"\${docPatched.name}\" - no revisions patched (may already be patched)\`);
  }
}

print('');
print('=== SUMMARY ===');
print('Total experiments processed: ' + docs.length);
print('Total revisions patched: ' + patched);
if (patched > 0) {
  print('✓ Readiness normalization patch applied successfully!');
  print('✓ Future experiment runs will wait for all pods to be Ready (1/1) before proceeding.');
} else {
  print('⚠ No revisions were patched');
}
ENDSCRIPT
mongosh --quiet 'mongodb://root:1234@localhost:27017/admin?authSource=admin&replicaSet=rs0' /tmp/patch_readiness_dynamic.js"

echo
echo "Cleaning up..."
wsl -d Ubuntu -- rm -f /tmp/patch_readiness_dynamic.js

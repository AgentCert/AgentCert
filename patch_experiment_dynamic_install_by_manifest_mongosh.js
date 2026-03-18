const d = db.getSiblingDB('litmus');
const targetExperimentID = '465be14d-e462-4589-a4f3-79c3c56b0e39';

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

  if (ensureInstallTimeoutParameter(wf)) changed = true;

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

  const entrypoint = wf.spec.entrypoint || 'argowf-chaos';
  const root = templates.find((t) => t && t.name === entrypoint);
  if (root && Array.isArray(root.steps)) {
    for (const group of root.steps) {
      if (!Array.isArray(group)) continue;

      const hasInstallApp = group.some((s) => s && s.name === 'install-application');
      const hasInstallAgent = group.some((s) => s && s.name === 'install-agent');

      if (hasInstallApp) {
        const helperStep = 'normalize-install-application-readiness';
        if (!group.some((s) => s && s.name === helperStep)) {
          group.push({ name: helperStep, template: helperStep, arguments: {} });
          changed = true;
        }
        if (!tplMap[helperStep]) {
          const appTpl = tplMap['install-application'];
          const ns = extractNamespace(appTpl?.container?.args) || '{{workflow.parameters.appNamespace}}';
          const helperTpl = buildNormalizeTemplate(helperStep, ns);
          templates.push(helperTpl);
          tplMap[helperStep] = helperTpl;
          changed = true;
        }
      }

      if (hasInstallAgent) {
        const helperStep = 'normalize-install-agent-readiness';
        if (!group.some((s) => s && s.name === helperStep)) {
          group.push({ name: helperStep, template: helperStep, arguments: {} });
          changed = true;
        }
        if (!tplMap[helperStep]) {
          const agentTpl = tplMap['install-agent'];
          const ns = extractNamespace(agentTpl?.container?.args) || '{{workflow.parameters.appNamespace}}';
          const helperTpl = buildNormalizeTemplate(helperStep, ns);
          templates.push(helperTpl);
          tplMap[helperStep] = helperTpl;
          changed = true;
        }
      }
    }
  }

  return changed;
}

let docsPatched = 0;
let revsPatched = 0;

d.chaosExperiments.find({}).forEach((doc) => {
  const revs = Array.isArray(doc.revision) ? doc.revision : [];
  let docChanged = false;

  revs.forEach((rev, i) => {
    const hasExperiment = typeof rev.experiment_manifest === 'string';
    const hasWorkflow = typeof rev.workflow_manifest === 'string';
    if (!hasExperiment && !hasWorkflow) return;

    const field = hasExperiment ? `revision.${i}.experiment_manifest` : `revision.${i}.workflow_manifest`;
    const raw = hasExperiment ? rev.experiment_manifest : rev.workflow_manifest;
    if (!raw || raw.indexOf(targetExperimentID) === -1) return;

    let wf;
    try { wf = JSON.parse(raw); } catch (e) { return; }

    if (!patchWorkflowObject(wf)) return;

    d.chaosExperiments.updateOne({ _id: doc._id }, { $set: { [field]: JSON.stringify(wf) } });
    revsPatched += 1;
    docChanged = true;
    print(`Patched chaosExperiments _id=${doc._id} revision=${i}`);
  });

  if (docChanged) docsPatched += 1;
});

print(`docsPatched=${docsPatched} revsPatched=${revsPatched}`);

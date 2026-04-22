const d = db.getSiblingDB('litmus');

function getNamespaceExpr(installTpl) {
  const args = installTpl?.container?.args;
  if (!Array.isArray(args)) return '{{workflow.parameters.appNamespace}}';
  for (const a of args) {
    if (typeof a === 'string' && a.startsWith('-namespace=')) {
      return a.slice('-namespace='.length);
    }
  }
  return '{{workflow.parameters.appNamespace}}';
}

function buildPodsUpTemplate(templateName, namespaceExpr) {
  const script = [
    'set -eu',
    `NS="${namespaceExpr}"`,
    'MAX_WAIT="${PODS_UP_TIMEOUT_SECONDS:-600}"',
    'DEADLINE=$(($(date +%s) + MAX_WAIT))',
    'echo "Checking pods up in namespace ${NS}"',
    'while [ "$(date +%s)" -lt "$DEADLINE" ]; do',
    '  PODS=$(kubectl get pods -n "$NS" --no-headers 2>/dev/null || true)',
    '  if [ -z "$PODS" ]; then',
    '    sleep 5',
    '    continue',
    '  fi',
    '  BAD=$(echo "$PODS" | awk "{ if ($3 != \"Running\" && $3 != \"Completed\") print $0 }")',
    '  if [ -z "$BAD" ]; then',
    '    echo "All pods are up (Running/Completed) in ${NS}"',
    '    exit 0',
    '  fi',
    '  sleep 5',
    'done',
    'echo "Timed out waiting for pods up in ${NS}"',
    'kubectl get pods -n "$NS" --no-headers || true',
    'exit 1'
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

function patchWorkflow(wf) {
  if (!wf?.spec?.templates || !Array.isArray(wf.spec.templates)) return false;

  let changed = false;
  const templates = wf.spec.templates;
  const entrypoint = wf.spec.entrypoint || 'argowf-chaos';
  const root = templates.find((t) => t?.name === entrypoint);
  if (!root || !Array.isArray(root.steps)) return false;

  const tmap = {};
  templates.forEach((t) => {
    if (t?.name) tmap[t.name] = t;
  });

  // 1) Remove installer internal wait so workflow controls gating behavior.
  const installTpl = tmap['install-application'];
  if (installTpl?.container && Array.isArray(installTpl.container.args)) {
    const before = installTpl.container.args.length;
    installTpl.container.args = installTpl.container.args.filter((a) => a !== '-wait');
    if (installTpl.container.args.length !== before) changed = true;
  }

  // 2) Add explicit pods-up gate after install-application step group.
  const gateStepName = 'check-application-pods-up';
  const gateTplName = 'check-application-pods-up';

  const appNsExpr = getNamespaceExpr(installTpl);

  if (!tmap[gateTplName]) {
    templates.push(buildPodsUpTemplate(gateTplName, appNsExpr));
    tmap[gateTplName] = true;
    changed = true;
  }

  for (let i = 0; i < root.steps.length; i++) {
    const group = root.steps[i];
    if (!Array.isArray(group)) continue;

    const hasInstallApp = group.some((s) => s?.name === 'install-application');
    if (!hasInstallApp) continue;

    const nextGroup = root.steps[i + 1];
    const nextHasGate = Array.isArray(nextGroup) && nextGroup.some((s) => s?.name === gateStepName);
    if (!nextHasGate) {
      root.steps.splice(i + 1, 0, [
        {
          name: gateStepName,
          template: gateTplName,
          arguments: {}
        }
      ]);
      changed = true;
    }
    break;
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

    let wf;
    try {
      wf = JSON.parse(raw);
    } catch {
      return;
    }

    if (!patchWorkflow(wf)) return;

    d.chaosExperiments.updateOne(
      { _id: doc._id },
      { $set: { [field]: JSON.stringify(wf) } }
    );

    print(`patched doc=${doc._id} rev=${i} name=${doc.name || ''}`);
    revsPatched += 1;
    docChanged = true;
  });

  if (docChanged) docsPatched += 1;
});

print(`docsPatched=${docsPatched} revisionsPatched=${revsPatched}`);

// Usage:
//   INSTALL_TIMEOUT=15m mongosh --quiet 'mongodb://root:1234@localhost:27017/admin?authSource=admin' patch_install_readiness_mongosh.js
//
// This patches stored chaos workflow manifests so any install-application or
// install-agent step runs in parallel with a readiness-normalization helper.
// The helper watches the target namespace and lowers only excessively large
// readinessProbe.initialDelaySeconds values, which prevents false rollout waits.

const dbName = process.env.DB_NAME || 'litmus';
const targetDb = db.getSiblingDB(dbName);

function extractNamespace(args) {
  if (!Array.isArray(args)) return '';
  for (const arg of args) {
    if (typeof arg === 'string' && arg.startsWith('-namespace=')) {
      return arg.slice('-namespace='.length);
    }
  }
  return '';
}

function helperTemplateName(stepName) {
  return `normalize-${stepName}-readiness`;
}

function buildHelperTemplate(templateName, namespace) {
  const script = [
    'set -eu',
    `NS="${namespace}"`,
    'MAX_DELAY="${READINESS_MAX_INITIAL_DELAY:-120}"',
    'TARGET_DELAY="${READINESS_TARGET_INITIAL_DELAY:-45}"',
    'PERIOD="${READINESS_PERIOD_SECONDS:-5}"',
    'TIMEOUT="${READINESS_TIMEOUT_SECONDS:-2}"',
    'DEADLINE=$(($(date +%s) + 900))',
    'echo "Normalizing readiness probes in namespace ${NS}"',
    'while [ "$(date +%s)" -lt "$DEADLINE" ]; do',
    '  if kubectl get ns "$NS" >/dev/null 2>&1; then',
    '    DEPLOYS=$(kubectl get deploy -n "$NS" -o jsonpath="{range .items[*]}{.metadata.name}{\"\\n\"}{end}" 2>/dev/null || true)',
    '    if [ -n "$DEPLOYS" ]; then',
    '      PATCHED=0',
    '      echo "$DEPLOYS" | while IFS= read -r DEPLOY; do',
    '        [ -z "$DEPLOY" ] && continue',
    '        CONTAINER=$(kubectl get deploy -n "$NS" "$DEPLOY" -o jsonpath="{.spec.template.spec.containers[0].name}" 2>/dev/null || true)',
    '        DELAY=$(kubectl get deploy -n "$NS" "$DEPLOY" -o jsonpath="{.spec.template.spec.containers[0].readinessProbe.initialDelaySeconds}" 2>/dev/null || true)',
    '        if echo "$DELAY" | grep -Eq "^[0-9]+$" && [ "$DELAY" -gt "$MAX_DELAY" ]; then',
    '          kubectl patch deployment "$DEPLOY" -n "$NS" --type=merge -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"$CONTAINER\",\"readinessProbe\":{\"initialDelaySeconds\":$TARGET_DELAY,\"periodSeconds\":$PERIOD,\"timeoutSeconds\":$TIMEOUT}}]}}}}" >/dev/null || true',
    '          echo "Patched deployment/$DEPLOY readiness delay ${DELAY}s -> ${TARGET_DELAY}s"',
    '          PATCHED=$((PATCHED + 1))',
    '        fi',
    '      done',
    '      echo "Readiness normalization complete for ${NS}"',
    '      exit 0',
    '    fi',
    '  fi',
    '  sleep 5',
    'done',
    'echo "No deployments detected to normalize in ${NS}; continuing"',
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

function ensureParallelNormalization(wf) {
  if (!wf || !wf.spec || !Array.isArray(wf.spec.templates)) {
    return false;
  }

  const templates = wf.spec.templates;
  const templateMap = {};
  for (const tpl of templates) {
    if (tpl && tpl.name) templateMap[tpl.name] = tpl;
  }

  const entrypoint = wf.spec.entrypoint || 'argowf-chaos';
  const root = templates.find((tpl) => tpl && tpl.name === entrypoint);
  if (!root || !Array.isArray(root.steps)) {
    return false;
  }

  let changed = false;

  for (const group of root.steps) {
    if (!Array.isArray(group)) continue;

    for (const step of group) {
      if (!step || (step.name !== 'install-application' && step.name !== 'install-agent')) {
        continue;
      }

      const sourceTemplate = templateMap[step.template || step.name];
      const namespace = extractNamespace(sourceTemplate?.container?.args);
      if (!namespace) continue;

      const normalizeStepName = helperTemplateName(step.name);
      const hasParallelHelper = group.some((s) => s && s.name === normalizeStepName);
      if (!hasParallelHelper) {
        group.push({
          name: normalizeStepName,
          template: normalizeStepName,
          arguments: {}
        });
        changed = true;
      }

      if (!templateMap[normalizeStepName]) {
        const helper = buildHelperTemplate(normalizeStepName, namespace);
        templates.push(helper);
        templateMap[normalizeStepName] = helper;
        changed = true;
      }
    }
  }

  return changed;
}

let patched = 0;

targetDb.chaosExperiments.find({}).forEach((doc) => {
  const revs = Array.isArray(doc.revision) ? doc.revision : [];
  revs.forEach((rev, idx) => {
    const hasExperiment = typeof rev.experiment_manifest === 'string';
    const hasWorkflow = typeof rev.workflow_manifest === 'string';
    if (!hasExperiment && !hasWorkflow) return;

    const field = hasExperiment ? `revision.${idx}.experiment_manifest` : `revision.${idx}.workflow_manifest`;
    const raw = hasExperiment ? rev.experiment_manifest : rev.workflow_manifest;

    let wf;
    try {
      wf = JSON.parse(raw);
    } catch (e) {
      return;
    }

    if (!ensureParallelNormalization(wf)) return;

    targetDb.chaosExperiments.updateOne(
      { _id: doc._id },
      { $set: { [field]: JSON.stringify(wf) } }
    );
    patched += 1;
    print(`Patched readiness normalization in ${doc.name || doc._id} revision ${idx}`);
  });
});

print(`Total revisions patched: ${patched}`);

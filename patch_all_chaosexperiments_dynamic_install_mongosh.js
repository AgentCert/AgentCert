const d = db.getSiblingDB('litmus');

function ensureInstallTimeoutParameter(wf) {
  if (!wf.spec) wf.spec = {};
  if (!wf.spec.arguments) wf.spec.arguments = {};
  if (!Array.isArray(wf.spec.arguments.parameters)) wf.spec.arguments.parameters = [];

  const params = wf.spec.arguments.parameters;
  const p = params.find((x) => x && x.name === 'installTimeout');
  if (!p) {
    params.push({ name: 'installTimeout', value: '15m' });
    return true;
  }
  if (!p.value) {
    p.value = '15m';
    return true;
  }
  return false;
}

function patchInstallArgs(templates) {
  let changed = false;
  for (const t of templates) {
    if (!t || !['install-application', 'install-agent'].includes(t.name)) continue;
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
  return changed;
}

function addHelperTemplatesAndSteps(wf) {
  if (!wf.spec || !Array.isArray(wf.spec.templates)) return false;
  const templates = wf.spec.templates;
  const entry = wf.spec.entrypoint || 'argowf-chaos';
  const root = templates.find((t) => t && t.name === entry);
  if (!root || !Array.isArray(root.steps)) return false;

  const names = new Set(templates.map((t) => t && t.name).filter(Boolean));
  let changed = false;

  function helperTemplate(name) {
    return {
      name,
      inputs: {},
      outputs: {},
      metadata: {},
      container: {
        name: '',
        image: 'litmuschaos/k8s:latest',
        command: ['sh', '-c'],
        args: [
          'set -eu\nNS="{{workflow.parameters.appNamespace}}"\nMAX_DELAY="${READINESS_MAX_INITIAL_DELAY:-120}"\nTARGET_DELAY="${READINESS_TARGET_INITIAL_DELAY:-45}"\nPERIOD="${READINESS_PERIOD_SECONDS:-5}"\nTIMEOUT="${READINESS_TIMEOUT_SECONDS:-2}"\nDEADLINE=$(($(date +%s) + 900))\nwhile [ "$(date +%s)" -lt "$DEADLINE" ]; do\n  if kubectl get ns "$NS" >/dev/null 2>&1; then\n    DEPLOYS=$(kubectl get deploy -n "$NS" -o jsonpath="{range .items[*]}{.metadata.name}{\\"\\n\\"}{end}" 2>/dev/null || true)\n    if [ -n "$DEPLOYS" ]; then\n      echo "$DEPLOYS" | while IFS= read -r DEPLOY; do\n        [ -z "$DEPLOY" ] && continue\n        CONTAINER=$(kubectl get deploy -n "$NS" "$DEPLOY" -o jsonpath="{.spec.template.spec.containers[0].name}" 2>/dev/null || true)\n        DELAY=$(kubectl get deploy -n "$NS" "$DEPLOY" -o jsonpath="{.spec.template.spec.containers[0].readinessProbe.initialDelaySeconds}" 2>/dev/null || true)\n        if echo "$DELAY" | grep -Eq "^[0-9]+$" && [ "$DELAY" -gt "$MAX_DELAY" ]; then\n          kubectl patch deployment "$DEPLOY" -n "$NS" --type=merge -p "{\\"spec\\":{\\"template\\":{\\"spec\\":{\\"containers\\":[{\\"name\\":\\"$CONTAINER\\",\\"readinessProbe\\":{\\"initialDelaySeconds\\":$TARGET_DELAY,\\"periodSeconds\\":$PERIOD,\\"timeoutSeconds\\":$TIMEOUT}}]}}}}" >/dev/null || true\n        fi\n      done\n      exit 0\n    fi\n  fi\n  sleep 5\ndone\nexit 0'
        ],
        resources: {}
      }
    };
  }

  for (const group of root.steps) {
    if (!Array.isArray(group)) continue;

    if (group.some((s) => s && s.name === 'install-application')) {
      const stepName = 'normalize-install-application-readiness';
      if (!group.some((s) => s && s.name === stepName)) {
        group.push({ name: stepName, template: stepName, arguments: {} });
        changed = true;
      }
      if (!names.has(stepName)) {
        templates.push(helperTemplate(stepName));
        names.add(stepName);
        changed = true;
      }
    }

    if (group.some((s) => s && s.name === 'install-agent')) {
      const stepName = 'normalize-install-agent-readiness';
      if (!group.some((s) => s && s.name === stepName)) {
        group.push({ name: stepName, template: stepName, arguments: {} });
        changed = true;
      }
      if (!names.has(stepName)) {
        templates.push(helperTemplate(stepName));
        names.add(stepName);
        changed = true;
      }
    }
  }

  return changed;
}

let docs = 0;
let revs = 0;

d.chaosExperiments.find({}).forEach((doc) => {
  const revision = Array.isArray(doc.revision) ? doc.revision : [];
  let docChanged = false;

  revision.forEach((rev, i) => {
    const hasExperiment = typeof rev.experiment_manifest === 'string';
    const hasWorkflow = typeof rev.workflow_manifest === 'string';
    if (!hasExperiment && !hasWorkflow) return;

    const field = hasExperiment ? `revision.${i}.experiment_manifest` : `revision.${i}.workflow_manifest`;
    const raw = hasExperiment ? rev.experiment_manifest : rev.workflow_manifest;
    if (!raw) return;

    let wf;
    try {
      wf = JSON.parse(raw);
    } catch (e) {
      return;
    }

    let changed = false;
    if (ensureInstallTimeoutParameter(wf)) changed = true;
    if (patchInstallArgs(wf.spec.templates || [])) changed = true;
    if (addHelperTemplatesAndSteps(wf)) changed = true;

    if (!changed) return;

    d.chaosExperiments.updateOne({ _id: doc._id }, { $set: { [field]: JSON.stringify(wf) } });
    revs += 1;
    docChanged = true;
    print(`patched doc=${doc._id} rev=${i} name=${doc.name || ''}`);
  });

  if (docChanged) docs += 1;
});

print(`docsPatched=${docs} revisionsPatched=${revs}`);

const d = db.getSiblingDB('litmus');

const doc = d.chaosExperiments.find({ name: 'sock-shop' }).sort({ _id: -1 }).limit(1).toArray()[0];
if (!doc) {
  print('no sock-shop doc found');
  quit(1);
}

const rev = (doc.revision || [])[0] || {};
const raw = rev.experiment_manifest || rev.workflow_manifest || '';
if (!raw) {
  print('no manifest in latest sock-shop doc');
  quit(1);
}

const wf = JSON.parse(raw);
const templates = wf.spec.templates || [];
const root = templates.find((t) => t.name === (wf.spec.entrypoint || 'argowf-chaos'));
const install = templates.find((t) => t.name === 'install-application');

const args = install?.container?.args || [];
print('doc=' + doc._id);
print('installArgs=' + JSON.stringify(args));
print('hasWait=' + args.includes('-wait'));
print('hasParamTimeout=' + args.includes('-timeout={{workflow.parameters.installTimeout}}'));

let hasGateGroup = false;
if (Array.isArray(root?.steps)) {
  for (const group of root.steps) {
    if (!Array.isArray(group)) continue;
    if (group.some((s) => s?.name === 'check-application-pods-up')) {
      hasGateGroup = true;
      break;
    }
  }
}

const hasGateTemplate = templates.some((t) => t?.name === 'check-application-pods-up');
print('hasGateStep=' + hasGateGroup + ' hasGateTemplate=' + hasGateTemplate);

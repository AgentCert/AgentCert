const d = db.getSiblingDB('litmus');
const targetExperimentID = '465be14d-e462-4589-a4f3-79c3c56b0e39';

let found = false;

d.chaosExperiments.find({}).forEach((doc) => {
  const revs = Array.isArray(doc.revision) ? doc.revision : [];
  revs.forEach((rev, i) => {
    const raw = rev.experiment_manifest || rev.workflow_manifest || '';
    if (!raw || raw.indexOf(targetExperimentID) === -1) return;

    found = true;
    let wf;
    try { wf = JSON.parse(raw); } catch (e) { return; }

    const templates = Array.isArray(wf?.spec?.templates) ? wf.spec.templates : [];
    const install = templates.find((t) => t?.name === 'install-application');
    const args = install?.container?.args || [];

    const params = ((wf.spec || {}).arguments || {}).parameters || [];
    const installTimeoutParam = params.find((p) => p?.name === 'installTimeout');

    const entrypoint = wf.spec?.entrypoint || 'argowf-chaos';
    const root = templates.find((t) => t?.name === entrypoint);
    const hasNormalizeStep = Array.isArray(root?.steps) && root.steps.some((g) => Array.isArray(g) && g.some((s) => String(s?.name || '').indexOf('normalize-install-application-readiness') === 0));
    const hasNormalizeTemplate = templates.some((t) => t?.name === 'normalize-install-application-readiness');

    print('doc=' + doc._id + ' revision=' + i);
    print('installArgs=' + JSON.stringify(args));
    print('installTimeoutParam=' + JSON.stringify(installTimeoutParam || null));
    print('hasNormalizeStep=' + hasNormalizeStep + ' hasNormalizeTemplate=' + hasNormalizeTemplate);
  });
});

if (!found) {
  print('target experiment manifest not found in chaosExperiments');
}

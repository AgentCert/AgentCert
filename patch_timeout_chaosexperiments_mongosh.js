const d = db.getSiblingDB('litmus');
const targetTimeout = (process.env.INSTALL_TIMEOUT || '15m');
let patched = 0;

d.chaosExperiments.find({}).forEach((doc) => {
  const revs = Array.isArray(doc.revision) ? doc.revision : [];
  revs.forEach((rev, idx) => {
    const hasExperiment = typeof rev.experiment_manifest === 'string';
    const hasWorkflow = typeof rev.workflow_manifest === 'string';
    if (!hasExperiment && !hasWorkflow) return;

    const field = hasExperiment ? `revision.${idx}.experiment_manifest` : `revision.${idx}.workflow_manifest`;
    const m = hasExperiment ? rev.experiment_manifest : rev.workflow_manifest;
    if (typeof m !== 'string' || m.indexOf('-timeout=5m') === -1) return;

    const updated = m.replace(/-timeout=5m/g, `-timeout=${targetTimeout}`);
    d.chaosExperiments.updateOne({ _id: doc._id }, { $set: { [field]: updated } });
    print(`Patched ${doc.name || doc._id} revision ${idx}`);
    patched += 1;
  });
});

print(`Total revisions patched: ${patched}`);

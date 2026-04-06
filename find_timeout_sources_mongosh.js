const d = db.getSiblingDB('litmus');
const cols = d.getCollectionNames();
print('collections=' + cols.length);

for (const c of cols) {
  let foundInCollection = 0;

  if (c === 'chaosExperiments') {
    d[c].find({}).forEach((doc) => {
      const revs = Array.isArray(doc.revision) ? doc.revision : [];
      for (const r of revs) {
        const m = (r.experiment_manifest || r.workflow_manifest || '');
        if (typeof m === 'string' && m.indexOf('-timeout=5m') !== -1) {
          print('COL=' + c + ' ID=' + doc._id + ' NAME=' + (doc.name || ''));
          foundInCollection += 1;
          break;
        }
      }
    });
    continue;
  }

  d[c].find({}, { projection: { _id: 1, name: 1, experiment_name: 1, experiment_manifest: 1, workflow_manifest: 1 } }).limit(200).forEach((doc) => {
    const em = typeof doc.experiment_manifest === 'string' ? doc.experiment_manifest : '';
    const wm = typeof doc.workflow_manifest === 'string' ? doc.workflow_manifest : '';
    if (em.indexOf('-timeout=5m') !== -1 || wm.indexOf('-timeout=5m') !== -1) {
      print('COL=' + c + ' ID=' + doc._id + ' NAME=' + (doc.name || doc.experiment_name || ''));
      foundInCollection += 1;
    }
  });
}

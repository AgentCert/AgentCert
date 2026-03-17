const d = db.getSiblingDB('litmus');
const cols = d.getCollectionNames();

function hasHardcode(doc) {
  try {
    const s = JSON.stringify(doc);
    return s.includes('"-timeout=5m"') || s.includes('-timeout=5m');
  } catch (e) {
    return false;
  }
}

for (const c of cols) {
  let count = 0;
  d[c].find({}).forEach((doc) => {
    if (!hasHardcode(doc)) return;
    if (count < 20) {
      print(`COL=${c} _id=${doc._id} name=${doc.name || doc.experimentName || doc.experimentID || ''}`);
    }
    count += 1;
  });
  if (count > 0) {
    print(`COL=${c} totalHardcoded=${count}`);
  }
}

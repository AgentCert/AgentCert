const d = db.getSiblingDB('litmus');
const targetExperimentID = '465be14d-e462-4589-a4f3-79c3c56b0e39';

const collections = d.getCollectionNames();
print('collections=' + collections.join(','));

for (const c of collections) {
  let hit = false;
  d[c].find({}).limit(500).forEach((doc) => {
    const asText = JSON.stringify(doc);
    if (asText.indexOf(targetExperimentID) !== -1) {
      if (!hit) {
        print('\nCOL=' + c);
        hit = true;
      }
      print('  _id=' + (doc._id || ''));
      if (doc.experimentID) print('  experimentID=' + doc.experimentID);
      if (doc.name) print('  name=' + doc.name);
      if (doc.experimentName) print('  experimentName=' + doc.experimentName);
    }
  });
}

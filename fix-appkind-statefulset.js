// Fix appkind values for database targets in chaos experiments
// Changes statefulset -> deployment for user-db and catalogue-db

const { MongoClient } = require("mongodb");

async function fixAppKindInExperiments() {
  const client = new MongoClient("mongodb://root:1234@localhost:27017/litmus?authSource=admin");

  try {
    await client.connect();
    const db = client.db("litmus");
    const collection = db.collection("chaosExperiments");

    console.log("Scanning chaosExperiments for appkind: statefulset...\n");

    // Find experiments with statefulset appkind for database targets
    const experiments = await collection.find({}).toArray();
    let fixed = 0;

    for (const exp of experiments) {
      if (!exp.revision || !Array.isArray(exp.revision)) continue;

      for (let revIdx = 0; revIdx < exp.revision.length; revIdx++) {
        const rev = exp.revision[revIdx];
        if (!rev.experimentManifest) continue;

        let manifest = rev.experimentManifest;
        let changed = false;

        // Fix: user-db and catalogue-db should be deployment, not statefulset
        if (manifest.includes(`appkind: statefulset`) && 
            (manifest.includes(`name=user-db`) || manifest.includes(`name=catalogue-db`))) {
          
          // Replace appkind: statefulset with appkind: deployment for these database targets
          manifest = manifest.replace(
            /appkind: statefulset[\s]*\n[\s]*applabel: name=(user-db|catalogue-db)/g,
            "appkind: deployment\n                applabel: name=$1"
          );
          
          if (manifest !== rev.experimentManifest) {
            rev.experimentManifest = manifest;
            changed = true;
          }
        }

        if (changed) {
          fixed++;
          console.log(`✓ Fixed: ${exp.name} (revision ${revIdx})`);
        }
      }

      if (fixed > 0 && experiments.indexOf(exp) % 5 === 0) {
        // Update document in batches
        const filter = { experiment_id: exp.experiment_id };
        await collection.updateOne(filter, { $set: { revision: exp.revision, updated_at: Date.now() } });
      }
    }

    // Final batch update for remaining documents
    for (const exp of experiments) {
      let hasChanges = false;
      if (exp.revision) {
        for (const rev of exp.revision) {
          if (rev.experimentManifest && rev.experimentManifest.includes("appkind: deployment")) {
            // Check if this was modified (already has correct appkind)
            if (exp.__updated) {
              hasChanges = true;
              break;
            }
          }
        }
      }
      
      if (hasChanges) {
        const filter = { experiment_id: exp.experiment_id };
        await collection.updateOne(filter, { $set: { revision: exp.revision, updated_at: Date.now() } });
      }
    }

    console.log(`\n✓ Fixed ${fixed} experiment revisions`);
  } catch (error) {
    console.error("Error:", error);
  } finally {
    await client.close();
  }
}

fixAppKindInExperiments();

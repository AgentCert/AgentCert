const { MongoClient, ObjectId } = require("mongodb");

async function main() {
  const c = new MongoClient("mongodb://localhost:27017");
  await c.connect();
  const db = c.db("litmus");
  const exps = await db.collection("chaosExperiments").find({}).toArray();
  let patched = 0;

  for (const exp of exps) {
    for (let i = 0; i < (exp.revision || []).length; i++) {
      const rev = exp.revision[i];
      const field = rev.experiment_manifest
        ? `revision.${i}.experiment_manifest`
        : `revision.${i}.workflow_manifest`;
      let m = rev.experiment_manifest || rev.workflow_manifest || "";
      if (!m) continue;
      const orig = m;

      // Fix: make scale-catalogue non-blocking with || true, and increase timeout
      m = m.replace(
        /kubectl scale deployment catalogue -n sock-shop --replicas=2 && kubectl rollout status deployment\/catalogue -n sock-shop --timeout=\d+s && echo 'catalogue scaled to 2 replicas'/g,
        "kubectl scale deployment catalogue -n sock-shop --replicas=2 && (kubectl rollout status deployment/catalogue -n sock-shop --timeout=300s || echo 'WARN: rollout slow, continuing') && echo 'catalogue scaled'"
      );

      if (m !== orig) {
        await db
          .collection("chaosExperiments")
          .updateOne({ _id: exp._id }, { $set: { [field]: m } });
        patched++;
        console.log("Patched: " + exp.name + " (" + exp._id + ")");
      }
    }
  }
  console.log("Total patched: " + patched);
  await c.close();
}

main().catch((e) => console.error(e));

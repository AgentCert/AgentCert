const { MongoClient } = require("mongodb");

// Patches chaos experiment workflow manifests so any install-application step
// uses a larger timeout window (default 15m).
const MONGO_URI = process.env.MONGO_URI || "mongodb://localhost:27017";
const DB_NAME = process.env.DB_NAME || "litmus";
const INSTALL_TIMEOUT = process.env.INSTALL_TIMEOUT || process.env.SOCKSHOP_INSTALL_TIMEOUT || "15m";

function patchManifestObject(wf) {
  if (!wf || !wf.spec || !Array.isArray(wf.spec.templates)) {
    return { changed: false, reason: "no-templates" };
  }

  let changed = false;

  for (const tpl of wf.spec.templates) {
    if (tpl?.name !== "install-application") continue;
    if (!tpl?.container || !Array.isArray(tpl.container.args)) continue;

    let hasTimeout = false;
    tpl.container.args = tpl.container.args.map((arg) => {
      if (typeof arg !== "string") return arg;
      if (arg.startsWith("-timeout=")) {
        hasTimeout = true;
        if (arg !== `-timeout=${INSTALL_TIMEOUT}`) {
          changed = true;
          return `-timeout=${INSTALL_TIMEOUT}`;
        }
      }
      return arg;
    });

    if (!hasTimeout) {
      tpl.container.args.push(`-timeout=${INSTALL_TIMEOUT}`);
      changed = true;
    }
  }

  return { changed, reason: changed ? "patched" : "no-change" };
}

function patchManifestString(manifest) {
  // Parse JSON and patch only install-application template args.
  try {
    const wf = JSON.parse(manifest);
    const result = patchManifestObject(wf);
    if (result.changed) {
      return { changed: true, manifest: JSON.stringify(wf), mode: "json" };
    }
    return { changed: false, manifest, mode: result.reason };
  } catch {
    return { changed: false, manifest, mode: "unparseable" };
  }
}

async function main() {
  const client = new MongoClient(MONGO_URI);
  await client.connect();
  const db = client.db(DB_NAME);

  const exps = await db.collection("chaosExperiments").find({}).toArray();
  let patched = 0;

  for (const exp of exps) {
    const revisions = exp.revision || [];
    for (let i = 0; i < revisions.length; i++) {
      const rev = revisions[i] || {};
      const hasExperiment = typeof rev.experiment_manifest === "string";
      const hasWorkflow = typeof rev.workflow_manifest === "string";
      if (!hasExperiment && !hasWorkflow) continue;

      const field = hasExperiment
        ? `revision.${i}.experiment_manifest`
        : `revision.${i}.workflow_manifest`;

      const manifest = hasExperiment ? rev.experiment_manifest : rev.workflow_manifest;
      if (!manifest) continue;

      const result = patchManifestString(manifest);
      if (!result.changed) continue;

      await db.collection("chaosExperiments").updateOne(
        { _id: exp._id },
        { $set: { [field]: result.manifest } }
      );

      patched++;
      console.log(`Patched timeout in ${exp.name || exp._id} revision ${i} (${result.mode})`);
    }
  }

  console.log(`Total revisions patched: ${patched}`);
  await client.close();
}

main().catch((err) => {
  console.error("Patch failed:", err);
  process.exit(1);
});

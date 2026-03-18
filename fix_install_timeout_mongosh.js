// Usage:
//   mongosh --quiet "$MONGO_URI" /path/to/fix_install_timeout_mongosh.js
// Optional envs:
//   DB_NAME=litmus
//   INSTALL_TIMEOUT=15m

const dbName = process.env.DB_NAME || "litmus";
const timeout = process.env.INSTALL_TIMEOUT || process.env.SOCKSHOP_INSTALL_TIMEOUT || "15m";
const targetDb = db.getSiblingDB(dbName);

function patchManifestText(text) {
  if (!text || typeof text !== "string") return { changed: false, value: text };

  let changed = false;

  try {
    const obj = JSON.parse(text);
    if (!obj?.spec?.templates || !Array.isArray(obj.spec.templates)) {
      return { changed: false, value: text, mode: "no-templates" };
    }

    for (const tpl of obj.spec.templates) {
      if (tpl?.name !== "install-application") continue;
      if (!tpl?.container || !Array.isArray(tpl.container.args)) continue;

      let hasTimeout = false;
      tpl.container.args = tpl.container.args.map((arg) => {
        if (typeof arg !== "string") return arg;
        if (arg.startsWith("-timeout=")) {
          hasTimeout = true;
          if (arg !== `-timeout=${timeout}`) {
            changed = true;
            return `-timeout=${timeout}`;
          }
        }
        return arg;
      });

      if (!hasTimeout) {
        tpl.container.args.push(`-timeout=${timeout}`);
        changed = true;
      }
    }

    if (!changed) return { changed: false, value: text, mode: "json-no-change" };
    return { changed: true, value: JSON.stringify(obj), mode: "json" };
  } catch (e) {
    return { changed: false, value: text, mode: "unparseable" };
  }
}

let patched = 0;

targetDb.chaosExperiments.find({}).forEach((exp) => {
  const revisions = exp.revision || [];
  revisions.forEach((rev, i) => {
    const hasExperiment = typeof rev.experiment_manifest === "string";
    const hasWorkflow = typeof rev.workflow_manifest === "string";
    if (!hasExperiment && !hasWorkflow) return;

    const field = hasExperiment ? `revision.${i}.experiment_manifest` : `revision.${i}.workflow_manifest`;
    const current = hasExperiment ? rev.experiment_manifest : rev.workflow_manifest;
    const result = patchManifestText(current);

    if (!result.changed) return;

    targetDb.chaosExperiments.updateOne(
      { _id: exp._id },
      { $set: { [field]: result.value } }
    );

    patched += 1;
    print(`Patched ${exp.name || exp._id} revision ${i} (${result.mode})`);
  });
});

print(`Total revisions patched: ${patched}`);

/**
 * migrate_probe_defaults.js
 *
 * Permanent migration script: updates ALL stored experiment manifests
 * and standalone probe documents in MongoDB to use hardened probe defaults.
 *
 * Run:  node migrate_probe_defaults.js
 *
 * Changes applied:
 *   - mode: Continuous  → Edge
 *   - probeTimeout: 1s  → 10s
 *   - interval: 100ms   → 5s
 *   - attempt: 2|3      → 5
 *   - responseTimeout: 100 → 5000
 *   - Adds initialDelay: 15s (if missing)
 *
 * Safe to re-run (idempotent).
 */
const { MongoClient } = require("mongodb");

const MONGO_URI = process.env.MONGO_URI || "mongodb://localhost:27017";
const DB_NAME = process.env.LITMUS_DB || "litmus";

async function main() {
  const client = new MongoClient(MONGO_URI);
  await client.connect();
  const db = client.db(DB_NAME);

  // ── 1. Patch all experiment manifests ──────────────────────────────────
  console.log("\n═══ Patching chaosExperiments manifests ═══");
  const experiments = await db.collection("chaosExperiments").find({}).toArray();
  let expPatched = 0;

  for (const exp of experiments) {
    if (!exp.revision || exp.revision.length === 0) continue;

    let changed = false;
    for (let i = 0; i < exp.revision.length; i++) {
      const rev = exp.revision[i];
      const fieldName = rev.experiment_manifest
        ? `revision.${i}.experiment_manifest`
        : `revision.${i}.workflow_manifest`;
      let manifest = rev.experiment_manifest || rev.workflow_manifest || "";
      if (!manifest) continue;

      const original = manifest;

      // Fix mode (unquoted and quoted YAML, and JSON annotation)
      manifest = manifest.replace(/mode: Continuous\b/g, "mode: Edge");
      manifest = manifest.replace(/mode: "Continuous"/g, 'mode: "Edge"');
      manifest = manifest.replace(/"mode":\s*"Continuous"/g, '"mode":"Edge"');

      // Fix probeTimeout
      manifest = manifest.replace(/probeTimeout: 1s\b/g, "probeTimeout: 10s");
      manifest = manifest.replace(/probeTimeout: 1000ms\b/g, "probeTimeout: 10s");
      manifest = manifest.replace(/probe_timeout: 1s\b/g, "probe_timeout: 10s");

      // Fix interval
      manifest = manifest.replace(/interval: 100ms\b/g, "interval: 5s");

      // Fix attempt
      manifest = manifest.replace(/attempt: [23]\b/g, "attempt: 5");

      // Fix responseTimeout
      manifest = manifest.replace(/responseTimeout: 100\b/g, "responseTimeout: 5000");

      // Add initialDelay after probeTimeout if missing
      if (!manifest.includes("initialDelay")) {
        manifest = manifest.replace(
          /(probeTimeout: 10s)/,
          "$1\n          initialDelay: 15s"
        );
      }

      if (manifest !== original) {
        await db
          .collection("chaosExperiments")
          .updateOne(
            { _id: exp._id },
            { $set: { [fieldName]: manifest } }
          );
        changed = true;
      }
    }

    if (changed) {
      expPatched++;
      console.log(`  ✅ ${exp.name} (${exp._id})`);
    }
  }
  console.log(`  Total experiments patched: ${expPatched}/${experiments.length}`);

  // ── 2. Patch all standalone probe documents ────────────────────────────
  console.log("\n═══ Patching chaosProbes documents ═══");
  const probes = await db.collection("chaosProbes").find({}).toArray();
  let probePatched = 0;

  for (const probe of probes) {
    const props =
      probe.kubernetes_http_properties ||
      probe.kubernetes_cmd_properties ||
      probe.kubernetes_prom_properties ||
      null;
    if (!props) continue;

    const updates = {};
    if (props.probe_timeout === "1s" || props.probe_timeout === "1000ms") {
      updates["kubernetes_http_properties.probe_timeout"] = "10s";
    }
    if (props.interval === "100ms") {
      updates["kubernetes_http_properties.interval"] = "5s";
    }
    if (props.attempt <= 3) {
      updates["kubernetes_http_properties.attempt"] = 5;
    }
    if (!props.retry || props.retry < 3) {
      updates["kubernetes_http_properties.retry"] = 3;
    }
    if (!props.initial_delay) {
      updates["kubernetes_http_properties.initial_delay"] = "15s";
    }
    if (props.response_timeout && props.response_timeout < 5000) {
      updates["kubernetes_http_properties.response_timeout"] = 5000;
    }

    if (Object.keys(updates).length > 0) {
      await db
        .collection("chaosProbes")
        .updateOne({ _id: probe._id }, { $set: updates });
      probePatched++;
      console.log(`  ✅ ${probe.name} (${probe._id})`);
    }
  }
  console.log(`  Total probes patched: ${probePatched}/${probes.length}`);

  // ── 3. Verify ──────────────────────────────────────────────────────────
  console.log("\n═══ Verification ═══");
  const verifyExps = await db.collection("chaosExperiments").find({}).toArray();
  let badExps = 0;
  for (const exp of verifyExps) {
    for (const rev of exp.revision || []) {
      const m = rev.experiment_manifest || rev.workflow_manifest || "";
      if (
        m.includes("probeTimeout: 1s") ||
        m.includes("interval: 100ms") ||
        m.includes("responseTimeout: 100") ||
        m.includes('mode: Continuous') ||
        m.includes('"mode":"Continuous"')
      ) {
        badExps++;
        console.log(`  ❌ Still has old values: ${exp.name}`);
      }
    }
  }

  const verifyProbes = await db.collection("chaosProbes").find({}).toArray();
  let badProbes = 0;
  for (const p of verifyProbes) {
    const hp = p.kubernetes_http_properties;
    if (hp && (hp.probe_timeout === "1s" || hp.interval === "100ms")) {
      badProbes++;
      console.log(`  ❌ Still has old values: ${p.name}`);
    }
  }

  if (badExps === 0 && badProbes === 0) {
    console.log("  ✅ All experiments and probes use hardened defaults");
  } else {
    console.log(`  ⚠️  ${badExps} experiments and ${badProbes} probes still have old values`);
  }

  await client.close();
  console.log("\nDone.");
}

main().catch((e) => {
  console.error("Migration failed:", e);
  process.exit(1);
});

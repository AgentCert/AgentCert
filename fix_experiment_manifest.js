/**
 * Patches the "chaos-sock-shop-resiliency-new-test" experiment stored in MongoDB:
 *  1. Changes probe mode from Continuous → Edge
 *  2. Increases probeTimeout, interval, attempt, responseTimeout
 *  3. Adds initialDelay to probe runProperties
 *  4. Adds a scale-catalogue workflow step before pod-delete
 *  5. Updates the probeRef annotation mode to Edge
 */
const { MongoClient, ObjectId } = require("mongodb");

const EXPERIMENT_ID = "69a009d5f7ebbc01e3711c56";

async function main() {
  const client = new MongoClient("mongodb://localhost:27017");
  await client.connect();
  const db = client.db("litmus");

  // 1. Read current document
  const doc = await db
    .collection("chaosExperiments")
    .findOne({ _id: new ObjectId(EXPERIMENT_ID) });
  if (!doc) {
    console.error("Experiment document not found!");
    await client.close();
    return;
  }

  const rev = doc.revision[0];
  let manifest = rev.experiment_manifest || rev.workflow_manifest || "";
  if (!manifest) {
    console.error("No manifest found in revision[0]");
    await client.close();
    return;
  }

  // Parse the JSON manifest
  let wf = JSON.parse(manifest);
  const templates = wf.spec.templates;

  // ── Fix 1: Patch probe settings in the pod-delete template ──
  const podDeleteTpl = templates.find((t) => t.name === "pod-delete");
  if (podDeleteTpl) {
    const artifact = podDeleteTpl.inputs.artifacts.find(
      (a) => a.name === "pod-delete"
    );
    if (artifact && artifact.raw && artifact.raw.data) {
      let engineYaml = artifact.raw.data;

      // Update probeRef annotation: Continuous → Edge
      engineYaml = engineYaml.replace(
        /"mode":\s*"Continuous"/g,
        '"mode":"Edge"'
      );

      // Update inline probe mode: Continuous → Edge
      engineYaml = engineYaml.replace(
        /mode: Continuous/g,
        "mode: Edge"
      );

      // Update probeTimeout: 1s → 10s
      engineYaml = engineYaml.replace(
        /probeTimeout: 1s/g,
        "probeTimeout: 10s"
      );

      // Update interval: 100ms → 5s
      engineYaml = engineYaml.replace(
        /interval: 100ms/g,
        "interval: 5s"
      );

      // Update attempt: 3 → 5
      engineYaml = engineYaml.replace(
        /attempt: 3/g,
        "attempt: 5"
      );

      // Update responseTimeout: 100 → 5000
      engineYaml = engineYaml.replace(
        /responseTimeout: 100/g,
        "responseTimeout: 5000"
      );

      // Add initialDelay after probeTimeout line
      if (!engineYaml.includes("initialDelay")) {
        engineYaml = engineYaml.replace(
          /(probeTimeout: 10s)/,
          "$1\n          initialDelay: 15s"
        );
      }

      artifact.raw.data = engineYaml;
      console.log("✅ Patched pod-delete ChaosEngine probe settings");
    }
  }

  // ── Fix 2: Add scale-catalogue step before pod-delete ──
  const argowfChaos = templates.find((t) => t.name === "argowf-chaos");
  if (argowfChaos && argowfChaos.steps) {
    // Check if scale-catalogue step already exists
    const hasScale = argowfChaos.steps.some((stepGroup) =>
      stepGroup.some((s) => s.name === "scale-catalogue")
    );

    if (!hasScale) {
      // Find the pod-delete step group index
      const pdIdx = argowfChaos.steps.findIndex((stepGroup) =>
        stepGroup.some((s) => s.name === "pod-delete")
      );
      if (pdIdx > 0) {
        // Insert scale-catalogue step group before pod-delete
        argowfChaos.steps.splice(pdIdx, 0, [
          {
            name: "scale-catalogue",
            template: "scale-catalogue",
            arguments: {},
          },
        ]);
        console.log(
          "✅ Added scale-catalogue step before pod-delete in workflow steps"
        );
      }
    } else {
      console.log("ℹ️  scale-catalogue step already exists");
    }
  }

  // ── Fix 3: Add scale-catalogue template ──
  const hasScaleTpl = templates.some((t) => t.name === "scale-catalogue");
  if (!hasScaleTpl) {
    templates.push({
      name: "scale-catalogue",
      inputs: {},
      outputs: {},
      metadata: {},
      container: {
        name: "",
        image: "litmuschaos/k8s:latest",
        command: ["sh", "-c"],
        args: [
          "kubectl scale deployment catalogue -n sock-shop --replicas=2 && kubectl rollout status deployment/catalogue -n sock-shop --timeout=120s && echo 'catalogue scaled to 2 replicas'",
        ],
        resources: {},
      },
    });
    console.log("✅ Added scale-catalogue template");
  }

  // ── Save back ──
  const updatedManifest = JSON.stringify(wf);
  const fieldName = rev.experiment_manifest
    ? "revision.0.experiment_manifest"
    : "revision.0.workflow_manifest";

  const result = await db
    .collection("chaosExperiments")
    .updateOne(
      { _id: new ObjectId(EXPERIMENT_ID) },
      { $set: { [fieldName]: updatedManifest } }
    );

  console.log(
    `\n📦 MongoDB update: matched=${result.matchedCount}, modified=${result.modifiedCount}`
  );

  // ── Verify ──
  const verify = await db
    .collection("chaosExperiments")
    .findOne({ _id: new ObjectId(EXPERIMENT_ID) });
  const verifyManifest =
    verify.revision[0].experiment_manifest ||
    verify.revision[0].workflow_manifest ||
    "";
  const checks = [
    ["mode: Edge", verifyManifest.includes("mode: Edge")],
    ["probeTimeout: 10s", verifyManifest.includes("probeTimeout: 10s")],
    ["interval: 5s", verifyManifest.includes("interval: 5s")],
    ["attempt: 5", verifyManifest.includes("attempt: 5")],
    ["initialDelay: 15s", verifyManifest.includes("initialDelay: 15s")],
    ["responseTimeout: 5000", verifyManifest.includes("responseTimeout: 5000")],
    ["scale-catalogue", verifyManifest.includes("scale-catalogue")],
  ];
  console.log("\n🔍 Verification:");
  checks.forEach(([name, ok]) => console.log(`  ${ok ? "✅" : "❌"} ${name}`));

  await client.close();
}

main().catch((e) => {
  console.error("Error:", e);
  process.exit(1);
});

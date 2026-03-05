const { MongoClient } = require("mongodb");

async function main() {
  const c = new MongoClient("mongodb://localhost:27017");
  await c.connect();
  const db = c.db("litmus");

  const updates = {
    "kubernetes_http_properties.probe_timeout": "10s",
    "kubernetes_http_properties.interval": "5s",
    "kubernetes_http_properties.attempt": 5,
    "kubernetes_http_properties.retry": 3,
    "kubernetes_http_properties.initial_delay": "15s",
    "kubernetes_http_properties.response_timeout": 5000,
  };

  const probeNames = [
    "check-catalogue-access-url-GG960m0cQVe1XjoKa9SPyQ",
    "check-catalogue-access-url-CJAkjce_Sdy8ZJOpkfjkaw",
    "check-catalogue-access-url-hS1E1KTtTTCUBm90ueAAeQ",
  ];

  for (const name of probeNames) {
    const r = await db
      .collection("chaosProbes")
      .updateOne({ name }, { $set: updates });
    console.log(`${name}: matched=${r.matchedCount} modified=${r.modifiedCount}`);
  }

  await c.close();
}

main().catch((e) => console.error(e));

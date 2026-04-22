const infraId = "e1aff84f-6f99-49ed-924f-38a56724146f";
const projectId = "67a6bdeb-4161-41d7-b6ef-19472fd48133";
const now = NumberLong(String(Date.now()));

const resInactive = db.chaosInfrastructures.updateMany(
  { project_id: projectId, infra_id: { $ne: infraId } },
  { $set: { is_active: false, updated_at: now } }
);

const resActive = db.chaosInfrastructures.updateOne(
  { project_id: projectId, infra_id: infraId },
  { $set: { is_active: true, updated_at: now } }
);

const doc = db.chaosInfrastructures.findOne(
  { project_id: projectId, infra_id: infraId }
);

printjson({ resInactive, resActive, doc });

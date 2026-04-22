const keep = 'e1aff84f-6f99-49ed-924f-38a56724146f';

const result = db.chaosInfrastructures.deleteMany({
  infra_id: { $ne: keep }
});

printjson(result);
print('--- remaining ---');

db.chaosInfrastructures.find(
  {},
  {
    _id: 0,
    infra_id: 1,
    name: 1,
    is_registered: 1,
    is_infra_confirmed: 1,
    is_active: 1,
    updated_at: 1,
  }
).forEach(doc => printjson(doc));

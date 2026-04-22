const docs = db.chaosInfrastructures
  .find({ is_removed: { $ne: true } })
  .toArray();

if (!docs.length) {
  print("NO_RECORDS");
} else {
  docs.forEach(d => print(
    "infra_id=" + d.infra_id +
    " is_active=" + d.is_active +
    " is_infra_confirmed=" + d.is_infra_confirmed
  ));
}

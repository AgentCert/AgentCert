var e = db.chaosExperiments.findOne({name: 'agent-dmeo'});
var m = e.revision[0].experiment_manifest;
var oldStr = '"image":"agentcert/agentcert-install-app:latest","args"';
var newStr = '"image":"agentcert/agentcert-install-app:latest","imagePullPolicy":"IfNotPresent","args"';
var fixed = m.replace(oldStr, newStr);
if (fixed === m) {
  print('NO CHANGE - pattern not found');
} else {
  var result = db.chaosExperiments.updateOne(
    {name: 'agent-dmeo'},
    {$set: {'revision.0.experiment_manifest': fixed}}
  );
  print('matched:' + result.matchedCount + ' modified:' + result.modifiedCount);
}

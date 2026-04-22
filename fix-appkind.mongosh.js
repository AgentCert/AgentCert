// Fix appkind values for database targets in chaos experiments

// Find all experiments
db.chaosExperiments.find().forEach(function(exp) {
  if (!exp.revision) return;
  
  exp.revision.forEach(function(rev, idx) {
    if (!rev.experimentManifest) return;
    
    let manifest = rev.experimentManifest;
    let original = manifest;
    
    // Replace appkind: statefulset with appkind: deployment for database targets
    // Match pattern for user-db
    manifest = manifest.replace(
      /(\s+)appkind: statefulset\n(\s+)applabel: name=user-db/g,
      "$1appkind: deployment\n$2applabel: name=user-db"
    );
    
    // Match pattern for catalogue-db
    manifest = manifest.replace(
      /(\s+)appkind: statefulset\n(\s+)applabel: name=catalogue-db/g,
      "$1appkind: deployment\n$2applabel: name=catalogue-db"
    );
    
    if (manifest !== original) {
      exp.revision[idx].experimentManifest = manifest;
      db.chaosExperiments.updateOne(
        { experiment_id: exp.experiment_id },
        { $set: { revision: exp.revision, updated_at: new Date().getTime() } }
      );
      print("✓ Fixed appkind for: " + exp.name + " (revision " + idx + ")");
    }
  });
});

print("\n✓ Patching complete!");

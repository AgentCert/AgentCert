#!/bin/bash
# Find the current experiment ID from MongoDB
wsl -d Ubuntu -- bash -c "cat > /tmp/find_experiment.js << 'ENDSCRIPT'
const d = db.getSiblingDB('litmus');
const exp = d.chaosExperiments.findOne({ name: 'sock-shop' }, { experimentID: 1, _id: 1 });
if (exp) {
  print('experimentID=' + exp.experimentID);
} else {
  print('NOT_FOUND');
}
ENDSCRIPT
mongosh --quiet 'mongodb://root:1234@localhost:27017/admin?authSource=admin&replicaSet=rs0' /tmp/find_experiment.js"

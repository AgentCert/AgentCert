#!/bin/bash
mongosh "mongodb://root:1234@172.20.127.236:27017/agentcert?authSource=admin" --quiet --eval '
var exp = db.chaosExperiments.findOne({});
if (!exp) { print("no experiments found"); quit(); }
print("name:", exp.name);
print("revision count:", (exp.revision || []).length);
var rev = (exp.revision || [])[0];
if (!rev) { print("no revisions"); quit(); }
print("rev keys:", Object.keys(rev).join(", "));
var manifest = rev.experiment_manifest || rev.workflow_manifest || null;
if (!manifest) { print("no manifest field"); quit(); }
print("manifest type:", typeof manifest);
print("manifest (first 2000 chars):", String(manifest).substring(0, 2000));
'

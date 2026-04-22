#!/bin/bash
export DB_NAME=agentcert
export INSTALL_TIMEOUT=20m
mongosh "mongodb://root:1234@172.20.127.236:27017/agentcert?authSource=admin" --quiet --file /mnt/d/Studies/AgentCert/fix_install_timeout_mongosh.js

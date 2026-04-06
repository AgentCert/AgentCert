#!/bin/bash
echo '=== WORKFLOW STATUS ==='
kubectl get workflows sock-shop-1773729489576 -n litmus-exp
echo
echo '=== POD READ COUNT ==='
kubectl get pods -n sock-shop --no-headers | awk '$3 == "Running" && $2 == "1/1" {count++} END {print "Ready: " count " of " NR}'

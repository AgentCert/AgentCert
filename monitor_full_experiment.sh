#!/bin/bash
# Monitor experiment until install-application completes (all pods running)

while true; do
  clear
  echo "=========================================="
  echo "  EXPERIMENT: sock-shop-1773729489576"
  echo "=========================================="
  date
  echo
  
  # Check workflow status
  echo "[WORKFLOW STATUS]"
  kubectl get workflows sock-shop-1773729489576 -n litmus-exp -o jsonpath='{.status.phase}'
  echo
  echo
  
  # Check pod statuses
  echo "[POD STATUS - sock-shop namespace]"
  echo "Pod Name | Status | Ready"
  echo "------|--------|-------"
  kubectl get pods -n sock-shop --no-headers | awk '{printf "%-40s %-20s %s\n", $1, $3, $2}'
  echo
  
  # Count ready pods
  ready=$(kubectl get pods -n sock-shop --no-headers 2>/dev/null | awk '$3 == "Running" && $2 == "1/1" {count++} END {print count+0}')
  total=$(kubectl get pods -n sock-shop --no-headers 2>/dev/null | wc -l)
  
  echo "PODS READY: $ready/$total"
  echo
  
  if [ "$ready" -eq "$total" ] && [ $total -gt 0 ]; then
    echo "✓✓✓ ALL PODS RUNNING - install-application COMPLETE ✓✓✓"
    echo
    echo "Now checking experiment workflow progress..."
    sleep 2
    break
  fi
  
  sleep 2
done

# After pods are running, monitor experiment phases
echo
echo "=========================================="
echo "  MONITORING EXPERIMENT PHASES"
echo "=========================================="
while true; do
  clear
  date
  echo
  kubectl get workflows sock-shop-1773729489576 -n litmus-exp -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,PROGRESS:.status.progress
  echo
  phase=$(kubectl get workflows sock-shop-1773729489576 -n litmus-exp -o jsonpath='{.status.phase}' 2>/dev/null)
  if [ "$phase" = "Succeeded" ] || [ "$phase" = "Failed" ]; then
    echo "Experiment completed with phase: $phase"
    break
  fi
  sleep 3
done

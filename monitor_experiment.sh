#!/bin/bash
while true; do
  clear
  echo '=== EXPERIMENT WORKFLOW STATUS ==='
  date +%H:%M:%S
  kubectl get workflows sock-shop-1773729489576 -n litmus-exp -o wide
  echo
  echo '=== SOCK-SHOP POD STATUS ==='
  kubectl get pods -n sock-shop --no-headers | awk '{print $1, $3, $2}'
  echo
  ready=$(kubectl get pods -n sock-shop --no-headers 2>/dev/null | awk '$3 == "Running" && $2 == "1/1" {count++} END {print count+0}')
  total=$(kubectl get pods -n sock-shop --no-headers 2>/dev/null | wc -l)
  echo "Ready: $ready/$total"
  if [ "$ready" -eq "$total" ] && [ $total -gt 0 ]; then
    echo '✓ ALL PODS RUNNING!'
    break
  fi
  sleep 3
done

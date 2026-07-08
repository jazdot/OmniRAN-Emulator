#!/usr/bin/env bash

SCENARIOS=(
  "periodic-reg"
  "mobility-reg"
  "emergency-reg"
  "handover"
  "xn-handover"
  "pdu-lifecycle"
  "full-lifecycle"
  "deregister"
  "r17-ntn"
  "r18-uav"
  "r19-sensing"
)

echo "=== STARTING ALL SCENARIOS VERIFICATION ==="

for SCENARIO in "${SCENARIOS[@]}"; do
  echo "----------------------------------------"
  echo "Running Scenario: ${SCENARIO}"
  echo "----------------------------------------"
  ./app scenario "${SCENARIO}"
  
  if [ $? -eq 0 ]; then
    echo "Result: ${SCENARIO} -> SUCCESS"
  else
    echo "Result: ${SCENARIO} -> FAILED"
  fi
  sleep 2
done

echo "=== ALL SCENARIOS VERIFICATION COMPLETE ==="

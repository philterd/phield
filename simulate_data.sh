#!/bin/bash

# Configuration
PHIELD_URL=${PHIELD_URL:-"http://localhost:8080"}
SOURCE_ID=${SOURCE_ID:-"simulated-server-01"}
ORGANIZATION=${ORGANIZATION:-"demo-org"}
CONTEXT=${CONTEXT:-"context1"}
ITERATIONS=${ITERATIONS:-50}
SLEEP_INTERVAL=${SLEEP_INTERVAL:-1}

echo "Starting PII data simulation for Phield..."
echo "Target URL: $PHIELD_URL"
echo "Source ID: $SOURCE_ID"
echo "Organization: $ORGANIZATION"
echo "Context: $CONTEXT"
echo "Baseline iterations: $ITERATIONS"

# Function to send data
send_data() {
    local cc=$1
    local email=$2
    local ssn=$3
    local name=$4
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    payload=$(cat <<EOF
{
  "timestamp": "$timestamp",
  "source_id": "$SOURCE_ID",
  "organization": "$ORGANIZATION",
  "context": "$CONTEXT",
  "pii_types": {
    "credit-card": $cc,
    "email": $email,
    "ssn": $ssn,
    "name": $name
  }
}
EOF
)

    curl -s -X POST "$PHIELD_URL/ingest" \
         -H "Content-Type: application/json" \
         -d "$payload" > /dev/null

    if [ $? -eq 0 ]; then
        echo "[$(date +'%H:%M:%S')] Sent: CC=$cc, Email=$email, SSN=$ssn, Name=$name"
    else
        echo "[$(date +'%H:%M:%S')] Failed to send data to $PHIELD_URL"
    fi
}

# 1. Simulate Baseline Data
echo "--- Simulating Baseline Trend ---"
for i in $(seq 1 $ITERATIONS); do
    # Generate somewhat random but stable counts
    CC_COUNT=$(( RANDOM % 10 + 20 ))   # 20-29
    EMAIL_COUNT=$(( RANDOM % 20 + 100 )) # 100-119
    SSN_COUNT=$(( RANDOM % 5 + 2 ))     # 2-6
    NAME_COUNT=$(( RANDOM % 15 + 50 ))  # 50-64

    send_data $CC_COUNT $EMAIL_COUNT $SSN_COUNT $NAME_COUNT
    sleep $SLEEP_INTERVAL
done

# 2. Simulate a Trend Change (Spike)
echo "--- Simulating Trend Change (Spike) ---"
for i in $(seq 1 10); do
    # Sudden spike in Credit Cards and SSNs
    CC_COUNT=$(( RANDOM % 50 + 200 ))   # 200-249 (Massive spike from ~25)
    EMAIL_COUNT=$(( RANDOM % 20 + 110 )) # Stable
    SSN_COUNT=$(( RANDOM % 20 + 50 ))    # 50-69 (Significant spike from ~4)
    NAME_COUNT=$(( RANDOM % 15 + 60 ))   # Slightly higher

    send_data $CC_COUNT $EMAIL_COUNT $SSN_COUNT $NAME_COUNT
    sleep $SLEEP_INTERVAL
done

echo "Simulation complete."

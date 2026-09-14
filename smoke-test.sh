#!/bin/bash
#
# Sends data to a running Phield and reads it back to check that ingestion,
# validation, trend detection, and the read APIs all work.
#
#   ./smoke-test.sh                       # http://localhost:8080
#   PHIELD_URL=https://localhost:8443 ./smoke-test.sh
#   PHIELD_API_KEY=secret ./smoke-test.sh
#
# Exits non-zero if any check fails.

PHIELD_URL=${PHIELD_URL:-"http://localhost:8080"}

# -k so a self-signed certificate, which is what the container generates on
# start, does not stop the test.
CURL="curl -sk"

AUTH_ARGS=()
if [ -n "$PHIELD_API_KEY" ]; then
    AUTH_ARGS=(-H "Authorization: Bearer $PHIELD_API_KEY")
fi

SOURCE_ID="smoke-test-$(date +%s)"
ORGANIZATION="smoke-test"
CONTEXT="smoke-test"
PII_TYPE="ssn"
BASELINE_POINTS=25

passed=0
failed=0

ok() {
    echo "  ok    $1"
    passed=$((passed + 1))
}

no() {
    echo "  FAIL  $1"
    failed=$((failed + 1))
}

check() { # check <what> <expected> <actual>
    if [ "$2" = "$3" ]; then ok "$1"; else no "$1 (expected $2, got $3)"; fi
}

contains() { # contains <what> <needle> <haystack>
    case "$3" in
        *"$2"*) ok "$1" ;;
        *) no "$1 (no $2 in the response)" ;;
    esac
}

number() { # number <field> <json>
    echo "$2" | grep -o "\"$1\":[0-9.]*" | head -1 | cut -d: -f2
}

ingest() { # ingest <count>
    $CURL -o /dev/null -w '%{http_code}' -X POST "$PHIELD_URL/ingest" \
        -H "Content-Type: application/json" "${AUTH_ARGS[@]}" \
        -d "{\"source_id\":\"$SOURCE_ID\",\"organization\":\"$ORGANIZATION\",\"context\":\"$CONTEXT\",\"pii_types\":{\"$PII_TYPE\":$1}}"
}

echo "Smoke testing $PHIELD_URL as $SOURCE_ID"

echo
echo "Health"
health=$($CURL "$PHIELD_URL/health")
contains "health reports UP" '"status":"UP"' "$health"
contains "health reports a version" '"applicationVersion"' "$health"

echo
echo "Ingest"
before=$(number total_entries "$($CURL "$PHIELD_URL/api/dashboard/summary?hours=1")")

# Counts vary so the baseline has a spread, which the z_score method needs.
accepted=0
for i in $(seq 1 $BASELINE_POINTS); do
    if [ "$(ingest $(( 8 + i % 5 )))" = "202" ]; then
        accepted=$((accepted + 1))
    fi
done
check "$BASELINE_POINTS baseline counts accepted" "$BASELINE_POINTS" "$accepted"

echo
echo "Validation"
invalid() { # invalid <payload>
    $CURL -o /dev/null -w '%{http_code}' -X POST "$PHIELD_URL/ingest" \
        -H "Content-Type: application/json" "${AUTH_ARGS[@]}" -d "$1"
}
check "a missing source_id is rejected" "400" "$(invalid '{"pii_types":{"ssn":1}}')"
check "missing counts are rejected" "400" "$(invalid "{\"source_id\":\"$SOURCE_ID\"}")"
check "a negative count is rejected" "400" "$(invalid "{\"source_id\":\"$SOURCE_ID\",\"pii_types\":{\"ssn\":-1}}")"
check "malformed JSON is rejected" "400" "$(invalid '{"source_id":')"

echo
echo "Read back"
summary=$($CURL "$PHIELD_URL/api/dashboard/summary?hours=1")
after=$(number total_entries "$summary")
check "every count was stored" "$BASELINE_POINTS" "$(( after - before ))"

flows=$($CURL "$PHIELD_URL/api/dashboard/flows?hours=1")
contains "the source appears in the flows" "$SOURCE_ID" "$flows"
contains "the context appears in the flows" "$CONTEXT" "$flows"

entities=$($CURL "$PHIELD_URL/api/dashboard/entities?hours=1")
contains "the PII type appears in the entity totals" "$PII_TYPE" "$entities"

trends=$($CURL "$PHIELD_URL/api/dashboard/trends?hours=1")
contains "the source has a baseline" "$SOURCE_ID" "$trends"

echo
echo "Trend detection"
check "a spike is accepted" "202" "$(ingest 500)"

alerts=$($CURL "$PHIELD_URL/api/dashboard/alerts?hours=1")
contains "the spike raised an alert" "$SOURCE_ID" "$alerts"
contains "the alert names the PII type" "\"pii_type\":\"$PII_TYPE\"" "$alerts"

echo
echo "Replay"
start=$(date -u -d '1 hour ago' +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -v-1H +"%Y-%m-%dT%H:%M:%SZ")
end=$(date -u -d '1 hour' +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -v+1H +"%Y-%m-%dT%H:%M:%SZ")
replay=$($CURL -X POST "$PHIELD_URL/replay" -H "Content-Type: application/json" "${AUTH_ARGS[@]}" \
    -d "{\"start_time\":\"$start\",\"end_time\":\"$end\",\"test_threshold\":0.2}")
replayed=$(number total_points_processed "$replay")
if [ -n "$replayed" ] && [ "$replayed" -ge "$BASELINE_POINTS" ]; then
    ok "replay processed $replayed points"
else
    no "replay processed $replayed points, expected at least $BASELINE_POINTS"
fi

echo
echo "Mute"
mute=$($CURL -X POST "$PHIELD_URL/mute" -H "Content-Type: application/json" "${AUTH_ARGS[@]}" \
    -d "{\"organization\":\"$ORGANIZATION\",\"context\":\"$CONTEXT\",\"minutes\":1}")
contains "the context can be muted" '"status":"muted"' "$mute"

echo
echo "Metrics"
metrics=$($CURL "$PHIELD_URL/metrics")
contains "ingest requests are counted" "phield_ingest_requests_total_24h" "$metrics"
contains "ingest latency is reported" "phield_ingest_latency_average_seconds_24h" "$metrics"

if [ -n "$PHIELD_API_KEY" ]; then
    echo
    echo "Authentication"
    unauthenticated=$($CURL -o /dev/null -w '%{http_code}' -X POST "$PHIELD_URL/ingest" \
        -H "Content-Type: application/json" \
        -d "{\"source_id\":\"$SOURCE_ID\",\"pii_types\":{\"$PII_TYPE\":1}}")
    check "ingest without the API key is refused" "401" "$unauthenticated"
    check "the dashboard is readable without the API key" "200" \
        "$($CURL -o /dev/null -w '%{http_code}' "$PHIELD_URL/api/dashboard/summary")"
fi

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]

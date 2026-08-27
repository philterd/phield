# API Usage

## Authentication

Set `PHIELD_API_KEY` to require an API key on every API request. The key is sent as a bearer token:

```bash
curl -k https://localhost:8443/health \
     -H "Authorization: Bearer $PHIELD_API_KEY"
```

Requests without the header, or with a key that does not match, are rejected with `401 Unauthorized`:

```json
{
  "error": "unauthorized"
}
```

Authentication applies to `/ingest`, `/mute`, and `/replay`. `/health` and `/metrics` are open, so probes and Prometheus scrapes need no credential.

When `PHIELD_API_KEY` is not set, the API accepts all requests. In that case, restrict access to Phield at the network level.

Phield accepts a single key, and the key is passed in plaintext on each request. Run Phield with TLS enabled (`PHIELD_CERT_FILE` and `PHIELD_KEY_FILE`) so the key is not sent in the clear.

The examples in the rest of this page omit the header. Add it to each request if your instance sets `PHIELD_API_KEY`.

### Dashboard

The [dashboard](dashboard.md) and its JSON endpoints are not authenticated either. The key exists to keep bad data out of Phield, and what these endpoints return is aggregate counts, which contain no PII. Restrict access to the dashboard at the network level, or turn it off with `PHIELD_DASHBOARD_ENABLED=false`.

### Kafka

Counts consumed from a [Kafka topic](kafka.md) are not affected by `PHIELD_API_KEY`. Use Kafka's own authentication to control who can write to the topic.

## Ingest PII Counts

**Endpoint**: `POST /ingest`

**Payload**:

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | string | ISO-8601 timestamp of the count. Defaults to current time if omitted. |
| `source_id` | string | **Required**. A unique identifier for the source of the PII counts (e.g., a server hostname). |
| `organization` | string | The organization or tenant name. Defaults to `default`. |
| `context` | string | The application context or environment (e.g., `production`, `billing-app`). Defaults to `default`. |
| `pii_types` | object | **Required**. A map of PII type names to their respective integer counts. |

### Validation

A request is rejected with `400 Bad Request` and a message naming the problem when:

*   `source_id` is missing or blank.
*   `pii_types` is missing or contains no counts.
*   A PII type name is blank, contains a `.`, or starts with a `$`. Counts are stored as document fields and queried by name, which those characters break.
*   A count is negative.

A body larger than `PHIELD_MAX_REQUEST_BYTES` is rejected with `413 Request Entity Too Large`.

For example:

```json
{
  "error": "source_id is required"
}
```

Rejected requests are not stored and do not affect trend analysis. The same rules apply to counts consumed from [Kafka](kafka.md), where an invalid message is logged and skipped.

### Organization and Context

The `organization` and `context` parameters are used to group PII counts for trend analysis:

*   **Organization**: Best used for multi-tenancy. If you are monitoring multiple clients or distinct business units, use the `organization` to keep their data completely separate.
*   **Context**: Best used for logical grouping within an organization. For example, you might use different contexts for different applications (`mobile-app` vs `web-portal`) or different environments (`production` vs `staging`).

Trend analysis is performed per `source_id`, `organization`, and `context`. This means a spike in `credit-card` counts in the `billing` context will not be averaged with counts from the `logging` context, even if they share the same `source_id` and `organization`.

**Example JSON**:

```json
{
  "timestamp": "2026-04-23T15:34:00Z",
  "source_id": "application-server-1",
  "organization": "default",
  "context": "default",
  "pii_types": {
    "credit-card": 50,
    "email": 120,
    "ssn": 5
  }
}
```

**Example Request**:

```bash
curl -k -X POST https://localhost:8443/ingest \
     -H "Content-Type: application/json" \
     -d '{
           "timestamp": "2026-04-23T15:34:00Z",
           "source_id": "test-source",
           "organization": "default",
           "context": "default",
           "pii_types": {"credit-card": 10}
         }'
```

### Simulating Data

To easily test the ingest API and trend detection, you can use the `simulate_data.sh` script provided in the repository.

```bash
./simulate_data.sh
```

This script generates realistic PII counts and then simulates a spike to trigger an alert. You can configure the simulation via environment variables:

*   `PHIELD_URL`: The URL of the Phield ingest API (default: `http://localhost:8080`).
*   `SOURCE_ID`: The source ID for the simulated data (default: `simulated-server-01`).
*   `ORGANIZATION`: The organization name (default: `demo-org`).
*   `CONTEXT`: The context name (default: `production`).
*   `ITERATIONS`: The number of baseline data points to send (default: `50`).
*   `SLEEP_INTERVAL`: Seconds between each request (default: `1`).
*   `PHIELD_API_KEY`: Sent as a bearer token when the instance requires an API key.

Example:
```bash
PHIELD_URL=http://localhost:8080 CONTEXT=billing-app ./simulate_data.sh
```

## Replay Trend Analysis

**Endpoint**: `POST /replay`

Re-runs trend detection over historical counts with a different threshold or sensitivity, which is how to tune a setting before applying it. Replay reports the breaches that the test setting would have produced. It does not send notifications to Slack or PagerDuty, does not record the breaches it reports, and does not read or change the stored baselines: each series is rebuilt from the counts in the window.

**Payload**:

| Field | Type | Description |
|-------|------|-------------|
| `start_time` | string | **Required**. ISO-8601 (RFC3339) timestamp of the start of the replay window. |
| `end_time` | string | **Required**. ISO-8601 (RFC3339) timestamp of the end of the replay window. |
| `test_threshold` | float | **Required**. The threshold or sensitivity to test, in place of `PHIELD_ALERT_THRESHOLD` or `PHIELD_SENSITIVITY`. For `percentage_delta`, this is the fractional increase (0.2 for 20%). For `z_score`, this is the sensitivity (number of standard deviations). |
| `pii_types` | array | Optional. A list of PII types to include in the replay. If omitted, all types are processed. |

### Adaptive Threshold (Z-Score)

When `PHIELD_TREND_METHOD` is set to `z_score`, Phield uses statistical significance to detect breaches.
An alert fires if: `(Current Count - Mean) / Standard Deviation > Sensitivity`.
Sensitivity defaults to `3.0` but can be configured via `PHIELD_SENSITIVITY` or `test_threshold` during replay.
Adaptive alerts are only active after a "warm-up" period (default 20 points) to ensure baseline stability.

**Example JSON**:

```json
{
  "start_time": "2026-04-20T00:00:00Z",
  "end_time": "2026-04-21T00:00:00Z",
  "test_threshold": 0.15,
  "pii_types": ["credit-card", "ssn"]
}
```

**Example Request**:

```bash
curl -k -X POST https://localhost:8443/replay \
     -H "Content-Type: application/json" \
     -d '{
           "start_time": "2026-04-20T00:00:00Z",
           "end_time": "2026-04-21T00:00:00Z",
           "test_threshold": 0.15,
           "pii_types": ["credit-card", "ssn"]
         }'
```

**Response**:

```json
{
  "total_points_processed": 1440,
  "virtual_breaches_detected": 2,
  "breach_details": [
    {
      "timestamp": "2026-04-20T14:30:00Z",
      "pii_type": "credit-card",
      "context": "default",
      "organization": "default",
      "source_id": "app-server-1",
      "count": 150,
      "average": 80.5
    }
  ]
}
```

Each breach carries a `z_score` field as well when `PHIELD_TREND_METHOD` is `z_score`.

## Health Check

**Endpoint**: `GET /health`

Reports `200 OK` when Phield can reach its storage, so a load balancer stops sending an instance counts it cannot persist. When MongoDB is not configured the data is held in this process, so there is nothing to reach and the check always succeeds.

**Example Request**:

```bash
curl -k https://localhost:8443/health
```

**Response**:

```json
{
  "status": "UP",
  "applicationVersion": "1.0.0"
}
```

When the storage cannot be reached, the response is `503 Service Unavailable`:

```json
{
  "status": "unavailable",
  "storage": "unreachable"
}
```

## Metrics

**Endpoint**: `GET /metrics`

Returns metrics in Prometheus text format. Includes the number of `/ingest` requests and average request latency for the past 24 hours.

Each `/ingest` request records one latency sample. When MongoDB is used, samples older than `PHIELD_METRICS_RETENTION_DAYS` (7 by default) are removed, so the collection does not grow without limit. Set it to `0` to keep every sample. MongoDB removes expired documents on its own schedule, so a sample may outlive the window by up to a minute.

**Example Request**:

```bash
curl -k https://localhost:8443/metrics
```

**Response**:

```text
# HELP phield_ingest_requests_total_24h Number of /ingest requests in the past 24 hours
# TYPE phield_ingest_requests_total_24h gauge
phield_ingest_requests_total_24h 1250
# HELP phield_ingest_latency_average_seconds_24h Average /ingest latency in the past 24 hours in seconds
# TYPE phield_ingest_latency_average_seconds_24h gauge
phield_ingest_latency_average_seconds_24h 0.004521
```

## Mute Context Alerts

**Endpoint**: `POST /mute`

Disables trend breach alerts for a specific context for a given number of minutes. Alerts are grouped by both `organization` and `context`.

**Payload**:

| Field | Type | Description |
|-------|------|-------------|
| `organization` | string | The organization or tenant name. Defaults to `default`. |
| `context` | string | The context name to mute. Alerts are scoped to both organization and context. |
| `minutes` | int | **Required**. The duration for which to mute alerts, in minutes. |

**Example JSON**:

```json
{
  "organization": "default",
  "context": "default",
  "minutes": 60
}
```

**Example Request**:

```bash
curl -k -X POST https://localhost:8443/mute \
     -H "Content-Type: application/json" \
     -d '{
           "organization": "default",
           "context": "default",
           "minutes": 60
         }'
```

**Response**:

```json
{
  "status": "muted",
  "organization": "default",
  "context": "default",
  "minutes": 60
}
```

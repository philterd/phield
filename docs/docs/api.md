# API Usage

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

Example:
```bash
PHIELD_URL=http://localhost:8080 CONTEXT=billing-app ./simulate_data.sh
```

## Health Check

**Endpoint**: `GET /health`

**Example Request**:

```bash
curl -k https://localhost:8443/health
```

**Response**:

```json
{
  "status": "ok"
}
```

## Metrics

**Endpoint**: `GET /metrics`

Returns metrics in Prometheus text format. Includes the number of `/ingest` requests and average request latency for the past 24 hours.

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

## Historical Replay

**Endpoint**: `POST /replay`

Simulates alerts over a specific historical time window using a test threshold. This does not send actual notifications to Slack or PagerDuty.

**Payload**:

| Field | Type | Description |
|-------|------|-------------|
| `start_time` | string | **Required**. RFC3339 start timestamp of the window. |
| `end_time` | string | **Required**. RFC3339 end timestamp of the window. |
| `test_threshold` | float | **Required**. Threshold value. For `percentage_delta`, this is the fractional increase (0.2 for 20%). For `z_score`, this is the sensitivity (number of standard deviations). |
| `pii_types` | array | Optional list of PII types to filter by. |

#### Adaptive Threshold (Z-Score)

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
           "test_threshold": 0.15
         }'
```

**Response**:

```json
{
  "total_points_processed": 1500,
  "virtual_breaches_detected": 3,
  "breach_details": [
    {
      "timestamp": "2026-04-20T14:30:00Z",
      "pii_type": "credit-card",
      "context": "default",
      "organization": "default",
      "source_id": "app-server-1",
      "count": 100,
      "average": 50.0
    }
  ]
}
```

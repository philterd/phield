# Dashboard

Phield includes a built-in web dashboard that provides real-time visibility into PII flows and alerts without requiring external tooling like Grafana or Splunk.

## Accessing the Dashboard

The dashboard is available at:

```
http://localhost:8080/dashboard
```

Replace the host and port with your configured values (e.g., `https://localhost:8443/dashboard` when using TLS in Docker).

## Features

### Summary Metrics

The top row displays key metrics for the selected time window:

- **Data Points** — total number of ingested PII count entries.
- **Alerts** — total number of trend breach alerts fired.
- **Sources** — number of unique source IDs reporting data.
- **PII Types** — number of distinct PII entity types observed.
- **Contexts** — number of unique application contexts.

### PII Entity Type Breakdown

A doughnut chart showing the relative volume of each PII type (e.g., email, SSN, credit-card) across all sources in the time window.

### Volume Over Time

A line chart showing PII counts over time, broken down by entity type. The time buckets adjust automatically based on the selected window:

- 1–6 hours: 15-minute buckets
- 6–24 hours: 1-hour buckets
- 24–168 hours: 4-hour buckets

### Baseline vs. Current Traffic

A table comparing the rolling baseline mean (from Welford's algorithm) against the recent mean for each source/context/PII type combination. The deviation percentage indicates whether current traffic is above or below the established baseline.

### PII Flows

Shows the relationships between data sources and application contexts, including which PII types are flowing through each path and the total volume.

### Alert Timeline

A chronological list of triggered breach alerts showing timestamp, PII type, source, context, count, and z-score (when using the z_score trend method).

## Time Range

Use the dropdown in the header to select the time window:

- Last 1 hour
- Last 6 hours
- Last 24 hours (default)
- Last 3 days
- Last 7 days

The dashboard auto-refreshes every 30 seconds.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PHIELD_DASHBOARD_ENABLED` | `true` | Set to `false` to disable the dashboard and its API endpoints. |

## Dashboard API

The dashboard UI is backed by JSON API endpoints that can be consumed programmatically for custom integrations or external dashboards.

All endpoints accept an optional `hours` query parameter (default: `24`, max: `168`).

### GET /api/dashboard/summary

Returns aggregate statistics for the time window.

**Example Response**:

```json
{
  "total_entries": 1250,
  "total_breaches": 3,
  "unique_sources": 5,
  "unique_types": 4,
  "unique_contexts": 2,
  "window_hours": 24
}
```

### GET /api/dashboard/alerts

Returns breach/alert events in reverse chronological order.

**Example Response**:

```json
{
  "alerts": [
    {
      "timestamp": "2026-05-25T14:30:00Z",
      "pii_type": "credit-card",
      "context": "production",
      "organization": "acme",
      "source_id": "app-server-1",
      "count": 150,
      "average": 42.5,
      "z_score": 4.2
    }
  ],
  "window_hours": 24
}
```

### GET /api/dashboard/entities

Returns PII type totals and a time-bucketed timeline for charting.

**Example Response**:

```json
{
  "totals": {
    "email": 5200,
    "ssn": 340,
    "credit-card": 1800
  },
  "timeline": {
    "email": [
      {"time": "2026-05-25T10:00:00Z", "count": 420},
      {"time": "2026-05-25T11:00:00Z", "count": 380}
    ]
  },
  "window_hours": 24
}
```

### GET /api/dashboard/flows

Returns source-to-context flow data with PII type breakdown and total volume.

**Example Response**:

```json
{
  "flows": [
    {
      "source": "app-server-1",
      "context": "production",
      "types": {"email": 2400, "credit-card": 900},
      "total": 3300
    }
  ],
  "window_hours": 24
}
```

### GET /api/dashboard/trends

Returns baseline vs. recent mean comparison for each tracked source/context/PII type.

**Example Response**:

```json
{
  "trends": [
    {
      "source_id": "app-server-1",
      "context": "production",
      "pii_type": "email",
      "baseline_mean": 40.5,
      "recent_mean": 48.2,
      "sample_count": 250,
      "deviation_pct": 19.0
    }
  ],
  "window_hours": 24
}
```

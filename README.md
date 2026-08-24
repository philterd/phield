# Phield

Phield is a PII (Personally Identifiable Information) drift and trend monitor. It is designed to receive PII type counts via an API or Kafka topic, store them in a MongoDB Time-Series collection (or in-memory if MongoDB is not available), and alert when a count deviates significantly from the established trend (e.g., a sudden spike in Credit Card numbers).

Phield can be used either prior to redaction or after redaction by [Philter](https://www.github.com/philterd/philter). Either way implemented, Phield will detect and alert on any significant drift in PII counts.

Phield is part of a suite that includes [Phinder](https://www.github.com/philterd/phinder) (discovery) and [Philter](https://www.github.com/philterd/philter) (redaction). Visit [Philterd](https://www.philterd.ai) for more details.

See the [documentation](https://philterd.github.io/phield) for installation, configuration, API usage, and notification channels.

## Quick start

```bash
docker pull philterd/phield
docker run -p 8443:8443 philterd/phield
```

Open the dashboard at `https://localhost:8443/dashboard`. The container generates its own certificate on start, so clients need `-k` until you supply one. Without `PHIELD_MONGO_URI` set, data is held in memory and lost on restart. See [Installation](https://philterd.github.io/phield/installation/).

## Capabilities

- Asynchronously receive PII counts via `POST /ingest` or from a Kafka topic.
- Optionally, can use MongoDB Time-Series collections for efficient storage and querying.
- Falls back to ephemeral in-memory storage if MongoDB is not provided.
- Background worker calculates a moving average or statistical baseline and detects trend changes.
- Configurable lookback window (default 24 hours) and detection method (Percentage Delta or Adaptive Z-Score).
- Adaptive Thresholding using Welford's algorithm to learn "normal" volatility and reduce false positives.
- Alert Cooldown to suppress notification storms for sustained breaches, with "Back to Normal" automatic reset.
- Replay Capability to test and fine-tune trend settings against historical data.
- Triggers structured log alerts and optional Slack/PagerDuty notifications when counts exceed a configurable threshold.
- Optional API key authentication via `PHIELD_API_KEY`, sent as a bearer token on the ingest, mute, and replay endpoints.
- Validates ingested counts and rejects malformed requests.

## Dashboard

Phield includes a built-in web dashboard for visualizing PII flows and alerts without requiring external tooling like Grafana or Splunk.

Access the dashboard at `http://localhost:8080/dashboard` (or your configured port). Requests to the root redirect there. Without MongoDB configured, the page shows a banner saying that data is held in memory and lost on restart. The dashboard is not covered by `PHIELD_API_KEY`, which protects the write endpoints rather than these aggregate, PII-free reads.

The dashboard provides:

- **Summary metrics** — total data points, alerts, unique sources, PII types, and contexts.
- **PII entity type breakdown** — doughnut chart showing the distribution of detected PII types.
- **Volume over time** — line chart of PII counts bucketed by time interval.
- **Baseline vs. current traffic** — comparison of rolling baseline mean against recent observations with deviation percentages.
- **PII flows** — source-to-context relationships showing which systems are sending what PII types.
- **Alert timeline** — chronological list of triggered breach alerts with z-scores and counts.

The time range is configurable (1 hour to 7 days) and the dashboard auto-refreshes every 30 seconds.

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PHIELD_DASHBOARD_ENABLED` | `true` | Set to `false` to disable the dashboard. |

### Dashboard API

The dashboard is backed by JSON API endpoints that can also be consumed programmatically:

| Endpoint | Description |
|----------|-------------|
| `GET /api/dashboard/summary?hours=24` | Summary statistics for the time window. |
| `GET /api/dashboard/alerts?hours=24` | Breach/alert events in the time window. |
| `GET /api/dashboard/entities?hours=24` | PII type totals and time-bucketed timeline. |
| `GET /api/dashboard/flows?hours=24` | Source-to-context flow data with volume. |
| `GET /api/dashboard/trends?hours=24` | Baseline mean vs. recent mean per PII type. |

## Simulation
        
To see Phield in action, you can use the included `simulate_data.sh` script. This script sends randomized but realistic PII counts to Phield and then simulates a sudden trend change (spike) to trigger an alert.

In one terminal, start Phield:

```bash
./phield
```

In a second terminal, start the simulated PII counts:

```bash
./simulate_data.sh
```

Now, switch back to the first terminal and you can see the alerts being generated as the simulated data trends up.

### Simulation Configuration

The script supports several environment variables for configuration:

| Variable | Default | Description |
|----------|---------|-------------|
| `PHIELD_URL` | `http://localhost:8080` | The URL of the Phield ingest API. |
| `SOURCE_ID` | `simulated-server-01` | The source ID for the simulated data. |
| `ORGANIZATION` | `demo-org` | The organization for the simulated data. |
| `CONTEXT` | `production` | The context for the simulated data. |
| `ITERATIONS` | `50` | The number of baseline iterations to send. |
| `SLEEP_INTERVAL` | `1` | The sleep interval (in seconds) between iterations. |
| `PHIELD_API_KEY` | `""` | Sent as a bearer token when the Phield instance requires an API key. |

Example using custom configuration:

```bash
PHIELD_URL=http://localhost:8080 ITERATIONS=100 ./simulate_data.sh
```

## Building the Docker image

`build-image.sh` builds the image for `linux/amd64` and `linux/arm64`. `push-image.sh` publishes it. Both take an optional version, defaulting to `latest`.

```bash
./build-image.sh 1.0.0
./push-image.sh 1.0.0
```

`build-image.sh` loads each architecture under its own tag (`1.0.0-amd64`, `1.0.0-arm64`), so both are available locally to run and test. `push-image.sh` pushes those two tags and joins them into the `1.0.0` tag that users pull. It builds nothing, so what is published is what was built and tested.

Set `ARCHES` to build a single architecture:

```bash
ARCHES=amd64 ./build-image.sh
```

## Tests

```bash
make test
```

The MongoDB storage tests run against a real MongoDB instance at `mongodb://localhost:27017`. Set `PHIELD_TEST_MONGO_URI` to use a different one. Each test creates a database of its own and drops it when it finishes. If no instance is reachable, those tests skip, except in CI where they fail.

```bash
docker run -d --rm -p 27017:27017 mongo:8.2.12
```

### Smoke test

`smoke-test.sh` checks a running instance end to end: it ingests a baseline, confirms malformed requests are rejected, reads the counts back through the dashboard API, triggers a spike and looks for the alert, then exercises replay, mute, and metrics. It exits non-zero if any check fails.

```bash
./smoke-test.sh
PHIELD_URL=https://localhost:8443 ./smoke-test.sh
PHIELD_API_KEY=your-key ./smoke-test.sh
```

## License

Copyright 2026 Philterd, LLC.

Apache License 2.0. See [LICENSE](LICENSE) for details.

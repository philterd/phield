# Changelog

All notable changes to Phield are recorded here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and Phield follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 1.0.0

Initial release.

### Ingestion

- `POST /ingest` accepts PII type counts, grouped for analysis by source, organization, and context.
- Counts can also be consumed from a Kafka topic, with consumer group management so several instances share the load.
- Requests are validated. A count without a source, without counts, with a negative count, or with a PII type name that cannot be stored is rejected with `400 Bad Request` and is not stored.

### Trend detection

- Two detection methods: `percentage_delta`, which alerts on a fractional increase over the moving average, and `z_score`, which alerts on statistical significance.
- The `z_score` baseline is maintained with Welford's algorithm, so each series learns its own normal volatility, and alerts are held back until a warm-up period has passed.
- Alert cooldown suppresses repeat alerts for a series, and the cooldown resets after a run of normal counts.
- `POST /mute` suppresses alerts for a context for a set number of minutes.
- `POST /replay` re-runs detection over historical counts with a test threshold, reporting what would have alerted without sending notifications, recording breaches, or changing stored baselines.

### Storage

- MongoDB time-series collections for persistent storage, with the collection and indexes created on first connection.
- Ephemeral in-memory storage when no MongoDB is configured, which the logs and the dashboard both state plainly.
- Baseline updates are versioned, so counts for the same series arriving at two instances at once are not lost.
- `/ingest` latency samples expire after `PHIELD_METRICS_RETENTION_DAYS`, seven days by default.

### Alerting

- Structured log alerts, with optional Slack and PagerDuty notifications.

### Dashboard

- Built-in web dashboard at `/dashboard`, also served from `/`, showing summary metrics, PII type breakdown, volume over time, baseline against current traffic, source to context flows, and an alert timeline.
- The same data is available as JSON under `/api/dashboard/`.

### Operations

- Optional API key authentication through `PHIELD_API_KEY`, required on `/ingest`, `/mute`, and `/replay`. The reads that stay open return aggregate counts and contain no PII.
- HTTPS with a certificate that the container generates on start, so each container has its own rather than one shared by every image pull. A mounted certificate is used as-is.
- Prometheus metrics at `/metrics` and a liveness check at `/health`.
- Graceful shutdown on `SIGINT` and `SIGTERM`.
- Configuration entirely through environment variables. See the [configuration reference](https://philterd.github.io/phield/configuration/).

# Configuration

Phield is configured via environment variables.

| Variable | Description | Default |
|----------|-------------|---------|
| `PHIELD_MONGO_URI` | MongoDB connection URI (including DB name). If not provided, in-memory storage is used. | `""` |
| `PHIELD_ALERT_THRESHOLD` | Threshold for trend breach (0.2 = 20% spike) | `0.2` |
| `PHIELD_WINDOW_SIZE` | Window size in hours for moving average | `24` |
| `PHIELD_TREND_METHOD` | Method used for trend detection ([`percentage_delta`, `z_score`](trend_methods.md)) | `percentage_delta` |
| `PHIELD_SENSITIVITY` | Z-Score sensitivity for the `z_score` method | `3.0` |
| `PHIELD_WARMUP_COUNT` | Minimum data points before firing adaptive alerts | `20` |
| `PHIELD_COOLDOWN_MINUTES` | Minutes to suppress subsequent alerts for the same source/PII type | `60` |
| `PHIELD_PORT` | Port for the REST API | `8080` (or `8443` in Docker) |
| `PHIELD_CERT_FILE` | Path to SSL certificate file | `""` |
| `PHIELD_KEY_FILE` | Path to SSL key file | `""` |
| `PHIELD_SLACK_WEBHOOK_URL` | Slack Incoming Webhook URL | `""` |
| `PHIELD_PAGERDUTY_ROUTING_KEY` | PagerDuty Integration Routing Key | `""` |
| `PHIELD_PAGERDUTY_SEVERITY` | PagerDuty Alert Severity | `critical` |
| `PHIELD_KAFKA_BROKERS` | Comma-separated list of Kafka brokers. If provided, Kafka consumption is enabled. | `""` |
| `PHIELD_KAFKA_TOPIC` | Kafka topic to consume PII counts from. | `phield-pii-counts` |
| `PHIELD_KAFKA_GROUP_ID` | Kafka consumer group ID. | `phield` |
| `PHIELD_URL` | Simulation: The URL of the Phield ingest API (used by `simulate_data.sh`) | `http://localhost:8080` |
| `SOURCE_ID` | Simulation: The source ID for the simulated data (used by `simulate_data.sh`) | `simulated-server-01` |
| `ITERATIONS` | Simulation: The number of baseline data points to send (used by `simulate_data.sh`) | `50` |

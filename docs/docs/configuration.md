# Configuration

Phield is configured via environment variables.

| Variable | Description | Default |
|----------|-------------|---------|
| `PHIELD_MONGO_URI` | MongoDB connection URI (including DB name) | `mongodb://localhost:27017/phield` |
| `PHIELD_ALERT_THRESHOLD` | Threshold for trend breach (0.2 = 20% spike) | `0.2` |
| `PHIELD_PORT` | Port for the REST API | `8080` (or `8443` in Docker) |
| `PHIELD_CERT_FILE` | Path to SSL certificate file | `""` |
| `PHIELD_KEY_FILE` | Path to SSL key file | `""` |
| `PHIELD_SLACK_WEBHOOK_URL` | Slack Incoming Webhook URL | `""` |
| `PHIELD_PAGERDUTY_ROUTING_KEY` | PagerDuty Integration Routing Key | `""` |
| `PHIELD_PAGERDUTY_SEVERITY` | PagerDuty Alert Severity | `critical` |

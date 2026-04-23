# Phield

Phield is a PII (Personally Identifiable Information) drift and trend monitor. It is designed to receive PII type counts via an API, store them in a MongoDB Time-Series collection, and alert when a count deviates significantly from the established trend (e.g., a sudden spike in Credit Card numbers).

Phield is part of a suite that includes [Phinder](https://www.github.com/philterd/phinder) (discovery) and [Philter](https://www.github.com/philterd/philter) (redaction). Visit [Philterd](https://www.philterd.ai) for more details.

## Capabilities

- **REST API Ingestion**: Asynchronously receive PII counts via `POST /ingest`.
- **Time-Series Storage**: Uses MongoDB Time-Series collections for efficient storage and querying.
- **Trend Detection**: Background worker calculates a moving average (24-hour window) and detects breaches.
- **Configurable Alerting**: Triggers structured log alerts and optional Slack notifications when counts exceed a configurable threshold.
- **Containerized**: Easy deployment with Docker and Docker Compose.

## Documentation

- [Installation and Running](docs/docs/installation.md)
- [Configuration](docs/docs/configuration.md)
- [API Usage](docs/docs/api.md)
- [Notification Channels](docs/docs/notifications.md)

## License

Copyright 2026 Philterd, LLC.

Apache License 2.0. See [LICENSE](LICENSE) for details.

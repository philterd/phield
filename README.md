# Phield

Phield is a PII (Personally Identifiable Information) drift and trend monitor. It is designed to receive PII type counts via an API, store them in a MongoDB Time-Series collection, and alert when a count deviates significantly from the established trend (e.g., a sudden spike in Credit Card numbers).

Phield is part of a suite that includes [Phinder](https://www.github.com/philterd/phinder) (discovery) and [Philter](https://www.github.com/philterd/philter) (redaction). Visit [Philterd](https://www.philterd.ai) for more details.

## Capabilities

- **REST API Ingestion**: Asynchronously receive PII counts via `POST /ingest`.
- **Time-Series Storage**: Uses MongoDB Time-Series collections for efficient storage and querying.
- **Trend Detection**: Background worker calculates a moving average (24-hour window) and detects breaches.
- **Configurable Alerting**: Triggers structured log alerts when counts exceed a configurable threshold.
- **Containerized**: Easy deployment with Docker and Docker Compose.

## Getting Started

### Running with Docker Compose

The easiest way to run Phield along with its MongoDB dependency is using Docker Compose:

```bash
docker compose build
docker compose up
```

Phield will now be running on port 8443 (HTTPS) and listening for ingest requests.

### Running Locally

If you have a MongoDB instance running locally, you can run Phield directly:

```bash
go build -o phield main.go
./phield
```

## Configuration

Phield is configured via environment variables.

| Variable | Description | Default |
|----------|-------------|---------|
| `PHIELD_MONGO_URI` | MongoDB connection URI (including DB name) | `mongodb://localhost:27017/phield` |
| `PHIELD_ALERT_THRESHOLD` | Threshold for trend breach (0.2 = 20% spike) | `0.2` |
| `PHIELD_PORT` | Port for the REST API | `8080` (or `8443` in Docker) |
| `PHIELD_CERT_FILE` | Path to SSL certificate file | `""` |
| `PHIELD_KEY_FILE` | Path to SSL key file | `""` |

## API Usage

### Ingest PII Counts

**Endpoint**: `POST /ingest`

**Payload**:

```json
{
  "timestamp": "2026-04-23T15:34:00Z",
  "source_id": "application-server-1",
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
           "pii_types": {"credit-card": 10}
         }'
```

## License

Copyright 2026 Philterd, LLC.

Apache License 2.0. See [LICENSE](LICENSE) for details.

# Kafka Ingestion

Phield can consume PII counts directly from a Kafka topic. This allows for real-time monitoring of PII trends in high-volume environments where data is already being published to Kafka.

## Configuration

To enable Kafka ingestion, you must provide the list of Kafka brokers. Other settings like the topic and group ID have sensible defaults but can be overridden.

| Variable | Description | Default |
|----------|-------------|---------|
| `PHIELD_KAFKA_BROKERS` | Comma-separated list of Kafka brokers (e.g., `localhost:9092`). If provided, Kafka consumption is enabled. | `""` |
| `PHIELD_KAFKA_TOPIC` | Kafka topic to consume PII counts from. | `phield-pii-counts` |
| `PHIELD_KAFKA_GROUP_ID` | Kafka consumer group ID. | `phield` |

## Message Format

The Kafka consumer expects JSON messages in the same format as the `/ingest` API endpoint.

Example message:

```json
{
  "sourceId": "server-01",
  "timestamp": "2026-04-24T14:34:00Z",
  "piiCounts": {
    "credit-card": 5,
    "email": 12,
    "ssn": 1,
    "name": 45
  }
}
```

## How it Works

When `PHIELD_KAFKA_BROKERS` is set, Phield starts a background consumer that listens to the specified topic. Each message received is processed identically to an HTTP ingestion request:

1. The JSON is parsed and validated.
2. The PII counts are stored in the configured storage (MongoDB or In-Memory).
3. Trend analysis is performed for each PII type.
4. If a trend breach is detected, notifications are sent via the configured channels.

## Benefits of Kafka Ingestion

- **Scalability**: Kafka handles high throughput and provides buffering.
- **Decoupling**: Phield can consume data asynchronously without affecting the performance of the producing services.
- **Reliability**: Phield's Kafka consumer uses group management, allowing for multiple instances to share the load and provide failover.

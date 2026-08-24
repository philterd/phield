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

1. The JSON is parsed and validated against the same rules as the [ingest API](api.md#validation). A message that fails either step is logged and skipped, and the consumer moves on to the next message.
2. The PII counts are stored in the configured storage (MongoDB or In-Memory).
3. Trend analysis is performed for each PII type.
4. If a trend breach is detected, notifications are sent via the configured channels.

`PHIELD_API_KEY` does not apply to Kafka messages. Use Kafka's own authentication to control who can write to the topic.

## Delivery

Phield commits an offset as it reads each message, so delivery is at most once. A message that fails to parse, fails validation, or cannot be stored is logged and skipped, and the consumer moves on. It is not retried, and Phield will not see it again.

That suits a counts feed, where a missing data point widens the gap in a baseline rather than corrupting it. If a count must not be lost, keep the record on the producing side, or send it to `POST /ingest`, which reports a failure to the caller in the response.

## Benefits of Kafka Ingestion

- **Scalability**: Kafka handles high throughput and provides buffering.
- **Decoupling**: Phield can consume data asynchronously without affecting the performance of the producing services.
- **Reliability**: Phield's Kafka consumer uses group management, allowing for multiple instances to share the load and provide failover.

When several instances share one MongoDB, counts for the same source, organization, context, and PII type can arrive at two of them at once. Each instance updates that baseline with a versioned write, and an instance whose write is rejected because another got there first redoes its analysis against the current baseline. No count is dropped from the baseline, and an alert is raised only after the baseline it came from has been stored.

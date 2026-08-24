# Phield

Phield is a PII (Personally Identifiable Information) drift and trend monitor. It is designed to receive PII type counts via an API, store them in a MongoDB Time-Series collection (or in-memory if MongoDB is not available), and alert when a count deviates significantly from the established trend (e.g., a sudden spike in Credit Card numbers).

Phield is part of a suite that includes [Phinder](https://www.github.com/philterd/phinder) (discovery) and [Philter](https://www.github.com/philterd/philter) (redaction). Visit [Philterd](https://www.philterd.ai) for more details.

See the [documentation](https://philterd.github.io/phield) for installation, configuration, API usage, and notification channels. Check out the [FAQ](faq.md) for common questions.

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
- Built-in web [dashboard](dashboard.md) for real-time PII flow visualization, alert timeline, entity type breakdowns, and baseline vs. current traffic comparison.

## Quick Start with Docker

```bash
docker pull philterd/phield
docker run -p 8443:8443 philterd/phield
```

Open the dashboard at `https://localhost:8443/dashboard`. The container generates its own certificate on start, so clients need `-k` until you supply one, and without `PHIELD_MONGO_URI` the data is held in memory and lost on restart. See [Installation](installation.md) for pinning a version, persisting to MongoDB, and supplying your own certificate.

## Quick Start with Simulation

To see Phield in action immediately, you can use the included `simulate_data.sh` script. This script sends randomized but realistic PII counts to Phield and then simulates a sudden trend change (spike) to trigger an alert.

1. Start Phield in one terminal: `./phield`
2. Run the simulation in another: `./simulate_data.sh`

For more details on simulation, see the [Simulation](installation.md#simulating-data) section in the installation guide.


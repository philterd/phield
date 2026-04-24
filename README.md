# Phield

Phield is a PII (Personally Identifiable Information) drift and trend monitor. It is designed to receive PII type counts via an API, store them in a MongoDB Time-Series collection (or in-memory if MongoDB is not available), and alert when a count deviates significantly from the established trend (e.g., a sudden spike in Credit Card numbers).

Phield can be used either prior to redaction or after redaction by [Philter](https://www.github.com/philterd/philter). Either way implemented, Phield will detect and alert on any significant drift in PII counts.

Phield is part of a suite that includes [Phinder](https://www.github.com/philterd/phinder) (discovery) and [Philter](https://www.github.com/philterd/philter) (redaction). Visit [Philterd](https://www.philterd.ai) for more details.

See the [documentation](https://philterd.github.io/phield) for installation, configuration, API usage, and notification channels.

## Capabilities

- Asynchronously receive PII counts via `POST /ingest`.
- Optionally, can use MongoDB Time-Series collections for efficient storage and querying.
- Falls back to ephemeral in-memory storage if MongoDB is not provided.
- Background worker calculates a moving average or statistical baseline and detects trend changes.
- Configurable lookback window (default 24 hours) and detection method (Percentage Delta or Adaptive Z-Score).
- Adaptive Thresholding using Welford's algorithm to learn "normal" volatility and reduce false positives.
- Alert Cooldown to suppress notification storms for sustained breaches, with "Back to Normal" automatic reset.
- Triggers structured log alerts and optional Slack/PagerDuty notifications when counts exceed a configurable threshold.

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

## Configuration

The script supports several environment variables for configuration:

| Variable | Default | Description |
|----------|---------|-------------|
| `PHIELD_URL` | `http://localhost:8080` | The URL of the Phield ingest API. |
| `SOURCE_ID` | `simulated-server-01` | The source ID for the simulated data. |
| `ORGANIZATION` | `demo-org` | The organization for the simulated data. |
| `CONTEXT` | `production` | The context for the simulated data. |
| `ITERATIONS` | `50` | The number of baseline iterations to send. |
| `SLEEP_INTERVAL` | `1` | The sleep interval (in seconds) between iterations. |

Example using custom configuration:

```bash
PHIELD_URL=http://localhost:8080 ITERATIONS=100 ./simulate_data.sh
```

## License

Copyright 2026 Philterd, LLC.

Apache License 2.0. See [LICENSE](LICENSE) for details.

# Trend Detection Methods

Phield supports multiple methods for detecting trends and anomalies in PII counts. You can configure the method using the `PHIELD_TREND_METHOD` environment variable.

## Percentage Delta (`percentage_delta`)

The `percentage_delta` method is a straightforward approach that compares the current PII count to a moving average of recent counts.

### How it Works
1.  **Moving Average**: Phield calculates the arithmetic mean ($\mu$) of PII counts for a specific source and PII type over the configured window (default 24 hours).
2.  **Threshold Comparison**: An alert is triggered if the new count ($x$) is greater than the moving average by more than a fixed percentage.

### Formula
An alert fires if:
$$x > \mu \times (1 + \text{threshold})$$

Where:
*   $x$: Current PII count.
*   $\mu$: Moving average of historical counts.
*   $\text{threshold}$: Configurable value (e.g., `0.2` for 20%) set via `PHIELD_ALERT_THRESHOLD`.

### Use Case
Best for environments with stable, predictable data where any sudden increase of a certain magnitude is considered a risk.

---

## Z-Score (`z_score`)

The `z_score` (Standard Score) method is an adaptive thresholding model. It learns the "normal" volatility of your data and only alerts when a spike is statistically significant.

### How it Works
1.  **Statistical Baseline**: Phield tracks the moving mean ($\mu$) and moving standard deviation ($\sigma$) using Welford's Online Algorithm.
2.  **Z-Score Calculation**: It calculates how many standard deviations the new data point is from the mean.
3.  **Sensitivity Check**: An alert is triggered if the Z-Score exceeds a configurable sensitivity level.

### Formula
$$Z = \frac{x - \mu}{\sigma}$$

An alert fires if:
$$Z > \text{sensitivity}$$

Where:
*   $x$: Current PII count.
*   $\mu$: Moving mean.
*   $\sigma$: Moving standard deviation.
*   $\text{sensitivity}$: Configurable value (default `3.0`) set via `PHIELD_SENSITIVITY`.

### Key Features
*   **Adaptive**: Automatically adjusts to the natural "noise" and fluctuation of the data.
*   **Warm-up Period**: To ensure statistical validity, this method requires a minimum number of data points (default `20`, set via `PHIELD_WARMUP_COUNT`) before it starts firing alerts.
*   **Reduced False Positives**: In highly volatile environments, it prevents alerts for fluctuations that are within the normal range for that specific source.

### Use Case
Best for dynamic environments where PII counts vary significantly and a fixed percentage threshold would cause too many false alarms.

---

## Testing with Simulation

You can test both trend detection methods by using the `simulate_data.sh` script. This script allows you to quickly generate a baseline and then trigger a spike to see how Phield responds under different configurations.

### Testing Percentage Delta
1. Set the method: `export PHIELD_TREND_METHOD=percentage_delta`
2. Start Phield: `./phield`
3. Run simulation: `./simulate_data.sh`

### Testing Z-Score
1. Set the method: `export PHIELD_TREND_METHOD=z_score`
2. Start Phield: `./phield`
3. Run simulation: `./simulate_data.sh`

Note: When testing Z-Score, ensure the `ITERATIONS` in the simulation script is higher than the `PHIELD_WARMUP_COUNT` (default 20) to ensure the statistical model has enough data to start alerting.

---

## Fine-tuning with Replay

While simulation is great for initial testing, the **Replay Capability** is designed for fine-tuning Phield against your actual historical data.

By using the `/replay` endpoint, you can "test-drive" different sensitivity settings to see how many alerts they would have generated in the past. This helps you find the "sweet spot" between missing real spikes and being overwhelmed by false positives.

### Replay Workflow
1.  **Collect Data**: Run Phield in your environment for a few days to build up a history of PII counts.
2.  **Identify a Period**: Choose a timeframe (e.g., the last 24 hours) where you know a spike occurred, or where you received a false alert.
3.  **Run Replay**: Call `POST /replay` with your chosen timeframe and a new `test_threshold`.
4.  **Evaluate**: Review the `virtual_breaches_detected` and `breach_details` in the response.
5.  **Adjust**: If you still see too many false positives, increase the threshold (for `percentage_delta`) or sensitivity (for `z_score`) and replay again.
6.  **Apply**: Once satisfied, update your environment variables (`PHIELD_ALERT_THRESHOLD` or `PHIELD_SENSITIVITY`) and restart Phield.

See the [API Usage](api.md#replay-trend-analysis) page for the full replay API specification.

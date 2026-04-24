# Frequently Asked Questions (FAQ)

### What is Phield?
Phield is a PII (Personally Identifiable Information) drift and trend monitor. It receives PII type counts and alerts you when those counts deviate significantly from an established baseline or trend.

### When would I use Phield?
You should use Phield whenever you need to monitor the flow of PII through your systems and want to be alerted to unexpected changes. Common use cases include:
*   **Detecting Data Leaks**: A sudden spike in `credit-card` or `ssn` types might indicate a misconfiguration or a breach.
*   **Monitoring Application Changes**: New software releases might inadvertently start logging or processing more PII than intended.
*   **Compliance Auditing**: Maintain a historical record of PII volumes to demonstrate control and oversight of sensitive data.
*   **Validating Data Pipelines**: Ensure that PII discovery and redaction tools (like Philter) are performing consistently over time.

### What PII types does Phield monitor?
Phield is PII-agnostic. It monitors whatever PII types you send to it. Common examples include `credit-card`, `email`, `ssn`, `name`, and `phone-number`.

### How do I send data to Phield?
You can send data to Phield in two ways:
1.  **REST API**: Send a `POST` request to the `/ingest` endpoint.
2.  **Kafka**: Phield can consume PII counts directly from a Kafka topic.

See the [API](api.md) and [Kafka Ingestion](kafka.md) pages for more details.

### Does Phield store my actual PII data?
**No.** Phield only receives and stores the *counts* of PII types (e.g., "we found 50 credit card numbers in this source"). It never sees or stores the actual PII values themselves.

### Where is the data stored?
Phield can store data in:
*   **MongoDB**: Using Time-Series collections for efficient, long-term storage.
*   **In-Memory**: If MongoDB is not configured, Phield uses ephemeral in-memory storage (data will be lost on restart).

### What trend detection methods are available?
Phield supports two main methods:
1.  **Percentage Delta**: Simple comparison against a moving average.
2.  **Z-Score**: Statistical approach that learns the normal volatility of your data and alerts on significant deviations.

Check out the [Trend Detection Methods](trend_methods.md) page for a deep dive.

### How can I avoid alert fatigue?
Phield includes several features to reduce false positives:
*   **Adaptive Thresholding (Z-Score)**: Automatically adjusts to the natural "noise" in your data.
*   **Alert Cooldown**: Suppresses repeated notifications for the same breach for a configurable period (default 60 minutes).
*   **Replay Capability**: Allows you to test your settings against historical data to find the optimal sensitivity before going live.

### How can I test my configuration?
You can use the included `simulate_data.sh` script to send realistic, randomized PII counts to Phield and trigger a trend breach. This is a great way to verify your notification settings and see how different trend methods respond.

### Is Phield part of a larger ecosystem?
Yes, Phield is part of the [Philterd](https://www.philterd.ai) suite, which also includes [Phinder](https://www.github.com/philterd/phinder) for PII discovery and [Philter](https://www.github.com/philterd/philter) for PII redaction.

### Is commercial support available?
Yes. Commercial support is available from [Philterd](https://www.philterd.ai).

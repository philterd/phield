# Notification Channels

Phield supports notifying external systems when a trend breach is detected. A trend breach occurs when the count of a specific PII type for a source exceeds the established moving average by a configurable threshold.

## Supported Channels

### Slack

Phield can send alerts to a Slack channel using Incoming Webhooks.

#### Configuration

To enable Slack notifications, set the following environment variable:

| Variable | Description | Example |
|----------|-------------|---------|
| `PHIELD_SLACK_WEBHOOK_URL` | The Incoming Webhook URL provided by Slack. | `https://hooks.slack.com/services/T000/B000/XXXX` |

#### Setup

1.  In your Slack workspace, create an "Incoming Webhook" app or integration.
2.  Select the channel where you want Phield to post alerts.
3.  Copy the Webhook URL.
4.  Provide this URL to Phield via the `PHIELD_SLACK_WEBHOOK_URL` environment variable.

### PagerDuty

Phield can send alerts to PagerDuty using the Events API V2.

#### Configuration

To enable PagerDuty notifications, set the following environment variable:

| Variable | Description | Example |
|----------|-------------|---------|
| `PHIELD_PAGERDUTY_ROUTING_KEY` | The Integration Key (Routing Key) from PagerDuty. | `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx` |
| `PHIELD_PAGERDUTY_SEVERITY` | The severity of the PagerDuty alert (info, warning, error, critical). | `critical` |

#### Setup

1.  In PagerDuty, go to **Services** -> **Service Directory**.
2.  Select or create a service.
3.  Go to the **Integrations** tab and add a new integration.
4.  Search for "Events API V2" and add it.
5.  Copy the **Integration Key**.
6.  Provide this key to Phield via the `PHIELD_PAGERDUTY_ROUTING_KEY` environment variable.

### Structured Logs

By default, Phield always logs trend breaches to standard output as structured log entries. This is useful for integration with log management systems like ELK, Splunk, or CloudWatch.

Example log entry:
`2026/04/23 15:43:00 [TREND BREACH] Source: app-1, PII Type: credit-card, Current: 100, Avg: 50.00, Increase: 100.00%`

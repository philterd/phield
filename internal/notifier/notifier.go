/*
 * Copyright 2026 Philterd, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Notifier interface {
	Notify(ctx context.Context, message string) error
}

type SlackNotifier struct {
	WebhookURL string
}

func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		WebhookURL: webhookURL,
	}
}

func (s *SlackNotifier) Notify(ctx context.Context, message string) error {
	if s.WebhookURL == "" {
		return nil
	}

	payload := map[string]string{
		"text": message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.WebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack notification failed with status: %s", resp.Status)
	}

	return nil
}

type PagerDutyNotifier struct {
	RoutingKey string
	Severity   string
}

func NewPagerDutyNotifier(routingKey, severity string) *PagerDutyNotifier {
	return &PagerDutyNotifier{
		RoutingKey: routingKey,
		Severity:   severity,
	}
}

func (p *PagerDutyNotifier) Notify(ctx context.Context, message string) error {
	if p.RoutingKey == "" {
		return nil
	}

	// PagerDuty Events API V2 payload
	payload := map[string]any{
		"routing_key":  p.RoutingKey,
		"event_action": "trigger",
		"payload": map[string]any{
			"summary":  message,
			"source":   "phield",
			"severity": p.Severity,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://events.pagerduty.com/v2/enqueue", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pagerduty notification failed with status: %s", resp.Status)
	}

	return nil
}

type MultiNotifier struct {
	notifiers []Notifier
}

func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	return &MultiNotifier{
		notifiers: notifiers,
	}
}

func (m *MultiNotifier) Notify(ctx context.Context, message string) error {
	for _, n := range m.notifiers {
		if err := n.Notify(ctx, message); err != nil {
			// Log error but continue with other notifiers
			fmt.Printf("Notification error: %v\n", err)
		}
	}
	return nil
}

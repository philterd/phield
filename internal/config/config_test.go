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

package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Clear environment variables before test
	os.Unsetenv("PHIELD_MONGO_URI")
	os.Unsetenv("PHIELD_ALERT_THRESHOLD")
	os.Unsetenv("PHIELD_PORT")
	os.Unsetenv("PHIELD_CERT_FILE")
	os.Unsetenv("PHIELD_KEY_FILE")
	os.Unsetenv("PHIELD_API_KEY")
	os.Unsetenv("PHIELD_SLACK_WEBHOOK_URL")
	os.Unsetenv("PHIELD_PAGERDUTY_ROUTING_KEY")
	os.Unsetenv("PHIELD_PAGERDUTY_SEVERITY")
	os.Unsetenv("PHIELD_WINDOW_SIZE")
	os.Unsetenv("PHIELD_METRICS_RETENTION_DAYS")
	os.Unsetenv("PHIELD_READ_TIMEOUT_SECONDS")
	os.Unsetenv("PHIELD_WRITE_TIMEOUT_SECONDS")
	os.Unsetenv("PHIELD_IDLE_TIMEOUT_SECONDS")
	os.Unsetenv("PHIELD_MAX_REQUEST_BYTES")

	t.Run("Default values", func(t *testing.T) {
		cfg := Load()
		if cfg.MongoURI != "" {
			t.Errorf("expected empty MongoURI, got %s", cfg.MongoURI)
		}
		if cfg.AlertThreshold != 0.2 {
			t.Errorf("expected 0.2 AlertThreshold, got %f", cfg.AlertThreshold)
		}
		if cfg.Port != "8080" {
			t.Errorf("expected 8080 Port, got %s", cfg.Port)
		}
		if cfg.PagerDutySeverity != "critical" {
			t.Errorf("expected critical PagerDutySeverity, got %s", cfg.PagerDutySeverity)
		}
		if cfg.WindowSize != 24 {
			t.Errorf("expected 24 WindowSize, got %d", cfg.WindowSize)
		}
		if cfg.APIKey != "" {
			t.Errorf("expected empty APIKey, got %s", cfg.APIKey)
		}
		if cfg.MetricsRetentionDays != 7 {
			t.Errorf("expected 7 MetricsRetentionDays, got %d", cfg.MetricsRetentionDays)
		}
		if cfg.ReadTimeout != 15*time.Second {
			t.Errorf("expected a 15s ReadTimeout, got %s", cfg.ReadTimeout)
		}
		if cfg.WriteTimeout != 120*time.Second {
			t.Errorf("expected a 120s WriteTimeout, got %s", cfg.WriteTimeout)
		}
		if cfg.IdleTimeout != 60*time.Second {
			t.Errorf("expected a 60s IdleTimeout, got %s", cfg.IdleTimeout)
		}
		if cfg.MaxRequestBytes != 1048576 {
			t.Errorf("expected 1048576 MaxRequestBytes, got %d", cfg.MaxRequestBytes)
		}
	})

	t.Run("Override values", func(t *testing.T) {
		os.Setenv("PHIELD_MONGO_URI", "mongodb://localhost:27017/test")
		os.Setenv("PHIELD_ALERT_THRESHOLD", "0.5")
		os.Setenv("PHIELD_PORT", "9090")
		os.Setenv("PHIELD_PAGERDUTY_SEVERITY", "error")
		os.Setenv("PHIELD_WINDOW_SIZE", "48")
		os.Setenv("PHIELD_API_KEY", "s3cret")
		os.Setenv("PHIELD_METRICS_RETENTION_DAYS", "30")
		os.Setenv("PHIELD_READ_TIMEOUT_SECONDS", "5")
		os.Setenv("PHIELD_MAX_REQUEST_BYTES", "2048")
		defer func() {
			os.Unsetenv("PHIELD_API_KEY")
			os.Unsetenv("PHIELD_METRICS_RETENTION_DAYS")
			os.Unsetenv("PHIELD_READ_TIMEOUT_SECONDS")
			os.Unsetenv("PHIELD_MAX_REQUEST_BYTES")
			os.Unsetenv("PHIELD_MONGO_URI")
			os.Unsetenv("PHIELD_ALERT_THRESHOLD")
			os.Unsetenv("PHIELD_PORT")
			os.Unsetenv("PHIELD_PAGERDUTY_SEVERITY")
			os.Unsetenv("PHIELD_WINDOW_SIZE")
		}()

		cfg := Load()
		if cfg.MongoURI != "mongodb://localhost:27017/test" {
			t.Errorf("expected mongodb://localhost:27017/test MongoURI, got %s", cfg.MongoURI)
		}
		if cfg.AlertThreshold != 0.5 {
			t.Errorf("expected 0.5 AlertThreshold, got %f", cfg.AlertThreshold)
		}
		if cfg.Port != "9090" {
			t.Errorf("expected 9090 Port, got %s", cfg.Port)
		}
		if cfg.APIKey != "s3cret" {
			t.Errorf("expected s3cret APIKey, got %s", cfg.APIKey)
		}
		if cfg.MetricsRetentionDays != 30 {
			t.Errorf("expected 30 MetricsRetentionDays, got %d", cfg.MetricsRetentionDays)
		}
		if cfg.ReadTimeout != 5*time.Second {
			t.Errorf("expected a 5s ReadTimeout, got %s", cfg.ReadTimeout)
		}
		if cfg.MaxRequestBytes != 2048 {
			t.Errorf("expected 2048 MaxRequestBytes, got %d", cfg.MaxRequestBytes)
		}
		if cfg.PagerDutySeverity != "error" {
			t.Errorf("expected error PagerDutySeverity, got %s", cfg.PagerDutySeverity)
		}
		if cfg.WindowSize != 48 {
			t.Errorf("expected 48 WindowSize, got %d", cfg.WindowSize)
		}
	})

	t.Run("Invalid float/int values use fallback", func(t *testing.T) {
		os.Setenv("PHIELD_ALERT_THRESHOLD", "invalid")
		os.Setenv("PHIELD_WINDOW_SIZE", "invalid")
		defer func() {
			os.Unsetenv("PHIELD_ALERT_THRESHOLD")
			os.Unsetenv("PHIELD_WINDOW_SIZE")
		}()

		cfg := Load()
		if cfg.AlertThreshold != 0.2 {
			t.Errorf("expected fallback 0.2 AlertThreshold, got %f", cfg.AlertThreshold)
		}
		if cfg.WindowSize != 24 {
			t.Errorf("expected fallback 24 WindowSize, got %d", cfg.WindowSize)
		}
	})
}

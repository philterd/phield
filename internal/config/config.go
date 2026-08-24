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
	"strconv"
	"time"
)

type Config struct {
	MongoURI            string
	AlertThreshold      float64
	Port                string
	CertFile            string
	KeyFile             string
	APIKey              string
	SlackWebhook        string
	PagerDutyRoutingKey string
	PagerDutySeverity   string
	WindowSize          int
	TrendMethod         string
	Sensitivity         float64
	WarmUpCount         int
	CooldownMinutes     int
	KafkaBrokers        string
	KafkaTopic          string
	KafkaGroupID        string
	DashboardEnabled    bool
	// MetricsRetentionDays is how long /ingest latency samples are kept. Zero
	// keeps them forever.
	MetricsRetentionDays int

	// ReadTimeout bounds how long a client may take to send a request, and
	// WriteTimeout how long a response may take. WriteTimeout is the more
	// generous of the two because a replay over a wide window is slow by
	// nature. IdleTimeout bounds a kept-alive connection between requests.
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// MaxRequestBytes bounds the size of a request body.
	MaxRequestBytes int64
}

func Load() *Config {
	return &Config{
		MongoURI:            getEnv("PHIELD_MONGO_URI", ""),
		AlertThreshold:      getEnvAsFloat("PHIELD_ALERT_THRESHOLD", 0.2), // 20%
		Port:                getEnv("PHIELD_PORT", "8080"),
		CertFile:            getEnv("PHIELD_CERT_FILE", ""),
		KeyFile:             getEnv("PHIELD_KEY_FILE", ""),
		APIKey:              getEnv("PHIELD_API_KEY", ""),
		SlackWebhook:        getEnv("PHIELD_SLACK_WEBHOOK_URL", ""),
		PagerDutyRoutingKey: getEnv("PHIELD_PAGERDUTY_ROUTING_KEY", ""),
		PagerDutySeverity:   getEnv("PHIELD_PAGERDUTY_SEVERITY", "critical"),
		WindowSize:          getEnvAsInt("PHIELD_WINDOW_SIZE", 24),
		TrendMethod:         getEnv("PHIELD_TREND_METHOD", "percentage_delta"),
		Sensitivity:         getEnvAsFloat("PHIELD_SENSITIVITY", 3.0),
		WarmUpCount:         getEnvAsInt("PHIELD_WARMUP_COUNT", 20),
		CooldownMinutes:     getEnvAsInt("PHIELD_COOLDOWN_MINUTES", 60),
		KafkaBrokers:        getEnv("PHIELD_KAFKA_BROKERS", ""),
		KafkaTopic:          getEnv("PHIELD_KAFKA_TOPIC", "phield-pii-counts"),
		KafkaGroupID:        getEnv("PHIELD_KAFKA_GROUP_ID", "phield"),
		DashboardEnabled:    getEnvAsBool("PHIELD_DASHBOARD_ENABLED", true),

		MetricsRetentionDays: getEnvAsInt("PHIELD_METRICS_RETENTION_DAYS", 7),

		ReadTimeout:  time.Duration(getEnvAsInt("PHIELD_READ_TIMEOUT_SECONDS", 15)) * time.Second,
		WriteTimeout: time.Duration(getEnvAsInt("PHIELD_WRITE_TIMEOUT_SECONDS", 120)) * time.Second,
		IdleTimeout:  time.Duration(getEnvAsInt("PHIELD_IDLE_TIMEOUT_SECONDS", 60)) * time.Second,

		MaxRequestBytes: int64(getEnvAsInt("PHIELD_MAX_REQUEST_BYTES", 1048576)),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsFloat(key string, fallback float64) float64 {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return fallback
}

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
)

type Config struct {
	MongoURI            string
	AlertThreshold      float64
	Port                string
	CertFile            string
	KeyFile             string
	SlackWebhook        string
	PagerDutyRoutingKey string
	PagerDutySeverity   string
	WindowSize          int
	TrendMethod         string
	Sensitivity         float64
	WarmUpCount         int
	CooldownMinutes     int
}

func Load() *Config {
	return &Config{
		MongoURI:            getEnv("PHIELD_MONGO_URI", ""),
		AlertThreshold:      getEnvAsFloat("PHIELD_ALERT_THRESHOLD", 0.2), // 20%
		Port:                getEnv("PHIELD_PORT", "8080"),
		CertFile:            getEnv("PHIELD_CERT_FILE", ""),
		KeyFile:             getEnv("PHIELD_KEY_FILE", ""),
		SlackWebhook:        getEnv("PHIELD_SLACK_WEBHOOK_URL", ""),
		PagerDutyRoutingKey: getEnv("PHIELD_PAGERDUTY_ROUTING_KEY", ""),
		PagerDutySeverity:   getEnv("PHIELD_PAGERDUTY_SEVERITY", "critical"),
		WindowSize:          getEnvAsInt("PHIELD_WINDOW_SIZE", 24),
		TrendMethod:         getEnv("PHIELD_TREND_METHOD", "percentage_delta"),
		Sensitivity:         getEnvAsFloat("PHIELD_SENSITIVITY", 3.0),
		WarmUpCount:         getEnvAsInt("PHIELD_WARMUP_COUNT", 20),
		CooldownMinutes:     getEnvAsInt("PHIELD_COOLDOWN_MINUTES", 60),
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

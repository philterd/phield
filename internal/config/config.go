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
	MongoURI       string
	AlertThreshold float64
	Port           string
	CertFile       string
	KeyFile        string
}

func Load() *Config {
	return &Config{
		MongoURI:       getEnv("PHIELD_MONGO_URI", "mongodb://localhost:27017/phield"),
		AlertThreshold: getEnvAsFloat("PHIELD_ALERT_THRESHOLD", 0.2), // 20%
		Port:           getEnv("PHIELD_PORT", "8080"),
		CertFile:       getEnv("PHIELD_CERT_FILE", ""),
		KeyFile:        getEnv("PHIELD_KEY_FILE", ""),
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

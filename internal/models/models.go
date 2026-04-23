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

package models

import "time"

type PIIEntry struct {
	Timestamp    time.Time      `bson:"timestamp" json:"timestamp"`
	SourceID     string         `bson:"source_id" json:"source_id"`
	Organization string         `bson:"organization" json:"organization"`
	Context      string         `bson:"context" json:"context"`
	PIITypes     map[string]int `bson:"pii_types" json:"pii_types"`
}

type IngestRequest struct {
	Timestamp    time.Time      `json:"timestamp"`
	SourceID     string         `json:"source_id"`
	Organization string         `json:"organization"`
	Context      string         `json:"context"`
	PIITypes     map[string]int `json:"pii_types"`
}

type MuteRequest struct {
	Organization string `json:"organization"`
	Context      string `json:"context"`
	Minutes      int    `json:"minutes"`
}

type ReplayRequest struct {
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	TestThreshold float64   `json:"test_threshold"`
	PIITypes      []string  `json:"pii_types,omitempty"`
}

type BreachDetail struct {
	Timestamp time.Time `json:"timestamp"`
	PIIType   string    `json:"pii_type"`
	Context   string    `json:"context"`
	Org       string    `json:"organization"`
	SourceID  string    `json:"source_id"`
	Count     int       `json:"count"`
	Average   float64   `json:"average"`
	ZScore    float64   `json:"z_score,omitempty"`
}

type Stats struct {
	Count             int       `bson:"count" json:"count"`
	Mean              float64   `bson:"mean" json:"mean"`
	M2                float64   `bson:"m2" json:"m2"`
	LastAlertTime     time.Time `bson:"last_alert_time" json:"last_alert_time"`
	ConsecutiveNormal int       `bson:"consecutive_normal" json:"consecutive_normal"`
}

type ReplayResponse struct {
	TotalPointsProcessed    int            `json:"total_points_processed"`
	VirtualBreachesDetected int            `json:"virtual_breaches_detected"`
	BreachDetails           []BreachDetail `json:"breach_details"`
}

type Mute struct {
	Organization string    `bson:"organization" json:"organization"`
	Context      string    `bson:"context" json:"context"`
	ExpiresAt    time.Time `bson:"expires_at" json:"expires_at"`
}

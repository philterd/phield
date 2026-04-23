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

package db

import (
	"context"
	"time"

	"github.com/philterd/phield/internal/models"
)

type Storage interface {
	Save(ctx context.Context, entry models.PIIEntry) error
	GetAverage(ctx context.Context, sourceID string, organization string, contextName string, piiType string, windowSizeHours int) (float64, error)
	GetStats(ctx context.Context, sourceID string, organization string, contextName string, piiType string) (models.Stats, error)
	SaveStats(ctx context.Context, sourceID string, organization string, contextName string, piiType string, stats models.Stats) error
	SaveMetric(ctx context.Context, latency time.Duration) error
	GetMetrics(ctx context.Context, windowSizeHours int) (int, float64, error) // count, avg latency in seconds
	SaveMute(ctx context.Context, organization string, contextName string, minutes int) error
	IsMuted(ctx context.Context, organization string, contextName string) (bool, error)
	GetEntries(ctx context.Context, startTime time.Time, endTime time.Time) (<-chan models.PIIEntry, <-chan error)
}

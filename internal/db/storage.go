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
	"errors"
	"time"

	"github.com/philterd/phield/internal/models"
)

// ErrStatsConflict is returned by SaveStats when the stored stats have been
// updated since they were read, which happens when two ingests for the same
// series overlap. The caller should read the stats again and redo its work.
var ErrStatsConflict = errors.New("stats were modified concurrently")

type Storage interface {
	Save(ctx context.Context, entry models.PIIEntry) error
	GetAverage(ctx context.Context, sourceID string, organization string, contextName string, piiType string, windowSizeHours int) (float64, error)
	GetStats(ctx context.Context, sourceID string, organization string, contextName string, piiType string) (models.Stats, error)
	// SaveStats stores stats for a series. It succeeds only if the stored
	// version still matches stats.Version, and returns ErrStatsConflict
	// otherwise, so a read-modify-write cycle cannot silently lose an update.
	SaveStats(ctx context.Context, sourceID string, organization string, contextName string, piiType string, stats models.Stats) error
	SaveMetric(ctx context.Context, latency time.Duration) error
	GetMetrics(ctx context.Context, windowSizeHours int) (int, float64, error) // count, avg latency in seconds
	SaveMute(ctx context.Context, organization string, contextName string, minutes int) error
	IsMuted(ctx context.Context, organization string, contextName string) (bool, error)
	GetEntries(ctx context.Context, startTime time.Time, endTime time.Time) (<-chan models.PIIEntry, <-chan error)
	SaveBreach(ctx context.Context, breach models.BreachDetail) error
	GetBreaches(ctx context.Context, startTime time.Time, endTime time.Time) ([]models.BreachDetail, error)
	GetAllStats(ctx context.Context) ([]models.StatsEntry, error)
}

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
	"slices"
	"sync"
	"time"

	"github.com/philterd/phield/internal/models"
)

type InMemoryStorage struct {
	mu      sync.RWMutex
	entries []models.PIIEntry
	metrics []models.MetricEntry
	mutes   map[string]time.Time    // key is organization:context
	stats   map[string]models.Stats // key is organization:context:sourceID:piiType
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		entries: make([]models.PIIEntry, 0),
		metrics: make([]models.MetricEntry, 0),
		mutes:   make(map[string]time.Time),
		stats:   make(map[string]models.Stats),
	}
}

func (s *InMemoryStorage) Save(ctx context.Context, entry models.PIIEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	return nil
}

func (s *InMemoryStorage) SaveMetric(ctx context.Context, latency time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = append(s.metrics, models.MetricEntry{
		Timestamp: time.Now(),
		Latency:   latency,
	})
	return nil
}

func (s *InMemoryStorage) GetMetrics(ctx context.Context, windowSizeHours int) (int, float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lookback := time.Now().Add(-time.Duration(windowSizeHours) * time.Hour)
	var count int
	var totalLatency time.Duration

	for _, m := range s.metrics {
		if m.Timestamp.After(lookback) || m.Timestamp.Equal(lookback) {
			count++
			totalLatency += m.Latency
		}
	}

	if count == 0 {
		return 0, 0, nil
	}

	avgLatencySeconds := totalLatency.Seconds() / float64(count)
	return count, avgLatencySeconds, nil
}

func (s *InMemoryStorage) GetAverage(ctx context.Context, sourceID string, organization string, contextName string, piiType string, windowSizeHours int) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lookback := time.Now().Add(-time.Duration(windowSizeHours) * time.Hour)
	var total int
	var count int

	for _, entry := range s.entries {
		if entry.SourceID == sourceID && entry.Organization == organization && entry.Context == contextName && (entry.Timestamp.After(lookback) || entry.Timestamp.Equal(lookback)) {
			if val, ok := entry.PIITypes[piiType]; ok {
				total += val
				count++
			} else {
				// According to MongoDB aggregation pipeline, we use $ifNull and $avg
				// If the field is missing, it counts as 0 in $avg if we explicitly project it as 0
				// Our MongoDB project: "count": bson.M{"$ifNull": []any{fmt.Sprintf("$pii_types.%s", piiType), 0}}
				// This means every entry for that source in the window counts towards the average.
				count++
			}
		}
	}

	if count == 0 {
		return 0, nil
	}

	return float64(total) / float64(count), nil

}

func (s *InMemoryStorage) GetStats(ctx context.Context, sourceID string, organization string, contextName string, piiType string) (models.Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := organization + ":" + contextName + ":" + sourceID + ":" + piiType
	if stats, ok := s.stats[key]; ok {
		return stats, nil
	}
	return models.Stats{}, nil
}

func (s *InMemoryStorage) SaveStats(ctx context.Context, sourceID string, organization string, contextName string, piiType string, stats models.Stats) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := organization + ":" + contextName + ":" + sourceID + ":" + piiType
	s.stats[key] = stats
	return nil
}

func (s *InMemoryStorage) SaveMute(ctx context.Context, organization string, contextName string, minutes int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := organization + ":" + contextName
	s.mutes[key] = time.Now().Add(time.Duration(minutes) * time.Minute)
	return nil
}

func (s *InMemoryStorage) IsMuted(ctx context.Context, organization string, contextName string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := organization + ":" + contextName
	if expiry, ok := s.mutes[key]; ok {
		if time.Now().Before(expiry) {
			return true, nil
		}
	}
	return false, nil
}

func (s *InMemoryStorage) GetEntries(ctx context.Context, startTime time.Time, endTime time.Time) (<-chan models.PIIEntry, <-chan error) {
	entryChan := make(chan models.PIIEntry)
	errChan := make(chan error, 1)

	go func() {
		defer close(entryChan)
		defer close(errChan)

		s.mu.RLock()
		defer s.mu.RUnlock()

		// We need to sort by timestamp
		sortedEntries := make([]models.PIIEntry, 0)
		for _, entry := range s.entries {
			if (entry.Timestamp.After(startTime) || entry.Timestamp.Equal(startTime)) &&
				(entry.Timestamp.Before(endTime) || entry.Timestamp.Equal(endTime)) {
				sortedEntries = append(sortedEntries, entry)
			}
		}

		slices.SortFunc(sortedEntries, func(a, b models.PIIEntry) int {
			return a.Timestamp.Compare(b.Timestamp)
		})

		for _, entry := range sortedEntries {
			select {
			case entryChan <- entry:
			case <-ctx.Done():
				return
			}
		}
	}()

	return entryChan, errChan
}

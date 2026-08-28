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

package api

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/philterd/phield/internal/db"
	"github.com/philterd/phield/internal/models"
)

// stubStorage wraps a real storage so a single operation can be made to fail.
type stubStorage struct {
	db.Storage

	mu             sync.Mutex
	getStatsErr    error
	conflictsLeft  int
	saveStatsCalls int
	getStatsCalls  int
}

func (s *stubStorage) GetStats(ctx context.Context, sourceID string, organization string, contextName string, piiType string) (models.Stats, error) {
	s.mu.Lock()
	s.getStatsCalls++
	err := s.getStatsErr
	s.mu.Unlock()

	if err != nil {
		return models.Stats{}, err
	}
	return s.Storage.GetStats(ctx, sourceID, organization, contextName, piiType)
}

func (s *stubStorage) SaveStats(ctx context.Context, sourceID string, organization string, contextName string, piiType string, stats models.Stats) error {
	s.mu.Lock()
	s.saveStatsCalls++
	conflict := s.conflictsLeft > 0
	if conflict {
		s.conflictsLeft--
	}
	s.mu.Unlock()

	if conflict {
		return db.ErrStatsConflict
	}
	return s.Storage.SaveStats(ctx, sourceID, organization, contextName, piiType, stats)
}

func (s *stubStorage) counts() (getStats int, saveStats int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getStatsCalls, s.saveStatsCalls
}

type countingNotifier struct {
	mu       sync.Mutex
	messages []string
}

func (n *countingNotifier) Notify(ctx context.Context, message string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.messages = append(n.messages, message)
	return nil
}

func (n *countingNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.messages)
}

func ingest(t *testing.T, a *API, sourceID string, count int) {
	t.Helper()

	err := a.ProcessIngest(context.Background(), models.IngestRequest{
		Timestamp:    time.Now(),
		SourceID:     sourceID,
		Organization: "org-1",
		Context:      "default",
		PIITypes:     map[string]int{"ssn": count},
	})
	if err != nil {
		t.Fatalf("ProcessIngest: %v", err)
	}
}

// A baseline that cannot be read must be left alone. Analyzing anyway would
// treat the series as brand new and write that back, discarding the history the
// z-score method depends on.
func TestAnalyzeTrendKeepsBaselineWhenStatsCannotBeRead(t *testing.T) {
	storage := db.NewInMemoryStorage()
	stub := &stubStorage{Storage: storage}
	a := NewAPI(stub, "1.0.0", 0.2, "z_score", 24, 3.0, 20, 60, nil)

	// Build up a baseline.
	for i := 0; i < 25; i++ {
		ingest(t, a, "source-1", 10)
	}

	before, err := storage.GetStats(context.Background(), "source-1", "org-1", "default", "ssn")
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if before.Count != 25 {
		t.Fatalf("expected a baseline of 25 points, got %d", before.Count)
	}

	// The next read fails.
	stub.mu.Lock()
	stub.getStatsErr = errors.New("storage unavailable")
	stub.mu.Unlock()

	_, savesBefore := stub.counts()
	ingest(t, a, "source-1", 10)
	_, savesAfter := stub.counts()

	if savesAfter != savesBefore {
		t.Errorf("expected no stats write while the baseline was unreadable, got %d", savesAfter-savesBefore)
	}

	stub.mu.Lock()
	stub.getStatsErr = nil
	stub.mu.Unlock()

	after, err := storage.GetStats(context.Background(), "source-1", "org-1", "default", "ssn")
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if after.Count != before.Count || after.Mean != before.Mean || after.M2 != before.M2 {
		t.Errorf("expected the baseline to survive, was %+v and is now %+v", before, after)
	}
}

// Concurrent ingests for one series must all reach the baseline. Reading,
// updating, and writing the stats without a version check loses some of them.
func TestAnalyzeTrendDoesNotLoseConcurrentUpdates(t *testing.T) {
	storage := db.NewInMemoryStorage()
	a := NewAPI(storage, "1.0.0", 0.2, "z_score", 24, 3.0, 20, 60, nil)

	const ingests = 50

	var wg sync.WaitGroup
	for i := 0; i < ingests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = a.ProcessIngest(context.Background(), models.IngestRequest{
				Timestamp:    time.Now(),
				SourceID:     "source-1",
				Organization: "org-1",
				Context:      "default",
				PIITypes:     map[string]int{"ssn": 10},
			})
		}()
	}
	wg.Wait()

	stats, err := storage.GetStats(context.Background(), "source-1", "org-1", "default", "ssn")
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}

	if stats.Count != ingests {
		t.Errorf("expected all %d counts in the baseline, got %d", ingests, stats.Count)
	}
	if stats.Mean < 9.99 || stats.Mean > 10.01 {
		t.Errorf("expected a mean of 10, got %f", stats.Mean)
	}
	if stats.Version != ingests {
		t.Errorf("expected the version to advance once per write, got %d", stats.Version)
	}
}

// A conflicting write is redone against the current stats, and the retry must
// not alert twice for the one count.
func TestAnalyzeTrendRetriesOnConflictAndAlertsOnce(t *testing.T) {
	storage := db.NewInMemoryStorage()
	stub := &stubStorage{Storage: storage}
	notifier := &countingNotifier{}
	a := NewAPI(stub, "1.0.0", 0.2, "percentage_delta", 24, 3.0, 20, 60, notifier)

	// A steady baseline, so the spike below is a breach.
	for i := 0; i < 5; i++ {
		ingest(t, a, "source-1", 10)
	}
	if notifier.count() != 0 {
		t.Fatalf("expected no alerts from the baseline, got %d", notifier.count())
	}

	getsBefore, savesBefore := stub.counts()

	// The first write of the spike loses a race.
	stub.mu.Lock()
	stub.conflictsLeft = 1
	stub.mu.Unlock()

	ingest(t, a, "source-1", 500)

	gets, saves := stub.counts()
	if gets-getsBefore != 2 {
		t.Errorf("expected the analysis to be redone once (2 reads), got %d", gets-getsBefore)
	}
	if saves-savesBefore != 2 {
		t.Errorf("expected two write attempts, got %d", saves-savesBefore)
	}

	if notifier.count() != 1 {
		t.Errorf("expected exactly one alert, got %d", notifier.count())
	}

	breaches, err := storage.GetBreaches(context.Background(), time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("GetBreaches: %v", err)
	}
	if len(breaches) != 1 {
		t.Errorf("expected exactly one recorded breach, got %d", len(breaches))
	}

	stats, err := storage.GetStats(context.Background(), "source-1", "org-1", "default", "ssn")
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.Count != 6 {
		t.Errorf("expected all 6 counts in the baseline, got %d", stats.Count)
	}
}

// Nothing is reported when the write never lands.
func TestAnalyzeTrendGivesUpAfterRepeatedConflicts(t *testing.T) {
	storage := db.NewInMemoryStorage()
	stub := &stubStorage{Storage: storage}
	notifier := &countingNotifier{}
	a := NewAPI(stub, "1.0.0", 0.2, "percentage_delta", 24, 3.0, 20, 60, notifier)

	for i := 0; i < 5; i++ {
		ingest(t, a, "source-1", 10)
	}

	_, savesBefore := stub.counts()

	stub.mu.Lock()
	stub.conflictsLeft = maxStatsAttempts
	stub.mu.Unlock()

	ingest(t, a, "source-1", 500)

	_, saves := stub.counts()
	if saves-savesBefore != maxStatsAttempts {
		t.Errorf("expected %d attempts, got %d", maxStatsAttempts, saves-savesBefore)
	}
	if notifier.count() != 0 {
		t.Errorf("expected no alert when the stats were never stored, got %d", notifier.count())
	}
}

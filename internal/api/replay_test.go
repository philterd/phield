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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/philterd/phield/internal/db"
	"github.com/philterd/phield/internal/models"
	"github.com/philterd/phield/internal/trend"
)

// replaySimEntry mirrors the point history the pre-incremental replay kept per
// series. It exists only for referenceReplay.
type replaySimEntry struct {
	timestamp time.Time
	count     int
}

// referenceReplay reproduces the replay algorithm as it stood before the
// incremental rewrite: the z-score path rebuilt a tracker from every earlier
// point of the series on each point. The rewrite applies the same Welford
// updates in the same order, so results must match exactly.
func referenceReplay(entries []models.PIIEntry, method string, warmUpCount int, windowSize int, threshold float64) models.ReplayResponse {
	history := make(map[string][]replaySimEntry)
	resp := models.ReplayResponse{BreachDetails: make([]models.BreachDetail, 0)}

	for _, entry := range entries {
		resp.TotalPointsProcessed++

		for piiType, currentCount := range entry.PIITypes {
			key := fmt.Sprintf("%s:%s:%s:%s", entry.Organization, entry.Context, entry.SourceID, piiType)

			var val float64
			var breached bool
			var avg float64

			if method == trend.MethodZScore {
				currentStats := history[key]
				tracker := trend.StatTracker{}
				for _, h := range currentStats {
					tracker.Update(float64(h.count))
				}

				if tracker.Count >= warmUpCount {
					val, breached = trend.CalculateBreach(method, currentCount, tracker.Mean, threshold, tracker.StdDev())
				}
				avg = tracker.Mean

				history[key] = append(history[key], replaySimEntry{timestamp: entry.Timestamp, count: currentCount})
			} else {
				lookback := entry.Timestamp.Add(-time.Duration(windowSize) * time.Hour)

				currentHistory := history[key]
				var filteredHistory []replaySimEntry
				var sum int
				for _, h := range currentHistory {
					if h.timestamp.After(lookback) || h.timestamp.Equal(lookback) {
						filteredHistory = append(filteredHistory, h)
						sum += h.count
					}
				}

				if len(filteredHistory) > 0 {
					avg = float64(sum) / float64(len(filteredHistory))
					val, breached = trend.CalculateBreach(method, currentCount, avg, threshold, 0)
				}

				filteredHistory = append(filteredHistory, replaySimEntry{timestamp: entry.Timestamp, count: currentCount})
				history[key] = filteredHistory
			}

			if breached {
				resp.VirtualBreachesDetected++
				detail := models.BreachDetail{
					Timestamp: entry.Timestamp,
					PIIType:   piiType,
					Context:   entry.Context,
					Org:       entry.Organization,
					SourceID:  entry.SourceID,
					Count:     currentCount,
					Average:   avg,
				}
				if method == trend.MethodZScore {
					detail.ZScore = val
				}
				resp.BreachDetails = append(resp.BreachDetails, detail)
			}
		}
	}

	return resp
}

// replayFixture builds a deterministic series of counts with periodic spikes.
// Each entry carries a single PII type so the order breaches are recorded in
// does not depend on map iteration order.
func replayFixture(t *testing.T, points int, start time.Time) (db.Storage, []models.PIIEntry) {
	t.Helper()

	storage := db.NewInMemoryStorage()
	entries := make([]models.PIIEntry, 0, points)

	sources := []string{"source-a", "source-b"}
	piiTypes := []string{"ssn", "credit_card"}

	for i := 0; i < points; i++ {
		count := 100 + (i%7)*3
		if i%37 == 0 && i > 0 {
			count = 900
		}

		entry := models.PIIEntry{
			Timestamp:    start.Add(time.Duration(i) * time.Minute),
			SourceID:     sources[i%len(sources)],
			Organization: "default",
			Context:      "default",
			PIITypes:     map[string]int{piiTypes[(i/2)%len(piiTypes)]: count},
		}

		if err := storage.Save(context.Background(), entry); err != nil {
			t.Fatalf("saving fixture entry: %v", err)
		}
		entries = append(entries, entry)
	}

	return storage, entries
}

// TestReplayMatchesPreIncrementalResults pins the rewrite to the algorithm it
// replaced. The comparison is exact: the same updates in the same order
// produce bit-identical statistics, so any difference is a real one.
func TestReplayMatchesPreIncrementalResults(t *testing.T) {
	for _, method := range []string{trend.MethodZScore, "percentage_delta"} {
		t.Run(method, func(t *testing.T) {
			start := time.Now().Add(-24 * time.Hour)
			storage, entries := replayFixture(t, 400, start)

			a := NewAPI(storage, "1.0.0", 0.2, method, 24, 3.0, 20, 60, nil)

			got, err := a.RunReplay(context.Background(), models.ReplayRequest{
				StartTime:     start.Add(-time.Hour),
				EndTime:       start.Add(500 * time.Minute),
				TestThreshold: 2.0,
			})
			if err != nil {
				t.Fatalf("RunReplay: %v", err)
			}

			want := referenceReplay(entries, method, 20, 24, 2.0)

			if got.TotalPointsProcessed != want.TotalPointsProcessed {
				t.Errorf("points processed: got %d, want %d", got.TotalPointsProcessed, want.TotalPointsProcessed)
			}
			if got.VirtualBreachesDetected != want.VirtualBreachesDetected {
				t.Errorf("breaches detected: got %d, want %d", got.VirtualBreachesDetected, want.VirtualBreachesDetected)
			}
			if got.VirtualBreachesDetected == 0 {
				t.Fatal("fixture produced no breaches, so the comparison proves nothing")
			}
			if len(got.BreachDetails) != len(want.BreachDetails) {
				t.Fatalf("breach details: got %d, want %d", len(got.BreachDetails), len(want.BreachDetails))
			}

			for i := range want.BreachDetails {
				if got.BreachDetails[i] != want.BreachDetails[i] {
					t.Errorf("breach detail %d:\n got %+v\nwant %+v", i, got.BreachDetails[i], want.BreachDetails[i])
				}
			}
		})
	}
}

// TestReplayHandlesLongSeries covers a series long enough to show the cost the
// rewrite removed. Rebuilding the tracker from every earlier point made this
// fixture roughly forty times slower (about 2s against 0.05s here), and that
// gap grows with the square of the series length, so a replay over a
// production-sized history was effectively unbounded.
func TestReplayHandlesLongSeries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long replay in short mode")
	}

	start := time.Now().Add(-90 * 24 * time.Hour)
	storage, _ := replayFixture(t, 50000, start)

	a := NewAPI(storage, "1.0.0", 0.2, trend.MethodZScore, 24, 3.0, 20, 60, nil)
	a.SetReplayLimits(ReplayLimits{MaxWindow: 0, MaxBreachDetails: 0, MaxConcurrent: 1})

	resp, err := a.RunReplay(context.Background(), models.ReplayRequest{
		StartTime:     start.Add(-time.Hour),
		EndTime:       start.Add(50001 * time.Minute),
		TestThreshold: 2.0,
	})
	if err != nil {
		t.Fatalf("RunReplay: %v", err)
	}

	if resp.TotalPointsProcessed != 50000 {
		t.Errorf("points processed: got %d, want 50000", resp.TotalPointsProcessed)
	}
	if resp.VirtualBreachesDetected == 0 {
		t.Error("expected breaches over a long series")
	}
}

func TestReplayTruncatesBreachDetails(t *testing.T) {
	start := time.Now().Add(-24 * time.Hour)
	storage, _ := replayFixture(t, 400, start)

	a := NewAPI(storage, "1.0.0", 0.2, trend.MethodZScore, 24, 3.0, 20, 60, nil)

	unlimited, err := a.RunReplay(context.Background(), models.ReplayRequest{
		StartTime:     start.Add(-time.Hour),
		EndTime:       start.Add(500 * time.Minute),
		TestThreshold: 2.0,
	})
	if err != nil {
		t.Fatalf("RunReplay: %v", err)
	}
	if unlimited.BreachDetailsTruncated {
		t.Fatal("default limit truncated the fixture, so the capped case proves nothing")
	}
	if unlimited.VirtualBreachesDetected < 3 {
		t.Fatalf("fixture needs at least 3 breaches to test truncation, got %d", unlimited.VirtualBreachesDetected)
	}

	a.SetReplayLimits(ReplayLimits{MaxWindow: DefaultReplayLimits.MaxWindow, MaxBreachDetails: 2, MaxConcurrent: 1})

	capped, err := a.RunReplay(context.Background(), models.ReplayRequest{
		StartTime:     start.Add(-time.Hour),
		EndTime:       start.Add(500 * time.Minute),
		TestThreshold: 2.0,
	})
	if err != nil {
		t.Fatalf("RunReplay: %v", err)
	}

	if len(capped.BreachDetails) != 2 {
		t.Errorf("breach details: got %d, want 2", len(capped.BreachDetails))
	}
	if !capped.BreachDetailsTruncated {
		t.Error("expected breach_details_truncated to be set")
	}
	if capped.VirtualBreachesDetected != unlimited.VirtualBreachesDetected {
		t.Errorf("total breaches changed under truncation: got %d, want %d",
			capped.VirtualBreachesDetected, unlimited.VirtualBreachesDetected)
	}
	for i := range capped.BreachDetails {
		if capped.BreachDetails[i] != unlimited.BreachDetails[i] {
			t.Errorf("truncated detail %d differs from the full run", i)
		}
	}
}

func postReplay(t *testing.T, a *API, req models.ReplayRequest) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	a.RegisterRoutes(r)

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshaling request: %v", err)
	}

	w := httptest.NewRecorder()
	httpReq, err := http.NewRequest("POST", "/replay", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	return w
}

func TestReplayRequestValidation(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		limits     ReplayLimits
		req        models.ReplayRequest
		wantStatus int
		wantError  string
	}{
		{
			name:       "window at the limit is accepted",
			limits:     ReplayLimits{MaxWindow: 24 * time.Hour, MaxBreachDetails: 1000, MaxConcurrent: 1},
			req:        models.ReplayRequest{StartTime: now.Add(-24 * time.Hour), EndTime: now, TestThreshold: 0.2},
			wantStatus: http.StatusOK,
		},
		{
			name:       "window over the limit is rejected",
			limits:     ReplayLimits{MaxWindow: 24 * time.Hour, MaxBreachDetails: 1000, MaxConcurrent: 1},
			req:        models.ReplayRequest{StartTime: now.Add(-25 * time.Hour), EndTime: now, TestThreshold: 0.2},
			wantStatus: http.StatusBadRequest,
			wantError:  "replay window must not exceed 24 hours",
		},
		{
			name:       "inverted range is rejected",
			limits:     DefaultReplayLimits,
			req:        models.ReplayRequest{StartTime: now, EndTime: now.Add(-time.Hour), TestThreshold: 0.2},
			wantStatus: http.StatusBadRequest,
			wantError:  "end_time must not precede start_time",
		},
		{
			name:       "equal start and end is an empty range, not an error",
			limits:     DefaultReplayLimits,
			req:        models.ReplayRequest{StartTime: now, EndTime: now, TestThreshold: 0.2},
			wantStatus: http.StatusOK,
		},
		{
			// time.Sub saturates rather than wrapping, so a range wider than a
			// Duration can hold is still measured as too wide.
			name:       "a range wider than Duration can represent is rejected",
			limits:     DefaultReplayLimits,
			req:        models.ReplayRequest{StartTime: time.Date(1000, 1, 1, 0, 0, 0, 0, time.UTC), EndTime: time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC), TestThreshold: 0.2},
			wantStatus: http.StatusBadRequest,
			wantError:  "replay window must not exceed 2160 hours",
		},
		{
			name:       "an unlimited window accepts any range",
			limits:     ReplayLimits{MaxWindow: 0, MaxBreachDetails: 1000, MaxConcurrent: 1},
			req:        models.ReplayRequest{StartTime: now.Add(-10000 * time.Hour), EndTime: now, TestThreshold: 0.2},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAPI(db.NewInMemoryStorage(), "1.0.0", 0.2, "percentage_delta", 24, 3.0, 20, 60, nil)
			a.SetReplayLimits(tt.limits)

			w := postReplay(t, a, tt.req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d (body %s)", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantError != "" {
				var resp map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("unmarshaling error response: %v", err)
				}
				if resp["error"] != tt.wantError {
					t.Errorf("error: got %q, want %q", resp["error"], tt.wantError)
				}
			}
		})
	}
}

// blockingStorage holds a replay inside GetEntries until it is released, so a
// second replay can be issued while the first is in flight.
type blockingStorage struct {
	db.Storage

	started  chan struct{}
	release  chan struct{}
	startOne bool
}

func (s *blockingStorage) GetEntries(ctx context.Context, startTime time.Time, endTime time.Time) (<-chan models.PIIEntry, <-chan error) {
	entryChan := make(chan models.PIIEntry)
	errChan := make(chan error, 1)

	go func() {
		defer close(entryChan)
		defer close(errChan)

		if !s.startOne {
			s.startOne = true
			close(s.started)
			<-s.release
		}
	}()

	return entryChan, errChan
}

func TestReplayConcurrencyLimit(t *testing.T) {
	now := time.Now()
	storage := &blockingStorage{
		Storage: db.NewInMemoryStorage(),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}

	a := NewAPI(storage, "1.0.0", 0.2, "percentage_delta", 24, 3.0, 20, 60, nil)
	a.SetReplayLimits(ReplayLimits{MaxWindow: DefaultReplayLimits.MaxWindow, MaxBreachDetails: 1000, MaxConcurrent: 1})

	req := models.ReplayRequest{StartTime: now.Add(-time.Hour), EndTime: now, TestThreshold: 0.2}

	firstDone := make(chan int, 1)
	go func() {
		firstDone <- postReplay(t, a, req).Code
	}()

	select {
	case <-storage.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the first replay never reached storage")
	}

	w := postReplay(t, a, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("second replay status: got %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshaling error response: %v", err)
	}
	if resp["error"] != "too many concurrent replays; at most 1 may run at once" {
		t.Errorf("unexpected error: %q", resp["error"])
	}

	close(storage.release)

	select {
	case code := <-firstDone:
		if code != http.StatusOK {
			t.Errorf("first replay status: got %d, want 200", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the first replay did not finish")
	}

	// The slot is released when the first replay returns, so a later replay is
	// admitted rather than rejected for good.
	if w := postReplay(t, a, req); w.Code != http.StatusOK {
		t.Errorf("replay after the first finished: got %d, want 200", w.Code)
	}
}

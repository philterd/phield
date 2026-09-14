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
	"testing"
	"time"

	"github.com/philterd/phield/internal/db"
	"github.com/philterd/phield/internal/models"
)

func mute(t *testing.T, storage db.Storage, minutes int) {
	t.Helper()

	if err := storage.SaveMute(context.Background(), "org-1", "default", minutes); err != nil {
		t.Fatalf("SaveMute: %v", err)
	}
}

// unmute ends a mute early. It goes through storage because the API has no way
// to do it: POST /mute rejects a duration of zero, so over HTTP a mute can only
// be waited out.
func unmute(t *testing.T, storage db.Storage) {
	t.Helper()

	if err := storage.SaveMute(context.Background(), "org-1", "default", 0); err != nil {
		t.Fatalf("clearing the mute: %v", err)
	}
}

func statsFor(t *testing.T, storage db.Storage, sourceID string) models.Stats {
	t.Helper()

	stats, err := storage.GetStats(context.Background(), sourceID, "org-1", "default", "ssn")
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	return stats
}

func breaches(t *testing.T, storage db.Storage) []models.BreachDetail {
	t.Helper()

	found, err := storage.GetBreaches(context.Background(), time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("GetBreaches: %v", err)
	}
	return found
}

// A mute suppresses alerting, not learning, so counts that arrive during one
// still reach the baseline. Both trend methods maintain the same stats.
func TestMutedContextKeepsLearning(t *testing.T) {
	for _, method := range []string{"z_score", "percentage_delta"} {
		t.Run(method, func(t *testing.T) {
			storage := db.NewInMemoryStorage()
			a := NewAPI(storage, "1.0.0", 0.2, method, 24, 3.0, 20, 60, nil)

			for i := 0; i < 5; i++ {
				ingest(t, a, "source-1", 100)
			}
			before := statsFor(t, storage, "source-1")

			mute(t, storage, 10)
			for i := 0; i < 5; i++ {
				ingest(t, a, "source-1", 100)
			}
			after := statsFor(t, storage, "source-1")

			if after.Count != before.Count+5 {
				t.Errorf("baseline count across a mute: got %d, want %d", after.Count, before.Count+5)
			}
			if after.Mean < 99.99 || after.Mean > 100.01 {
				t.Errorf("baseline mean across a mute: got %f, want 100", after.Mean)
			}
		})
	}
}

// The z-score warm-up counter advances during a mute, so detection is ready as
// soon as the mute ends rather than starting its warm-up over.
func TestMuteDoesNotStallWarmUp(t *testing.T) {
	storage := db.NewInMemoryStorage()
	notifier := &countingNotifier{}
	a := NewAPI(storage, "1.0.0", 0.2, "z_score", 24, 3.0, 5, 60, notifier)

	mute(t, storage, 10)
	for i := 0; i < 10; i++ {
		ingest(t, a, "source-1", 100+i%3)
	}

	stats := statsFor(t, storage, "source-1")
	if stats.Count != 10 {
		t.Fatalf("warm-up counter during a mute: got %d, want 10", stats.Count)
	}

	unmute(t, storage)

	ingest(t, a, "source-1", 5000)

	if notifier.count() != 1 {
		t.Errorf("a spike after the mute should alert at once: got %d notifications", notifier.count())
	}
}

// A breach during a mute is recorded so the dashboard timeline shows what
// happened while the context was quiet, but nothing is sent.
func TestMutedBreachIsRecordedButNotNotified(t *testing.T) {
	storage := db.NewInMemoryStorage()
	notifier := &countingNotifier{}
	a := NewAPI(storage, "1.0.0", 0.2, "z_score", 24, 3.0, 5, 60, notifier)

	for i := 0; i < 10; i++ {
		ingest(t, a, "source-1", 100+i%3)
	}
	if got := len(breaches(t, storage)); got != 0 {
		t.Fatalf("the baseline should not have breached: got %d breaches", got)
	}

	mute(t, storage, 10)
	ingest(t, a, "source-1", 5000)

	if notifier.count() != 0 {
		t.Errorf("a muted breach must not notify: got %d notifications", notifier.count())
	}

	recorded := breaches(t, storage)
	if len(recorded) != 1 {
		t.Fatalf("a muted breach should be recorded: got %d breaches", len(recorded))
	}
	if recorded[0].Count != 5000 {
		t.Errorf("recorded breach count: got %d, want 5000", recorded[0].Count)
	}
	if recorded[0].SourceID != "source-1" || recorded[0].PIIType != "ssn" {
		t.Errorf("recorded breach does not identify the series: %+v", recorded[0])
	}
}

// The cooldown a muted breach starts throttles what is recorded during the
// mute, so a sustained breach is not written once per count.
func TestMutedBreachesAreThrottledByCooldown(t *testing.T) {
	storage := db.NewInMemoryStorage()
	a := NewAPI(storage, "1.0.0", 0.2, "z_score", 24, 3.0, 5, 60, nil)

	for i := 0; i < 10; i++ {
		ingest(t, a, "source-1", 100+i%3)
	}

	mute(t, storage, 10)
	for i := 0; i < 4; i++ {
		ingest(t, a, "source-1", 5000)
	}

	if got := len(breaches(t, storage)); got != 1 {
		t.Errorf("a sustained muted breach should be recorded once per cooldown: got %d breaches", got)
	}
}

// A cooldown begun by a muted breach reached nobody, so it must not silence the
// first alert after the mute ends.
func TestMutedBreachDoesNotSilenceTheNextAlert(t *testing.T) {
	storage := db.NewInMemoryStorage()
	notifier := &countingNotifier{}
	a := NewAPI(storage, "1.0.0", 0.2, "z_score", 24, 3.0, 5, 60, notifier)

	for i := 0; i < 10; i++ {
		ingest(t, a, "source-1", 100+i%3)
	}

	mute(t, storage, 10)
	ingest(t, a, "source-1", 5000)

	stats := statsFor(t, storage, "source-1")
	if !stats.LastAlertMuted {
		t.Fatal("a muted breach should mark the cooldown it started as muted")
	}

	unmute(t, storage)

	// Well inside the 60 minute cooldown the muted breach started.
	ingest(t, a, "source-1", 5000)

	if notifier.count() != 1 {
		t.Fatalf("the first breach after a mute should alert: got %d notifications", notifier.count())
	}

	stats = statsFor(t, storage, "source-1")
	if stats.LastAlertMuted {
		t.Error("an announced alert should clear the muted marker")
	}

	// The cooldown from the announced alert applies normally.
	ingest(t, a, "source-1", 5000)
	if notifier.count() != 1 {
		t.Errorf("a breach inside the cooldown of an announced alert should be suppressed: got %d notifications", notifier.count())
	}
}

// The counts that arrive during a mute are the baseline when it ends, so a
// series that settles at a new volume while muted does not alert on that
// volume afterward.
func TestMutedSeriesDoesNotAlertOnItsNewNormal(t *testing.T) {
	storage := db.NewInMemoryStorage()
	notifier := &countingNotifier{}
	a := NewAPI(storage, "1.0.0", 0.2, "z_score", 24, 3.0, 5, 60, notifier)

	for i := 0; i < 10; i++ {
		ingest(t, a, "source-1", 100+i%3)
	}

	// A migration raises steady-state volume while the context is muted.
	mute(t, storage, 10)
	for i := 0; i < 40; i++ {
		ingest(t, a, "source-1", 500+i%3)
	}

	unmute(t, storage)

	before := notifier.count()
	for i := 0; i < 10; i++ {
		ingest(t, a, "source-1", 500+i%3)
	}

	if notifier.count() != before {
		t.Errorf("the new normal should not alert after the mute ends: got %d new notifications", notifier.count()-before)
	}
}

// The back-to-normal reset runs during a mute like any other state, so a series
// that settles clears the cooldown its muted breach started.
func TestMutedSeriesClearsCooldownAfterNormalCounts(t *testing.T) {
	storage := db.NewInMemoryStorage()
	a := NewAPI(storage, "1.0.0", 0.2, "z_score", 24, 3.0, 5, 60, nil)

	for i := 0; i < 10; i++ {
		ingest(t, a, "source-1", 100+i%3)
	}

	mute(t, storage, 10)
	ingest(t, a, "source-1", 5000)

	if stats := statsFor(t, storage, "source-1"); stats.LastAlertTime.IsZero() {
		t.Fatal("a muted breach should start a cooldown")
	}

	// Enough normal counts to trip the back-to-normal reset. The spike is in
	// the baseline now, so counts near the old normal read as normal.
	for i := 0; i < 6; i++ {
		ingest(t, a, "source-1", 100+i%3)
	}

	stats := statsFor(t, storage, "source-1")
	if stats.ConsecutiveNormal < 3 {
		t.Fatalf("normal counts during a mute should advance the counter: got %d", stats.ConsecutiveNormal)
	}
	if !stats.LastAlertTime.IsZero() {
		t.Error("the back-to-normal reset should clear the cooldown during a mute")
	}
	if stats.LastAlertMuted {
		t.Error("the back-to-normal reset should clear the muted marker")
	}
}

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
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/philterd/phield/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// These tests run against a real MongoDB instance. Set PHIELD_TEST_MONGO_URI to
// point at one, otherwise mongodb://localhost:27017 is used. Each test gets its
// own database, which is dropped when the test finishes.
//
// When MongoDB is unreachable the tests skip, so that `go test ./...` still
// works on a machine without one. In CI they fail instead: the workflow starts a
// MongoDB service, and a skip there would silently drop the coverage.

const defaultTestMongoURI = "mongodb://localhost:27017"

// defaultTestRetention matches the PHIELD_METRICS_RETENTION_DAYS default.
const defaultTestRetention = 7 * 24 * time.Hour

var (
	mongoProbeOnce sync.Once
	mongoBaseURI   string
	mongoProbeErr  error
)

// probeMongo waits for MongoDB to accept connections and returns the base URI.
// The wait is generous in CI, where the service container may still be starting,
// and short elsewhere so a developer without MongoDB is not held up.
func probeMongo() (string, error) {
	mongoProbeOnce.Do(func() {
		uri := os.Getenv("PHIELD_TEST_MONGO_URI")
		if uri == "" {
			uri = defaultTestMongoURI
		}

		wait := 2 * time.Second
		if os.Getenv("CI") != "" {
			wait = 60 * time.Second
		}

		deadline := time.Now().Add(wait)
		for {
			mongoProbeErr = pingMongo(uri)
			if mongoProbeErr == nil {
				mongoBaseURI = uri
				return
			}
			if time.Now().After(deadline) {
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	})

	return mongoBaseURI, mongoProbeErr
}

func pingMongo(uri string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(2 * time.Second))
	if err != nil {
		return err
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	return client.Ping(ctx, nil)
}

// testURI builds a URI for a database of its own, preserving any query string on
// the base URI.
func testURI(base string, dbName string) string {
	query := ""
	if idx := strings.Index(base, "?"); idx != -1 {
		base, query = base[:idx], base[idx:]
	}
	return strings.TrimSuffix(base, "/") + "/" + dbName + query
}

// testDBName derives a database name that is unique to this test and this run.
func testDBName(t *testing.T) string {
	sanitized := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, t.Name())

	if len(sanitized) > 20 {
		sanitized = sanitized[:20]
	}

	return fmt.Sprintf("phield_test_%s_%d", sanitized, time.Now().UnixNano())
}

// newTestMongo returns a MongoDB backed by a database of its own.
func newTestMongo(t *testing.T) *MongoDB {
	t.Helper()

	base, err := probeMongo()
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("MongoDB is required in CI but was unreachable: %v", err)
		}
		t.Skipf("skipping: no MongoDB at %s (set PHIELD_TEST_MONGO_URI to override): %v", defaultTestMongoURI, err)
	}

	m, err := Connect(testURI(base, testDBName(t)), defaultTestRetention)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := m.DB.Drop(ctx); err != nil {
			t.Errorf("dropping test database: %v", err)
		}
		if err := m.Client.Disconnect(ctx); err != nil {
			t.Errorf("disconnecting: %v", err)
		}
	})

	return m
}

func closeEnough(got, want float64) bool {
	return math.Abs(got-want) < 0.0001
}

func TestMongoConnect(t *testing.T) {
	m := newTestMongo(t)
	ctx := context.Background()

	// Connect creates pii_counts as a time-series collection.
	specs, err := m.DB.ListCollectionSpecifications(ctx, map[string]any{"name": "pii_counts"})
	if err != nil {
		t.Fatalf("ListCollectionSpecifications: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected pii_counts to exist, found %d matching collections", len(specs))
	}
	if specs[0].Type != "timeseries" {
		t.Errorf("expected pii_counts to be a timeseries collection, got %q", specs[0].Type)
	}

	// Connecting again to the same database leaves the existing collection alone.
	second, err := Connect(testURI(mongoBaseURI, m.DB.Name()), defaultTestRetention)
	if err != nil {
		t.Fatalf("second Connect: %v", err)
	}
	defer func() { _ = second.Client.Disconnect(ctx) }()

	names, err := second.DB.ListCollectionNames(ctx, map[string]any{"name": "pii_counts"})
	if err != nil {
		t.Fatalf("ListCollectionNames: %v", err)
	}
	if len(names) != 1 {
		t.Errorf("expected exactly one pii_counts collection, got %d", len(names))
	}
}

func TestMongoConnectUnreachable(t *testing.T) {
	// Port 1 is reserved and nothing listens on it.
	if _, err := Connect("mongodb://127.0.0.1:1/phield?serverSelectionTimeoutMS=500", defaultTestRetention); err == nil {
		t.Error("expected an error connecting to an unreachable server")
	}
}

func TestMongoSaveAndGetAverage(t *testing.T) {
	m := newTestMongo(t)
	ctx := context.Background()
	now := time.Now()

	entries := []models.PIIEntry{
		{Timestamp: now.Add(-1 * time.Hour), SourceID: "source-1", Organization: "org-1", Context: "default", PIITypes: map[string]int{"ssn": 10}},
		{Timestamp: now.Add(-2 * time.Hour), SourceID: "source-1", Organization: "org-1", Context: "default", PIITypes: map[string]int{"ssn": 20}},
		{Timestamp: now.Add(-25 * time.Hour), SourceID: "source-1", Organization: "org-1", Context: "default", PIITypes: map[string]int{"ssn": 100}}, // outside the window
		{Timestamp: now.Add(-3 * time.Hour), SourceID: "source-2", Organization: "org-1", Context: "default", PIITypes: map[string]int{"ssn": 50}},   // other source
		{Timestamp: now.Add(-1 * time.Hour), SourceID: "source-1", Organization: "org-2", Context: "default", PIITypes: map[string]int{"ssn": 700}},  // other organization
		{Timestamp: now.Add(-1 * time.Hour), SourceID: "source-1", Organization: "org-1", Context: "other", PIITypes: map[string]int{"ssn": 500}},    // other context
	}

	for _, entry := range entries {
		if err := m.Save(ctx, entry); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	t.Run("averages only the matching entries in the window", func(t *testing.T) {
		avg, err := m.GetAverage(ctx, "source-1", "org-1", "default", "ssn", 24)
		if err != nil {
			t.Fatalf("GetAverage: %v", err)
		}
		if !closeEnough(avg, 15) {
			t.Errorf("expected average 15, got %f", avg)
		}
	})

	t.Run("entries missing the PII type count as zero", func(t *testing.T) {
		if err := m.Save(ctx, models.PIIEntry{
			Timestamp:    now.Add(-4 * time.Hour),
			SourceID:     "source-1",
			Organization: "org-1",
			Context:      "default",
			PIITypes:     map[string]int{"email": 10},
		}); err != nil {
			t.Fatalf("Save: %v", err)
		}

		avg, err := m.GetAverage(ctx, "source-1", "org-1", "default", "ssn", 24)
		if err != nil {
			t.Fatalf("GetAverage: %v", err)
		}
		if !closeEnough(avg, 10) {
			t.Errorf("expected average 10 over three entries, got %f", avg)
		}
	})

	t.Run("a wider window picks up older entries", func(t *testing.T) {
		avg, err := m.GetAverage(ctx, "source-1", "org-1", "default", "ssn", 48)
		if err != nil {
			t.Fatalf("GetAverage: %v", err)
		}
		if !closeEnough(avg, 32.5) {
			t.Errorf("expected average 32.5, got %f", avg)
		}
	})

	t.Run("unknown source returns zero", func(t *testing.T) {
		avg, err := m.GetAverage(ctx, "no-such-source", "org-1", "default", "ssn", 24)
		if err != nil {
			t.Fatalf("GetAverage: %v", err)
		}
		if avg != 0 {
			t.Errorf("expected 0, got %f", avg)
		}
	})

	t.Run("unknown PII type returns zero", func(t *testing.T) {
		avg, err := m.GetAverage(ctx, "source-1", "org-1", "default", "passport", 24)
		if err != nil {
			t.Fatalf("GetAverage: %v", err)
		}
		if avg != 0 {
			t.Errorf("expected 0, got %f", avg)
		}
	})
}

func TestMongoStats(t *testing.T) {
	m := newTestMongo(t)
	ctx := context.Background()

	t.Run("missing stats read back empty without an error", func(t *testing.T) {
		stats, err := m.GetStats(ctx, "source-1", "org-1", "default", "ssn")
		if err != nil {
			t.Fatalf("GetStats: %v", err)
		}
		if stats.Count != 0 || stats.Mean != 0 || stats.M2 != 0 {
			t.Errorf("expected zero stats, got %+v", stats)
		}
	})

	alertTime := time.Now().Add(-30 * time.Minute).UTC().Truncate(time.Millisecond)
	saved := models.Stats{Count: 5, Mean: 12.5, M2: 7.25, LastAlertTime: alertTime, ConsecutiveNormal: 2}

	t.Run("stats round trip", func(t *testing.T) {
		if err := m.SaveStats(ctx, "source-1", "org-1", "default", "ssn", saved); err != nil {
			t.Fatalf("SaveStats: %v", err)
		}

		got, err := m.GetStats(ctx, "source-1", "org-1", "default", "ssn")
		if err != nil {
			t.Fatalf("GetStats: %v", err)
		}
		if got.Count != saved.Count || !closeEnough(got.Mean, saved.Mean) || !closeEnough(got.M2, saved.M2) {
			t.Errorf("expected %+v, got %+v", saved, got)
		}
		if got.ConsecutiveNormal != saved.ConsecutiveNormal {
			t.Errorf("expected ConsecutiveNormal %d, got %d", saved.ConsecutiveNormal, got.ConsecutiveNormal)
		}
		if !got.LastAlertTime.Equal(alertTime) {
			t.Errorf("expected LastAlertTime %v, got %v", alertTime, got.LastAlertTime)
		}
	})

	t.Run("saving again updates in place", func(t *testing.T) {
		// A write carries the version it read. See TestMongoSaveStatsVersioning.
		current, err := m.GetStats(ctx, "source-1", "org-1", "default", "ssn")
		if err != nil {
			t.Fatalf("GetStats: %v", err)
		}

		updated := models.Stats{Count: 6, Mean: 13, M2: 8, LastAlertTime: alertTime, ConsecutiveNormal: 3, Version: current.Version}
		if err := m.SaveStats(ctx, "source-1", "org-1", "default", "ssn", updated); err != nil {
			t.Fatalf("SaveStats: %v", err)
		}

		count, err := m.DB.Collection("pii_stats").CountDocuments(ctx, map[string]any{"source_id": "source-1"})
		if err != nil {
			t.Fatalf("CountDocuments: %v", err)
		}
		if count != 1 {
			t.Errorf("expected the upsert to update in place, found %d documents", count)
		}

		got, err := m.GetStats(ctx, "source-1", "org-1", "default", "ssn")
		if err != nil {
			t.Fatalf("GetStats: %v", err)
		}
		if got.Count != 6 {
			t.Errorf("expected Count 6, got %d", got.Count)
		}
	})

	t.Run("stats are scoped per source, organization, context, and type", func(t *testing.T) {
		for _, key := range []struct{ source, org, context, piiType string }{
			{"source-2", "org-1", "default", "ssn"},
			{"source-1", "org-2", "default", "ssn"},
			{"source-1", "org-1", "other", "ssn"},
			{"source-1", "org-1", "default", "email"},
		} {
			got, err := m.GetStats(ctx, key.source, key.org, key.context, key.piiType)
			if err != nil {
				t.Fatalf("GetStats: %v", err)
			}
			if got.Count != 0 {
				t.Errorf("expected no stats for %+v, got %+v", key, got)
			}
		}
	})

	t.Run("GetAllStats returns every saved series", func(t *testing.T) {
		if err := m.SaveStats(ctx, "source-2", "org-1", "billing", "email", models.Stats{Count: 3, Mean: 4, M2: 1}); err != nil {
			t.Fatalf("SaveStats: %v", err)
		}

		all, err := m.GetAllStats(ctx)
		if err != nil {
			t.Fatalf("GetAllStats: %v", err)
		}
		if len(all) != 2 {
			t.Fatalf("expected 2 stats entries, got %d", len(all))
		}

		found := false
		for _, entry := range all {
			if entry.SourceID == "source-2" {
				found = true
				if entry.Organization != "org-1" || entry.Context != "billing" || entry.PIIType != "email" {
					t.Errorf("unexpected key fields: %+v", entry)
				}
				if entry.Stats.Count != 3 || !closeEnough(entry.Stats.Mean, 4) || !closeEnough(entry.Stats.M2, 1) {
					t.Errorf("unexpected stats: %+v", entry.Stats)
				}
			}
		}
		if !found {
			t.Error("expected source-2 in the results")
		}
	})
}

func TestMongoGetAllStatsEmpty(t *testing.T) {
	m := newTestMongo(t)

	all, err := m.GetAllStats(context.Background())
	if err != nil {
		t.Fatalf("GetAllStats: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected no stats, got %d", len(all))
	}
}

func TestMongoMetrics(t *testing.T) {
	m := newTestMongo(t)
	ctx := context.Background()

	t.Run("no metrics yet", func(t *testing.T) {
		count, avg, err := m.GetMetrics(ctx, 24)
		if err != nil {
			t.Fatalf("GetMetrics: %v", err)
		}
		if count != 0 || avg != 0 {
			t.Errorf("expected 0 and 0, got %d and %f", count, avg)
		}
	})

	t.Run("count and average latency in seconds", func(t *testing.T) {
		for _, latency := range []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond} {
			if err := m.SaveMetric(ctx, latency); err != nil {
				t.Fatalf("SaveMetric: %v", err)
			}
		}

		count, avg, err := m.GetMetrics(ctx, 24)
		if err != nil {
			t.Fatalf("GetMetrics: %v", err)
		}
		if count != 3 {
			t.Errorf("expected 3 metrics, got %d", count)
		}
		if !closeEnough(avg, 0.02) {
			t.Errorf("expected average 0.02 seconds, got %f", avg)
		}
	})

	t.Run("metrics outside the window are excluded", func(t *testing.T) {
		if _, err := m.DB.Collection("metrics").InsertOne(ctx, models.MetricEntry{
			Timestamp: time.Now().Add(-48 * time.Hour),
			Latency:   time.Second,
		}); err != nil {
			t.Fatalf("InsertOne: %v", err)
		}

		count, avg, err := m.GetMetrics(ctx, 24)
		if err != nil {
			t.Fatalf("GetMetrics: %v", err)
		}
		if count != 3 {
			t.Errorf("expected the older metric to be excluded, got %d", count)
		}
		if !closeEnough(avg, 0.02) {
			t.Errorf("expected average 0.02 seconds, got %f", avg)
		}

		count, avg, err = m.GetMetrics(ctx, 72)
		if err != nil {
			t.Fatalf("GetMetrics: %v", err)
		}
		if count != 4 {
			t.Errorf("expected 4 metrics in a 72 hour window, got %d", count)
		}
		if !closeEnough(avg, 0.265) {
			t.Errorf("expected average 0.265 seconds, got %f", avg)
		}
	})
}

func TestMongoMutes(t *testing.T) {
	m := newTestMongo(t)
	ctx := context.Background()

	t.Run("nothing is muted initially", func(t *testing.T) {
		muted, err := m.IsMuted(ctx, "org-1", "default")
		if err != nil {
			t.Fatalf("IsMuted: %v", err)
		}
		if muted {
			t.Error("expected not muted")
		}
	})

	t.Run("an active mute reads back as muted", func(t *testing.T) {
		if err := m.SaveMute(ctx, "org-1", "default", 60); err != nil {
			t.Fatalf("SaveMute: %v", err)
		}

		muted, err := m.IsMuted(ctx, "org-1", "default")
		if err != nil {
			t.Fatalf("IsMuted: %v", err)
		}
		if !muted {
			t.Error("expected muted")
		}
	})

	t.Run("mutes are scoped to organization and context", func(t *testing.T) {
		for _, key := range []struct{ org, context string }{
			{"org-2", "default"},
			{"org-1", "other"},
		} {
			muted, err := m.IsMuted(ctx, key.org, key.context)
			if err != nil {
				t.Fatalf("IsMuted: %v", err)
			}
			if muted {
				t.Errorf("expected %s/%s not to be muted", key.org, key.context)
			}
		}
	})

	t.Run("muting again updates the existing mute", func(t *testing.T) {
		if err := m.SaveMute(ctx, "org-1", "default", 120); err != nil {
			t.Fatalf("SaveMute: %v", err)
		}

		count, err := m.DB.Collection("mutes").CountDocuments(ctx, map[string]any{"organization": "org-1", "context": "default"})
		if err != nil {
			t.Fatalf("CountDocuments: %v", err)
		}
		if count != 1 {
			t.Errorf("expected a single mute document, got %d", count)
		}
	})

	t.Run("an expired mute is not muted", func(t *testing.T) {
		if err := m.SaveMute(ctx, "org-1", "expired", -1); err != nil {
			t.Fatalf("SaveMute: %v", err)
		}

		muted, err := m.IsMuted(ctx, "org-1", "expired")
		if err != nil {
			t.Fatalf("IsMuted: %v", err)
		}
		if muted {
			t.Error("expected an expired mute not to mute")
		}
	})
}

func TestMongoBreaches(t *testing.T) {
	m := newTestMongo(t)
	ctx := context.Background()
	now := time.Now()

	t.Run("no breaches yet", func(t *testing.T) {
		breaches, err := m.GetBreaches(ctx, now.Add(-24*time.Hour), now)
		if err != nil {
			t.Fatalf("GetBreaches: %v", err)
		}
		if len(breaches) != 0 {
			t.Errorf("expected no breaches, got %d", len(breaches))
		}
	})

	saved := []models.BreachDetail{
		{Timestamp: now.Add(-3 * time.Hour), PIIType: "ssn", Context: "default", Org: "org-1", SourceID: "source-1", Count: 30, Average: 10, ZScore: 3.5},
		{Timestamp: now.Add(-1 * time.Hour), PIIType: "email", Context: "default", Org: "org-1", SourceID: "source-1", Count: 90, Average: 40},
		{Timestamp: now.Add(-30 * time.Hour), PIIType: "ssn", Context: "default", Org: "org-1", SourceID: "source-1", Count: 25, Average: 10},
	}
	for _, breach := range saved {
		if err := m.SaveBreach(ctx, breach); err != nil {
			t.Fatalf("SaveBreach: %v", err)
		}
	}

	t.Run("breaches come back newest first within the window", func(t *testing.T) {
		breaches, err := m.GetBreaches(ctx, now.Add(-24*time.Hour), now)
		if err != nil {
			t.Fatalf("GetBreaches: %v", err)
		}
		if len(breaches) != 2 {
			t.Fatalf("expected 2 breaches in the window, got %d", len(breaches))
		}
		if breaches[0].PIIType != "email" || breaches[1].PIIType != "ssn" {
			t.Errorf("expected newest first, got %s then %s", breaches[0].PIIType, breaches[1].PIIType)
		}
		if breaches[0].Count != 90 || !closeEnough(breaches[0].Average, 40) {
			t.Errorf("unexpected breach fields: %+v", breaches[0])
		}
		if !closeEnough(breaches[1].ZScore, 3.5) {
			t.Errorf("expected z-score 3.5, got %f", breaches[1].ZScore)
		}
	})

	t.Run("a wider window includes the older breach", func(t *testing.T) {
		breaches, err := m.GetBreaches(ctx, now.Add(-72*time.Hour), now)
		if err != nil {
			t.Fatalf("GetBreaches: %v", err)
		}
		if len(breaches) != 3 {
			t.Errorf("expected 3 breaches, got %d", len(breaches))
		}
	})

	t.Run("a window with no breaches is empty", func(t *testing.T) {
		breaches, err := m.GetBreaches(ctx, now.Add(-10*time.Minute), now)
		if err != nil {
			t.Fatalf("GetBreaches: %v", err)
		}
		if len(breaches) != 0 {
			t.Errorf("expected no breaches, got %d", len(breaches))
		}
	})
}

func TestMongoGetEntries(t *testing.T) {
	m := newTestMongo(t)
	ctx := context.Background()
	now := time.Now()

	saved := []models.PIIEntry{
		{Timestamp: now.Add(-2 * time.Hour), SourceID: "source-1", Organization: "org-1", Context: "default", PIITypes: map[string]int{"ssn": 20}},
		{Timestamp: now.Add(-1 * time.Hour), SourceID: "source-2", Organization: "org-1", Context: "billing", PIITypes: map[string]int{"email": 5}},
		{Timestamp: now.Add(-30 * time.Hour), SourceID: "source-1", Organization: "org-1", Context: "default", PIITypes: map[string]int{"ssn": 99}},
	}
	for _, entry := range saved {
		if err := m.Save(ctx, entry); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	collect := func(t *testing.T, start, end time.Time) []models.PIIEntry {
		t.Helper()

		entryChan, errChan := m.GetEntries(ctx, start, end)
		var entries []models.PIIEntry
		for entry := range entryChan {
			entries = append(entries, entry)
		}
		if err := <-errChan; err != nil {
			t.Fatalf("GetEntries: %v", err)
		}
		return entries
	}

	t.Run("entries stream oldest first within the window", func(t *testing.T) {
		entries := collect(t, now.Add(-24*time.Hour), now)
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries[0].SourceID != "source-1" || entries[1].SourceID != "source-2" {
			t.Errorf("expected oldest first, got %s then %s", entries[0].SourceID, entries[1].SourceID)
		}
		if entries[0].PIITypes["ssn"] != 20 {
			t.Errorf("expected ssn 20, got %d", entries[0].PIITypes["ssn"])
		}
		if entries[1].Context != "billing" {
			t.Errorf("expected context billing, got %s", entries[1].Context)
		}
	})

	t.Run("a wider window includes the older entry", func(t *testing.T) {
		if entries := collect(t, now.Add(-72*time.Hour), now); len(entries) != 3 {
			t.Errorf("expected 3 entries, got %d", len(entries))
		}
	})

	t.Run("a window with no entries yields nothing", func(t *testing.T) {
		if entries := collect(t, now.Add(-10*time.Minute), now); len(entries) != 0 {
			t.Errorf("expected no entries, got %d", len(entries))
		}
	})

	t.Run("a cancelled context reports an error", func(t *testing.T) {
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		entryChan, errChan := m.GetEntries(cancelledCtx, now.Add(-24*time.Hour), now)
		for range entryChan {
		}
		if err := <-errChan; err == nil {
			t.Error("expected an error from a cancelled context")
		}
	})
}

func TestMongoSaveStatsVersioning(t *testing.T) {
	m := newTestMongo(t)
	ctx := context.Background()

	t.Run("the first write inserts at version 1", func(t *testing.T) {
		if err := m.SaveStats(ctx, "source-1", "org-1", "default", "ssn", models.Stats{Count: 1, Mean: 10, Version: 0}); err != nil {
			t.Fatalf("SaveStats: %v", err)
		}

		stored, err := m.GetStats(ctx, "source-1", "org-1", "default", "ssn")
		if err != nil {
			t.Fatalf("GetStats: %v", err)
		}
		if stored.Version != 1 {
			t.Errorf("expected version 1, got %d", stored.Version)
		}
	})

	t.Run("a stale write is rejected and changes nothing", func(t *testing.T) {
		err := m.SaveStats(ctx, "source-1", "org-1", "default", "ssn", models.Stats{Count: 99, Mean: 99, Version: 0})
		if !errors.Is(err, ErrStatsConflict) {
			t.Fatalf("expected ErrStatsConflict, got %v", err)
		}

		stored, err := m.GetStats(ctx, "source-1", "org-1", "default", "ssn")
		if err != nil {
			t.Fatalf("GetStats: %v", err)
		}
		if stored.Count != 1 {
			t.Errorf("expected the rejected write to change nothing, got count %d", stored.Count)
		}

		count, err := m.DB.Collection("pii_stats").CountDocuments(ctx, bson.M{"source_id": "source-1"})
		if err != nil {
			t.Fatalf("CountDocuments: %v", err)
		}
		if count != 1 {
			t.Errorf("expected the rejected write not to insert, found %d documents", count)
		}
	})

	t.Run("a current write succeeds and advances the version", func(t *testing.T) {
		if err := m.SaveStats(ctx, "source-1", "org-1", "default", "ssn", models.Stats{Count: 2, Mean: 11, Version: 1}); err != nil {
			t.Fatalf("SaveStats: %v", err)
		}

		stored, err := m.GetStats(ctx, "source-1", "org-1", "default", "ssn")
		if err != nil {
			t.Fatalf("GetStats: %v", err)
		}
		if stored.Count != 2 || stored.Version != 2 {
			t.Errorf("expected count 2 at version 2, got count %d at version %d", stored.Count, stored.Version)
		}
	})

	// Stats written before versioning have no version field at all.
	t.Run("stats from an earlier release upgrade in place", func(t *testing.T) {
		if _, err := m.DB.Collection("pii_stats").InsertOne(ctx, bson.M{
			"source_id":          "legacy-source",
			"organization":       "org-1",
			"context":            "default",
			"pii_type":           "ssn",
			"count":              7,
			"mean":               12.5,
			"m2":                 3.0,
			"consecutive_normal": 1,
		}); err != nil {
			t.Fatalf("InsertOne: %v", err)
		}

		stored, err := m.GetStats(ctx, "legacy-source", "org-1", "default", "ssn")
		if err != nil {
			t.Fatalf("GetStats: %v", err)
		}
		if stored.Count != 7 || stored.Version != 0 {
			t.Fatalf("expected count 7 at version 0, got count %d at version %d", stored.Count, stored.Version)
		}

		if err := m.SaveStats(ctx, "legacy-source", "org-1", "default", "ssn", models.Stats{Count: 8, Mean: 12.6, Version: 0}); err != nil {
			t.Fatalf("SaveStats over a document with no version: %v", err)
		}

		stored, err = m.GetStats(ctx, "legacy-source", "org-1", "default", "ssn")
		if err != nil {
			t.Fatalf("GetStats: %v", err)
		}
		if stored.Count != 8 || stored.Version != 1 {
			t.Errorf("expected count 8 at version 1, got count %d at version %d", stored.Count, stored.Version)
		}

		count, err := m.DB.Collection("pii_stats").CountDocuments(ctx, bson.M{"source_id": "legacy-source"})
		if err != nil {
			t.Fatalf("CountDocuments: %v", err)
		}
		if count != 1 {
			t.Errorf("expected the upgrade to stay in one document, found %d", count)
		}
	})

	// Two writers creating the same series at once: the unique index makes one
	// of them a conflict rather than a second document.
	t.Run("concurrent first writes produce one document", func(t *testing.T) {
		const writers = 8

		var wg sync.WaitGroup
		results := make([]error, writers)
		for i := 0; i < writers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				results[i] = m.SaveStats(ctx, "race-source", "org-1", "default", "ssn", models.Stats{Count: 1, Mean: float64(i), Version: 0})
			}(i)
		}
		wg.Wait()

		succeeded := 0
		for i, err := range results {
			switch {
			case err == nil:
				succeeded++
			case errors.Is(err, ErrStatsConflict):
			default:
				t.Errorf("writer %d: unexpected error %v", i, err)
			}
		}
		if succeeded != 1 {
			t.Errorf("expected exactly one writer to succeed, got %d", succeeded)
		}

		count, err := m.DB.Collection("pii_stats").CountDocuments(ctx, bson.M{"source_id": "race-source"})
		if err != nil {
			t.Fatalf("CountDocuments: %v", err)
		}
		if count != 1 {
			t.Errorf("expected one stats document for the series, found %d", count)
		}
	})
}

type indexSpec struct {
	Name               string `bson:"name"`
	Unique             bool   `bson:"unique"`
	ExpireAfterSeconds *int32 `bson:"expireAfterSeconds"`
}

func listIndexes(t *testing.T, m *MongoDB, collection string) map[string]indexSpec {
	t.Helper()

	cursor, err := m.DB.Collection(collection).Indexes().List(context.Background())
	if err != nil {
		t.Fatalf("listing indexes on %s: %v", collection, err)
	}
	defer cursor.Close(context.Background())

	var specs []indexSpec
	if err := cursor.All(context.Background(), &specs); err != nil {
		t.Fatalf("reading indexes on %s: %v", collection, err)
	}

	byName := make(map[string]indexSpec, len(specs))
	for _, spec := range specs {
		byName[spec.Name] = spec
	}
	return byName
}

// Without these the hot paths fall back to collection scans, and GetBreaches
// sorts in memory until MongoDB refuses the query.
func TestMongoIndexes(t *testing.T) {
	m := newTestMongo(t)

	// The collections are created lazily, so write to each one first.
	ctx := context.Background()
	if err := m.SaveStats(ctx, "source-1", "org-1", "default", "ssn", models.Stats{Count: 1}); err != nil {
		t.Fatalf("SaveStats: %v", err)
	}
	if err := m.SaveMute(ctx, "org-1", "default", 5); err != nil {
		t.Fatalf("SaveMute: %v", err)
	}
	if err := m.SaveBreach(ctx, models.BreachDetail{Timestamp: time.Now(), PIIType: "ssn"}); err != nil {
		t.Fatalf("SaveBreach: %v", err)
	}
	if err := m.SaveMetric(ctx, time.Millisecond); err != nil {
		t.Fatalf("SaveMetric: %v", err)
	}

	// Connect ran before those writes, so create the indexes again. Doing so is
	// a no-op when they already exist, which is what makes startup creation safe.
	if err := setupIndexes(ctx, m.DB, defaultTestRetention); err != nil {
		t.Fatalf("setupIndexes: %v", err)
	}

	t.Run("pii_stats has a unique series key", func(t *testing.T) {
		index, ok := listIndexes(t, m, "pii_stats")["series_key"]
		if !ok {
			t.Fatal("expected a series_key index")
		}
		if !index.Unique {
			t.Error("expected series_key to be unique")
		}
	})

	t.Run("mutes has a unique key and a TTL", func(t *testing.T) {
		indexes := listIndexes(t, m, "mutes")

		key, ok := indexes["mute_key"]
		if !ok {
			t.Fatal("expected a mute_key index")
		}
		if !key.Unique {
			t.Error("expected mute_key to be unique")
		}

		expiry, ok := indexes["mute_expiry"]
		if !ok {
			t.Fatal("expected a mute_expiry index")
		}
		if expiry.ExpireAfterSeconds == nil || *expiry.ExpireAfterSeconds != 0 {
			t.Errorf("expected mute_expiry to expire at expires_at, got %v", expiry.ExpireAfterSeconds)
		}
	})

	t.Run("breaches and metrics are indexed by time", func(t *testing.T) {
		if _, ok := listIndexes(t, m, "breaches")["breach_timestamp"]; !ok {
			t.Error("expected a breach_timestamp index")
		}
		if _, ok := listIndexes(t, m, "metrics")["metric_timestamp"]; !ok {
			t.Error("expected a metric_timestamp index")
		}
	})

	t.Run("creating them again is a no-op", func(t *testing.T) {
		if err := setupIndexes(ctx, m.DB, defaultTestRetention); err != nil {
			t.Errorf("expected repeated index creation to succeed, got %v", err)
		}
	})
}

// GetBreaches must not fall back to a blocking in-memory sort, which MongoDB
// abandons once the sort passes its memory limit.
func TestMongoGetBreachesUsesTheIndex(t *testing.T) {
	m := newTestMongo(t)
	ctx := context.Background()
	now := time.Now()

	for i := 0; i < 200; i++ {
		if err := m.SaveBreach(ctx, models.BreachDetail{
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			PIIType:   "ssn",
			SourceID:  "source-1",
		}); err != nil {
			t.Fatalf("SaveBreach: %v", err)
		}
	}
	if err := setupIndexes(ctx, m.DB, defaultTestRetention); err != nil {
		t.Fatalf("setupIndexes: %v", err)
	}

	filter := bson.M{"timestamp": bson.M{"$gte": now.Add(-24 * time.Hour), "$lte": now}}

	var explained bson.M
	err := m.DB.RunCommand(ctx, bson.D{
		{Key: "explain", Value: bson.D{
			{Key: "find", Value: "breaches"},
			{Key: "filter", Value: filter},
			{Key: "sort", Value: bson.D{{Key: "timestamp", Value: -1}}},
		}},
		{Key: "verbosity", Value: "queryPlanner"},
	}).Decode(&explained)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}

	plan := fmt.Sprintf("%v", explained["queryPlanner"])
	if strings.Contains(plan, "COLLSCAN") {
		t.Errorf("expected an index scan, got a collection scan: %s", plan)
	}
	if strings.Contains(plan, "SORT") {
		t.Errorf("expected the index to provide the order, got a blocking sort: %s", plan)
	}

	// The query still returns what it should.
	breaches, err := m.GetBreaches(ctx, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("GetBreaches: %v", err)
	}
	if len(breaches) != 200 {
		t.Errorf("expected 200 breaches, got %d", len(breaches))
	}
}

func TestMongoMetricsRetention(t *testing.T) {
	m := newTestMongo(t)
	ctx := context.Background()

	if err := m.SaveMetric(ctx, time.Millisecond); err != nil {
		t.Fatalf("SaveMetric: %v", err)
	}

	t.Run("the default keeps samples for the configured window", func(t *testing.T) {
		if err := setupIndexes(ctx, m.DB, defaultTestRetention); err != nil {
			t.Fatalf("setupIndexes: %v", err)
		}

		index, ok := listIndexes(t, m, "metrics")["metric_timestamp"]
		if !ok {
			t.Fatal("expected a metric_timestamp index")
		}
		if index.ExpireAfterSeconds == nil {
			t.Fatal("expected the index to expire samples")
		}
		if want := int32(defaultTestRetention.Seconds()); *index.ExpireAfterSeconds != want {
			t.Errorf("expected %d seconds, got %d", want, *index.ExpireAfterSeconds)
		}
	})

	// MongoDB will not amend an index in place, so a changed retention has to
	// replace it.
	t.Run("a changed retention is applied", func(t *testing.T) {
		if err := setupIndexes(ctx, m.DB, 24*time.Hour); err != nil {
			t.Fatalf("setupIndexes: %v", err)
		}

		index := listIndexes(t, m, "metrics")["metric_timestamp"]
		if index.ExpireAfterSeconds == nil || *index.ExpireAfterSeconds != 86400 {
			t.Errorf("expected 86400 seconds, got %v", index.ExpireAfterSeconds)
		}

		indexes := listIndexes(t, m, "metrics")
		if len(indexes) != 2 {
			t.Errorf("expected the index to be replaced, not added to: %v", indexes)
		}
	})

	t.Run("zero keeps samples forever but stays indexed", func(t *testing.T) {
		if err := setupIndexes(ctx, m.DB, 0); err != nil {
			t.Fatalf("setupIndexes: %v", err)
		}

		index, ok := listIndexes(t, m, "metrics")["metric_timestamp"]
		if !ok {
			t.Fatal("expected the metric_timestamp index to remain")
		}
		if index.ExpireAfterSeconds != nil {
			t.Errorf("expected no expiry, got %d seconds", *index.ExpireAfterSeconds)
		}
	})

	t.Run("metrics still read back", func(t *testing.T) {
		count, _, err := m.GetMetrics(ctx, 24)
		if err != nil {
			t.Fatalf("GetMetrics: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 metric, got %d", count)
		}
	})
}

func TestMongoPing(t *testing.T) {
	m := newTestMongo(t)
	ctx := context.Background()

	if err := m.Ping(ctx); err != nil {
		t.Errorf("expected a reachable server, got %v", err)
	}

	// A connection of its own, so disconnecting it leaves the one the test
	// cleanup uses alone. A disconnected client is the state /health must
	// notice.
	second, err := Connect(testURI(mongoBaseURI, m.DB.Name()), defaultTestRetention)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := second.Ping(ctx); err != nil {
		t.Errorf("expected a reachable server, got %v", err)
	}

	if err := second.Client.Disconnect(ctx); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if err := second.Ping(ctx); err == nil {
		t.Error("expected an error after disconnecting")
	}
}

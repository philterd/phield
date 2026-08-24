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
	"log"
	"strings"
	"time"

	"github.com/philterd/phield/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoDB struct {
	Client *mongo.Client
	DB     *mongo.Database
}

func (m *MongoDB) Save(ctx context.Context, entry models.PIIEntry) error {
	coll := m.DB.Collection("pii_counts")
	_, err := coll.InsertOne(ctx, entry)
	return err
}

func (m *MongoDB) SaveMetric(ctx context.Context, latency time.Duration) error {
	coll := m.DB.Collection("metrics")
	metric := models.MetricEntry{
		Timestamp: time.Now(),
		Latency:   latency,
	}
	_, err := coll.InsertOne(ctx, metric)
	return err
}

func (m *MongoDB) GetMetrics(ctx context.Context, windowSizeHours int) (int, float64, error) {
	coll := m.DB.Collection("metrics")
	lookback := time.Now().Add(-time.Duration(windowSizeHours) * time.Hour)

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"timestamp": bson.M{"$gte": lookback},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":        nil,
			"count":      bson.M{"$sum": 1},
			"avgLatency": bson.M{"$avg": "$latency"},
		}}},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, 0, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		Count      int     `bson:"count"`
		AvgLatency float64 `bson:"avgLatency"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return 0, 0, err
	}

	if len(results) > 0 {
		// AvgLatency from MongoDB will be in nanoseconds (time.Duration)
		avgLatencySeconds := results[0].AvgLatency / float64(time.Second)
		return results[0].Count, avgLatencySeconds, nil
	}

	return 0, 0, nil
}

func (m *MongoDB) SaveMute(ctx context.Context, organization string, contextName string, minutes int) error {
	coll := m.DB.Collection("mutes")
	mute := models.Mute{
		Organization: organization,
		Context:      contextName,
		ExpiresAt:    time.Now().Add(time.Duration(minutes) * time.Minute),
	}
	opts := options.UpdateOne().SetUpsert(true)
	filter := bson.M{
		"organization": organization,
		"context":      contextName,
	}
	update := bson.M{"$set": mute}
	_, err := coll.UpdateOne(ctx, filter, update, opts)
	return err
}

func (m *MongoDB) IsMuted(ctx context.Context, organization string, contextName string) (bool, error) {
	coll := m.DB.Collection("mutes")
	filter := bson.M{
		"organization": organization,
		"context":      contextName,
		"expires_at":   bson.M{"$gt": time.Now()},
	}
	count, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (m *MongoDB) GetAverage(ctx context.Context, sourceID string, organization string, contextName string, piiType string, windowSizeHours int) (float64, error) {
	coll := m.DB.Collection("pii_counts")
	lookback := time.Now().Add(-time.Duration(windowSizeHours) * time.Hour)

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"source_id":    sourceID,
			"organization": organization,
			"context":      contextName,
			"timestamp":    bson.M{"$gte": lookback},
		}}},
		{{Key: "$project", Value: bson.M{
			"count": bson.M{"$ifNull": []any{fmt.Sprintf("$pii_types.%s", piiType), 0}},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":      nil,
			"avgCount": bson.M{"$avg": "$count"},
		}}},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		AvgCount float64 `bson:"avgCount"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return 0, err
	}

	if len(results) > 0 {
		return results[0].AvgCount, nil
	}

	return 0, nil
}

func (m *MongoDB) GetStats(ctx context.Context, sourceID string, organization string, contextName string, piiType string) (models.Stats, error) {
	coll := m.DB.Collection("pii_stats")
	filter := bson.M{
		"source_id":    sourceID,
		"organization": organization,
		"context":      contextName,
		"pii_type":     piiType,
	}
	var stats models.Stats
	err := coll.FindOne(ctx, filter).Decode(&stats)
	if err == mongo.ErrNoDocuments {
		return models.Stats{}, nil
	}
	return stats, err
}

func (m *MongoDB) SaveStats(ctx context.Context, sourceID string, organization string, contextName string, piiType string, stats models.Stats) error {
	coll := m.DB.Collection("pii_stats")
	filter := bson.M{
		"source_id":    sourceID,
		"organization": organization,
		"context":      contextName,
		"pii_type":     piiType,
	}

	if stats.Version == 0 {
		// Stats written before versioning carry no version field, which an
		// equality filter on 0 would not match. $in with nil matches both.
		filter["version"] = bson.M{"$in": bson.A{0, nil}}
	} else {
		filter["version"] = stats.Version
	}

	update := bson.M{"$set": bson.M{
		"count":              stats.Count,
		"mean":               stats.Mean,
		"m2":                 stats.M2,
		"last_alert_time":    stats.LastAlertTime,
		"consecutive_normal": stats.ConsecutiveNormal,
		"version":            stats.Version + 1,
	}}

	// Only the first write of a series may insert. Later writes must match an
	// existing version, and the unique index on the key fields turns two
	// concurrent first writes into a duplicate key error, which is the same
	// conflict as a stale version.
	opts := options.UpdateOne().SetUpsert(stats.Version == 0)

	res, err := coll.UpdateOne(ctx, filter, update, opts)
	if mongo.IsDuplicateKeyError(err) {
		return ErrStatsConflict
	}
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 && res.UpsertedCount == 0 {
		return ErrStatsConflict
	}

	return nil
}

// Connect opens a connection and prepares the collections. metricsRetention is
// how long /ingest latency samples are kept; zero keeps them forever.
func Connect(uri string, metricsRetention time.Duration) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	dbName := extractDBName(uri)
	db := client.Database(dbName)

	// Ensure Time-Series collection exists
	err = setupTimeSeries(ctx, db)
	if err != nil {
		return nil, err
	}

	if err := setupIndexes(ctx, db, metricsRetention); err != nil {
		// Phield still runs without them, but queries fall back to collection
		// scans, and without the unique indexes two writers creating the same
		// series or mute at once can produce a duplicate document. A unique
		// index also fails to build if duplicates are already there.
		log.Printf("WARNING: could not create every index: %v", err)
		log.Printf("WARNING: check for duplicate pii_stats or mutes documents, remove them, and restart")
	}

	return &MongoDB{
		Client: client,
		DB:     db,
	}, nil
}

func extractDBName(uri string) string {
	// Simple extraction of database name from URI as v2 doesn't expose ConnString easily
	// Format: mongodb://[username:password@]host1[:port1][,...hostN[:portN]][/[defaultauthdb][?options]]
	dbName := "phield" // default
	parts := strings.Split(uri, "/")
	if len(parts) > 3 {
		lastPart := parts[len(parts)-1]
		// Remove query parameters if present
		if idx := strings.Index(lastPart, "?"); idx != -1 {
			lastPart = lastPart[:idx]
		}
		if lastPart != "" {
			dbName = lastPart
		}
	}
	return dbName
}

func setupTimeSeries(ctx context.Context, db *mongo.Database) error {
	collName := "pii_counts"

	// Check if collection exists
	names, err := db.ListCollectionNames(ctx, bson.M{"name": collName})
	if err != nil {
		return err
	}

	if len(names) == 0 {
		tsOptions := options.CreateCollection().
			SetTimeSeriesOptions(options.TimeSeries().
				SetTimeField("timestamp").
				SetMetaField("source_id").
				SetGranularity("minutes"))

		err := db.CreateCollection(ctx, collName, tsOptions)
		if err != nil {
			return fmt.Errorf("failed to create time-series collection: %w", err)
		}
		log.Printf("Created time-series collection: %s", collName)
	}

	return nil
}

// setupIndexes creates the indexes the queries above depend on. Creating an
// index that already exists is a no-op, so this runs on every start and needs no
// separate migration step. On a collection that is already large, the build
// costs time and I/O once.
//
// pii_counts needs nothing here: a time-series collection is indexed on its time
// and metadata fields when it is created.
func setupIndexes(ctx context.Context, db *mongo.Database, metricsRetention time.Duration) error {
	// GetMetrics only ever reads a recent window, but the collection gains a
	// document per ingest request, so the same index expires old samples.
	metricsIndex := options.Index().SetName("metric_timestamp")
	if metricsRetention > 0 {
		metricsIndex = metricsIndex.SetExpireAfterSeconds(int32(metricsRetention.Seconds()))
	}

	indexes := []struct {
		collection string
		name       string
		model      mongo.IndexModel
	}{
		{
			// One stats document per series. This is what makes the versioned
			// write in SaveStats safe against two writers creating the same
			// series at once, and it is the index GetStats and SaveStats look
			// the series up by.
			collection: "pii_stats",
			name:       "series_key",
			model: mongo.IndexModel{
				Keys: bson.D{
					{Key: "source_id", Value: 1},
					{Key: "organization", Value: 1},
					{Key: "context", Value: 1},
					{Key: "pii_type", Value: 1},
				},
				Options: options.Index().SetName("series_key").SetUnique(true),
			},
		},
		{
			// IsMuted runs on every PII type of every ingest, and SaveMute
			// upserts against the same key.
			collection: "mutes",
			name:       "mute_key",
			model: mongo.IndexModel{
				Keys: bson.D{
					{Key: "organization", Value: 1},
					{Key: "context", Value: 1},
				},
				Options: options.Index().SetName("mute_key").SetUnique(true),
			},
		},
		{
			// IsMuted already ignores an expired mute. This clears them out so
			// the collection does not grow without limit.
			collection: "mutes",
			name:       "mute_expiry",
			model: mongo.IndexModel{
				Keys:    bson.D{{Key: "expires_at", Value: 1}},
				Options: options.Index().SetName("mute_expiry").SetExpireAfterSeconds(0),
			},
		},
		{
			// GetBreaches filters on a time range and sorts by time. Without
			// this it sorts in memory, which MongoDB gives up on once the sort
			// passes its memory limit.
			collection: "breaches",
			name:       "breach_timestamp",
			model: mongo.IndexModel{
				Keys:    bson.D{{Key: "timestamp", Value: -1}},
				Options: options.Index().SetName("breach_timestamp"),
			},
		},
		{
			collection: "metrics",
			name:       "metric_timestamp",
			model: mongo.IndexModel{
				Keys:    bson.D{{Key: "timestamp", Value: -1}},
				Options: metricsIndex,
			},
		},
	}

	var errs []error
	for _, index := range indexes {
		if err := createIndex(ctx, db.Collection(index.collection), index.name, index.model); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", index.collection, err))
		}
	}

	return errors.Join(errs...)
}

// MongoDB error codes for an index that already exists under the same name but
// with different options or a different key.
const (
	indexOptionsConflict  = 85
	indexKeySpecsConflict = 86
)

// createIndex creates an index, replacing one that exists under the same name
// with different settings. That happens when PHIELD_METRICS_RETENTION_DAYS
// changes: MongoDB will not amend an index in place, so it is rebuilt, which
// costs time and I/O once on a collection that is already large.
func createIndex(ctx context.Context, coll *mongo.Collection, name string, model mongo.IndexModel) error {
	_, err := coll.Indexes().CreateOne(ctx, model)
	if err == nil {
		return nil
	}

	var serverErr mongo.ServerError
	if !errors.As(err, &serverErr) || !(serverErr.HasErrorCode(indexOptionsConflict) || serverErr.HasErrorCode(indexKeySpecsConflict)) {
		return err
	}

	if err := coll.Indexes().DropOne(ctx, name); err != nil {
		return fmt.Errorf("replacing index %s: %w", name, err)
	}

	_, err = coll.Indexes().CreateOne(ctx, model)
	return err
}

func (m *MongoDB) SaveBreach(ctx context.Context, breach models.BreachDetail) error {
	coll := m.DB.Collection("breaches")
	_, err := coll.InsertOne(ctx, breach)
	return err
}

func (m *MongoDB) GetBreaches(ctx context.Context, startTime time.Time, endTime time.Time) ([]models.BreachDetail, error) {
	coll := m.DB.Collection("breaches")
	filter := bson.M{
		"timestamp": bson.M{
			"$gte": startTime,
			"$lte": endTime,
		},
	}
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})

	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []models.BreachDetail
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (m *MongoDB) GetAllStats(ctx context.Context) ([]models.StatsEntry, error) {
	coll := m.DB.Collection("pii_stats")
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []models.StatsEntry
	for cursor.Next(ctx) {
		var doc struct {
			SourceID     string  `bson:"source_id"`
			Organization string  `bson:"organization"`
			Context      string  `bson:"context"`
			PIIType      string  `bson:"pii_type"`
			Count        int     `bson:"count"`
			Mean         float64 `bson:"mean"`
			M2           float64 `bson:"m2"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		results = append(results, models.StatsEntry{
			SourceID:     doc.SourceID,
			Organization: doc.Organization,
			Context:      doc.Context,
			PIIType:      doc.PIIType,
			Stats: models.Stats{
				Count: doc.Count,
				Mean:  doc.Mean,
				M2:    doc.M2,
			},
		})
	}
	return results, nil
}

func (m *MongoDB) GetEntries(ctx context.Context, startTime time.Time, endTime time.Time) (<-chan models.PIIEntry, <-chan error) {
	entryChan := make(chan models.PIIEntry)
	errChan := make(chan error, 1)

	go func() {
		defer close(entryChan)
		defer close(errChan)

		coll := m.DB.Collection("pii_counts")
		filter := bson.M{
			"timestamp": bson.M{
				"$gte": startTime,
				"$lte": endTime,
			},
		}
		opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})

		cursor, err := coll.Find(ctx, filter, opts)
		if err != nil {
			errChan <- err
			return
		}
		defer cursor.Close(ctx)

		for cursor.Next(ctx) {
			var entry models.PIIEntry
			if err := cursor.Decode(&entry); err != nil {
				errChan <- err
				return
			}
			entryChan <- entry
		}

		if err := cursor.Err(); err != nil {
			errChan <- err
		}
	}()

	return entryChan, errChan
}

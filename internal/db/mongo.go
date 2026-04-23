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
	update := bson.M{"$set": bson.M{
		"count":              stats.Count,
		"mean":               stats.Mean,
		"m2":                 stats.M2,
		"last_alert_time":    stats.LastAlertTime,
		"consecutive_normal": stats.ConsecutiveNormal,
	}}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := coll.UpdateOne(ctx, filter, update, opts)
	return err
}

func Connect(uri string) (*MongoDB, error) {
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

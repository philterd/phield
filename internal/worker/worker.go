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

package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/philterd/phield/internal/models"
	"github.com/philterd/phield/internal/notifier"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Worker struct {
	db             *mongo.Database
	ingestChan     chan models.PIIEntry
	alertThreshold float64 // e.g. 0.2 for 20%
	notifier       notifier.Notifier
}

func NewWorker(db *mongo.Database, ingestChan chan models.PIIEntry, threshold float64, n notifier.Notifier) *Worker {
	return &Worker{
		db:             db,
		ingestChan:     ingestChan,
		alertThreshold: threshold,
		notifier:       n,
	}
}

func (w *Worker) Start(ctx context.Context) {
	log.Println("Background trend analysis worker started")
	for {
		select {
		case entry := <-w.ingestChan:
			w.processEntry(ctx, entry)
		case <-ctx.Done():
			log.Println("Background worker shutting down")
			return
		}
	}
}

func (w *Worker) processEntry(ctx context.Context, entry models.PIIEntry) {
	coll := w.db.Collection("pii_counts")

	// Persist the entry
	_, err := coll.InsertOne(ctx, entry)
	if err != nil {
		log.Printf("Error persisting entry: %v", err)
		return
	}

	// For each PII type, calculate moving average and check for breach
	for piiType, currentCount := range entry.PIITypes {
		w.analyzeTrend(ctx, entry.SourceID, piiType, currentCount)
	}
}

func (w *Worker) analyzeTrend(ctx context.Context, sourceID string, piiType string, currentCount int) {
	coll := w.db.Collection("pii_counts")

	// Look back 24 hours for moving average
	lookback := time.Now().Add(-24 * time.Hour)

	// MongoDB Aggregation to calculate average for this PII type and source
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"source_id": sourceID,
			"timestamp": bson.M{"$gte": lookback},
		}}},
		{{Key: "$project", Value: bson.M{
			"count": bson.M{"$ifNull": []interface{}{fmt.Sprintf("$pii_types.%s", piiType), 0}},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":      nil,
			"avgCount": bson.M{"$avg": "$count"},
		}}},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("Error aggregating trend: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var results []struct {
		AvgCount float64 `bson:"avgCount"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("Error decoding trend results: %v", err)
		return
	}

	if len(results) > 0 {
		avg := results[0].AvgCount
		if avg > 0 {
			diff := (float64(currentCount) - avg) / avg
			if diff > w.alertThreshold {
				msg := fmt.Sprintf("[TREND BREACH] Source: %s, PII Type: %s, Current: %d, Avg: %.2f, Increase: %.2f%%",
					sourceID, piiType, currentCount, avg, diff*100)
				log.Print(msg)

				if w.notifier != nil {
					if err := w.notifier.Notify(ctx, msg); err != nil {
						log.Printf("Error sending notification: %v", err)
					}
				}
			}
		}
	}

}

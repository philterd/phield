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

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoDB struct {
	Client *mongo.Client
	DB     *mongo.Database
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

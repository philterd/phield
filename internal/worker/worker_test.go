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
	"testing"
	"time"

	"github.com/philterd/phield/internal/models"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestTrendBreach(t *testing.T) {
	// This test requires a running MongoDB.
	// If it fails to connect, we skip it.
	uri := "mongodb://localhost:27017"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	err = client.Ping(ctx, nil)
	if err != nil {
		t.Skip("MongoDB not pingable, skipping integration test")
		return
	}
	defer client.Disconnect(ctx)

	db := client.Database("phield_test")
	defer db.Drop(ctx)

	ingestChan := make(chan models.PIIEntry, 10)
	w := NewWorker(db, ingestChan, 0.2, nil) // 20% threshold, no notifier

	// Helper to send entry
	sendEntry := func(count int) {
		entry := models.PIIEntry{
			Timestamp: time.Now(),
			SourceID:  "test-source",
			PIITypes:  map[string]int{"credit-card": count},
		}
		w.processEntry(ctx, entry)
	}

	// 1. Establish baseline
	for i := 0; i < 5; i++ {
		sendEntry(10)
	}

	// 2. Trigger breach (30 is much higher than average of 10)
	// We'll capture logs if possible, but for now we'll just ensure it doesn't crash
	// and we manually check output or add a callback if we wanted to be more robust.
	fmt.Println("Expecting trend breach log below:")
	sendEntry(30)
}

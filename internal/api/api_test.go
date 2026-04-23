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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/philterd/phield/internal/db"
	"github.com/philterd/phield/internal/models"
)

func TestHandleIngest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := db.NewInMemoryStorage()

	t.Run("Successful ingest", func(t *testing.T) {
		a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil)
		r := gin.Default()
		a.RegisterRoutes(r)

		reqBody := models.IngestRequest{
			Timestamp:    time.Now(),
			SourceID:     "test-source",
			Organization: "org-1",
			Context:      "test-context",
			PIITypes:     map[string]int{"credit-card": 10},
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/ingest", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("expected status 202, got %d", w.Code)
		}

		// Verify storage has the entry
		entriesChan, _ := storage.GetEntries(context.Background(), time.Now().Add(-1*time.Minute), time.Now().Add(1*time.Minute))
		found := false
		for entry := range entriesChan {
			if entry.SourceID == "test-source" && entry.Organization == "org-1" && entry.Context == "test-context" {
				found = true
				break
			}
		}
		if !found {
			t.Error("entry not found in storage")
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil)
		r := gin.Default()
		a.RegisterRoutes(r)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/ingest", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("Default organization", func(t *testing.T) {
		a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil)
		r := gin.Default()
		a.RegisterRoutes(r)

		reqBody := models.IngestRequest{
			SourceID: "test-source-default-org",
			Context:  "test-context",
			PIITypes: map[string]int{"credit-card": 10},
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/ingest", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("expected status 202, got %d", w.Code)
		}

		// Verify storage has the entry with default org
		entriesChan, _ := storage.GetEntries(context.Background(), time.Now().Add(-1*time.Minute), time.Now().Add(1*time.Minute))
		found := false
		for entry := range entriesChan {
			if entry.SourceID == "test-source-default-org" {
				if entry.Organization != "default" {
					t.Errorf("expected organization default, got %s", entry.Organization)
				}
				found = true
				break
			}
		}
		if !found {
			t.Error("entry not found in storage")
		}
	})

	t.Run("Health check", func(t *testing.T) {
		a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil)
		r := gin.Default()
		a.RegisterRoutes(r)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		if err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if response["status"] != "ok" {
			t.Errorf("expected status ok, got %s", response["status"])
		}
	})

	t.Run("Metrics", func(t *testing.T) {
		testStorage := db.NewInMemoryStorage()
		a := NewAPI(testStorage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil)
		r := gin.Default()
		a.RegisterRoutes(r)

		reqBody := models.IngestRequest{
			SourceID: "test-source",
			PIITypes: map[string]int{"credit-card": 10},
		}
		body, _ := json.Marshal(reqBody)
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest("POST", "/ingest", bytes.NewBuffer(body))
		req1.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w1, req1)

		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/metrics", nil)
		r.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w2.Code)
		}

		bodyStr := w2.Body.String()
		if !strings.Contains(bodyStr, "phield_ingest_requests_total_24h 1") {
			t.Errorf("expected metric phield_ingest_requests_total_24h 1, got %s", bodyStr)
		}
	})

	t.Run("Mute endpoint", func(t *testing.T) {
		a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil)
		r := gin.Default()
		a.RegisterRoutes(r)

		muteReq := models.MuteRequest{
			Organization: "org-1",
			Context:      "test-context",
			Minutes:      10,
		}
		body, _ := json.Marshal(muteReq)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/mute", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response map[string]any
		json.Unmarshal(w.Body.Bytes(), &response)
		if response["status"] != "muted" {
			t.Errorf("expected status muted, got %v", response["status"])
		}

		muted, _ := storage.IsMuted(context.Background(), "org-1", "test-context")
		if !muted {
			t.Error("expected context to be muted in storage")
		}
	})

	t.Run("Handle replay", func(t *testing.T) {
		testStorage := db.NewInMemoryStorage()
		now := time.Now()

		entries := []models.PIIEntry{
			{
				Timestamp: now.Add(-2 * time.Hour),
				SourceID:  "source-1",
				PIITypes:  map[string]int{"ssn": 10},
			},
			{
				Timestamp: now.Add(-1 * time.Hour),
				SourceID:  "source-1",
				PIITypes:  map[string]int{"ssn": 30}, // Breach!
			},
		}
		for _, e := range entries {
			_ = testStorage.Save(context.Background(), e)
		}

		a := NewAPI(testStorage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil)
		r := gin.Default()
		a.RegisterRoutes(r)

		reqBody := models.ReplayRequest{
			StartTime:     now.Add(-3 * time.Hour),
			EndTime:       now,
			TestThreshold: 0.2,
		}
		body, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/replay", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp models.ReplayResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.TotalPointsProcessed != 2 {
			t.Errorf("expected 2 points processed, got %d", resp.TotalPointsProcessed)
		}
		if resp.VirtualBreachesDetected != 1 {
			t.Errorf("expected 1 virtual breach, got %d", resp.VirtualBreachesDetected)
		}
	})
}

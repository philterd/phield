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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/philterd/phield/internal/auth"
	"github.com/philterd/phield/internal/db"
	"github.com/philterd/phield/internal/models"
)

func TestHandleIngest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := db.NewInMemoryStorage()

	t.Run("Successful ingest", func(t *testing.T) {
		a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")
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
		a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")
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
		a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")
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
		a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")
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

		if response["status"] != "UP" || response["applicationVersion"] != "test-version" {
			t.Errorf("unexpected health response: %v", response)
		}
	})

	t.Run("Metrics", func(t *testing.T) {
		testStorage := db.NewInMemoryStorage()
		a := NewAPI(testStorage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")
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
		a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")
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

		a := NewAPI(testStorage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")
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

func TestRegisterRoutesAppliesMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	a := NewAPI(db.NewInMemoryStorage(), 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")
	r := gin.New()
	a.RegisterRoutes(r, auth.BearerToken("s3cret"))

	do := func(method, path, authHeader string) int {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(method, path, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		r.ServeHTTP(w, req)
		return w.Code
	}

	// Endpoints that change what Phield holds need the key.
	protected := []struct {
		method string
		path   string
	}{
		{"POST", "/ingest"},
		{"POST", "/mute"},
		{"POST", "/replay"},
	}

	for _, route := range protected {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			if code := do(route.method, route.path, ""); code != http.StatusUnauthorized {
				t.Errorf("without a key: expected 401, got %d", code)
			}
			if code := do(route.method, route.path, "Bearer s3cret"); code == http.StatusUnauthorized {
				t.Error("with a key: expected the request to be authenticated, got 401")
			}
		})
	}

	// Probes and scrapes need no credential.
	for _, path := range []string{"/health", "/metrics"} {
		t.Run("GET "+path, func(t *testing.T) {
			if code := do("GET", path, ""); code != http.StatusOK {
				t.Errorf("without a key: expected 200, got %d", code)
			}
		})
	}
}

func TestHandleIngestValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string
	}{
		{
			name:     "valid request",
			body:     `{"source_id":"source-1","pii_types":{"ssn":3}}`,
			wantCode: http.StatusAccepted,
		},
		{
			name:     "malformed JSON",
			body:     `{"source_id":`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "wrong type for pii_types",
			body:     `{"source_id":"source-1","pii_types":"ssn"}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "non-integer count",
			body:     `{"source_id":"source-1","pii_types":{"ssn":"three"}}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "empty body",
			body:     `{}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "source_id is required",
		},
		{
			name:     "missing source_id",
			body:     `{"pii_types":{"ssn":3}}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "source_id is required",
		},
		{
			name:     "missing pii_types",
			body:     `{"source_id":"source-1"}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "pii_types is required",
		},
		{
			name:     "empty pii_types",
			body:     `{"source_id":"source-1","pii_types":{}}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "pii_types is required",
		},
		{
			name:     "empty PII type name",
			body:     `{"source_id":"source-1","pii_types":{"":3}}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "empty PII type name",
		},
		{
			name:     "PII type name containing a dot",
			body:     `{"source_id":"source-1","pii_types":{"credit.card":3}}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "must not contain",
		},
		{
			name:     "negative count",
			body:     `{"source_id":"source-1","pii_types":{"ssn":-1}}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "must not be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := db.NewInMemoryStorage()
			a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")
			r := gin.New()
			a.RegisterRoutes(r)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/ingest", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Fatalf("expected status %d, got %d (%s)", tt.wantCode, w.Code, w.Body.String())
			}

			if tt.wantErr != "" && !strings.Contains(w.Body.String(), tt.wantErr) {
				t.Errorf("expected the response to mention %q, got %s", tt.wantErr, w.Body.String())
			}

			// A rejected request must not reach storage.
			if tt.wantCode == http.StatusBadRequest {
				entries := drainEntries(t, storage)
				if len(entries) != 0 {
					t.Errorf("expected nothing to be stored, got %d entries", len(entries))
				}
			}
		})
	}
}

func TestProcessIngestValidatesForKafka(t *testing.T) {
	storage := db.NewInMemoryStorage()
	a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")

	// The Kafka consumer calls ProcessIngest directly, bypassing the handler.
	err := a.ProcessIngest(context.Background(), models.IngestRequest{PIITypes: map[string]int{"ssn": 3}})
	if err == nil {
		t.Fatal("expected an error for a request without a source_id")
	}

	if entries := drainEntries(t, storage); len(entries) != 0 {
		t.Errorf("expected nothing to be stored, got %d entries", len(entries))
	}
}

func drainEntries(t *testing.T, storage db.Storage) []models.PIIEntry {
	t.Helper()

	entryChan, errChan := storage.GetEntries(context.Background(), time.Now().Add(-24*time.Hour), time.Now().Add(time.Hour))

	var entries []models.PIIEntry
	for entry := range entryChan {
		entries = append(entries, entry)
	}
	if err := <-errChan; err != nil {
		t.Fatalf("GetEntries: %v", err)
	}
	return entries
}

func TestLimitRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newRouter := func(maxBytes int64) *gin.Engine {
		a := NewAPI(db.NewInMemoryStorage(), 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")
		r := gin.New()
		r.Use(LimitRequestBody(maxBytes))
		a.RegisterRoutes(r)
		return r
	}

	post := func(r *gin.Engine, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/ingest", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}

	// A body with enough PII types to exceed a small limit.
	large := `{"source_id":"s1","pii_types":{`
	for i := 0; i < 200; i++ {
		if i > 0 {
			large += ","
		}
		large += fmt.Sprintf(`"type-%d":%d`, i, i)
	}
	large += "}}"

	small := `{"source_id":"s1","pii_types":{"ssn":1}}`

	t.Run("an oversized body is refused", func(t *testing.T) {
		w := post(newRouter(256), large)

		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected status 413, got %d (%s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "must not exceed 256 bytes") {
			t.Errorf("expected the limit in the response, got %s", w.Body.String())
		}
	})

	t.Run("a body within the limit is accepted", func(t *testing.T) {
		if w := post(newRouter(256), small); w.Code != http.StatusAccepted {
			t.Errorf("expected status 202, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("a large body is accepted under a large limit", func(t *testing.T) {
		if w := post(newRouter(1048576), large); w.Code != http.StatusAccepted {
			t.Errorf("expected status 202, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("zero disables the limit", func(t *testing.T) {
		if w := post(newRouter(0), large); w.Code != http.StatusAccepted {
			t.Errorf("expected status 202, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("malformed JSON is still a bad request", func(t *testing.T) {
		if w := post(newRouter(1048576), `{"source_id":`); w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

// failingPingStorage reports the storage as unreachable.
type failingPingStorage struct {
	db.Storage
	err error
}

func (s *failingPingStorage) Ping(ctx context.Context) error {
	return s.err
}

func TestHandleHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	get := func(storage db.Storage) *httptest.ResponseRecorder {
		a := NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")
		r := gin.New()
		a.RegisterRoutes(r)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		r.ServeHTTP(w, req)
		return w
	}

	// In-memory storage is part of the process, so there is nothing to reach.
	t.Run("healthy with in-memory storage", func(t *testing.T) {
		w := get(db.NewInMemoryStorage())

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), `"status":"UP"`) || !strings.Contains(w.Body.String(), `"applicationVersion":"test-version"`) {
			t.Errorf("unexpected body: %s", w.Body.String())
		}
	})

	t.Run("unhealthy when the storage cannot be reached", func(t *testing.T) {
		w := get(&failingPingStorage{
			Storage: db.NewInMemoryStorage(),
			err:     errors.New("no reachable servers"),
		})

		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status 503, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), `"storage":"unreachable"`) {
			t.Errorf("unexpected body: %s", w.Body.String())
		}
	})
}

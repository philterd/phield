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

package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/philterd/phield/internal/db"
	"github.com/philterd/phield/internal/models"
)

func setupTestDashboard() (*gin.Engine, db.Storage) {
	gin.SetMode(gin.TestMode)
	storage := db.NewInMemoryStorage()
	r := gin.New()
	d := New(storage)
	d.RegisterRoutes(r)
	return r, storage
}

func TestSummaryEndpoint(t *testing.T) {
	r, storage := setupTestDashboard()

	storage.Save(context.Background(), models.PIIEntry{
		Timestamp:    time.Now(),
		SourceID:     "app-1",
		Organization: "org-1",
		Context:      "production",
		PIITypes:     map[string]int{"email": 10, "ssn": 5},
	})
	storage.Save(context.Background(), models.PIIEntry{
		Timestamp:    time.Now(),
		SourceID:     "app-2",
		Organization: "org-1",
		Context:      "staging",
		PIITypes:     map[string]int{"credit-card": 3},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/summary?hours=24", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	if int(resp["total_entries"].(float64)) != 2 {
		t.Errorf("expected 2 entries, got %v", resp["total_entries"])
	}
	if int(resp["unique_sources"].(float64)) != 2 {
		t.Errorf("expected 2 sources, got %v", resp["unique_sources"])
	}
	if int(resp["unique_types"].(float64)) != 3 {
		t.Errorf("expected 3 types, got %v", resp["unique_types"])
	}
}

func TestAlertsEndpoint(t *testing.T) {
	r, storage := setupTestDashboard()

	storage.SaveBreach(context.Background(), models.BreachDetail{
		Timestamp: time.Now(),
		PIIType:   "email",
		Context:   "production",
		Org:       "org-1",
		SourceID:  "app-1",
		Count:     100,
		Average:   20,
		ZScore:    4.5,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/alerts?hours=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	alerts := resp["alerts"].([]any)
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}
}

func TestFlowsEndpoint(t *testing.T) {
	r, storage := setupTestDashboard()

	storage.Save(context.Background(), models.PIIEntry{
		Timestamp:    time.Now(),
		SourceID:     "app-1",
		Organization: "org-1",
		Context:      "production",
		PIITypes:     map[string]int{"email": 10},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/flows?hours=24", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	flows := resp["flows"].([]any)
	if len(flows) != 1 {
		t.Errorf("expected 1 flow, got %d", len(flows))
	}
	flow := flows[0].(map[string]any)
	if flow["source"] != "app-1" {
		t.Errorf("expected source app-1, got %v", flow["source"])
	}
}

func TestTrendsEndpoint(t *testing.T) {
	r, storage := setupTestDashboard()

	storage.SaveStats(context.Background(), "app-1", "org-1", "production", "email", models.Stats{
		Count: 50,
		Mean:  20.0,
		M2:    100.0,
	})
	storage.Save(context.Background(), models.PIIEntry{
		Timestamp:    time.Now(),
		SourceID:     "app-1",
		Organization: "org-1",
		Context:      "production",
		PIITypes:     map[string]int{"email": 30},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/trends?hours=24", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	trends := resp["trends"].([]any)
	if len(trends) != 1 {
		t.Errorf("expected 1 trend, got %d", len(trends))
	}
}

func TestDashboardUI(t *testing.T) {
	r, _ := setupTestDashboard()

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content type, got %s", w.Header().Get("Content-Type"))
	}
}

func TestEmptyAlertsReturnsEmptyArray(t *testing.T) {
	r, _ := setupTestDashboard()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/alerts?hours=24", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	alerts := resp["alerts"].([]any)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

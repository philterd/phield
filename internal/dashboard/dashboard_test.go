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
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/philterd/phield/internal/api"
	"github.com/philterd/phield/internal/auth"
	"github.com/philterd/phield/internal/db"
	"github.com/philterd/phield/internal/models"
)

func setupTestDashboard() (*gin.Engine, db.Storage) {
	gin.SetMode(gin.TestMode)
	storage := db.NewInMemoryStorage()
	r := gin.New()
	d := New(storage, false)
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

// The API key keeps bad data out. The dashboard only reads aggregate counts, so
// it stays reachable when one is configured.
func TestDashboardIsNotBehindTheAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := db.NewInMemoryStorage()

	// Wired the way main.go wires it: the API is authenticated, the dashboard is not.
	r := gin.New()
	a := api.NewAPI(storage, 0.2, "percentage_delta", 24, 3.0, 20, 60, nil, "test-version")
	a.RegisterRoutes(r, auth.BearerToken("s3cret"))
	New(storage, false).RegisterRoutes(r)

	get := func(path string) int {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", path, nil)
		r.ServeHTTP(w, req)
		return w.Code
	}

	for _, path := range []string{
		"/dashboard",
		"/api/dashboard/summary",
		"/api/dashboard/alerts",
		"/api/dashboard/entities",
		"/api/dashboard/flows",
		"/api/dashboard/trends",
	} {
		if code := get(path); code != http.StatusOK {
			t.Errorf("%s: expected 200 without an API key, got %d", path, code)
		}
	}

	// The write endpoints stay protected.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ingest", strings.NewReader(`{"source_id":"s1","pii_types":{"ssn":1}}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("/ingest: expected 401 without an API key, got %d", w.Code)
	}
}

func TestRootRedirectsToDashboard(t *testing.T) {
	r, _ := setupTestDashboard()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", w.Code)
	}
	if location := w.Header().Get("Location"); location != "/dashboard" {
		t.Errorf("expected a redirect to /dashboard, got %q", location)
	}
}

func TestDashboardFooter(t *testing.T) {
	r, _ := setupTestDashboard()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	for _, want := range []string{
		"Copyright 2026 Philterd, LLC",
		`href="https://www.philterd.ai"`,
		`href="https://philterd.ai/phield/"`,
		`href="https://github.com/philterd/phield"`,
		`href="https://philterd.ai/support/"`,
	} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("expected the page to contain %q", want)
		}
	}
}

func TestStorageBanner(t *testing.T) {
	gin.SetMode(gin.TestMode)

	serve := func(ephemeral bool) string {
		r := gin.New()
		New(db.NewInMemoryStorage(), ephemeral).RegisterRoutes(r)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/dashboard", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		return w.Body.String()
	}

	t.Run("shown without MongoDB", func(t *testing.T) {
		page := serve(true)

		for _, want := range []string{
			"In-memory storage.",
			"lost when Phield restarts",
			"PHIELD_MONGO_URI",
			`href="https://philterd.github.io/phield/configuration/"`,
		} {
			if !strings.Contains(page, want) {
				t.Errorf("expected the page to contain %q", want)
			}
		}
	})

	t.Run("hidden with MongoDB", func(t *testing.T) {
		page := serve(false)

		if strings.Contains(page, "In-memory storage.") {
			t.Error("expected no storage banner")
		}
		if strings.Contains(page, storageBannerMarker) {
			t.Error("expected the marker to be removed from the page")
		}
		// The rest of the page is still there.
		if !strings.Contains(page, "Phield") || !strings.Contains(page, "Alert Timeline") {
			t.Error("expected the dashboard to render")
		}
	})
}

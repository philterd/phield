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

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newRouter(apiKey string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BearerToken(apiKey))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func doRequest(r *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestBearerTokenDisabled(t *testing.T) {
	r := newRouter("")

	for _, header := range []string{"", "Bearer anything", "garbage"} {
		if w := doRequest(r, header); w.Code != http.StatusOK {
			t.Errorf("no API key configured: expected 200 for header %q, got %d", header, w.Code)
		}
	}
}

func TestBearerTokenEnabled(t *testing.T) {
	r := newRouter("s3cret")

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"valid token", "Bearer s3cret", http.StatusOK},
		{"lowercase scheme", "bearer s3cret", http.StatusOK},
		{"mixed case scheme", "BeArEr s3cret", http.StatusOK},
		{"surrounding whitespace", "Bearer  s3cret ", http.StatusOK},
		{"missing header", "", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong", http.StatusUnauthorized},
		{"token is a prefix of the key", "Bearer s3cre", http.StatusUnauthorized},
		{"token extends the key", "Bearer s3cretx", http.StatusUnauthorized},
		{"empty token", "Bearer ", http.StatusUnauthorized},
		{"wrong scheme", "Basic s3cret", http.StatusUnauthorized},
		{"raw key without scheme", "s3cret", http.StatusUnauthorized},
		{"case differs from key", "Bearer S3CRET", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doRequest(r, tt.header)
			if w.Code != tt.want {
				t.Errorf("expected status %d, got %d", tt.want, w.Code)
			}
			if tt.want == http.StatusUnauthorized {
				if got := w.Header().Get("WWW-Authenticate"); got != "Bearer" {
					t.Errorf("expected WWW-Authenticate: Bearer, got %q", got)
				}
				if body := w.Body.String(); body != `{"error":"unauthorized"}` {
					t.Errorf("unexpected body: %s", body)
				}
			}
		})
	}
}

func TestBearerTokenDoesNotLeakHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BearerToken("s3cret"))

	called := false
	r.GET("/protected", func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	})

	doRequest(r, "Bearer wrong")
	if called {
		t.Error("handler ran despite a rejected request")
	}
}

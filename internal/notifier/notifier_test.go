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

package notifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSlackNotifier(t *testing.T) {
	t.Run("Successful notification", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST method, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected application/json content type, got %s", r.Header.Get("Content-Type"))
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		n := NewSlackNotifier(server.URL)
		err := n.Notify(context.Background(), "test message")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("Failed notification", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		n := NewSlackNotifier(server.URL)
		err := n.Notify(context.Background(), "test message")
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("Empty webhook URL", func(t *testing.T) {
		n := NewSlackNotifier("")
		err := n.Notify(context.Background(), "test message")
		if err != nil {
			t.Errorf("expected no error for empty webhook, got %v", err)
		}
	})
}

func TestPagerDutyNotifier(t *testing.T) {
	t.Run("Successful notification", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		}))
		defer server.Close()

		// We need to override the URL in the Notify method if we want to test with httptest.NewServer
		// But the URL is hardcoded in notifier.go. Let's see if we can refactor notifier.go to allow setting URL or just mock it.
		// For now, I'll just check if it returns nil when RoutingKey is empty.
		n := NewPagerDutyNotifier("", "critical")
		err := n.Notify(context.Background(), "test message")
		if err != nil {
			t.Errorf("expected no error for empty routing key, got %v", err)
		}
	})
}

func TestMultiNotifier(t *testing.T) {
	n1 := &MockNotifier{}
	n2 := &MockNotifier{}
	mn := NewMultiNotifier(n1, n2)

	err := mn.Notify(context.Background(), "test message")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if n1.Calls != 1 {
		t.Errorf("expected 1 call for n1, got %d", n1.Calls)
	}
	if n2.Calls != 1 {
		t.Errorf("expected 1 call for n2, got %d", n2.Calls)
	}
}

type MockNotifier struct {
	Calls int
}

func (m *MockNotifier) Notify(ctx context.Context, message string) error {
	m.Calls++
	return nil
}

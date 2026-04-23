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
	"testing"
	"time"

	"github.com/philterd/phield/internal/models"
)

func TestInMemoryStorage(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStorage()

	// Test Save and GetAverage
	now := time.Now()
	entries := []models.PIIEntry{
		{
			Timestamp:    now.Add(-1 * time.Hour),
			SourceID:     "source-1",
			Organization: "org-1",
			Context:      "default",
			PIITypes:     map[string]int{"ssn": 10},
		},
		{
			Timestamp:    now.Add(-2 * time.Hour),
			SourceID:     "source-1",
			Organization: "org-1",
			Context:      "default",
			PIITypes:     map[string]int{"ssn": 20},
		},
		{
			Timestamp:    now.Add(-25 * time.Hour), // Outside 24h window
			SourceID:     "source-1",
			Organization: "org-1",
			Context:      "default",
			PIITypes:     map[string]int{"ssn": 100},
		},
		{
			Timestamp:    now.Add(-3 * time.Hour),
			SourceID:     "source-2", // Different source
			Organization: "org-1",
			Context:      "default",
			PIITypes:     map[string]int{"ssn": 50},
		},
		{
			Timestamp:    now.Add(-4 * time.Hour),
			SourceID:     "source-1",
			Organization: "org-1",
			Context:      "default",
			PIITypes:     map[string]int{"email": 10}, // Missing "ssn"
		},
		{
			Timestamp:    now.Add(-1 * time.Hour),
			SourceID:     "source-1",
			Organization: "org-1",
			Context:      "other",
			PIITypes:     map[string]int{"ssn": 500},
		},
		{
			Timestamp:    now.Add(-1 * time.Hour),
			SourceID:     "source-1",
			Organization: "org-2", // Different organization
			Context:      "default",
			PIITypes:     map[string]int{"ssn": 1000},
		},
	}

	for _, e := range entries {
		if err := s.Save(ctx, e); err != nil {
			t.Fatalf("failed to save entry: %v", err)
		}
	}

	t.Run("Average for source-1 org-1 default context ssn within 24h", func(t *testing.T) {
		// Entries for source-1, org-1, context default in 24h:
		// 1. ssn: 10
		// 2. ssn: 20
		// 3. email: 10 (ssn is missing, counts as 0)
		// Total: 10 + 20 + 0 = 30
		// Count: 3
		// Average: 30 / 3 = 10
		avg, err := s.GetAverage(ctx, "source-1", "org-1", "default", "ssn", 24)
		if err != nil {
			t.Fatalf("failed to get average: %v", err)
		}
		expected := 10.0
		if avg != expected {
			t.Errorf("expected average %f, got %f", expected, avg)
		}
	})

	t.Run("Average for source-1 org-2 default context ssn within 24h", func(t *testing.T) {
		// Entries for source-1, org-2, context default in 24h:
		// 1. ssn: 1000
		// Total: 1000, Count: 1, Avg: 1000
		avg, err := s.GetAverage(ctx, "source-1", "org-2", "default", "ssn", 24)
		if err != nil {
			t.Fatalf("failed to get average: %v", err)
		}
		expected := 1000.0
		if avg != expected {
			t.Errorf("expected average %f, got %f", expected, avg)
		}
	})

	t.Run("Average for source-2 org-1 default context ssn within 24h", func(t *testing.T) {
		// Entries for source-2, org-1, context default in 24h:
		// 1. ssn: 50
		// Total: 50, Count: 1, Avg: 50
		avg, err := s.GetAverage(ctx, "source-2", "org-1", "default", "ssn", 24)
		if err != nil {
			t.Fatalf("failed to get average: %v", err)
		}
		expected := 50.0
		if avg != expected {
			t.Errorf("expected average %f, got %f", expected, avg)
		}
	})

	t.Run("Average for non-existent piiType", func(t *testing.T) {
		// All source-1 org-1 default context entries count as 0 for non-existent piiType
		avg, err := s.GetAverage(ctx, "source-1", "org-1", "default", "non-existent", 24)
		if err != nil {
			t.Fatalf("failed to get average: %v", err)
		}
		expected := 0.0
		if avg != expected {
			t.Errorf("expected average %f, got %f", expected, avg)
		}
	})

	t.Run("Average for other context", func(t *testing.T) {
		// Entries for source-1, org-1, context other in 24h:
		// 1. ssn: 500
		// Total: 500, Count: 1, Avg: 500
		avg, err := s.GetAverage(ctx, "source-1", "org-1", "other", "ssn", 24)
		if err != nil {
			t.Fatalf("failed to get average: %v", err)
		}
		expected := 500.0
		if avg != expected {
			t.Errorf("expected average %f, got %f", expected, avg)
		}
	})
}

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

package models

import (
	"strings"
	"testing"
	"time"
)

func TestIngestRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		request IngestRequest
		wantErr string
	}{
		{
			name:    "minimal valid request",
			request: IngestRequest{SourceID: "source-1", PIITypes: map[string]int{"ssn": 3}},
		},
		{
			name: "all fields populated",
			request: IngestRequest{
				Timestamp:    time.Now(),
				SourceID:     "source-1",
				Organization: "org-1",
				Context:      "billing",
				PIITypes:     map[string]int{"ssn": 3, "email": 0},
			},
		},
		{
			name:    "a zero count is allowed",
			request: IngestRequest{SourceID: "source-1", PIITypes: map[string]int{"ssn": 0}},
		},
		{
			name:    "missing source_id",
			request: IngestRequest{PIITypes: map[string]int{"ssn": 3}},
			wantErr: "source_id is required",
		},
		{
			name:    "blank source_id",
			request: IngestRequest{SourceID: "   ", PIITypes: map[string]int{"ssn": 3}},
			wantErr: "source_id is required",
		},
		{
			name:    "missing pii_types",
			request: IngestRequest{SourceID: "source-1"},
			wantErr: "pii_types is required",
		},
		{
			name:    "empty pii_types",
			request: IngestRequest{SourceID: "source-1", PIITypes: map[string]int{}},
			wantErr: "pii_types is required",
		},
		{
			name:    "empty PII type name",
			request: IngestRequest{SourceID: "source-1", PIITypes: map[string]int{"": 3}},
			wantErr: "empty PII type name",
		},
		{
			name:    "blank PII type name",
			request: IngestRequest{SourceID: "source-1", PIITypes: map[string]int{"  ": 3}},
			wantErr: "empty PII type name",
		},
		{
			name:    "PII type name containing a dot",
			request: IngestRequest{SourceID: "source-1", PIITypes: map[string]int{"credit.card": 3}},
			wantErr: "must not contain",
		},
		{
			name:    "PII type name starting with a dollar sign",
			request: IngestRequest{SourceID: "source-1", PIITypes: map[string]int{"$ssn": 3}},
			wantErr: "must not contain",
		},
		{
			name:    "negative count",
			request: IngestRequest{SourceID: "source-1", PIITypes: map[string]int{"ssn": -1}},
			wantErr: "must not be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected an error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

// A dollar sign anywhere other than the start of the name is allowed, as is a
// name that only looks unusual.
func TestIngestRequestValidateAllowsUncommonNames(t *testing.T) {
	req := IngestRequest{
		SourceID: "source-1",
		PIITypes: map[string]int{"us$d-amount": 1, "credit-card": 2, "CREDIT_CARD": 3, "iban/bban": 4},
	}

	if err := req.Validate(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

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

package trend

import (
	"testing"
)

func TestCalculateBreach(t *testing.T) {
	tests := []struct {
		name         string
		currentCount int
		avg          float64
		threshold    float64
		wantDiff     float64
		wantBreach   bool
	}{
		{
			name:         "Breach detected",
			currentCount: 30,
			avg:          10.0,
			threshold:    0.2,
			wantDiff:     2.0,
			wantBreach:   true,
		},
		{
			name:         "No breach",
			currentCount: 11,
			avg:          10.0,
			threshold:    0.2,
			wantDiff:     0.1,
			wantBreach:   false,
		},
		{
			name:         "Exact threshold no breach",
			currentCount: 12,
			avg:          10.0,
			threshold:    0.2,
			wantDiff:     0.2,
			wantBreach:   false,
		},
		{
			name:         "Zero average",
			currentCount: 10,
			avg:          0.0,
			threshold:    0.2,
			wantDiff:     0.0,
			wantBreach:   false,
		},
		{
			name:         "Negative average (should not happen but handled)",
			currentCount: 10,
			avg:          -1.0,
			threshold:    0.2,
			wantDiff:     0.0,
			wantBreach:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff, breach := CalculateBreach(MethodPercentageDelta, tt.currentCount, tt.avg, tt.threshold, 0)
			if breach != tt.wantBreach {
				t.Errorf("CalculateBreach() breach = %v, want %v", breach, tt.wantBreach)
			}
			if diff != tt.wantDiff {
				t.Errorf("CalculateBreach() diff = %v, want %v", diff, tt.wantDiff)
			}
		})
	}
}

func TestCalculateBreach_ZScore(t *testing.T) {
	tests := []struct {
		name         string
		currentCount int
		avg          float64
		sensitivity  float64
		stdDev       float64
		wantZScore   float64
		wantBreach   bool
	}{
		{
			name:         "Z-Score breach",
			currentCount: 20,
			avg:          10.0,
			sensitivity:  3.0,
			stdDev:       2.0,
			wantZScore:   5.0,
			wantBreach:   true,
		},
		{
			name:         "Z-Score no breach",
			currentCount: 15,
			avg:          10.0,
			sensitivity:  3.0,
			stdDev:       2.0,
			wantZScore:   2.5,
			wantBreach:   false,
		},
		{
			name:         "Z-Score zero StdDev",
			currentCount: 20,
			avg:          10.0,
			sensitivity:  3.0,
			stdDev:       0.0,
			wantZScore:   0.0,
			wantBreach:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zScore, breach := CalculateBreach(MethodZScore, tt.currentCount, tt.avg, tt.sensitivity, tt.stdDev)
			if breach != tt.wantBreach {
				t.Errorf("CalculateBreach(ZScore) breach = %v, want %v", breach, tt.wantBreach)
			}
			if zScore != tt.wantZScore {
				t.Errorf("CalculateBreach(ZScore) zScore = %v, want %v", zScore, tt.wantZScore)
			}
		})
	}
}

func TestCalculateBreach_DefaultMethod(t *testing.T) {
	diff, breach := CalculateBreach("unknown", 30, 10.0, 0.2, 0)
	if !breach {
		t.Errorf("CalculateBreach() with unknown method should fallback to percentage_delta and detect breach")
	}
	if diff != 2.0 {
		t.Errorf("CalculateBreach() with unknown method diff = %v, want 2.0", diff)
	}
}

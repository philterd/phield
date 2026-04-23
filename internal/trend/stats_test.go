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
	"math"
	"testing"
)

func TestStatTracker_Update(t *testing.T) {
	s := &StatTracker{}
	values := []float64{10, 20, 30}

	for _, v := range values {
		s.Update(v)
	}

	if s.Count != 3 {
		t.Errorf("expected count 3, got %d", s.Count)
	}

	expectedMean := 20.0
	if s.Mean != expectedMean {
		t.Errorf("expected mean %f, got %f", expectedMean, s.Mean)
	}
}

func TestStatTracker_VarianceAndStdDev(t *testing.T) {
	s := &StatTracker{}

	// Single value should have 0 variance
	s.Update(10)
	if s.Variance() != 0 {
		t.Errorf("expected variance 0 for single value, got %f", s.Variance())
	}
	if s.StdDev() != 0 {
		t.Errorf("expected stddev 0 for single value, got %f", s.StdDev())
	}

	// Two values: 10, 20
	// Mean = 15
	// Population Variance = ((10-15)^2 + (20-15)^2) / 2 = (25 + 25) / 2 = 25
	s.Update(20)
	expectedVar := 25.0
	if s.Variance() != expectedVar {
		t.Errorf("expected variance %f, got %f", expectedVar, s.Variance())
	}

	expectedStdDev := math.Sqrt(25.0)
	if s.StdDev() != expectedStdDev {
		t.Errorf("expected stddev %f, got %f", expectedStdDev, s.StdDev())
	}
}

func TestStatTracker_ZScore(t *testing.T) {
	s := &StatTracker{}

	// stddev 0 case
	s.Update(10)
	if z := s.ZScore(20); z != 0 {
		t.Errorf("expected z-score 0 when stddev is 0, got %f", z)
	}

	// Values: 10, 20, 30
	// Mean = 20
	// Variance = ((10-20)^2 + (20-20)^2 + (30-20)^2) / 3 = (100 + 0 + 100) / 3 = 66.6666...
	s.Update(20)
	s.Update(30)

	mean := 20.0
	variance := 200.0 / 3.0
	stdDev := math.Sqrt(variance)

	testVal := 40.0
	expectedZ := (testVal - mean) / stdDev

	z := s.ZScore(testVal)
	if math.Abs(z-expectedZ) > 1e-9 {
		t.Errorf("expected z-score %f, got %f", expectedZ, z)
	}
}

func TestStatTracker_LargeValues(t *testing.T) {
	s := &StatTracker{}
	// Test stability with large values
	s.Update(1e12 + 1)
	s.Update(1e12 + 2)
	s.Update(1e12 + 3)

	expectedMean := 1e12 + 2.0
	if math.Abs(s.Mean-expectedMean) > 1e-3 {
		t.Errorf("expected mean around %f, got %f", expectedMean, s.Mean)
	}

	// Variance should be ((1-2)^2 + (2-2)^2 + (3-2)^2) / 3 = 2/3
	expectedVar := 2.0 / 3.0
	if math.Abs(s.Variance()-expectedVar) > 1e-3 {
		t.Errorf("expected variance %f, got %f", expectedVar, s.Variance())
	}
}

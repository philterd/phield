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

import "math"

// StatTracker implements Welford's Online Algorithm for incremental mean and variance.
type StatTracker struct {
	Count int     `json:"count" bson:"count"`
	Mean  float64 `json:"mean" bson:"mean"`
	M2    float64 `json:"m2" bson:"m2"` // Sum of squares of differences from the mean
}

// Update adds a new value to the tracker and updates mean and M2.
func (s *StatTracker) Update(x float64) {
	s.Count++
	delta := x - s.Mean
	s.Mean += delta / float64(s.Count)
	delta2 := x - s.Mean
	s.M2 += delta * delta2
}

// Variance returns the current variance.
func (s *StatTracker) Variance() float64 {
	if s.Count < 2 {
		return 0
	}
	return s.M2 / float64(s.Count)
}

// StdDev returns the current standard deviation.
func (s *StatTracker) StdDev() float64 {
	return math.Sqrt(s.Variance())
}

// ZScore calculates the Z-Score for a given value.
func (s *StatTracker) ZScore(x float64) float64 {
	stdDev := s.StdDev()
	if stdDev == 0 {
		return 0
	}
	return (x - s.Mean) / stdDev
}

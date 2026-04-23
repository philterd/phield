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

const (
	// MethodPercentageDelta is the default trend detection method.
	MethodPercentageDelta = "percentage_delta"
	// MethodZScore is the adaptive threshold method.
	MethodZScore = "z_score"
)

// CalculateBreach checks if the current count exceeds the average by the given threshold using the specified method.
// It returns the difference (or z-score) and a boolean indicating if a breach occurred.
func CalculateBreach(method string, currentCount int, avg float64, threshold float64, stdDev float64) (float64, bool) {
	switch method {
	case MethodZScore:
		return calculateZScoreBreach(currentCount, avg, threshold, stdDev)
	case MethodPercentageDelta:
		fallthrough
	default:
		return calculatePercentageDelta(currentCount, avg, threshold)
	}
}

func calculatePercentageDelta(currentCount int, avg float64, threshold float64) (float64, bool) {
	if avg <= 0 {
		return 0, false
	}

	diff := (float64(currentCount) - avg) / avg
	if diff > threshold {
		return diff, true
	}

	return diff, false
}

func calculateZScoreBreach(currentCount int, avg float64, sensitivity float64, stdDev float64) (float64, bool) {
	if stdDev == 0 {
		return 0, false
	}

	zScore := (float64(currentCount) - avg) / stdDev
	if zScore > sensitivity {
		return zScore, true
	}

	return zScore, false
}

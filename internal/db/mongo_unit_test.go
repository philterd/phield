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
	"testing"
)

func TestExtractDBName(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"mongodb://localhost:27017", "phield"},
		{"mongodb://localhost:27017/", "phield"},
		{"mongodb://localhost:27017/mydb", "mydb"},
		{"mongodb://localhost:27017/mydb?authSource=admin", "mydb"},
		{"mongodb://user:pass@localhost:27017/mydb", "mydb"},
		{"mongodb://user:pass@localhost:27017/mydb?options", "mydb"},
		{"mongodb+srv://user:pass@cluster.mongodb.net/mydb", "mydb"},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			got := extractDBName(tt.uri)
			if got != tt.expected {
				t.Errorf("extractDBName(%s) = %s; want %s", tt.uri, got, tt.expected)
			}
		})
	}
}

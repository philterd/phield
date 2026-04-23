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

package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/philterd/phield/internal/models"
)

type API struct {
	ingestChan chan models.PIIEntry
}

func NewAPI(ingestChan chan models.PIIEntry) *API {
	return &API{
		ingestChan: ingestChan,
	}
}

func (a *API) RegisterRoutes(r *gin.Engine) {
	r.POST("/ingest", a.handleIngest)
}

func (a *API) handleIngest(c *gin.Context) {
	var req models.IngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default to current time if zero
	ts := req.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	entry := models.PIIEntry{
		Timestamp: ts,
		SourceID:  req.SourceID,
		PIITypes:  req.PIITypes,
	}

	// Send to background worker
	select {
	case a.ingestChan <- entry:
		c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
	default:
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ingestion queue full"})
	}

}

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
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/philterd/phield/internal/db"
	"github.com/philterd/phield/internal/models"
	"github.com/philterd/phield/internal/notifier"
	"github.com/philterd/phield/internal/trend"
)

type API struct {
	storage        db.Storage
	alertThreshold float64
	trendMethod    string
	windowSize     int
	sensitivity    float64
	warmUpCount    int
	cooldownMins   int
	notifier       notifier.Notifier
}

func NewAPI(storage db.Storage, alertThreshold float64, trendMethod string, windowSize int, sensitivity float64, warmUpCount int, cooldownMins int, n notifier.Notifier) *API {
	return &API{
		storage:        storage,
		alertThreshold: alertThreshold,
		trendMethod:    trendMethod,
		windowSize:     windowSize,
		sensitivity:    sensitivity,
		warmUpCount:    warmUpCount,
		cooldownMins:   cooldownMins,
		notifier:       n,
	}
}

func (a *API) RegisterRoutes(r *gin.Engine) {
	r.Use(a.metricsMiddleware())
	r.POST("/ingest", a.handleIngest)
	r.POST("/mute", a.handleMute)
	r.POST("/replay", a.handleReplay)
	r.GET("/health", a.handleHealth)
	r.GET("/metrics", a.handleMetrics)
}

func (a *API) metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path != "/ingest" {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()
		latency := time.Since(start)

		if a.storage != nil {
			_ = a.storage.SaveMetric(context.Background(), latency)
		}
	}
}

func (a *API) handleMetrics(c *gin.Context) {
	count, avgLatency, err := a.storage.GetMetrics(c.Request.Context(), 24)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error retrieving metrics")
		return
	}

	metrics := fmt.Sprintf("# HELP phield_ingest_requests_total_24h Number of /ingest requests in the past 24 hours\n"+
		"# TYPE phield_ingest_requests_total_24h gauge\n"+
		"phield_ingest_requests_total_24h %d\n"+
		"# HELP phield_ingest_latency_average_seconds_24h Average /ingest latency in the past 24 hours in seconds\n"+
		"# TYPE phield_ingest_latency_average_seconds_24h gauge\n"+
		"phield_ingest_latency_average_seconds_24h %.6f\n",
		count, avgLatency)

	c.String(http.StatusOK, metrics)
}

func (a *API) handleMute(c *gin.Context) {
	var req models.MuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Context == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context is required"})
		return
	}

	org := req.Organization
	if org == "" {
		org = "default"
	}

	if req.Minutes <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "minutes must be greater than 0"})
		return
	}

	err := a.storage.SaveMute(c.Request.Context(), org, req.Context, req.Minutes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save mute"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "muted", "organization": org, "context": req.Context, "minutes": req.Minutes})
}

func (a *API) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (a *API) handleIngest(c *gin.Context) {
	var req models.IngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := a.ProcessIngest(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
}

func (a *API) ProcessIngest(ctx context.Context, req models.IngestRequest) error {
	ts := req.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	org := req.Organization
	if org == "" {
		org = "default"
	}

	entry := models.PIIEntry{
		Timestamp:    ts,
		SourceID:     req.SourceID,
		Organization: org,
		Context:      req.Context,
		PIITypes:     req.PIITypes,
	}

	// Synchronously persist and analyze trend to ensure statelessness.
	// In a high-traffic production environment, this could be offloaded to a distributed task queue (like RabbitMQ or Redis).
	// For Phield's stateless requirement with MongoDB, direct processing ensures all instances share the same view.
	if err := a.storage.Save(ctx, entry); err != nil {
		log.Printf("Error persisting entry: %v", err)
		return fmt.Errorf("failed to persist entry")
	}

	// For each PII type, calculate moving average and check for breach
	for piiType, currentCount := range entry.PIITypes {
		a.analyzeTrend(ctx, entry.SourceID, entry.Organization, entry.Context, piiType, currentCount)
	}

	return nil
}

func (a *API) analyzeTrend(ctx context.Context, sourceID string, organization string, contextName string, piiType string, currentCount int) {
	// Check if context is muted
	muted, err := a.storage.IsMuted(ctx, organization, contextName)
	if err != nil {
		log.Printf("Error checking mute status: %v", err)
	}
	if muted {
		return
	}

	// Get current stats
	stats, err := a.storage.GetStats(ctx, sourceID, organization, contextName, piiType)
	if err != nil {
		log.Printf("Error getting stats: %v", err)
	}

	tracker := trend.StatTracker{
		Count: stats.Count,
		Mean:  stats.Mean,
		M2:    stats.M2,
	}

	lastAlertTime := stats.LastAlertTime
	consecutiveNormal := stats.ConsecutiveNormal

	var breached bool
	var val float64
	var msg string

	if a.trendMethod == trend.MethodZScore {
		if tracker.Count >= a.warmUpCount {
			val, breached = trend.CalculateBreach(a.trendMethod, currentCount, tracker.Mean, a.sensitivity, tracker.StdDev())
			if breached {
				msg = fmt.Sprintf("[TREND BREACH] Source: %s, Organization: %s, Context: %s, PII Type: %s, Current: %d, Mean: %.2f, StdDev: %.2f, Z-Score: %.2f",
					sourceID, organization, contextName, piiType, currentCount, tracker.Mean, tracker.StdDev(), val)
			}
		}
	} else {
		avg, err := a.storage.GetAverage(ctx, sourceID, organization, contextName, piiType, a.windowSize)
		if err != nil {
			log.Printf("Error getting average: %v", err)
		} else {
			val, breached = trend.CalculateBreach(a.trendMethod, currentCount, avg, a.alertThreshold, 0)
			if breached {
				msg = fmt.Sprintf("[TREND BREACH] Source: %s, Organization: %s, Context: %s, PII Type: %s, Current: %d, Avg: %.2f, Increase: %.2f%%",
					sourceID, organization, contextName, piiType, currentCount, avg, val*100)
			}
		}
	}

	// Update stats regardless of breach
	tracker.Update(float64(currentCount))

	isNormal := false
	if a.trendMethod == trend.MethodZScore {
		z := tracker.ZScore(float64(currentCount))
		if z < 1.0 {
			isNormal = true
		}
	} else {
		avg := tracker.Mean
		if avg > 0 {
			diff := (float64(currentCount) - avg) / avg
			if diff < a.alertThreshold/2 {
				isNormal = true
			}
		}
	}

	if isNormal {
		consecutiveNormal++
		if consecutiveNormal >= 3 {
			lastAlertTime = time.Time{}
		}
	} else if !breached {
		consecutiveNormal = 0
	}

	if breached {
		consecutiveNormal = 0
		cooldown := time.Duration(a.cooldownMins) * time.Minute
		if !lastAlertTime.IsZero() && time.Since(lastAlertTime) < cooldown {
			log.Printf("[SUPPRESSED] Alert for %s/%s/%s suppressed due to cooldown (last alert: %v)", sourceID, piiType, contextName, lastAlertTime)
		} else {
			// Print to standard out in addition to notification channels.
			fmt.Println(msg)

			log.Print(msg)
			lastAlertTime = time.Now()
			if a.notifier != nil {
				if err := a.notifier.Notify(ctx, msg); err != nil {
					log.Printf("Error sending notification: %v", err)
				}
			}
		}
	}

	err = a.storage.SaveStats(ctx, sourceID, organization, contextName, piiType, models.Stats{
		Count:             tracker.Count,
		Mean:              tracker.Mean,
		M2:                tracker.M2,
		LastAlertTime:     lastAlertTime,
		ConsecutiveNormal: consecutiveNormal,
	})
	if err != nil {
		log.Printf("Error saving stats: %v", err)
	}
}

func (a *API) handleReplay(c *gin.Context) {
	var req models.ReplayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.StartTime.IsZero() || req.EndTime.IsZero() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_time and end_time are required"})
		return
	}

	if req.TestThreshold <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "test_threshold must be greater than 0"})
		return
	}

	resp, err := a.RunReplay(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

type simEntry struct {
	timestamp time.Time
	count     int
}

func (a *API) RunReplay(ctx context.Context, req models.ReplayRequest) (models.ReplayResponse, error) {
	entryChan, errChan := a.storage.GetEntries(ctx, req.StartTime, req.EndTime)

	history := make(map[string][]simEntry)

	resp := models.ReplayResponse{
		BreachDetails: make([]models.BreachDetail, 0),
	}

	for {
		select {
		case entry, ok := <-entryChan:
			if !ok {
				return resp, nil
			}

			resp.TotalPointsProcessed++

			for piiType, currentCount := range entry.PIITypes {
				if len(req.PIITypes) > 0 {
					found := false
					for _, t := range req.PIITypes {
						if t == piiType {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}

				key := fmt.Sprintf("%s:%s:%s:%s", entry.Organization, entry.Context, entry.SourceID, piiType)

				var val float64
				var breached bool
				var avg float64

				if a.trendMethod == trend.MethodZScore {
					currentStats := history[key]
					tracker := trend.StatTracker{}
					for _, h := range currentStats {
						tracker.Update(float64(h.count))
					}

					if tracker.Count >= a.warmUpCount {
						val, breached = trend.CalculateBreach(a.trendMethod, currentCount, tracker.Mean, req.TestThreshold, tracker.StdDev())
					}
					avg = tracker.Mean

					// Update history for next point
					history[key] = append(history[key], simEntry{timestamp: entry.Timestamp, count: currentCount})
				} else {
					lookback := entry.Timestamp.Add(-time.Duration(a.windowSize) * time.Hour)

					// Filter history for the window
					currentHistory := history[key]
					var filteredHistory []simEntry
					var sum int
					for _, h := range currentHistory {
						if h.timestamp.After(lookback) || h.timestamp.Equal(lookback) {
							filteredHistory = append(filteredHistory, h)
							sum += h.count
						}
					}

					if len(filteredHistory) > 0 {
						avg = float64(sum) / float64(len(filteredHistory))
						val, breached = trend.CalculateBreach(a.trendMethod, currentCount, avg, req.TestThreshold, 0)
					}

					// Update history with the current point
					filteredHistory = append(filteredHistory, simEntry{timestamp: entry.Timestamp, count: currentCount})
					history[key] = filteredHistory
				}

				if breached {
					resp.VirtualBreachesDetected++
					detail := models.BreachDetail{
						Timestamp: entry.Timestamp,
						PIIType:   piiType,
						Context:   entry.Context,
						Org:       entry.Organization,
						SourceID:  entry.SourceID,
						Count:     currentCount,
						Average:   avg,
					}
					if a.trendMethod == trend.MethodZScore {
						detail.ZScore = val
					}
					resp.BreachDetails = append(resp.BreachDetails, detail)
				}
			}

		case err := <-errChan:
			if err != nil {
				return resp, err
			}
		case <-ctx.Done():
			return resp, ctx.Err()
		}
	}
}

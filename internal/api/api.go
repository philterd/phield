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
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"math/rand/v2"
	"net/http"
	"sync"
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

// LimitRequestBody returns middleware that refuses a request body larger than
// maxBytes, so an oversized payload is rejected as it arrives rather than being
// read into memory. A maxBytes of zero or less disables the limit.
func LimitRequestBody(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// bindJSON reads the request body into req. It writes the response and returns
// false when the body is unreadable, so a handler can simply return.
func bindJSON(c *gin.Context, req any) bool {
	err := c.ShouldBindJSON(req)
	if err == nil {
		return true
	}

	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("request body must not exceed %d bytes", tooLarge.Limit),
		})
		return false
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	return false
}

// RegisterRoutes registers the API endpoints. Any middleware passed in is
// applied to the endpoints that change what Phield holds, which is how API key
// authentication is enabled. Health and metrics stay open so probes and scrapes
// need no credential: the key exists to keep bad data out, and neither endpoint
// exposes PII.
func (a *API) RegisterRoutes(r *gin.Engine, middleware ...gin.HandlerFunc) {
	r.Use(a.metricsMiddleware())

	r.GET("/health", a.handleHealth)
	r.GET("/metrics", a.handleMetrics)

	g := r.Group("/", middleware...)
	g.POST("/ingest", a.handleIngest)
	g.POST("/mute", a.handleMute)
	g.POST("/replay", a.handleReplay)
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
	if !bindJSON(c, &req) {
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

// healthCheckTimeout bounds the storage check so a hung backend cannot hold the
// health endpoint open.
const healthCheckTimeout = 2 * time.Second

func (a *API) handleHealth(c *gin.Context) {
	// Reports unhealthy when the storage cannot be reached, so a load balancer
	// stops sending an instance counts it cannot persist. With in-memory
	// storage there is nothing to reach and this always succeeds.
	ctx, cancel := context.WithTimeout(c.Request.Context(), healthCheckTimeout)
	defer cancel()

	if err := a.storage.Ping(ctx); err != nil {
		log.Printf("Health check failed: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "storage": "unreachable"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (a *API) handleIngest(c *gin.Context) {
	var req models.IngestRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := a.ProcessIngest(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
}

// ProcessIngest validates and stores a set of counts, then analyzes the trend
// for each PII type. It is called for both API requests and Kafka messages, so
// it validates the request rather than relying on the caller to have done so.
func (a *API) ProcessIngest(ctx context.Context, req models.IngestRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}

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

// maxStatsAttempts bounds how many times a trend analysis is redone when a
// concurrent ingest for the same series updates the stats first.
const maxStatsAttempts = 8

// seriesLockCount is the number of locks that serialize same-series work within
// this process. Series are spread across them by hash, so the memory cost is
// fixed no matter how many series a deployment has.
const seriesLockCount = 256

var seriesLocks [seriesLockCount]sync.Mutex

// lockSeries serializes the read-modify-write of one series inside this process,
// leaving the versioned write in storage to handle the other instances. Without
// it, every goroutine holding the same series would collide on the write and
// burn through its retries.
func lockSeries(sourceID string, organization string, contextName string, piiType string) func() {
	h := fnv.New32a()
	_, _ = h.Write([]byte(sourceID + "\x00" + organization + "\x00" + contextName + "\x00" + piiType))

	lock := &seriesLocks[h.Sum32()%seriesLockCount]
	lock.Lock()
	return lock.Unlock
}

// trendOutcome is the result of one pass over a series: the stats to store and
// whatever should be reported once they are stored.
type trendOutcome struct {
	stats      models.Stats
	message    string
	breached   bool
	suppressed bool
	breach     models.BreachDetail
}

func (a *API) analyzeTrend(ctx context.Context, sourceID string, organization string, contextName string, piiType string, currentCount int) {
	muted, err := a.storage.IsMuted(ctx, organization, contextName)
	if err != nil {
		// The mute state is unknown, so carry on as if not muted: a spurious
		// alert is a better failure than a missed one.
		log.Printf("Error checking mute status: %v", err)
	}
	if muted {
		return
	}

	unlock := lockSeries(sourceID, organization, contextName, piiType)
	defer unlock()

	// Stats are read, updated, and written back, so an ingest for the same
	// series on another instance can land in between. SaveStats refuses a write
	// computed from stats that have since moved on, and the analysis is redone
	// against the current ones. Reporting waits until the write lands so a retry
	// cannot alert twice.
	for attempt := 1; attempt <= maxStatsAttempts; attempt++ {
		outcome, err := a.evaluateTrend(ctx, sourceID, organization, contextName, piiType, currentCount)
		if err != nil {
			// Analyzing against a baseline we could not read would replace it
			// with one built from this single count, so stop here instead.
			log.Printf("Error analyzing %s/%s/%s/%s: %v", sourceID, organization, contextName, piiType, err)
			return
		}

		err = a.storage.SaveStats(ctx, sourceID, organization, contextName, piiType, outcome.stats)
		if errors.Is(err, db.ErrStatsConflict) {
			// Back off a random, growing moment so instances that collided do
			// not line up and collide again, and so a series under sustained
			// write pressure from several instances still converges.
			time.Sleep(time.Duration(attempt) * time.Duration(rand.IntN(10)+5) * time.Millisecond)
			continue
		}
		if err != nil {
			log.Printf("Error saving stats: %v", err)
			return
		}

		a.reportTrend(ctx, outcome)
		return
	}

	log.Printf("Gave up updating stats for %s/%s/%s/%s after %d concurrent updates",
		sourceID, organization, contextName, piiType, maxStatsAttempts)
}

// evaluateTrend reads the current stats for a series and works out what this
// count does to them. It changes nothing.
func (a *API) evaluateTrend(ctx context.Context, sourceID string, organization string, contextName string, piiType string, currentCount int) (trendOutcome, error) {
	stats, err := a.storage.GetStats(ctx, sourceID, organization, contextName, piiType)
	if err != nil {
		return trendOutcome{}, fmt.Errorf("reading stats: %w", err)
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
			// The average feeds the breach decision but not the stored stats,
			// so skip detection for this count and keep the baseline current.
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

	outcome := trendOutcome{message: msg, breached: breached}

	if breached {
		consecutiveNormal = 0
		cooldown := time.Duration(a.cooldownMins) * time.Minute
		if !lastAlertTime.IsZero() && time.Since(lastAlertTime) < cooldown {
			outcome.suppressed = true
			outcome.message = fmt.Sprintf("[SUPPRESSED] Alert for %s/%s/%s suppressed due to cooldown (last alert: %v)", sourceID, piiType, contextName, lastAlertTime)
		} else {
			lastAlertTime = time.Now()
			outcome.breach = models.BreachDetail{
				Timestamp: lastAlertTime,
				PIIType:   piiType,
				Context:   contextName,
				Org:       organization,
				SourceID:  sourceID,
				Count:     currentCount,
				Average:   tracker.Mean,
				ZScore:    val,
			}
		}
	}

	outcome.stats = models.Stats{
		Count:             tracker.Count,
		Mean:              tracker.Mean,
		M2:                tracker.M2,
		LastAlertTime:     lastAlertTime,
		ConsecutiveNormal: consecutiveNormal,
		Version:           stats.Version,
	}

	return outcome, nil
}

// reportTrend records and announces a breach. It runs only after the stats it
// was derived from have been stored.
func (a *API) reportTrend(ctx context.Context, outcome trendOutcome) {
	if !outcome.breached {
		return
	}

	if outcome.suppressed {
		log.Print(outcome.message)
		return
	}

	fmt.Println(outcome.message)
	log.Print(outcome.message)

	if err := a.storage.SaveBreach(ctx, outcome.breach); err != nil {
		log.Printf("Error saving breach: %v", err)
	}

	if a.notifier != nil {
		if err := a.notifier.Notify(ctx, outcome.message); err != nil {
			log.Printf("Error sending notification: %v", err)
		}
	}
}

func (a *API) handleReplay(c *gin.Context) {
	var req models.ReplayRequest
	if !bindJSON(c, &req) {
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

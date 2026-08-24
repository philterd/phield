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

package dashboard

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/philterd/phield/internal/db"
	"github.com/philterd/phield/internal/models"
)

type Dashboard struct {
	storage db.Storage
	// ephemeral is true when Phield is running without MongoDB, which the page
	// says plainly rather than letting a reader trust numbers that vanish on
	// the next restart.
	ephemeral bool
}

func New(storage db.Storage, ephemeral bool) *Dashboard {
	return &Dashboard{storage: storage, ephemeral: ephemeral}
}

// RegisterRoutes registers the dashboard UI and its JSON endpoints. These are
// not covered by PHIELD_API_KEY: the key exists to keep bad data out, and the
// dashboard only reads aggregate counts, never PII.
func (d *Dashboard) RegisterRoutes(r *gin.Engine) {
	// The dashboard is the only page Phield serves, so the root goes to it.
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/dashboard")
	})
	r.GET("/dashboard", d.serveUI)
	api := r.Group("/api/dashboard")
	{
		api.GET("/summary", d.handleSummary)
		api.GET("/alerts", d.handleAlerts)
		api.GET("/entities", d.handleEntities)
		api.GET("/flows", d.handleFlows)
		api.GET("/trends", d.handleTrends)
	}
}

func (d *Dashboard) getWindowHours(c *gin.Context) int {
	hours, err := strconv.Atoi(c.DefaultQuery("hours", "24"))
	if err != nil || hours <= 0 {
		return 24
	}
	if hours > 168 {
		return 168
	}
	return hours
}

func (d *Dashboard) handleSummary(c *gin.Context) {
	hours := d.getWindowHours(c)
	ctx := c.Request.Context()
	start := time.Now().Add(-time.Duration(hours) * time.Hour)
	end := time.Now()

	entryChan, errChan := d.storage.GetEntries(ctx, start, end)

	var totalEntries int
	sources := make(map[string]bool)
	piiTypes := make(map[string]int)
	contexts := make(map[string]bool)

	for entry := range entryChan {
		totalEntries++
		sources[entry.SourceID] = true
		contexts[entry.Context] = true
		for piiType, count := range entry.PIITypes {
			piiTypes[piiType] += count
		}
	}

	select {
	case err := <-errChan:
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	default:
	}

	breaches, err := d.storage.GetBreaches(ctx, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_entries":   totalEntries,
		"total_breaches":  len(breaches),
		"unique_sources":  len(sources),
		"unique_types":    len(piiTypes),
		"unique_contexts": len(contexts),
		"window_hours":    hours,
	})
}

func (d *Dashboard) handleAlerts(c *gin.Context) {
	hours := d.getWindowHours(c)
	ctx := c.Request.Context()
	start := time.Now().Add(-time.Duration(hours) * time.Hour)
	end := time.Now()

	breaches, err := d.storage.GetBreaches(ctx, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if breaches == nil {
		breaches = make([]models.BreachDetail, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"alerts":       breaches,
		"window_hours": hours,
	})
}

func (d *Dashboard) handleEntities(c *gin.Context) {
	hours := d.getWindowHours(c)
	ctx := c.Request.Context()
	start := time.Now().Add(-time.Duration(hours) * time.Hour)
	end := time.Now()

	entryChan, errChan := d.storage.GetEntries(ctx, start, end)

	typeTotals := make(map[string]int)
	typeTimeline := make(map[string][]timePoint)

	bucketSize := time.Hour
	if hours <= 6 {
		bucketSize = 15 * time.Minute
	} else if hours <= 24 {
		bucketSize = time.Hour
	} else {
		bucketSize = 4 * time.Hour
	}

	buckets := make(map[string]map[string]int) // bucket_key -> pii_type -> count

	for entry := range entryChan {
		bucketKey := entry.Timestamp.Truncate(bucketSize).Format(time.RFC3339)
		if buckets[bucketKey] == nil {
			buckets[bucketKey] = make(map[string]int)
		}
		for piiType, count := range entry.PIITypes {
			typeTotals[piiType] += count
			buckets[bucketKey][piiType] += count
		}
	}

	select {
	case err := <-errChan:
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	default:
	}

	for bucketKey, types := range buckets {
		for piiType, count := range types {
			typeTimeline[piiType] = append(typeTimeline[piiType], timePoint{
				Time:  bucketKey,
				Count: count,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"totals":       typeTotals,
		"timeline":     typeTimeline,
		"window_hours": hours,
	})
}

func (d *Dashboard) handleFlows(c *gin.Context) {
	hours := d.getWindowHours(c)
	ctx := c.Request.Context()
	start := time.Now().Add(-time.Duration(hours) * time.Hour)
	end := time.Now()

	entryChan, errChan := d.storage.GetEntries(ctx, start, end)

	type flowKey struct {
		Source  string
		Context string
	}
	flows := make(map[flowKey]map[string]int) // flow -> pii_type -> count

	for entry := range entryChan {
		key := flowKey{Source: entry.SourceID, Context: entry.Context}
		if flows[key] == nil {
			flows[key] = make(map[string]int)
		}
		for piiType, count := range entry.PIITypes {
			flows[key][piiType] += count
		}
	}

	select {
	case err := <-errChan:
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	default:
	}

	type flowResult struct {
		Source  string         `json:"source"`
		Context string         `json:"context"`
		Types   map[string]int `json:"types"`
		Total   int            `json:"total"`
	}

	var results []flowResult
	for key, types := range flows {
		total := 0
		for _, count := range types {
			total += count
		}
		results = append(results, flowResult{
			Source:  key.Source,
			Context: key.Context,
			Types:   types,
			Total:   total,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"flows":        results,
		"window_hours": hours,
	})
}

func (d *Dashboard) handleTrends(c *gin.Context) {
	ctx := c.Request.Context()

	allStats, err := d.storage.GetAllStats(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	hours := d.getWindowHours(c)
	start := time.Now().Add(-time.Duration(hours) * time.Hour)
	end := time.Now()

	entryChan, errChan := d.storage.GetEntries(ctx, start, end)

	type trendKey struct {
		Source  string
		Context string
		PIIType string
	}

	recentCounts := make(map[trendKey][]int)

	for entry := range entryChan {
		for piiType, count := range entry.PIITypes {
			key := trendKey{Source: entry.SourceID, Context: entry.Context, PIIType: piiType}
			recentCounts[key] = append(recentCounts[key], count)
		}
	}

	select {
	case err := <-errChan:
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	default:
	}

	type trendResult struct {
		SourceID     string  `json:"source_id"`
		Context      string  `json:"context"`
		PIIType      string  `json:"pii_type"`
		BaselineMean float64 `json:"baseline_mean"`
		RecentMean   float64 `json:"recent_mean"`
		SampleCount  int     `json:"sample_count"`
		Deviation    float64 `json:"deviation_pct"`
	}

	var results []trendResult
	for _, s := range allStats {
		key := trendKey{Source: s.SourceID, Context: s.Context, PIIType: s.PIIType}
		recent := recentCounts[key]
		var recentMean float64
		if len(recent) > 0 {
			sum := 0
			for _, v := range recent {
				sum += v
			}
			recentMean = float64(sum) / float64(len(recent))
		}

		var deviation float64
		if s.Stats.Mean > 0 {
			deviation = ((recentMean - s.Stats.Mean) / s.Stats.Mean) * 100
		}

		results = append(results, trendResult{
			SourceID:     s.SourceID,
			Context:      s.Context,
			PIIType:      s.PIIType,
			BaselineMean: s.Stats.Mean,
			RecentMean:   recentMean,
			SampleCount:  s.Stats.Count,
			Deviation:    deviation,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"trends":       results,
		"window_hours": hours,
	})
}

type timePoint struct {
	Time  string `json:"time"`
	Count int    `json:"count"`
}

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

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/philterd/phield/internal/api"
	"github.com/philterd/phield/internal/config"
	"github.com/philterd/phield/internal/db"
	"github.com/philterd/phield/internal/models"
	"github.com/philterd/phield/internal/notifier"
	"github.com/philterd/phield/internal/worker"
)

func main() {
	cfg := config.Load()

	// Connect to MongoDB
	mongoDB, err := db.Connect(cfg.MongoURI)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		if err := mongoDB.Client.Disconnect(context.Background()); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	// Channel for ingestion
	ingestChan := make(chan models.PIIEntry, 1000)

	// Start Background Worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var notifiers []notifier.Notifier
	if cfg.SlackWebhook != "" {
		notifiers = append(notifiers, notifier.NewSlackNotifier(cfg.SlackWebhook))
	}
	if cfg.PagerDutyRoutingKey != "" {
		notifiers = append(notifiers, notifier.NewPagerDutyNotifier(cfg.PagerDutyRoutingKey, cfg.PagerDutySeverity))
	}

	var n notifier.Notifier
	if len(notifiers) > 0 {
		n = notifier.NewMultiNotifier(notifiers...)
	}

	w := worker.NewWorker(mongoDB.DB, ingestChan, cfg.AlertThreshold, n)
	go w.Start(ctx)

	// Setup API
	r := gin.Default()
	a := api.NewAPI(ingestChan)
	a.RegisterRoutes(r)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		if cfg.CertFile != "" && cfg.KeyFile != "" {
			log.Printf("Phield API starting with HTTPS on port %s", cfg.Port)
			if err := srv.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %s\n", err)
			}
		} else {
			log.Printf("Phield API starting with HTTP on port %s", cfg.Port)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %s\n", err)
			}
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	cancel() // Stop background worker

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}

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

package kafka

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/philterd/phield/internal/api"
	"github.com/philterd/phield/internal/models"
	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
	api    *api.API
}

func NewConsumer(brokers string, topic string, groupID string, api *api.API) *Consumer {
	log.Printf("Initializing Kafka consumer for topic %s and brokers %s", topic, brokers)
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  strings.Split(brokers, ","),
			Topic:    topic,
			GroupID:  groupID,
			MinBytes: 10e3, // 10KB
			MaxBytes: 10e6, // 10MB
		}),
		api: api,
	}
}

func (c *Consumer) Start(ctx context.Context) {
	log.Println("Starting Kafka consumer loop...")
	for {
		m, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Printf("Kafka consumer stopping: %v", ctx.Err())
				return
			}
			log.Printf("Error reading from Kafka: %v", err)
			continue
		}

		var req models.IngestRequest
		if err := json.Unmarshal(m.Value, &req); err != nil {
			log.Printf("Error unmarshaling Kafka message: %v", err)
			continue
		}

		if err := c.api.ProcessIngest(ctx, req); err != nil {
			log.Printf("Error processing Kafka ingest: %v", err)
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

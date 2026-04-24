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
	"errors"
	"testing"
	"time"

	"github.com/philterd/phield/internal/models"
	"github.com/segmentio/kafka-go"
)

type mockKafkaReader struct {
	messages []kafka.Message
	err      error
	closed   bool
}

func (m *mockKafkaReader) ReadMessage(ctx context.Context) (kafka.Message, error) {
	if m.err != nil {
		return kafka.Message{}, m.err
	}
	if len(m.messages) == 0 {
		<-ctx.Done()
		return kafka.Message{}, ctx.Err()
	}
	msg := m.messages[0]
	m.messages = m.messages[1:]
	return msg, nil
}

func (m *mockKafkaReader) Close() error {
	m.closed = true
	return nil
}

type mockIngestProcessor struct {
	requests []models.IngestRequest
	err      error
}

func (m *mockIngestProcessor) ProcessIngest(ctx context.Context, req models.IngestRequest) error {
	m.requests = append(m.requests, req)
	return m.err
}

func TestConsumer_Start(t *testing.T) {
	req := models.IngestRequest{
		SourceID:     "test-source",
		Organization: "test-org",
		Context:      "test-context",
		PIITypes:     map[string]int{"email": 10},
	}
	val, _ := json.Marshal(req)

	mockReader := &mockKafkaReader{
		messages: []kafka.Message{
			{Value: val},
		},
	}
	mockProcessor := &mockIngestProcessor{}

	consumer := &Consumer{
		reader: mockReader,
		api:    mockProcessor,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	// Start in a goroutine because Start is an infinite loop
	go consumer.Start(ctx)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	if len(mockProcessor.requests) != 1 {
		t.Errorf("expected 1 request, got %d", len(mockProcessor.requests))
	}

	if mockProcessor.requests[0].SourceID != "test-source" {
		t.Errorf("expected source-id test-source, got %s", mockProcessor.requests[0].SourceID)
	}
}

func TestConsumer_Start_UnmarshalError(t *testing.T) {
	mockReader := &mockKafkaReader{
		messages: []kafka.Message{
			{Value: []byte("invalid json")},
		},
	}
	mockProcessor := &mockIngestProcessor{}

	consumer := &Consumer{
		reader: mockReader,
		api:    mockProcessor,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	go consumer.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	if len(mockProcessor.requests) != 0 {
		t.Errorf("expected 0 requests due to unmarshal error, got %d", len(mockProcessor.requests))
	}
}

func TestConsumer_Start_ReadError(t *testing.T) {
	mockReader := &mockKafkaReader{
		err: errors.New("kafka error"),
	}
	mockProcessor := &mockIngestProcessor{}

	consumer := &Consumer{
		reader: mockReader,
		api:    mockProcessor,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	go consumer.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	if len(mockProcessor.requests) != 0 {
		t.Errorf("expected 0 requests due to read error, got %d", len(mockProcessor.requests))
	}
}

func TestConsumer_Close(t *testing.T) {
	mockReader := &mockKafkaReader{}
	consumer := &Consumer{
		reader: mockReader,
	}

	if err := consumer.Close(); err != nil {
		t.Errorf("unexpected error closing consumer: %v", err)
	}

	if !mockReader.closed {
		t.Error("expected reader to be closed")
	}
}

func TestNewConsumer(t *testing.T) {
	processor := &mockIngestProcessor{}
	consumer := NewConsumer("localhost:9092", "test-topic", "test-group", processor)

	if consumer == nil {
		t.Fatal("expected consumer to be non-nil")
	}

	if consumer.api != processor {
		t.Errorf("expected processor to be set")
	}

	if consumer.reader == nil {
		t.Errorf("expected reader to be set")
	}
}

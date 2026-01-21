package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	log.Println("Starting Consumer...")

	// 1. Configure Reader
	// We use FetchMessage + CommitMessages for "At Least Once" delivery
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{os.Getenv("KAFKA_BROKER")},
		GroupID: "whatsapp-group",
		Topic:   "whatsapp_messages",
		// Prevents the consumer from timing out during low traffic
		MaxWait: 500 * time.Millisecond, 
	})
	defer reader.Close()

	// Use a background context for the lifecycle of the consumer
	ctx := context.Background()

	for {
		// 2. Fetch the message from Kafka
		m, err := reader.FetchMessage(ctx)
		if err != nil {
			log.Printf("Reader Error: %v", err)
			continue
		}

		log.Printf("Processing queued message for User: %s", string(m.Key))

		// 3. Robust Retry Logic with a Max Limit
		success := false
		maxRetries := 5 // Prevent "Poison Pill" messages from blocking the queue
		
		for i := 0; i < maxRetries; i++ {
			if sendToFastAPI(m.Value) {
				success = true
				break
			}
			
			log.Printf("Retry %d/%d for user %s in 5s...", i+1, maxRetries, string(m.Key))
			time.Sleep(20 * time.Second)
		}

		// 4. Commit or Discard
		if success {
			if err := reader.CommitMessages(ctx, m); err != nil {
				log.Printf("Failed to commit message: %v", err)
			} else {
				log.Printf("Successfully delivered and committed message for: %s", string(m.Key))
			}
		} else {
			// If it fails after 5 retries, we log a critical error and move on
			// to prevent the entire bot from being stuck forever.
			log.Printf("CRITICAL: Message for %s discarded after %d failed attempts", string(m.Key), maxRetries)
			reader.CommitMessages(ctx, m) 
		}
	}
}

func sendToFastAPI(payload []byte) bool {
	url := os.Getenv("FASTAPI_URL")
	
	// Create a client with a timeout so the consumer doesn't hang
	client := &http.Client{Timeout: 10 * time.Second}
	
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("FastAPI unreachable: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("FastAPI returned error: %d", resp.StatusCode)
		return false
	}

	return true
}
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
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{os.Getenv("KAFKA_BROKER")},
		GroupID: "whatsapp-group",
		Topic:   "whatsapp_messages",
	})
	defer reader.Close()

	for {
		m, err := reader.FetchMessage(context.Background())
		if err != nil {
			// log.Printf("Reader Error: %v", err)
			continue
		}

		// Keep trying until FastAPI is up
		for {
			resp, err := http.Post(os.Getenv("FASTAPI_URL"), "application/json", bytes.NewBuffer(m.Value))
			if err == nil && resp.StatusCode == http.StatusOK {
				reader.CommitMessages(context.Background(), m)
				// log.Printf("Successfully delivered queued message for user: %s", string(m.Key))
				break
			}
			// log.Printf("Retrying queued message for %s in 5s...", string(m.Key))
			time.Sleep(5 * time.Second)
		}
	}
}

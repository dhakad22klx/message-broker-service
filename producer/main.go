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

var kafkaWriter *kafka.Writer

func main() {


	// Root health check
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ok, successful"))
	})

	kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(os.Getenv("KAFKA_BROKER")),
		Topic:    "whatsapp_messages",
		Balancer: &kafka.Hash{}, // Vital: Hashes Key (UserID) to same partition
	}
	defer kafkaWriter.Close()

	http.HandleFunc("/webhook", handleWebhook)
	// log.Println("Producer running on :8080")
	// log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {

	// 1. Meta Verification (GET)
	if r.Method == "GET" {
		if r.URL.Query().Get("hub.verify_token") == os.Getenv("VERIFY_TOKEN") {
			w.Write([]byte(r.URL.Query().Get("hub.challenge")))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// 2. Fallback Logic: Try FastAPI, then Kafka
	payload := []byte(`{"event": "placeholder_payload"}`) // Replace with actual parsing
	userID := "user_phone_number"                      // Replace with parsed fromID

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(os.Getenv("FASTAPI_URL"), "application/json", bytes.NewBuffer(payload))

	if err == nil && resp.StatusCode == http.StatusOK {
		// log.Println("Directly processed via FastAPI")
		w.WriteHeader(http.StatusOK)
		return
	}

	// 3. Store in Kafka if FastAPI is down
	// log.Printf("FastAPI down, queuing message for %s", userID)
	err = kafkaWriter.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(userID),
		Value: payload,
	})
	if err != nil {
		// log.Printf("Kafka Error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
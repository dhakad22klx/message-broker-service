package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

// We define the full struct to ensure we can still access specific fields (like 'From')
// even though we are sending the raw '[]byte' to the downstream services.
type WhatsAppRequest struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Value struct {
				MessagingProduct string `json:"messaging_product"`
				Metadata         struct {
					DisplayPhoneNumber string `json:"display_phone_number"`
					PhoneNumberID      string `json:"phone_number_id"`
				} `json:"metadata"`
				Contacts []struct {
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
					WaID string `json:"wa_id"`
				} `json:"contacts"`
				Messages []struct {
					From      string `json:"from"`
					ID        string `json:"id"`
					Timestamp string `json:"timestamp"`
					Text      struct {
						Body string `json:"body"`
					} `json:"text"`
					Type string `json:"type"`
				} `json:"messages"`
			} `json:"value"`
			Field string `json:"field"`
		} `json:"changes"`
	} `json:"entry"`
}

var kafkaWriter *kafka.Writer

func main() {
	// Initialize Kafka Writer
	kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(os.Getenv("KAFKA_BROKER")),
		Topic:    "whatsapp_messages",
		Balancer: &kafka.Hash{}, // Vital: Keeps messages from the same user in order
	}
	defer kafkaWriter.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Producer is active"))
	})

	http.HandleFunc("/webhook", handleWebhook)

	log.Println("🚀 Producer running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	// 1. Handle Meta Verification (GET)
	if r.Method == http.MethodGet {
		verifyToken := os.Getenv("VERIFY_TOKEN")
		if r.URL.Query().Get("hub.verify_token") == verifyToken {
			log.Println("✅ Webhook verified by Meta")
			w.Write([]byte(r.URL.Query().Get("hub.challenge")))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// 2. Read RAW Body (This keeps the COMPLETE JSON)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("❌ Error reading request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 3. Extract the 'From' number to use as a Kafka Key
	// We unmarshal into the struct just to find the ID, but we DON'T use this struct for forwarding.
	var waReq WhatsAppRequest
	userID := "system"
	userName := "Unknown"
	if err := json.Unmarshal(rawBody, &waReq); err == nil {
		if len(waReq.Entry) > 0 && len(waReq.Entry[0].Changes) > 0 {
			val := waReq.Entry[0].Changes[0].Value
			if len(val.Messages) > 0 {
				userID = val.Messages[0].From
			}
			if len(val.Contacts) > 0 {
				userName = val.Contacts[0].Profile.Name
			}
		}
	}

	log.Printf("📥 Incoming Webhook from %s (%s)", userName, userID)

	// 4. Try sending COMPLETE raw JSON to FastAPI
	if forwardToFastAPI(rawBody) {
		log.Printf("🏁 Success: Full payload sent to FastAPI for %s", userID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 5. Fallback: Save COMPLETE raw JSON to Kafka
	log.Printf("🛰️ Fallback: FastAPI down. Queuing full payload in Kafka for %s", userID)
	err = kafkaWriter.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(userID),
		Value: rawBody, // This sends the 100% original JSON bytes
	})

	if err != nil {
		log.Printf("❌ Kafka Error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func forwardToFastAPI(payload []byte) bool {
	apiURL := os.Getenv("FASTAPI_URL")
	if apiURL == "" {
		return false
	}

	// 5 second timeout to prevent the server from hanging
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(payload))

	if err != nil {
		log.Printf("⚠️ FastAPI Connection Error: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ FastAPI returned non-200 status: %d", resp.StatusCode)
		return false
	}

	return true
}
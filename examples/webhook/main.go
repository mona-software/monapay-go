package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	monapay "github.com/mona-software/monapay-go"
)

func main() {
	secret := os.Getenv("MONA_WEBHOOK_SECRET")
	http.HandleFunc("/webhooks/monapay", func(w http.ResponseWriter, r *http.Request) {
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		result, err := monapay.VerifyWebhook(rawBody, r.Header.Get("X-Mona-Timestamp"), r.Header.Get("X-Mona-Signature"), secret)
		if err != nil || !result.OK {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		payload, _ := result.Payload.(map[string]any)
		log.Printf("transaction_code=%v", payload["transaction_code"])
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

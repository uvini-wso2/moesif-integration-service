package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/uvini-wso2/plg-activity-service/internal/handler"
	"github.com/uvini-wso2/plg-activity-service/internal/moesif"
)

func main() {
	// Load .env if present — silently ignored if it doesn't exist.
	_ = godotenv.Load()

	apiKey := os.Getenv("MOESIF_API_KEY")
	baseURL := os.Getenv("MOESIF_BASE_URL")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	client := moesif.NewClient(moesif.Config{
		APIKey:  apiKey,
		BaseURL: baseURL,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /events", handler.Events(client))

	slog.Info("starting server", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server failed", "error", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

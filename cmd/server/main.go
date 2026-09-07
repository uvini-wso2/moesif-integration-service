package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/uvini-wso2/moesif-integration-service/internal/moesif"
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

	// One-off test call at startup — proves the request shape works,
	// even though it will fail auth with a placeholder key.
	testMoesifCall(client)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)

	slog.Info("starting server", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server failed", "error", err)
	}
}

func testMoesifCall(client *moesif.Client) {
	postFilter := map[string]interface{}{
		"bool": map[string]interface{}{
			"should": map[string]interface{}{
				"match_all": map[string]interface{}{},
			},
		},
	}

	result, err := client.SearchEvents("-1d", "now", postFilter)
	if err != nil {
		slog.Warn("moesif test call failed (expected with placeholder key)", "error", err)
		return
	}
	slog.Info("moesif test call succeeded", "response", string(result))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

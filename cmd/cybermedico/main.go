package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("resources/config/cybermedico/compose.env"); err != nil {
		slog.Error("No .env file found or error loading it", "error", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := fmt.Fprintln(w, "ok"); err != nil {
			slog.Error("failed to write health response", "error", err)
		}
	})

	addr := ":" + os.Getenv("PORT")
	httpServer := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	slog.Info("Starting HTTP server", "addr", addr)
	if err := httpServer.ListenAndServe(); err != nil {
		slog.Error("HTTP server stopped", "error", err)
		os.Exit(1)
	}
}

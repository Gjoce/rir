package main

import (
	"context"
	"cybermedico/cmd/cybermedico/api/errors"
	"cybermedico/cmd/cybermedico/api/html"
	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/api/util"
	"cybermedico/cmd/cybermedico/ogen"
	"embed"
	stderrors "errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

//go:embed static
var staticFiles embed.FS

func main() {
	ctx := context.Background()

	if err := godotenv.Load("resources/config/cybermedico/compose.env"); err != nil {
		log.Logger.Error("No .env file found or error loading it", "error", err)
	}

	htmlServer := initializeHTMLServer()
	httpServer := initializeHTTPServer(htmlServer)
	startHTTPServer(httpServer)

	waitForShutdown()
	shutdownHTTPServer(ctx, httpServer)
}

func initializeHTMLServer() *ogen.Server {
	htmlServer, err := ogen.NewServer(struct {
		html.RootHandler
		html.ThemeHandler
		html.DashboardHandler
	}{
		html.RootHandler{},
		html.ThemeHandler{},
		html.DashboardHandler{},
	},
		ogen.WithErrorHandler(errors.ErrorHandler),
		ogen.WithNotFound(errors.NotFoundHandler),
	)
	if err != nil {
		log.Logger.Error("Failed to create HTML server", "error", err)
		panic(err)
	}

	return htmlServer
}

func initializeHTTPServer(htmlServer *ogen.Server) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/", util.InjectRequestToContext(htmlServer))
	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func startHTTPServer(httpServer *http.Server) {
	log.Logger.Info("Starting API", "addr", httpServer.Addr)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !stderrors.Is(err, http.ErrServerClosed) {
			log.Logger.Error("Server error", "error", err)
			panic(err)
		}
	}()
}

func waitForShutdown() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	<-signalChan
}

func shutdownHTTPServer(ctx context.Context, httpServer *http.Server) {
	// Long-lived SSE connections would otherwise block shutdown forever.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Logger.Error("Failed to shutdown HTTP server", "error", err)
	}
}

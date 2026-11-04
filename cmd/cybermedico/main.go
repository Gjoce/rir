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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

//go:embed static
var staticFiles embed.FS

//go:embed migrations/*.up.sql
var migrationsFS embed.FS

func main() {
	ctx := context.Background()

	if err := godotenv.Load("resources/config/cybermedico/compose.env"); err != nil {
		log.Logger.Error("No .env file found or error loading it", "error", err)
	}

	dbPool := initializeDatabase()
	defer dbPool.Close()

	htmlServer := initializeHTMLServer()
	httpServer := initializeHTTPServer(htmlServer)
	startHTTPServer(httpServer)

	waitForShutdown()
	shutdownHTTPServer(ctx, httpServer)
}

func initializeDatabase() *pgxpool.Pool {
	if err := runMigrations(); err != nil {
		log.Logger.Error("Database migrations failed", "error", err)
		panic(err)
	}

	dbPool, err := setupDB()
	if err != nil {
		log.Logger.Error("Database connection failed", "error", err)
		panic(err)
	}

	return dbPool
}

func setupDB() (*pgxpool.Pool, error) {
	connStr := os.Getenv("POSTGRES_URL")
	if connStr == "" {
		return nil, fmt.Errorf("POSTGRES_URL is not set")
	}

	dbPool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	return dbPool, nil
}

func runMigrations() error {
	//Update conection string to use pgx5 driver
	connectionString := os.Getenv("POSTGRES_URL")
	connectionString = strings.Replace(connectionString, "postgresql://", "pgx5://", 1)

	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	migrations, err := migrate.NewWithSourceInstance("iofs", d, connectionString)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	version, dirty, err := migrations.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	if dirty {
		if version == 0 {
			return fmt.Errorf("database is in a dirty state at version 0, skipping force")
		} else {
			if version > 1<<31-1 { // Prevent overflow on 32-bit systems
				return fmt.Errorf("migration version too large: %d", version)
			}
			forceVer := int(version - 1)
			if err := migrations.Force(forceVer); err != nil {
				return fmt.Errorf("failed to force clean state: %w", err)
			}
			log.Logger.Info("Forced database to clean state")
		}
	}

	err = migrations.Up()
	if err == migrate.ErrNoChange {
		log.Logger.Info("No new migrations to apply")

		return nil
	}

	if err != nil {
		currentVersion, _, _ := migrations.Version()
		latestMigrationVersion, _ := getLatestMigrationVersion()

		if currentVersion > latestMigrationVersion {
			log.Logger.Info("Database is ahead of the latest migration version")

			return nil
		}

		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Logger.Info("Migrations applied successfully")

	return nil
}

func getLatestMigrationVersion() (uint, error) {
	files, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return 0, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var latestVersion uint
	for _, file := range files {
		var version uint
		if _, err := fmt.Sscanf(file.Name(), "%d_", &version); err != nil {
			log.Logger.Warn("failed to parse migration version from filename", "filename", file.Name(), "error", err)

			continue
		}
		if version > latestVersion {
			latestVersion = version
		}
	}

	return latestVersion, nil
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

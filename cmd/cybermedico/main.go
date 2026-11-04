package main

import (
	"context"
	"cybermedico/cmd/cybermedico/api/errors"
	"cybermedico/cmd/cybermedico/api/html"
	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/api/util"
	"cybermedico/cmd/cybermedico/email"
	"cybermedico/cmd/cybermedico/ogen"
	"cybermedico/workers"
	"embed"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
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
	if err := runSeeds(dbPool); err != nil {
		log.Logger.Warn("Failed to apply seed data", "error", err)
	}
	defer dbPool.Close()

	mailer := initializeMailer()

	riverClient := initializeRiver(ctx, dbPool, &mailer)
	defer shutdownRiver(ctx, riverClient)

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

func runSeeds(dbPool *pgxpool.Pool) error {
	seedFilePath := os.Getenv("CYBERMEDICO_SEED_FILE")
	if seedFilePath == "" {
		return fmt.Errorf("CYBERMEDICO_SEED_FILE not found")
	}
	// Prevent path traversal
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	absSeedPath, err := filepath.Abs(seedFilePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for seed file: %w", err)
	}
	if !strings.HasPrefix(absSeedPath, currentDir+string(filepath.Separator)) {
		return fmt.Errorf("seed file path is outside allowed directory")
	}
	seedSQL, err := os.ReadFile(absSeedPath) // #nosec G703,G304
	if err != nil {
		return fmt.Errorf("failed to read seed file from %s: %w", seedFilePath, err)
	}

	_, err = dbPool.Exec(context.Background(), string(seedSQL))
	if err != nil {
		return fmt.Errorf("failed to execute seed sql: %w", err)
	}

	log.Logger.Info("Seed file applied successfully")

	return nil
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

func initializeMailer() email.Emailer {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	emailSenderName := os.Getenv("EMAIL_SENDER_NAME")
	emailSenderAddress := os.Getenv("EMAIL_SENDER_ADDRESS")
	smtpUsername := os.Getenv("SMTP_USERNAME")
	smtpPassword := os.Getenv("SMTP_PASSWORD")

	emailConfigured := smtpHost != "" && smtpPort != "" && emailSenderAddress != ""

	if !emailConfigured {
		log.Logger.Warn("SMTP not configured, emails will be dropped")
	}

	var auth smtp.Auth = nil
	if smtpUsername != "" && smtpPassword != "" && smtpHost != "" {
		auth = smtp.PlainAuth("", smtpUsername, smtpPassword, smtpHost)
	}

	mailer := email.Emailer{
		Address: func() string {
			if smtpHost == "" {
				smtpHost = "localhost"
			}
			if smtpPort == "" {
				smtpPort = "1025"
			}

			return smtpHost + ":" + smtpPort
		}(),
		Sender: email.EmailAddress{
			Name: func() string {
				if emailSenderName == "" {
					emailSenderName = "CyberMedico"
				}

				return emailSenderName
			}(),
			Email: func() string {
				if emailSenderAddress == "" {
					emailSenderAddress = "noreply@cybermedico.local"
				}

				return emailSenderAddress
			}(),
		},
		Auth: auth,
	}

	log.Logger.Info("Initialized mailer", "smtp_address", mailer.Address, "sender_email", mailer.Sender.Email)

	return mailer
}

func initializeRiver(ctx context.Context, dbPool *pgxpool.Pool, mailer *email.Emailer) *river.Client[pgx.Tx] {
	migrator, err := rivermigrate.New(riverpgxv5.New(dbPool), nil)
	if err != nil {
		log.Logger.Error("Failed to create River migrator", "error", err)
		panic(fmt.Errorf("failed to create river migrator: %w", err))
	}

	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, &rivermigrate.MigrateOpts{})
	if err != nil {
		log.Logger.Error("River migration error", "error", err)
		panic(fmt.Errorf("river migration error: %w", err))
	}
	log.Logger.Info("River migrations applied successfully")

	riverWorkers := river.NewWorkers()

	doctorAnswerEmailWorker := workers.NewDoctorAnswerEmailWorker(mailer, log.Logger)

	river.AddWorker(riverWorkers, doctorAnswerEmailWorker)

	riverClient, err := river.NewClient(riverpgxv5.New(dbPool), &river.Config{
		Logger: log.Logger,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
		},
		Workers: riverWorkers,
	})
	if err != nil {
		log.Logger.Error("Failed to create River client", "error", err)
		panic(fmt.Errorf("failed to create River client: %w", err))
	}

	if err := riverClient.Start(ctx); err != nil {
		log.Logger.Error("Failed to start River client", "error", err)
		panic(fmt.Errorf("failed to start River client: %w", err))
	}
	log.Logger.Info("River client started successfully")

	return riverClient
}

func shutdownRiver(ctx context.Context, riverClient *river.Client[pgx.Tx]) {
	if err := riverClient.Stop(ctx); err != nil {
		log.Logger.Error("Failed to stop river client", "error", err)
	} else {
		log.Logger.Info("River client stopped successfully")
	}
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

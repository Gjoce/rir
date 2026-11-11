package main

import (
	"context"
	"crypto/sha256"
	"cybermedico/authorization"
	"cybermedico/cmd/cybermedico/api/errors"
	"cybermedico/cmd/cybermedico/api/html"
	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/api/util"
	"cybermedico/cmd/cybermedico/email"
	"cybermedico/cmd/cybermedico/ogen"
	"cybermedico/cmd/cybermedico/signing"
	"cybermedico/cmd/cybermedico/storage"
	"cybermedico/video"
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
	_ "time/tzdata" // Workers format times in Europe/Skopje, wherever the app runs.

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

// maxRequestBytes fits the largest valid order: 5 attachments of 10 MB and the form fields.
const maxRequestBytes = 52 * 1024 * 1024

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

	keyCipher := initializeSigning()
	if err := ensureDoctorKeys(ctx, dbPool, keyCipher); err != nil {
		log.Logger.Error("Failed to create doctor signing keys", "error", err)
		panic(err)
	}

	mailer := initializeMailer()

	storageService := initializeStorage(ctx)

	videoURLBuilder := initializeVideoURLBuilder()

	riverClient := initializeRiver(ctx, dbPool, &mailer, videoURLBuilder, storageService, keyCipher)
	defer shutdownRiver(ctx, riverClient)

	authService, _ := initializeAuth(ctx, dbPool)

	videoAuthorize := html.NewVideoJoinAuthorizer(dbPool, authService, videoURLBuilder.JoinSecret)
	signalHandler := initializeVideo(ctx, dbPool, videoAuthorize)

	htmlServer := initializeHTMLServer(dbPool, authService, storageService, riverClient, videoURLBuilder, videoAuthorize)
	httpServer := initializeHTTPServer(htmlServer, authService, signalHandler)
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

func initializeStorage(ctx context.Context) *storage.Service {
	svc, err := storage.New(ctx, storage.Config{
		Endpoint:         os.Getenv("S3_ENDPOINT"),
		Region:           os.Getenv("S3_REGION"),
		Bucket:           os.Getenv("S3_BUCKET"),
		AccessKey:        os.Getenv("S3_ACCESS_KEY"),
		SecretKey:        os.Getenv("S3_SECRET_KEY"),
		SignaturesBucket: os.Getenv("S3_SIGNATURES_BUCKET"),
	})
	if err != nil {
		log.Logger.Error("Failed to initialize storage service", "error", err)
		panic(err)
	}

	log.Logger.Info("Initialized production-ready storage service")

	return svc
}

// initializeSigning reads SIGNING_KEY, the key that encrypts doctors' private
// signing keys. Generate one with: openssl rand -base64 32
func initializeSigning() *signing.KeyCipher {
	keyCipher, err := signing.NewKeyCipher(os.Getenv("SIGNING_KEY"))
	if err != nil {
		panic(fmt.Errorf("SIGNING_KEY: %w", err))
	}

	return keyCipher
}

// ensureDoctorKeys gives every doctor without a key pair a new one, so a doctor
// can sign as soon as their account exists.
func ensureDoctorKeys(ctx context.Context, dbPool *pgxpool.Pool, keyCipher *signing.KeyCipher) error {
	rows, err := dbPool.Query(ctx, `
		SELECT u.id::text
		FROM users u
		JOIN users_roles ur ON ur.user_id = u.id
		WHERE ur.role_id = 'doctor' AND u.public_key IS NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to query doctors without keys: %w", err)
	}
	doctorIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("failed to read doctors without keys: %w", err)
	}

	for _, doctorID := range doctorIDs {
		publicKey, encryptedPrivateKey, err := keyCipher.GenerateKeyPair(doctorID)
		if err != nil {
			return err
		}

		// The condition keeps a key that another instance created meanwhile.
		_, err = dbPool.Exec(ctx, `
			UPDATE users SET public_key = @public_key, encrypted_private_key = @encrypted_private_key
			WHERE id = @id AND public_key IS NULL
		`, pgx.NamedArgs{"id": doctorID, "public_key": publicKey, "encrypted_private_key": encryptedPrivateKey})
		if err != nil {
			return fmt.Errorf("failed to store doctor key pair: %w", err)
		}
	}

	if len(doctorIDs) > 0 {
		log.Logger.Info("Created signing keys for doctors", "count", len(doctorIDs))
	}

	return nil
}

// initializeVideoURLBuilder reads the env vars needed to mint patient join links.
//
// Required env vars:
//   - WEBRTC_JOIN_SECRET — any non-empty string; SHA-256 derives the HMAC key for patient tokens
//   - PUBLIC_BASE_URL    — origin used to build patient email links (e.g. https://cybermedico.net)
func initializeVideoURLBuilder() *video.URLBuilder {
	joinSecretEnv := os.Getenv("WEBRTC_JOIN_SECRET")
	if joinSecretEnv == "" {
		panic("WEBRTC_JOIN_SECRET is not set")
	}

	// Derive a 32-byte HMAC key from whatever string ops provided. SHA-256
	// gives full strength regardless of how the env var was generated.
	joinSecretHash := sha256.Sum256([]byte(joinSecretEnv))

	urlBuilder, err := video.NewURLBuilder(os.Getenv("PUBLIC_BASE_URL"), joinSecretHash[:])
	if err != nil {
		log.Logger.Error("Failed to initialize video URL builder", "error", err)
		panic(err)
	}

	return urlBuilder
}

func initializeRiver(ctx context.Context, dbPool *pgxpool.Pool, mailer *email.Emailer, videoURLBuilder *video.URLBuilder, storageService *storage.Service, keyCipher *signing.KeyCipher) *river.Client[pgx.Tx] {
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

	doctorAnswerEmailWorker := workers.NewDoctorAnswerEmailWorker(dbPool, storageService, mailer, log.Logger)

	orderConfirmationEmailWorker := workers.NewOrderConfirmationEmailWorker(dbPool, mailer, log.Logger)

	river.AddWorker(riverWorkers, doctorAnswerEmailWorker)
	river.AddWorker(riverWorkers, orderConfirmationEmailWorker)
	river.AddWorker(riverWorkers, workers.NewScheduledVideoEmailWorker(dbPool, mailer, videoURLBuilder, log.Logger))
	river.AddWorker(riverWorkers, workers.NewVideoCallReminderWorker(dbPool, mailer, videoURLBuilder, log.Logger))
	river.AddWorker(riverWorkers, workers.NewConsultationSignatureWorker(dbPool, storageService, keyCipher, log.Logger))

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

func initializeAuth(ctx context.Context, db *pgxpool.Pool) (*html.AuthService, authorization.Authorizer) {
	authorizer, err := authorization.NewAuthorizer(ctx, db)
	if err != nil {
		log.Logger.Error("Failed to initialize Authorizer", "error", err)
		panic(err)
	}

	authService, err := html.NewAuthService(db, authorizer)
	if err != nil {
		log.Logger.Error("Failed to initialize AuthService", "error", err)
		panic(err)
	}

	return authService, authorizer
}

func initializeHTMLServer(dbPool *pgxpool.Pool, authService *html.AuthService, storageService *storage.Service, riverClient *river.Client[pgx.Tx], videoURLBuilder *video.URLBuilder, videoAuthorize video.AuthorizeFunc) *ogen.Server {
	authHandler := html.NewAuthHandler(dbPool, authService)

	htmlServer, err := ogen.NewServer(struct {
		html.RootHandler
		html.SpecialistsHandler
		html.OrdersHandler
		html.ThemeHandler
		html.AuthHandler
		html.DashboardHandler
		html.ConsultationsHandler
		html.VideoPageHandler
		html.ICEHandler
	}{
		html.NewRootHandler(dbPool),
		html.NewSpecialistsHandler(dbPool),
		html.NewOrdersHandler(dbPool, storageService, riverClient),
		html.ThemeHandler{},
		authHandler,
		html.NewDashboardHandler(authService),
		html.NewConsultationsHandler(dbPool, authService, storageService, riverClient),
		html.NewVideoPageHandler(dbPool, authService, videoURLBuilder.JoinSecret),
		html.NewICEHandler(initializeTURNConfig(), videoAuthorize),
	},
		authHandler,
		ogen.WithErrorHandler(errors.ErrorHandler),
		ogen.WithNotFound(errors.NotFoundHandler),
	)
	if err != nil {
		log.Logger.Error("Failed to create HTML server", "error", err)
		panic(err)
	}

	return htmlServer
}

// initializeVideo wires up the broker and the signaling handler for video calls.
func initializeVideo(ctx context.Context, dbPool *pgxpool.Pool, authorize video.AuthorizeFunc) http.Handler {
	broker := video.NewBroker()
	go runBrokerEvictor(ctx, broker)

	return video.NewSignalingHandler(broker, authorize, video.NewPGEventRecorder(dbPool))
}

// initializeTURNConfig reads the TURN settings, or returns nil when they are not
// set. Calls then only connect directly, which works when both sides share a
// network but not across most home and office routers.
//
// Required env vars:
//   - TURN_HOSTNAME       — public hostname of coturn
//   - TURN_SHARED_SECRET  — coturn's static-auth-secret
//
// Optional:
//   - TURN_CREDENTIAL_TTL — Go duration, defaults to 2h
func initializeTURNConfig() *html.TURNConfig {
	turnHost := os.Getenv("TURN_HOSTNAME")
	turnSecret := os.Getenv("TURN_SHARED_SECRET")

	missing := []string{}
	if turnHost == "" {
		missing = append(missing, "TURN_HOSTNAME")
	}
	if turnSecret == "" {
		missing = append(missing, "TURN_SHARED_SECRET")
	}
	if len(missing) > 0 {
		log.Logger.Warn("TURN config incomplete, video calls will only connect directly",
			"missing", strings.Join(missing, ","),
		)

		return nil
	}

	credTTL := 2 * time.Hour
	if v := os.Getenv("TURN_CREDENTIAL_TTL"); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil || parsed <= 0 {
			log.Logger.Warn("invalid TURN_CREDENTIAL_TTL, using default", "value", v, "default", credTTL)
		} else {
			credTTL = parsed
		}
	}

	return &html.TURNConfig{
		Hostname:      turnHost,
		SharedSecret:  []byte(turnSecret),
		CredentialTTL: credTTL,
	}
}

// runBrokerEvictor calls EvictIdle on a fixed schedule until ctx is done.
func runBrokerEvictor(ctx context.Context, broker *video.Broker) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n := broker.EvictIdle(); n > 0 {
				log.Logger.Debug("WebRTC broker: evicted idle rooms", "count", n)
			}
		}
	}
}

func initializeHTTPServer(htmlServer *ogen.Server, authService *html.AuthService, signalHandler http.Handler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/", authService.SessionMiddleware(util.InjectRequestToContext(htmlServer)))
	// The session tells the authorizer whether a doctor is calling. Patients have no
	// session; their link token is checked instead.
	mux.Handle("GET /ws/video/{id}/signal", authService.SessionMiddleware(signalHandler))
	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &http.Server{
		Addr: ":" + port,
		// Multipart uploads are written to disk before a handler can check them, and
		// the order form is public, so the limit has to apply before parsing.
		Handler:           http.MaxBytesHandler(mux, maxRequestBytes),
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

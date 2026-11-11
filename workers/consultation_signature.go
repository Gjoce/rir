package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/signing"
	"cybermedico/cmd/cybermedico/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// ConsultationSignatureArgs signs a finished consultation's opinion. It is
// enqueued in the same transaction that finishes the consultation.
type ConsultationSignatureArgs struct {
	ConsultationID string `json:"consultation_id"`
}

func (ConsultationSignatureArgs) Kind() string { return "consultation_signature" }

// ConsultationSignatureWorker signs the consultation data and the opinion with
// the doctor's key and stores the record write-once in the signatures bucket.
type ConsultationSignatureWorker struct {
	river.WorkerDefaults[ConsultationSignatureArgs]
	db        *pgxpool.Pool
	storage   *storage.Service
	keyCipher *signing.KeyCipher
	logger    *log.LoggerType
}

func NewConsultationSignatureWorker(db *pgxpool.Pool, storage *storage.Service, keyCipher *signing.KeyCipher, logger *log.LoggerType) *ConsultationSignatureWorker {
	return &ConsultationSignatureWorker{
		db:        db,
		storage:   storage,
		keyCipher: keyCipher,
		logger:    logger,
	}
}

func (w *ConsultationSignatureWorker) Work(ctx context.Context, job *river.Job[ConsultationSignatureArgs]) error {
	consultationID := job.Args.ConsultationID

	var signatureKey *string
	err := w.db.QueryRow(ctx, `
		SELECT signature_doctor_url FROM consultations WHERE id = @id
	`, pgx.NamedArgs{"id": consultationID}).Scan(&signatureKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return river.JobCancel(fmt.Errorf("consultation %s not found", consultationID))
	}
	if err != nil {
		return fmt.Errorf("failed to query consultation: %w", err)
	}
	if signatureKey != nil {
		// Signed by an earlier attempt.
		return nil
	}

	payload, err := signing.LoadPayload(ctx, w.db, consultationID)
	if errors.Is(err, signing.ErrNotFinished) {
		return river.JobCancel(err)
	}
	if err != nil {
		return err
	}

	record, err := w.sign(ctx, payload)
	if err != nil {
		return err
	}

	key, err := w.store(ctx, payload, record)
	if err != nil {
		return err
	}

	_, err = w.db.Exec(ctx, `
		UPDATE consultations SET signature_doctor_url = @key
		WHERE id = @id AND signature_doctor_url IS NULL
	`, pgx.NamedArgs{"id": consultationID, "key": key})
	if err != nil {
		return fmt.Errorf("failed to store signature key: %w", err)
	}

	w.logger.Info("consultation opinion signed", "consultation_id", consultationID)

	return nil
}

func (w *ConsultationSignatureWorker) sign(ctx context.Context, payload signing.SignableConsultation) (signing.SignatureRecord, error) {
	var publicKey, encryptedPrivateKey []byte
	var revokedAt *time.Time
	err := w.db.QueryRow(ctx, `
		SELECT public_key, encrypted_private_key, key_revoked_at FROM users WHERE id = @id
	`, pgx.NamedArgs{"id": payload.DoctorID}).Scan(&publicKey, &encryptedPrivateKey, &revokedAt)
	if err != nil {
		return signing.SignatureRecord{}, fmt.Errorf("failed to query doctor key: %w", err)
	}
	if publicKey == nil || encryptedPrivateKey == nil {
		// Keys are created at startup; retrying picks them up after a restart.
		return signing.SignatureRecord{}, fmt.Errorf("doctor %s has no signing key", payload.DoctorID)
	}
	if revokedAt != nil {
		return signing.SignatureRecord{}, river.JobCancel(fmt.Errorf("doctor %s signing key is revoked", payload.DoctorID))
	}

	priv, err := w.keyCipher.PrivateKey(payload.DoctorID, publicKey, encryptedPrivateKey)
	if err != nil {
		return signing.SignatureRecord{}, err
	}
	defer signing.ZeroPrivateKey(priv)

	payload.SignedAt = time.Now().UTC().Format(time.RFC3339)

	return signing.Sign(priv, payload)
}

// store uploads the record and returns its key. If an earlier attempt already
// stored a record, that one is kept, as long as it is a valid signature of
// this consultation's data.
func (w *ConsultationSignatureWorker) store(ctx context.Context, payload signing.SignableConsultation, record signing.SignatureRecord) (string, error) {
	recordJSON, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("failed to marshal signature record: %w", err)
	}

	key, err := w.storage.UploadSignature(ctx, payload.ConsultationID, recordJSON)
	if !errors.Is(err, storage.ErrSignatureExists) {
		return key, err
	}

	existingJSON, err := w.storage.GetSignature(ctx, payload.ConsultationID)
	if err != nil {
		return "", err
	}
	var existing signing.SignatureRecord
	if err := json.Unmarshal(existingJSON, &existing); err != nil || !signing.VerifyAgainst(existing, payload) {
		// Never overwritten: someone has to look at it.
		return "", river.JobCancel(fmt.Errorf("a different signature is already stored for consultation %s", payload.ConsultationID))
	}

	return key, nil
}

package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/email"
	"cybermedico/cmd/cybermedico/signing"
	"cybermedico/cmd/cybermedico/storage"
	"cybermedico/email_templates"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// fingerprintLength is how much of the signed data's hash the email shows.
const fingerprintLength = 16

// DoctorAnswerEmailArgs emails the patient the signed opinion. It is enqueued by
// the signature job, once the signature is stored.
type DoctorAnswerEmailArgs struct {
	ConsultationID string `json:"consultation_id"`
}

func (DoctorAnswerEmailArgs) Kind() string { return "doctor_answer_email" }

type DoctorAnswerEmailWorker struct {
	river.WorkerDefaults[DoctorAnswerEmailArgs]
	db           *pgxpool.Pool
	storage      *storage.Service
	emailService email.Service
	logger       *log.LoggerType
}

func NewDoctorAnswerEmailWorker(db *pgxpool.Pool, storage *storage.Service, emailService email.Service, logger *log.LoggerType) *DoctorAnswerEmailWorker {
	return &DoctorAnswerEmailWorker{
		db:           db,
		storage:      storage,
		emailService: emailService,
		logger:       logger,
	}
}

func (w *DoctorAnswerEmailWorker) Work(ctx context.Context, job *river.Job[DoctorAnswerEmailArgs]) error {
	consultationID := job.Args.ConsultationID

	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			w.logger.Error("failed to rollback transaction", "error", err)
		}
	}()

	// The row lock and the flag send the email once, even if the job runs twice
	// at the same time or is retried after it was sent.
	var finishedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT finished_at FROM consultations
		WHERE id = @id AND email_sent_patient = FALSE
		FOR UPDATE
	`, pgx.NamedArgs{"id": consultationID}).Scan(&finishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // already sent, or the consultation is gone
	}
	if err != nil {
		return fmt.Errorf("failed to lock consultation: %w", err)
	}

	opinion, patientEmail, err := w.signedOpinion(ctx, tx, consultationID, finishedAt)
	if err != nil {
		return err
	}

	if err := w.emailService.Send(ctx,
		[]email.EmailAddress{{Name: opinion.PatientName, Email: patientEmail}},
		opinion.Title(),
		email_templates.ConsultationAnswerSubmitted(opinion),
	); err != nil {
		return fmt.Errorf("failed to send doctor answer email: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE consultations SET email_sent_patient = TRUE WHERE id = @id`,
		pgx.NamedArgs{"id": consultationID}); err != nil {
		return fmt.Errorf("failed to mark email as sent: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	w.logger.Info("signed opinion sent to patient", "consultation_id", consultationID)

	return nil
}

// signedOpinion builds the email from the stored signature, after checking that
// it still matches the consultation: the patient gets exactly what was signed.
func (w *DoctorAnswerEmailWorker) signedOpinion(ctx context.Context, tx pgx.Tx, consultationID string, finishedAt *time.Time) (email_templates.SignedOpinion, string, error) {
	current, err := signing.LoadPayload(ctx, tx, consultationID)
	if errors.Is(err, signing.ErrNotFinished) {
		return email_templates.SignedOpinion{}, "", river.JobCancel(err)
	}
	if err != nil {
		return email_templates.SignedOpinion{}, "", err
	}

	recordJSON, err := w.storage.GetSignature(ctx, consultationID)
	if err != nil {
		return email_templates.SignedOpinion{}, "", err
	}
	var record signing.SignatureRecord
	if err := json.Unmarshal(recordJSON, &record); err != nil || !signing.VerifyAgainst(record, current) {
		// Never send an opinion whose signature doesn't hold.
		return email_templates.SignedOpinion{}, "", river.JobCancel(
			fmt.Errorf("signature of consultation %s does not verify, opinion not sent", consultationID))
	}

	signed := record.SignedPayload
	opinion := email_templates.SignedOpinion{
		PatientName: signed.PatientName + " " + signed.PatientSurname,
		DoctorName:  signed.DoctorName,
		OpinionHTML: signed.Answer,
		Fingerprint: record.PayloadHash[:fingerprintLength],
		IsVideo:     signed.ConsultationType == "video",
	}
	if signedAt, err := time.Parse(time.RFC3339, signed.SignedAt); err == nil {
		opinion.SignedAt, err = formatInSkopje(signedAt)
		if err != nil {
			return email_templates.SignedOpinion{}, "", err
		}
	}
	if finishedAt != nil {
		opinion.SubmittedAt, err = formatInSkopje(*finishedAt)
		if err != nil {
			return email_templates.SignedOpinion{}, "", err
		}
	}

	return opinion, signed.PatientEmail, nil
}

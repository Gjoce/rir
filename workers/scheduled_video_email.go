package workers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/email"
	"cybermedico/email_templates"
	"cybermedico/video"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// ScheduledVideoEmailArgs holds only the id: the personal link in this email lets its
// holder join the call, so neither it nor patient details stay in the job table.
type ScheduledVideoEmailArgs struct {
	ConsultationID string `json:"consultation_id"`
}

func (ScheduledVideoEmailArgs) Kind() string { return "scheduled_video_email" }

type scheduledVideo struct {
	PatientName    string
	PatientSurname string
	PatientEmail   string
	DoctorName     string
	ScheduledAt    time.Time
}

type ScheduledVideoEmailWorker struct {
	river.WorkerDefaults[ScheduledVideoEmailArgs]
	db              *pgxpool.Pool
	emailService    email.Service
	videoURLBuilder *video.URLBuilder
	logger          *log.LoggerType
}

func NewScheduledVideoEmailWorker(db *pgxpool.Pool, emailService email.Service, videoURLBuilder *video.URLBuilder, logger *log.LoggerType) *ScheduledVideoEmailWorker {
	return &ScheduledVideoEmailWorker{
		db:              db,
		emailService:    emailService,
		videoURLBuilder: videoURLBuilder,
		logger:          logger,
	}
}

// loadScheduledVideo cancels the job when the consultation is gone or no longer
// scheduled, because retrying can't help then.
func loadScheduledVideo(ctx context.Context, db *pgxpool.Pool, consultationID string) (scheduledVideo, error) {
	rows, err := db.Query(ctx, `
		SELECT
			c.patient_name,
			c.patient_surname,
			c.patient_email,
			p.display_name AS doctor_name,
			v.scheduled_at
		FROM consultations c
		JOIN doctor_profile p ON p.id = c.doctor_id
		JOIN video_consultations v ON v.consultation_id = c.id
		WHERE c.id = @id AND c.consultation_status = 'scheduled'
	`, pgx.NamedArgs{"id": consultationID})
	if err != nil {
		return scheduledVideo{}, fmt.Errorf("failed to query consultation: %w", err)
	}

	consultation, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[scheduledVideo])
	if errors.Is(err, pgx.ErrNoRows) {
		return scheduledVideo{}, river.JobCancel(fmt.Errorf("scheduled consultation %s not found: %w", consultationID, err))
	}
	if err != nil {
		return scheduledVideo{}, fmt.Errorf("failed to collect consultation: %w", err)
	}

	return consultation, nil
}

func formatInSkopje(t time.Time) (string, error) {
	location, err := time.LoadLocation("Europe/Skopje")
	if err != nil {
		return "", fmt.Errorf("failed to load time zone: %w", err)
	}

	return t.In(location).Format(orderDeadlineFormat), nil
}

func (w *ScheduledVideoEmailWorker) Work(ctx context.Context, job *river.Job[ScheduledVideoEmailArgs]) error {
	consultation, err := loadScheduledVideo(ctx, w.db, job.Args.ConsultationID)
	if err != nil {
		return err
	}

	dateTime, err := formatInSkopje(consultation.ScheduledAt)
	if err != nil {
		return err
	}

	joinLink, err := w.videoURLBuilder.PatientJoinURL(job.Args.ConsultationID, consultation.ScheduledAt)
	if err != nil {
		return fmt.Errorf("failed to build join link: %w", err)
	}

	if err := w.emailService.Send(ctx,
		[]email.EmailAddress{{Name: consultation.PatientName + " " + consultation.PatientSurname, Email: consultation.PatientEmail}},
		"Your video consultation is scheduled",
		email_templates.ScheduledVideoConsultation(consultation.DoctorName, dateTime, joinLink),
	); err != nil {
		w.logger.Error("failed to send scheduled video email", "error", err, "consultation_id", job.Args.ConsultationID)

		return fmt.Errorf("failed to send scheduled video email: %w", err)
	}

	w.logger.Info("Scheduled video email sent", "consultation_id", job.Args.ConsultationID)

	return nil
}

package workers

import (
	"context"
	"fmt"

	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/email"
	"cybermedico/email_templates"
	"cybermedico/video"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// VideoCallReminderArgs holds only the id, for the same reason as ScheduledVideoEmailArgs.
type VideoCallReminderArgs struct {
	ConsultationID string `json:"consultation_id"`
}

func (VideoCallReminderArgs) Kind() string { return "video_call_reminder" }

type VideoCallReminderWorker struct {
	river.WorkerDefaults[VideoCallReminderArgs]
	db              *pgxpool.Pool
	emailService    email.Service
	videoURLBuilder *video.URLBuilder
	logger          *log.LoggerType
}

func NewVideoCallReminderWorker(db *pgxpool.Pool, emailService email.Service, videoURLBuilder *video.URLBuilder, logger *log.LoggerType) *VideoCallReminderWorker {
	return &VideoCallReminderWorker{
		db:              db,
		emailService:    emailService,
		videoURLBuilder: videoURLBuilder,
		logger:          logger,
	}
}

func (w *VideoCallReminderWorker) Work(ctx context.Context, job *river.Job[VideoCallReminderArgs]) error {
	consultation, err := loadScheduledVideo(ctx, w.db, job.Args.ConsultationID)
	if err != nil {
		return err
	}

	dateTime, err := formatInSkopje(consultation.ScheduledAt)
	if err != nil {
		return err
	}

	// The token depends only on the consultation and the appointment, so this is
	// the same link the patient received when the call was scheduled.
	joinLink, err := w.videoURLBuilder.PatientJoinURL(job.Args.ConsultationID, consultation.ScheduledAt)
	if err != nil {
		return fmt.Errorf("failed to build join link: %w", err)
	}

	if err := w.emailService.Send(ctx,
		[]email.EmailAddress{{Name: consultation.PatientName + " " + consultation.PatientSurname, Email: consultation.PatientEmail}},
		"Reminder: your video consultation starts in about 1 hour",
		email_templates.VideoConsultationReminder(consultation.DoctorName, dateTime, joinLink),
	); err != nil {
		w.logger.Error("failed to send video call reminder", "error", err, "consultation_id", job.Args.ConsultationID)

		return fmt.Errorf("failed to send video call reminder: %w", err)
	}

	w.logger.Info("Video call reminder sent", "consultation_id", job.Args.ConsultationID)

	return nil
}

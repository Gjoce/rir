package workers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/email"
	"cybermedico/email_templates"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// Deadline names the doctor's deadline a reminder is for.
type Deadline string

const (
	// DeadlineWrittenAnswer is the 72 hours to answer a written consultation.
	DeadlineWrittenAnswer Deadline = "written_answer"
	// DeadlineVideoScheduling is the time to set the time of a video call.
	DeadlineVideoScheduling Deadline = "video_scheduling"
	// DeadlineVideoOpinion is the 24 hours after a call for the written opinion.
	DeadlineVideoOpinion Deadline = "video_opinion"
)

// How long before each deadline the doctor is reminded.
const (
	WrittenAnswerReminderLead   = 12 * time.Hour
	VideoSchedulingReminderLead = 3 * time.Hour
	VideoOpinionReminderLead    = 3 * time.Hour
)

// deadlineReminders describes each deadline: what the doctor still has to do,
// and the query that locks the consultation while that is still the case and
// no reminder was sent. Each returns the patient, the doctor, and the deadline.
var deadlineReminders = map[Deadline]struct {
	consultationType string
	action           string
	lockQuery        string
	markSentQuery    string
}{
	DeadlineWrittenAnswer: {
		consultationType: "Written consultation",
		action:           "answer the patient",
		lockQuery: `
			SELECT c.patient_name, c.patient_surname, p.display_name, u.email, wc.expires_at
			FROM consultations c
			JOIN written_consultations wc ON wc.consultation_id = c.id
			JOIN doctor_profile p ON p.id = c.doctor_id
			JOIN users u ON u.id = c.doctor_id
			WHERE c.id = @id AND c.consultation_status = 'pending' AND NOT wc.email_reminder_sent
			FOR UPDATE OF wc`,
		markSentQuery: `UPDATE written_consultations SET email_reminder_sent = TRUE WHERE consultation_id = @id`,
	},
	DeadlineVideoScheduling: {
		consultationType: "Video consultation",
		action:           "schedule the video call",
		lockQuery: `
			SELECT c.patient_name, c.patient_surname, p.display_name, u.email, vc.requires_response_by_schedule
			FROM consultations c
			JOIN video_consultations vc ON vc.consultation_id = c.id
			JOIN doctor_profile p ON p.id = c.doctor_id
			JOIN users u ON u.id = c.doctor_id
			WHERE c.id = @id AND c.consultation_status = 'pending' AND NOT vc.reminder_sent_scheduling
			FOR UPDATE OF vc`,
		markSentQuery: `UPDATE video_consultations SET reminder_sent_scheduling = TRUE WHERE consultation_id = @id`,
	},
	DeadlineVideoOpinion: {
		consultationType: "Video consultation",
		action:           "write the opinion after the call",
		lockQuery: `
			SELECT c.patient_name, c.patient_surname, p.display_name, u.email, vc.expires_at_after_video
			FROM consultations c
			JOIN video_consultations vc ON vc.consultation_id = c.id
			JOIN doctor_profile p ON p.id = c.doctor_id
			JOIN users u ON u.id = c.doctor_id
			WHERE c.id = @id AND c.consultation_status = 'scheduled' AND NOT vc.reminder_sent_answer
			FOR UPDATE OF vc`,
		markSentQuery: `UPDATE video_consultations SET reminder_sent_answer = TRUE WHERE consultation_id = @id`,
	},
}

// DoctorDeadlineReminderArgs reminds the doctor of a deadline. It is scheduled
// for the lead time before the deadline when the deadline is set.
type DoctorDeadlineReminderArgs struct {
	ConsultationID string   `json:"consultation_id"`
	Deadline       Deadline `json:"deadline"`
}

func (DoctorDeadlineReminderArgs) Kind() string { return "doctor_deadline_reminder" }

// DoctorDeadlineReminderJob builds the reminder job for a deadline, scheduled
// the lead time before it. Unique by its arguments: one reminder per
// consultation and deadline.
func DoctorDeadlineReminderJob(consultationID string, deadline Deadline, at time.Time) (DoctorDeadlineReminderArgs, *river.InsertOpts) {
	lead := map[Deadline]time.Duration{
		DeadlineWrittenAnswer:   WrittenAnswerReminderLead,
		DeadlineVideoScheduling: VideoSchedulingReminderLead,
		DeadlineVideoOpinion:    VideoOpinionReminderLead,
	}[deadline]

	return DoctorDeadlineReminderArgs{ConsultationID: consultationID, Deadline: deadline},
		&river.InsertOpts{ScheduledAt: at.Add(-lead), UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type DoctorDeadlineReminderWorker struct {
	river.WorkerDefaults[DoctorDeadlineReminderArgs]
	db           *pgxpool.Pool
	emailService email.Service
	baseURL      string
	logger       *log.LoggerType
}

// NewDoctorDeadlineReminderWorker builds the worker; baseURL is the app's public
// origin, for the link to the consultation.
func NewDoctorDeadlineReminderWorker(db *pgxpool.Pool, emailService email.Service, baseURL string, logger *log.LoggerType) *DoctorDeadlineReminderWorker {
	return &DoctorDeadlineReminderWorker{
		db:           db,
		emailService: emailService,
		baseURL:      baseURL,
		logger:       logger,
	}
}

func (w *DoctorDeadlineReminderWorker) Work(ctx context.Context, job *river.Job[DoctorDeadlineReminderArgs]) error {
	reminder, ok := deadlineReminders[job.Args.Deadline]
	if !ok {
		return river.JobCancel(fmt.Errorf("unknown deadline %q", job.Args.Deadline))
	}

	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			w.logger.Error("failed to rollback transaction", "error", err)
		}
	}()

	var patientName, patientSurname, doctorName, doctorEmail string
	var deadline *time.Time
	err = tx.QueryRow(ctx, reminder.lockQuery, pgx.NamedArgs{"id": job.Args.ConsultationID}).
		Scan(&patientName, &patientSurname, &doctorName, &doctorEmail, &deadline)
	if errors.Is(err, pgx.ErrNoRows) {
		// The doctor already responded, or the reminder was sent.
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to lock consultation: %w", err)
	}
	if deadline == nil || !time.Now().Before(*deadline) {
		// A reminder after the deadline is no use; marking it is another job's work.
		return nil
	}

	deadlineText, err := formatInSkopje(*deadline)
	if err != nil {
		return err
	}

	if err := w.emailService.Send(ctx,
		[]email.EmailAddress{{Name: doctorName, Email: doctorEmail}},
		"Reminder: "+reminder.action+" by "+deadlineText,
		email_templates.DoctorDeadlineReminder(email_templates.DeadlineReminder{
			DoctorName:       doctorName,
			PatientName:      patientName + " " + patientSurname,
			ConsultationType: reminder.consultationType,
			Action:           reminder.action,
			Deadline:         deadlineText,
			Link:             fmt.Sprintf("%s/consultations/%s", w.baseURL, job.Args.ConsultationID),
		}),
	); err != nil {
		return fmt.Errorf("failed to send deadline reminder: %w", err)
	}

	if _, err := tx.Exec(ctx, reminder.markSentQuery, pgx.NamedArgs{"id": job.Args.ConsultationID}); err != nil {
		return fmt.Errorf("failed to mark reminder as sent: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	w.logger.Info("deadline reminder sent to doctor",
		"consultation_id", job.Args.ConsultationID,
		"deadline", job.Args.Deadline,
	)

	return nil
}

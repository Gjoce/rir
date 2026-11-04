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

const orderDeadlineFormat = "02.01.2006 15:04"

// OrderConfirmationEmailArgs holds only the id: the job table should not keep
// patient details, so the worker loads them when it runs.
type OrderConfirmationEmailArgs struct {
	ConsultationID string `json:"consultation_id"`
}

func (OrderConfirmationEmailArgs) Kind() string { return "order_confirmation_email" }

type orderConfirmation struct {
	PatientName      string
	PatientSurname   string
	PatientEmail     string
	ConsultationType string
	DoctorName       string
	Deadline         time.Time
}

type OrderConfirmationEmailWorker struct {
	river.WorkerDefaults[OrderConfirmationEmailArgs]
	db           *pgxpool.Pool
	emailService email.Service
	logger       *log.LoggerType
}

func NewOrderConfirmationEmailWorker(db *pgxpool.Pool, emailService email.Service, logger *log.LoggerType) *OrderConfirmationEmailWorker {
	return &OrderConfirmationEmailWorker{
		db:           db,
		emailService: emailService,
		logger:       logger,
	}
}

func (w *OrderConfirmationEmailWorker) Work(ctx context.Context, job *river.Job[OrderConfirmationEmailArgs]) error {
	rows, err := w.db.Query(ctx, `
		SELECT
			c.patient_name,
			c.patient_surname,
			c.patient_email,
			c.consultation_type::text AS consultation_type,
			p.display_name AS doctor_name,
			COALESCE(wc.expires_at, vc.requires_response_by_schedule) AS deadline
		FROM consultations c
		JOIN doctor_profile p ON p.id = c.doctor_id
		LEFT JOIN written_consultations wc ON wc.consultation_id = c.id
		LEFT JOIN video_consultations vc ON vc.consultation_id = c.id
		WHERE c.id = @id
	`, pgx.NamedArgs{"id": job.Args.ConsultationID})
	if err != nil {
		return fmt.Errorf("failed to query consultation: %w", err)
	}

	order, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[orderConfirmation])
	if errors.Is(err, pgx.ErrNoRows) {
		// The consultation is gone, so retrying can't help.
		return river.JobCancel(fmt.Errorf("consultation %s not found: %w", job.Args.ConsultationID, err))
	}
	if err != nil {
		return fmt.Errorf("failed to collect consultation: %w", err)
	}

	location, err := time.LoadLocation("Europe/Skopje")
	if err != nil {
		return fmt.Errorf("failed to load time zone: %w", err)
	}

	patientFullName := order.PatientName + " " + order.PatientSurname
	isVideo := order.ConsultationType == "video"

	subject := "Your written consultation was received"
	if isVideo {
		subject = "Your video consultation was received"
	}

	if err := w.emailService.Send(ctx,
		[]email.EmailAddress{{Name: patientFullName, Email: order.PatientEmail}},
		subject,
		email_templates.OrderConfirmation(patientFullName, order.DoctorName, order.Deadline.In(location).Format(orderDeadlineFormat), isVideo),
	); err != nil {
		w.logger.Error("failed to send order confirmation email", "error", err, "consultation_id", job.Args.ConsultationID)

		return fmt.Errorf("failed to send order confirmation email: %w", err)
	}

	w.logger.Info("Order confirmation sent", "consultation_id", job.Args.ConsultationID)

	return nil
}

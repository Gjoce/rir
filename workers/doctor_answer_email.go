package workers

import (
	"context"
	"fmt"

	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/email"
	"cybermedico/email_templates"

	"github.com/riverqueue/river"
)

type DoctorAnswerEmailArgs struct {
	PatientName      string `json:"patient_name"`
	PatientSurname   string `json:"patient_surname"`
	PatientEmail     string `json:"patient_email"`
	DoctorName       string `json:"doctor_name"`
	Answer           string `json:"answer"`
	ConsultationType string `json:"consultation_type"`
}

func (DoctorAnswerEmailArgs) Kind() string { return "doctor_answer_email" }

type DoctorAnswerEmailWorker struct {
	river.WorkerDefaults[DoctorAnswerEmailArgs]
	emailService email.Service
	logger       *log.LoggerType
}

func NewDoctorAnswerEmailWorker(emailService email.Service, logger *log.LoggerType) *DoctorAnswerEmailWorker {
	return &DoctorAnswerEmailWorker{
		emailService: emailService,
		logger:       logger,
	}
}

func (w *DoctorAnswerEmailWorker) Work(ctx context.Context, job *river.Job[DoctorAnswerEmailArgs]) error {
	patientFullName := job.Args.PatientName + " " + job.Args.PatientSurname

	if err := w.emailService.Send(ctx,
		[]email.EmailAddress{{Name: patientFullName, Email: job.Args.PatientEmail}},
		func() string {
			if job.Args.ConsultationType == "video" {
				return "Written opinion for your video consultation"
			}

			return "Answer to your written consultation"
		}(),
		email_templates.ConsultationAnswerSubmitted(
			patientFullName, job.Args.DoctorName, job.Args.Answer,
			fmt.Sprintf("https://cybermedico.net/unsubscribe?email=%s", job.Args.PatientEmail),
			job.Args.ConsultationType == "video",
		),
	); err != nil {
		log.Logger.Error("failed to send doctor answer email", "error", err, "patient_email", job.Args.PatientEmail)

		return fmt.Errorf("failed to send doctor answer email: %w", err)
	}

	log.Logger.Info("Doctor answer sent to patient successfully!", "doctor", job.Args.DoctorName, "patient", patientFullName)

	return nil
}

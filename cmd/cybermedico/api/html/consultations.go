package html

import (
	"bytes"
	"context"
	"cybermedico/authorization"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	"cybermedico/cmd/cybermedico/api/util"
	api "cybermedico/cmd/cybermedico/ogen"
	"cybermedico/cmd/cybermedico/storage"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starfederation/datastar-go/datastar"
)

const consultationsPerPage = 10

type ConsultationsHandler struct {
	db          *pgxpool.Pool
	authService *AuthService
	storage     *storage.Service
}

func NewConsultationsHandler(db *pgxpool.Pool, authService *AuthService, storageService *storage.Service) ConsultationsHandler {
	return ConsultationsHandler{db: db, authService: authService, storage: storageService}
}

type Consultations struct {
	ID                      string
	PatientName             string
	PatientSurname          string
	PatientEmail            string
	ConsultationType        string
	ConsultationStatus      string
	Urgent                  bool
	Price                   float64
	CreatedAt               *time.Time
	DoctorID                string
	DoctorDisplayName       string
	ExpiresAt               *time.Time
	VideoExpiresAt          *time.Time
	VideoRequiresResponseBy *time.Time
}

type ConsultationFilters struct {
	Type    string `json:"type"`
	Page    int    `json:"page"`
	PerPage int    `json:"perPage"`
}

func (h ConsultationsHandler) ConsultationsForm(ctx context.Context, params api.ConsultationsFormParams) (api.ConsultationsFormRes, error) {
	r := util.RequestFromContext(ctx)
	w := util.ResponseFromContext(ctx)

	allowed, err := h.authService.IsAllowed(ctx, "consultations", authorization.Target{})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, herrors.HtmlError{
			Code:       http.StatusForbidden,
			Msg:        "You don't have permission to view consultations",
			ButtonLink: "/",
		}
	}

	role := h.authService.SessionManager.GetString(ctx, SessionKeyRoles)
	userId := h.authService.SessionManager.GetString(ctx, SessionKeyUserID)

	filters := ConsultationFilters{
		Type: "pending",
		Page: 1,
	}
	if err = datastar.ReadSignals(r, &filters); err != nil {
		return nil, herrors.HtmlError{Code: http.StatusBadRequest, Msg: "This filter is not valid", Err: err, ButtonLink: "/consultations"}
	}

	// These come from the browser. A page size of 0 would divide by zero below, and
	// a large one would let anyone load the whole table at once.
	filters.PerPage = consultationsPerPage
	if filters.Page < 1 {
		filters.Page = 1
	}

	switch filters.Type {
	case "pending", "scheduled", "finished":
	case "not_attended":
		canViewNotAttended, err := h.authService.IsAllowed(ctx, "view_not_attended", authorization.Target{})
		if err != nil {
			return nil, err
		}
		if !canViewNotAttended {
			return nil, herrors.HtmlError{
				Code:       http.StatusForbidden,
				Msg:        "You don't have permission to view these consultations",
				ButtonLink: "/consultations",
			}
		}
	default:
		return nil, herrors.HtmlError{Code: http.StatusBadRequest, Msg: "This filter is not valid", ButtonLink: "/consultations"}
	}

	var count int
	countQueryDoctor := `SELECT COUNT(*) FROM consultations WHERE consultation_status = @status AND doctor_id = @doctor_id`
	countQueryAll := `SELECT COUNT(*) FROM consultations WHERE consultation_status = @status`

	switch role {
	case "doctor":
		err = h.db.QueryRow(ctx, countQueryDoctor, pgx.NamedArgs{
			"status":    filters.Type,
			"doctor_id": userId,
		}).Scan(&count)
	case "admin":
		err = h.db.QueryRow(ctx, countQueryAll, pgx.NamedArgs{
			"status": filters.Type,
		}).Scan(&count)
	default:
		return nil, fmt.Errorf("unauthorized role: %s", role)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to count consultations: %w", err)
	}

	pageCount := (count + filters.PerPage - 1) / filters.PerPage
	if pageCount == 0 {
		pageCount = 1
	}

	if filters.Page > pageCount {
		filters.Page = 1
	}

	offset := (filters.Page - 1) * filters.PerPage

	queryDoctor := `
	SELECT
		c.id,
		c.patient_name,
		c.patient_surname,
		c.patient_email,
		c.consultation_type,
		c.consultation_status,
		c.urgent,
		c.price,
		c.created_at,
		c.doctor_id,
		dp.display_name AS doctor_display_name,
		wc.expires_at AS expires_at,
		vc.expires_at_after_video AS video_expires_at,
		vc.requires_response_by_schedule AS video_requires_response_by
	FROM consultations c
	LEFT JOIN doctor_profile dp ON c.doctor_id = dp.id
	LEFT JOIN written_consultations wc ON wc.consultation_id = c.id
	LEFT JOIN video_consultations vc ON vc.consultation_id = c.id
	WHERE c.consultation_status = @status
	AND c.doctor_id = @doctor_id
	ORDER BY c.created_at DESC
	LIMIT @limit OFFSET @offset
	`

	queryAll := `
	SELECT
		c.id,
		c.patient_name,
		c.patient_surname,
		c.patient_email,
		c.consultation_type,
		c.consultation_status,
		c.urgent,
		c.price,
		c.created_at,
		c.doctor_id,
		dp.display_name AS doctor_display_name,
		wc.expires_at AS expires_at,
		vc.expires_at_after_video AS video_expires_at,
		vc.requires_response_by_schedule AS video_requires_response_by
	FROM consultations c
	LEFT JOIN doctor_profile dp ON c.doctor_id = dp.id
	LEFT JOIN written_consultations wc ON wc.consultation_id = c.id
	LEFT JOIN video_consultations vc ON vc.consultation_id = c.id
	WHERE c.consultation_status = @status
	ORDER BY c.created_at DESC
	LIMIT @limit OFFSET @offset
	`

	var rows pgx.Rows

	switch role {
	case "doctor":
		rows, err = h.db.Query(ctx, queryDoctor, pgx.NamedArgs{
			"status":    filters.Type,
			"doctor_id": userId,
			"limit":     filters.PerPage,
			"offset":    offset,
		})
	case "admin":
		rows, err = h.db.Query(ctx, queryAll, pgx.NamedArgs{
			"status": filters.Type,
			"limit":  filters.PerPage,
			"offset": offset,
		})
	default:
		return nil, fmt.Errorf("unauthorized role: %s", role)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query consultations: %w", err)
	}

	consultations, err := pgx.CollectRows(rows, pgx.RowToStructByName[Consultations])
	if err != nil {
		return nil, fmt.Errorf("failed to collect consultations: %w", err)
	}

	showTimeLeft := filters.Type == "pending" || filters.Type == "scheduled"
	comp := ConsultationsSSE(consultations, filters, pageCount, showTimeLeft)

	if params.DatastarRequest.Set {
		sse := datastar.NewSSE(w, r)
		err = sse.PatchElementTempl(comp)
		if err != nil {
			return nil, err
		}

		return nil, herrors.ErrNoop
	}

	canViewNotAttended, err := h.authService.IsAllowed(ctx, "view_not_attended", authorization.Target{})
	if err != nil {
		return nil, err
	}

	var buff bytes.Buffer

	return &api.R200{Data: &buff}, ConsultationsForm(comp, filters.Type, canViewNotAttended).Render(ctx, &buff)
}

type ConsultationAttachment struct {
	Filename string
	URL      string
}

type Consultation struct {
	ID                      string
	ConsultationType        string
	ConsultationStatus      string
	Price                   float64
	Urgent                  bool
	NeedTranslator          bool
	TermsAccepted           bool
	PatientName             string
	PatientSurname          string
	PatientAge              int
	PatientPlaceOfLiving    *string // nullable
	MedicalProblem          string
	PatientDiagnosis        *string // nullable
	Allergies               *string // nullable
	Medication              *string // nullable
	Attachments             []ConsultationAttachment
	CreatedAt               time.Time
	FinishedAt              *time.Time // nullable
	DoctorAnswer            *string    // nullable
	ScheduledAt             *time.Time // nullable
	ExpiresAt               *time.Time // nullable
	VideoExpiresAt          *time.Time // nullable
	VideoRequiresResponseBy *time.Time // nullable
}

func (h ConsultationsHandler) ConsultationForm(ctx context.Context, params api.ConsultationFormParams) (api.ConsultationFormRes, error) {
	role := h.authService.SessionManager.GetString(ctx, SessionKeyRoles)
	userIDd := h.authService.SessionManager.GetString(ctx, SessionKeyUserID)

	doctorQuery := `
	SELECT doctor_id, consultation_status FROM consultations
	WHERE id = @id
	`
	var doctorID, consultationStatus string
	err := h.db.QueryRow(ctx, doctorQuery, pgx.NamedArgs{"id": params.ID.String()}).Scan(&doctorID, &consultationStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, herrors.HtmlError{
			Code:       http.StatusNotFound,
			Msg:        "Consultation not found",
			ButtonLink: "/consultations",
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query consultation owner: %w", err)
	}

	allowed, err := h.authService.IsAllowed(ctx, "consultation", authorization.Target{
		UserID:             doctorID,
		ConsultationStatus: consultationStatus,
	})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, herrors.HtmlError{
			Code:       http.StatusForbidden,
			Msg:        "You don't have permission to view this consultation",
			ButtonLink: "/",
		}
	}

	var consultation Consultation

	var query string
	var args pgx.NamedArgs

	const baseQuery = `
	SELECT c.id, c.consultation_type, c.consultation_status, c.price,
		c.urgent, c.need_translator, c.terms_accepted, c.patient_name, c.patient_surname, c.patient_age,
		c.patient_place_of_living, c.medical_problem, c.patient_diagnosis, c.allergies,
		c.medication, c.created_at, c.finished_at,
		COALESCE(wc.doctor_answer, vc.doctor_answer_after_video) AS doctor_answer,
		vc.scheduled_at,
		wc.expires_at,
		vc.expires_at_after_video,
		vc.requires_response_by_schedule
	FROM consultations c
	LEFT JOIN written_consultations wc ON wc.consultation_id = c.id
	LEFT JOIN video_consultations vc ON vc.consultation_id = c.id
	WHERE c.id = @id
	`

	switch role {
	case "admin":
		query = baseQuery
		args = pgx.NamedArgs{"id": params.ID.String()}
	case "doctor":
		query = baseQuery + "AND c.doctor_id = @doctorID"
		args = pgx.NamedArgs{"id": params.ID.String(), "doctorID": userIDd}
	default:
		return nil, fmt.Errorf("unauthorized role: %s", role)
	}

	err = h.db.QueryRow(ctx, query, args).Scan(
		&consultation.ID,
		&consultation.ConsultationType,
		&consultation.ConsultationStatus,
		&consultation.Price,
		&consultation.Urgent,
		&consultation.NeedTranslator,
		&consultation.TermsAccepted,
		&consultation.PatientName,
		&consultation.PatientSurname,
		&consultation.PatientAge,
		&consultation.PatientPlaceOfLiving,
		&consultation.MedicalProblem,
		&consultation.PatientDiagnosis,
		&consultation.Allergies,
		&consultation.Medication,
		&consultation.CreatedAt,
		&consultation.FinishedAt,
		&consultation.DoctorAnswer,
		&consultation.ScheduledAt,
		&consultation.ExpiresAt,
		&consultation.VideoExpiresAt,
		&consultation.VideoRequiresResponseBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, herrors.HtmlError{
				Code:       http.StatusNotFound,
				Msg:        "Consultation not found",
				ButtonLink: "/consultations",
			}
		}

		return nil, fmt.Errorf("failed to query consultation: %w", err)
	}

	rows, err := h.db.Query(ctx, `
		SELECT id, s3_key, original_filename FROM consultation_attachments
		WHERE consultation_id = @consultation_id ORDER BY uploaded_at
	`, pgx.NamedArgs{"consultation_id": consultation.ID})
	if err != nil {
		return nil, fmt.Errorf("failed to query attachments: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var attachmentID, s3Key, filename string
		if err := rows.Scan(&attachmentID, &s3Key, &filename); err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}
		url, err := h.storage.PresignURL(ctx, s3Key, 5*time.Minute)
		if err != nil {
			return nil, fmt.Errorf("failed to presign attachment URL: %w", err)
		}
		consultation.Attachments = append(consultation.Attachments, ConsultationAttachment{
			Filename: filename,
			URL:      url,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate attachments: %w", err)
	}

	canAnswerConsultation, err := h.authService.IsAllowed(ctx, "answer_consultation", authorization.Target{
		UserID:             doctorID,
		ConsultationStatus: consultation.ConsultationStatus,
		ConsultationType:   consultation.ConsultationType,
		ScheduledAtPast:    consultation.ScheduledAt != nil && consultation.ScheduledAt.Before(time.Now()),
	})
	if err != nil {
		return nil, err
	}

	canJoinVideoCall, err := h.authService.IsAllowed(ctx, "join_video_consultation", authorization.Target{
		UserID:             doctorID,
		ConsultationStatus: consultation.ConsultationStatus,
		ConsultationType:   consultation.ConsultationType,
		WithinJoinWindow:   consultation.ScheduledAt != nil && util.IsWithinJoinWindow(*consultation.ScheduledAt, time.Now()),
	})
	if err != nil {
		return nil, err
	}

	var buff bytes.Buffer

	return &api.R200{Data: &buff}, ConsultationFormHtml(consultation, role, canAnswerConsultation, canJoinVideoCall).Render(ctx, &buff)
}

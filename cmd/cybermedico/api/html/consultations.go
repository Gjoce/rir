package html

import (
	"bytes"
	"context"
	"cybermedico/authorization"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/api/util"
	api "cybermedico/cmd/cybermedico/ogen"
	"cybermedico/cmd/cybermedico/signing"
	"cybermedico/cmd/cybermedico/storage"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/microcosm-cc/bluemonday"
	"github.com/starfederation/datastar-go/datastar"
)

const (
	consultationsPerPage = 10
	maxAnswerLength      = 50000
)

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

type answerConsultation struct {
	ID               string
	PatientName      string
	PatientSurname   string
	PatientAge       int
	ConsultationType string
	Urgent           bool
	Price            float64
	Answer           string
	AnswerErr        string
}

type answerConsultationTOTP struct {
	TOTPCode    string
	TOTPCodeErr string
}

func (a answerConsultation) Valid() bool {
	return a.AnswerErr == ""
}

func (t answerConsultationTOTP) Valid() bool {
	return t.TOTPCodeErr == ""
}

// cleanAnswer keeps only what the editor's toolbar can produce. The answer is shown
// as HTML on the detail page and in the patient's email, so nothing else can stay in it.
func cleanAnswer(answer string) string {
	policy := bluemonday.NewPolicy()
	policy.AllowElements("p", "br", "strong", "em", "u", "blockquote", "h1", "h2", "ol", "ul", "li")

	return strings.TrimSpace(policy.Sanitize(answer))
}

// answerHasText reports whether a person would see any words: the editor's empty
// state is markup such as <p><br></p>.
func answerHasText(answer string) bool {
	text := html.UnescapeString(bluemonday.StrictPolicy().Sanitize(answer))

	return strings.TrimSpace(strings.ReplaceAll(text, " ", " ")) != ""
}

func (h ConsultationsHandler) ConsultationAnswerForm(ctx context.Context, params api.ConsultationAnswerFormParams) (api.ConsultationAnswerFormRes, error) {
	var doctorID, consultationStatus, consultationType string
	var scheduledAt *time.Time

	err := h.db.QueryRow(ctx, `
		SELECT c.doctor_id, c.consultation_status, c.consultation_type, vc.scheduled_at
		FROM consultations c
		LEFT JOIN video_consultations vc ON vc.consultation_id = c.id
		WHERE c.id = @id
	`, pgx.NamedArgs{"id": params.ID.String()}).Scan(&doctorID, &consultationStatus, &consultationType, &scheduledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, herrors.HtmlError{Code: http.StatusNotFound, Msg: "Consultation not found", ButtonLink: "/consultations"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query consultation owner: %w", err)
	}

	allowed, err := h.authService.IsAllowed(ctx, "answer_consultation", authorization.Target{
		UserID:             doctorID,
		ConsultationStatus: consultationStatus,
		ConsultationType:   consultationType,
		ScheduledAtPast:    scheduledAt != nil && scheduledAt.Before(time.Now()),
	})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, herrors.HtmlError{
			Code:       http.StatusForbidden,
			Msg:        "You don't have permission to answer this consultation",
			ButtonLink: "/",
		}
	}

	var consultation answerConsultation

	query := `
	SELECT id, patient_name, patient_surname, patient_age, consultation_type, urgent, price
	FROM consultations
	WHERE id = @id
	`

	err = h.db.QueryRow(ctx, query, pgx.NamedArgs{"id": params.ID.String()}).Scan(&consultation.ID, &consultation.PatientName, &consultation.PatientSurname, &consultation.PatientAge, &consultation.ConsultationType, &consultation.Urgent, &consultation.Price)
	if err != nil {
		return nil, fmt.Errorf("failed to query consultation for answer: %w", err)
	}

	var buff bytes.Buffer

	return &api.R200{
		Data: &buff,
	}, ConsultationAnswerForm(consultation, answerConsultationTOTP{}).Render(ctx, &buff)
}

func (h ConsultationsHandler) ConsultationAnswerSubmit(ctx context.Context, request *api.ConsultationAnswerSubmitReq, params api.ConsultationAnswerSubmitParams) (api.ConsultationAnswerSubmitRes, error) {
	var doctorID, consultationStatus, consultationType string
	var scheduledAt *time.Time
	var totpSecret *string

	err := h.db.QueryRow(ctx, `
		SELECT c.doctor_id, c.consultation_status, c.consultation_type, vc.scheduled_at, u.totp_secret
		FROM consultations c
		LEFT JOIN video_consultations vc ON vc.consultation_id = c.id
		JOIN users u ON u.id = c.doctor_id
		WHERE c.id = @id
	`, pgx.NamedArgs{"id": params.ID.String()}).Scan(&doctorID, &consultationStatus, &consultationType, &scheduledAt, &totpSecret)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, herrors.HtmlError{Code: http.StatusNotFound, Msg: "Consultation not found", ButtonLink: "/consultations"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query consultation owner: %w", err)
	}

	allowed, err := h.authService.IsAllowed(ctx, "answer_consultation", authorization.Target{
		UserID:             doctorID,
		ConsultationStatus: consultationStatus,
		ConsultationType:   consultationType,
		ScheduledAtPast:    scheduledAt != nil && scheduledAt.Before(time.Now()),
	})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, herrors.HtmlError{
			Code:       http.StatusForbidden,
			Msg:        "You don't have permission to answer this consultation",
			ButtonLink: "/",
		}
	}

	answer := cleanAnswer(request.Answer)

	var answerCons answerConsultation
	switch {
	case !answerHasText(answer):
		answerCons.AnswerErr = "Answer cannot be empty"
	case utf8.RuneCountInString(answer) > maxAnswerLength:
		answerCons.AnswerErr = fmt.Sprintf("Answer can have at most %d characters", maxAnswerLength)
	}

	// Being logged in is not enough here. The answer is signed in the doctor's name,
	// so this proves it is really the doctor, and not someone at an unattended session.
	var totp answerConsultationTOTP
	var totpStep int64

	switch {
	case request.TotpCode == "":
		totp.TOTPCodeErr = "Please provide the TOTP code"
	case totpSecret == nil:
		totp.TOTPCodeErr = "Two-factor authentication not configured for this account."
	default:
		step, ok := signing.MatchTOTPStep(request.TotpCode, *totpSecret, time.Now())
		if !ok {
			totp.TOTPCodeErr = "Invalid code. Try again."
		}
		totpStep = step
	}

	r := util.RequestFromContext(ctx)
	w := util.ResponseFromContext(ctx)

	// The answer is checked first, so a code isn't used up on an answer that can't be sent.
	if !answerCons.Valid() {
		sse := datastar.NewSSE(w, r)
		if err := sse.PatchElementTempl(answerError(answerCons.AnswerErr)); err != nil {
			return nil, fmt.Errorf("failed to patch answer error: %w", err)
		}
		if err := sse.ExecuteScript("document.getElementById('answer-consultation-dialog').close()"); err != nil {
			return nil, fmt.Errorf("failed to close answer dialog: %w", err)
		}

		return nil, herrors.ErrNoop
	}
	if !totp.Valid() {
		sse := datastar.NewSSE(w, r)
		if err := sse.PatchElementTempl(answerConsultationModal(params.ID.String(), totp)); err != nil {
			return nil, fmt.Errorf("failed to patch TOTP error: %w", err)
		}

		return nil, herrors.ErrNoop
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Logger.Error("failed to rollback transaction", "error", err)
		}
	}()

	// Replay prevention in the UPDATE itself, like at login. If anything later fails,
	// the rollback gives the code back.
	tag, err := tx.Exec(ctx, `
		UPDATE users SET totp_last_used_step = @step
		WHERE id = @id AND (totp_last_used_step IS NULL OR totp_last_used_step < @step)
	`, pgx.NamedArgs{"step": totpStep, "id": doctorID})
	if err != nil {
		return nil, fmt.Errorf("failed to update totp step: %w", err)
	}
	if tag.RowsAffected() == 0 {
		sse := datastar.NewSSE(w, r)
		totp.TOTPCodeErr = "Code already used. Wait for the next code."
		if err := sse.PatchElementTempl(answerConsultationModal(params.ID.String(), totp)); err != nil {
			return nil, fmt.Errorf("failed to patch TOTP error: %w", err)
		}

		return nil, herrors.ErrNoop
	}

	// The status condition makes this safe against a double click or a second tab:
	// only one request can finish the consultation, so a sent answer can't be replaced.
	tag, err = tx.Exec(ctx, `
		UPDATE consultations SET consultation_status = 'finished', finished_at = NOW()
		WHERE id = @id AND consultation_status = @status
	`, pgx.NamedArgs{"id": params.ID.String(), "status": consultationStatus})
	if err != nil {
		return nil, fmt.Errorf("failed to update consultation status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, herrors.HtmlError{Code: http.StatusConflict, Msg: "This consultation was already answered", ButtonLink: "/consultations"}
	}

	switch consultationType {
	case "written":
		_, err = tx.Exec(ctx, `
			UPDATE written_consultations SET doctor_answer = @answer WHERE consultation_id = @id
		`, pgx.NamedArgs{"id": params.ID.String(), "answer": answer})
	case "video":
		_, err = tx.Exec(ctx, `
			UPDATE video_consultations SET doctor_answer_after_video = @answer WHERE consultation_id = @id
		`, pgx.NamedArgs{"id": params.ID.String(), "answer": answer})
	default:
		return nil, fmt.Errorf("unsupported consultation type: %s", consultationType)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update consultation answer: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	sse := datastar.NewSSE(w, r)
	if err := sse.Redirectf("/consultations/%s", params.ID.String()); err != nil {
		return nil, err
	}

	return nil, herrors.ErrNoop
}

const (
	maxScheduleAhead       = 72 * time.Hour
	maxScheduleAheadUrgent = 24 * time.Hour
)

type ScheduleConsultation struct {
	ID                      string
	PatientName             string
	PatientSurname          string
	PatientAge              int
	ConsultationType        string
	ConsultationStatus      string
	Urgent                  bool
	Price                   float64
	PreferredTime           string
	VideoRequiresResponseBy *time.Time
	ScheduledAt             string
	ScheduledAtErr          string
}

func (s ScheduleConsultation) Valid() bool {
	return s.ScheduledAtErr == ""
}

// loadScheduleConsultation loads what the form shows. It is used to show the form and
// to show it again with an error.
func (h ConsultationsHandler) loadScheduleConsultation(ctx context.Context, id string, scheduleCons *ScheduleConsultation) error {
	err := h.db.QueryRow(ctx, `
		SELECT c.id, c.patient_name, c.patient_surname, c.patient_age, c.consultation_type, c.consultation_status, c.urgent, c.price,
			vc.preferred_time, vc.requires_response_by_schedule
		FROM consultations c
		JOIN video_consultations vc ON vc.consultation_id = c.id
		WHERE c.id = @id
		AND c.consultation_status = 'pending'
		AND c.consultation_type = 'video'
	`, pgx.NamedArgs{"id": id}).Scan(
		&scheduleCons.ID,
		&scheduleCons.PatientName,
		&scheduleCons.PatientSurname,
		&scheduleCons.PatientAge,
		&scheduleCons.ConsultationType,
		&scheduleCons.ConsultationStatus,
		&scheduleCons.Urgent,
		&scheduleCons.Price,
		&scheduleCons.PreferredTime,
		&scheduleCons.VideoRequiresResponseBy,
	)
	if err != nil {
		return fmt.Errorf("failed to query consultation schedule: %w", err)
	}

	return nil
}

func (h ConsultationsHandler) ConsultationScheduleForm(ctx context.Context, params api.ConsultationScheduleFormParams) (api.ConsultationScheduleFormRes, error) {
	var doctorID, consultationStatus, consultationType string

	err := h.db.QueryRow(ctx, `
		SELECT doctor_id, consultation_status, consultation_type FROM consultations
		WHERE id = @id
	`, pgx.NamedArgs{"id": params.ID.String()}).Scan(&doctorID, &consultationStatus, &consultationType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, herrors.HtmlError{Code: http.StatusNotFound, Msg: "Consultation not found", ButtonLink: "/consultations"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query consultation owner: %w", err)
	}

	allowed, err := h.authService.IsAllowed(ctx, "schedule_consultation", authorization.Target{
		UserID:             doctorID,
		ConsultationStatus: consultationStatus,
		ConsultationType:   consultationType,
	})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, herrors.HtmlError{
			Code:       http.StatusForbidden,
			Msg:        "You don't have permission to schedule this consultation",
			ButtonLink: "/",
		}
	}

	var scheduleCons ScheduleConsultation
	if err := h.loadScheduleConsultation(ctx, params.ID.String(), &scheduleCons); err != nil {
		return nil, err
	}

	var buff bytes.Buffer

	return &api.R200{
		Data: &buff,
	}, ScheduleConsultationForm(scheduleCons).Render(ctx, &buff)
}

func (h ConsultationsHandler) ConsultationScheduleSubmit(ctx context.Context, request *api.ConsultationScheduleSubmitReq, params api.ConsultationScheduleSubmitParams) (api.ConsultationScheduleSubmitRes, error) {
	var doctorID, consultationStatus, consultationType string
	var urgent bool

	err := h.db.QueryRow(ctx, `
		SELECT doctor_id, consultation_status, consultation_type, urgent FROM consultations
		WHERE id = @id
	`, pgx.NamedArgs{"id": params.ID.String()}).Scan(&doctorID, &consultationStatus, &consultationType, &urgent)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, herrors.HtmlError{Code: http.StatusNotFound, Msg: "Consultation not found", ButtonLink: "/consultations"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query consultation owner: %w", err)
	}

	allowed, err := h.authService.IsAllowed(ctx, "schedule_consultation", authorization.Target{
		UserID:             doctorID,
		ConsultationStatus: consultationStatus,
		ConsultationType:   consultationType,
	})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, herrors.HtmlError{
			Code:       http.StatusForbidden,
			Msg:        "You don't have permission to schedule this consultation",
			ButtonLink: "/",
		}
	}

	var scheduleCons ScheduleConsultation
	var scheduledAt time.Time

	if request.Date == "" {
		scheduleCons.ScheduledAtErr = "Please provide a schedule"
	} else {
		scheduledAt, err = time.Parse(time.RFC3339, request.Date)
		if err != nil {
			scheduleCons.ScheduledAtErr = "Invalid date format"
		} else if scheduledAt.Before(time.Now()) {
			scheduleCons.ScheduledAtErr = "Scheduled time needs to be in the future"
		} else if urgent && scheduledAt.After(time.Now().Add(maxScheduleAheadUrgent)) {
			scheduleCons.ScheduledAtErr = "Urgent consultations must be scheduled within the next 24 hours"
		} else if !urgent && scheduledAt.After(time.Now().Add(maxScheduleAhead)) {
			scheduleCons.ScheduledAtErr = "Consultations must be scheduled within the next 72 hours"
		}
	}

	if !scheduleCons.Valid() {
		if err := h.loadScheduleConsultation(ctx, params.ID.String(), &scheduleCons); err != nil {
			return nil, err
		}

		var buff bytes.Buffer

		return &api.R200{
			Data: &buff,
		}, ScheduleConsultationForm(scheduleCons).Render(ctx, &buff)
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Logger.Error("failed to rollback transaction", "error", err)
		}
	}()

	// The status condition makes this safe against a double click or a second tab:
	// only one request can set the appointment.
	tag, err := tx.Exec(ctx, `
		UPDATE consultations
		SET consultation_status = 'scheduled'
		WHERE id = @id AND consultation_status = 'pending'
	`, pgx.NamedArgs{"id": params.ID.String()})
	if err != nil {
		return nil, fmt.Errorf("failed to update consultation status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, herrors.HtmlError{Code: http.StatusConflict, Msg: "This consultation was already scheduled", ButtonLink: fmt.Sprintf("/consultations/%s", params.ID.String())}
	}

	// A trigger sets the deadline for the opinion from scheduled_at.
	_, err = tx.Exec(ctx, `
		UPDATE video_consultations
		SET scheduled_at = @scheduled_at
		WHERE consultation_id = @id
	`, pgx.NamedArgs{"scheduled_at": scheduledAt, "id": params.ID.String()})
	if err != nil {
		return nil, fmt.Errorf("failed to update video consultation: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &api.R303{Location: api.NewOptString(fmt.Sprintf("/consultations/%s", params.ID.String()))}, nil
}

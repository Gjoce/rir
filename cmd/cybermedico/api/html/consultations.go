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
	"cybermedico/workers"
	"encoding/json"
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
	"github.com/riverqueue/river"
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
	riverClient *river.Client[pgx.Tx]
}

func NewConsultationsHandler(db *pgxpool.Pool, authService *AuthService, storageService *storage.Service, riverClient *river.Client[pgx.Tx]) ConsultationsHandler {
	return ConsultationsHandler{db: db, authService: authService, storage: storageService, riverClient: riverClient}
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
	// DeadlineAt is the deadline the consultation is waiting on, or was missed:
	// the written answer, scheduling the call, or the opinion after it.
	DeadlineAt *time.Time
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
		vc.requires_response_by_schedule AS video_requires_response_by,
		CASE
			WHEN c.consultation_type = 'written' THEN wc.expires_at
			WHEN vc.scheduled_at IS NOT NULL THEN vc.expires_at_after_video
			ELSE vc.requires_response_by_schedule
		END AS deadline_at
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
		vc.requires_response_by_schedule AS video_requires_response_by,
		CASE
			WHEN c.consultation_type = 'written' THEN wc.expires_at
			WHEN vc.scheduled_at IS NOT NULL THEN vc.expires_at_after_video
			ELSE vc.requires_response_by_schedule
		END AS deadline_at
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
	CallEvents              []CallEvent
	Signature               SignatureStatus
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

	if consultation.ConsultationStatus == "finished" {
		consultation.Signature = h.signatureStatus(ctx, consultation.ID)
	}

	if consultation.ConsultationType == "video" {
		consultation.CallEvents, err = h.loadCallEvents(ctx, consultation.ID)
		if err != nil {
			return nil, err
		}
	}

	canAnswerConsultation, err := h.authService.IsAllowed(ctx, "answer_consultation",
		answerTarget(doctorID, consultation.ConsultationStatus, consultation.ConsultationType, consultation.ScheduledAt, consultation.VideoExpiresAt, time.Now()))
	if err != nil {
		return nil, err
	}

	videoCall, err := h.videoCallEntry(ctx, doctorID, consultation, time.Now())
	if err != nil {
		return nil, err
	}

	var buff bytes.Buffer

	return &api.R200{Data: &buff}, ConsultationFormHtml(consultation, role, canAnswerConsultation, videoCall).Render(ctx, &buff)
}

// statusConflict explains why a consultation changed under the doctor: usually
// another tab, but it may also have been marked not attended at its deadline.
func (h ConsultationsHandler) statusConflict(ctx context.Context, consultationID, msg string) error {
	var status string
	err := h.db.QueryRow(ctx, `SELECT consultation_status FROM consultations WHERE id = @id`,
		pgx.NamedArgs{"id": consultationID}).Scan(&status)
	if err == nil && status == "not_attended" {
		msg = "The deadline passed, so this consultation was marked as not attended"
	}

	return herrors.HtmlError{Code: http.StatusConflict, Msg: msg, ButtonLink: "/consultations"}
}

// answerTarget describes a consultation for the answer_consultation policy.
// opinionDeadline is the video consultation's expires_at_after_video, 24 hours
// after the call.
func answerTarget(doctorID, status, consultationType string, scheduledAt, opinionDeadline *time.Time, now time.Time) authorization.Target {
	return authorization.Target{
		UserID:                doctorID,
		ConsultationStatus:    status,
		ConsultationType:      consultationType,
		ScheduledAtPast:       scheduledAt != nil && scheduledAt.Before(now),
		OpinionDeadlinePassed: opinionDeadline != nil && !now.Before(*opinionDeadline),
	}
}

func answerRefusal(consultationID string, target authorization.Target) error {
	msg := "You don't have permission to answer this consultation"
	if target.ConsultationType == "video" && target.ConsultationStatus == "scheduled" && target.OpinionDeadlinePassed {
		msg = "The 24 hours for the written opinion after the call have passed"
	}

	return herrors.HtmlError{
		Code:       http.StatusForbidden,
		Msg:        msg,
		ButtonLink: "/consultations/" + consultationID,
	}
}

// SignatureStatus is the result of checking a finished consultation's signature.
type SignatureStatus struct {
	// State is "pending", "valid", "invalid" or "unavailable".
	State    string
	SignedAt string
	Doctor   string
}

// signatureStatus checks the stored signature against the consultation as it is
// now in the database, so a change to the opinion or the patient's data after
// signing shows up as an invalid signature.
func (h ConsultationsHandler) signatureStatus(ctx context.Context, consultationID string) SignatureStatus {
	var signatureKey *string
	err := h.db.QueryRow(ctx, `SELECT signature_doctor_url FROM consultations WHERE id = @id`,
		pgx.NamedArgs{"id": consultationID}).Scan(&signatureKey)
	if err != nil {
		log.Logger.Error("failed to query signature", "error", err, "consultation_id", consultationID)

		return SignatureStatus{State: "unavailable"}
	}
	if signatureKey == nil {
		return SignatureStatus{State: "pending"}
	}

	recordJSON, err := h.storage.GetSignature(ctx, consultationID)
	if errors.Is(err, storage.ErrSignatureNotFound) {
		// The database says it was signed, but the record is gone.
		log.Logger.Error("signature record missing", "consultation_id", consultationID)

		return SignatureStatus{State: "invalid"}
	}
	if err != nil {
		log.Logger.Error("failed to read signature", "error", err, "consultation_id", consultationID)

		return SignatureStatus{State: "unavailable"}
	}

	current, err := signing.LoadPayload(ctx, h.db, consultationID)
	if err != nil {
		log.Logger.Error("failed to load signed data", "error", err, "consultation_id", consultationID)

		return SignatureStatus{State: "unavailable"}
	}

	var record signing.SignatureRecord
	if err := json.Unmarshal(recordJSON, &record); err != nil || !signing.VerifyAgainst(record, current) {
		log.Logger.Warn("signature does not verify", "consultation_id", consultationID)

		return SignatureStatus{State: "invalid"}
	}

	status := SignatureStatus{State: "valid", Doctor: record.SignedPayload.DoctorName}
	if signedAt, err := time.Parse(time.RFC3339, record.SignedPayload.SignedAt); err == nil {
		status.SignedAt = util.FormatInSkopje(signedAt, "02.01.2006 15:04")
	}

	return status
}

// CallEvent is one line of a video consultation's call log.
type CallEvent struct {
	Time  string
	Label string
}

// callEventLabels describes each event kind; %s is "doctor" or "patient" for the
// events that carry who caused them.
var callEventLabels = map[string]string{
	"doctor_joined":  "Doctor joined",
	"doctor_left":    "Doctor left",
	"patient_joined": "Patient joined",
	"patient_left":   "Patient left",
	"ice_connected":  "Connection established (%s)",
	"ice_relay_used": "Connected through the relay server (%s)",
	"ice_failed":     "Connection failed (%s)",
	"call_ended":     "Call ended by the %s",
}

func (h ConsultationsHandler) loadCallEvents(ctx context.Context, consultationID string) ([]CallEvent, error) {
	rows, err := h.db.Query(ctx, `
		SELECT kind, occurred_at, COALESCE(detail->>'by', '')
		FROM consultation_call_events
		WHERE consultation_id = @id
		ORDER BY occurred_at, id
	`, pgx.NamedArgs{"id": consultationID})
	if err != nil {
		return nil, fmt.Errorf("failed to query call events: %w", err)
	}

	events, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (CallEvent, error) {
		var kind, by string
		var occurredAt time.Time
		if err := row.Scan(&kind, &occurredAt, &by); err != nil {
			return CallEvent{}, err
		}

		return CallEvent{
			Time:  util.FormatInSkopje(occurredAt, "02.01.2006 15:04:05"),
			Label: callEventLabel(kind, by),
		}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read call events: %w", err)
	}

	return events, nil
}

func callEventLabel(kind, by string) string {
	label, ok := callEventLabels[kind]
	if !ok {
		return kind
	}
	if !strings.Contains(label, "%s") {
		return label
	}
	if by == "" {
		by = "unknown"
	}

	return fmt.Sprintf(label, by)
}

// VideoCallEntry is the join button on a video consultation.
type VideoCallEntry struct {
	// Show is true for the assigned doctor until the join window closes.
	Show bool
	// Open is true while the call can be entered.
	Open bool
	// OpensAt is when the call can first be entered.
	OpensAt string
}

func (h ConsultationsHandler) videoCallEntry(ctx context.Context, doctorID string, consultation Consultation, now time.Time) (VideoCallEntry, error) {
	if consultation.ScheduledAt == nil {
		return VideoCallEntry{}, nil
	}
	scheduledAt := *consultation.ScheduledAt

	target := authorization.Target{
		UserID:             doctorID,
		ConsultationStatus: consultation.ConsultationStatus,
		ConsultationType:   consultation.ConsultationType,
		WithinJoinWindow:   util.IsWithinJoinWindow(scheduledAt, now),
	}

	canView, err := h.authService.IsAllowed(ctx, "view_video_call", target)
	if err != nil {
		return VideoCallEntry{}, err
	}
	canJoin, err := h.authService.IsAllowed(ctx, "join_video_consultation", target)
	if err != nil {
		return VideoCallEntry{}, err
	}

	return VideoCallEntry{
		Show:    canView && now.Before(scheduledAt.Add(util.JoinWindowAfter)),
		Open:    canJoin,
		OpensAt: util.FormatAppointmentTime(util.JoinWindowOpensAt(scheduledAt)),
	}, nil
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
	var scheduledAt, opinionDeadline *time.Time

	err := h.db.QueryRow(ctx, `
		SELECT c.doctor_id, c.consultation_status, c.consultation_type, vc.scheduled_at, vc.expires_at_after_video
		FROM consultations c
		LEFT JOIN video_consultations vc ON vc.consultation_id = c.id
		WHERE c.id = @id
	`, pgx.NamedArgs{"id": params.ID.String()}).Scan(&doctorID, &consultationStatus, &consultationType, &scheduledAt, &opinionDeadline)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, herrors.HtmlError{Code: http.StatusNotFound, Msg: "Consultation not found", ButtonLink: "/consultations"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query consultation owner: %w", err)
	}

	target := answerTarget(doctorID, consultationStatus, consultationType, scheduledAt, opinionDeadline, time.Now())
	allowed, err := h.authService.IsAllowed(ctx, "answer_consultation", target)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, answerRefusal(params.ID.String(), target)
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
	var scheduledAt, opinionDeadline *time.Time
	var totpSecret *string
	var hasSigningKey bool

	err := h.db.QueryRow(ctx, `
		SELECT c.doctor_id, c.consultation_status, c.consultation_type, vc.scheduled_at, vc.expires_at_after_video, u.totp_secret,
			u.public_key IS NOT NULL AND u.encrypted_private_key IS NOT NULL AND u.key_revoked_at IS NULL
		FROM consultations c
		LEFT JOIN video_consultations vc ON vc.consultation_id = c.id
		JOIN users u ON u.id = c.doctor_id
		WHERE c.id = @id
	`, pgx.NamedArgs{"id": params.ID.String()}).Scan(&doctorID, &consultationStatus, &consultationType, &scheduledAt, &opinionDeadline, &totpSecret, &hasSigningKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, herrors.HtmlError{Code: http.StatusNotFound, Msg: "Consultation not found", ButtonLink: "/consultations"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query consultation owner: %w", err)
	}

	target := answerTarget(doctorID, consultationStatus, consultationType, scheduledAt, opinionDeadline, time.Now())
	allowed, err := h.authService.IsAllowed(ctx, "answer_consultation", target)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, answerRefusal(params.ID.String(), target)
	}

	// The opinion is signed in the doctor's name after it is sent, so it can't be
	// sent without a key to sign it with.
	if !hasSigningKey {
		return nil, herrors.HtmlError{
			Code:       http.StatusConflict,
			Msg:        "Your account has no signing key, so the opinion can't be signed. Contact the administrator.",
			ButtonLink: "/consultations/" + params.ID.String(),
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
		return nil, h.statusConflict(ctx, params.ID.String(), "This consultation was already answered")
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

	// Signed by a job, in the same transaction: a finished consultation always
	// gets its signature, and a failed commit leaves no job behind.
	if _, err := h.riverClient.InsertTx(ctx, tx, workers.ConsultationSignatureArgs{ConsultationID: params.ID.String()}, nil); err != nil {
		return nil, fmt.Errorf("failed to enqueue signature: %w", err)
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
	maxScheduleAhead        = 72 * time.Hour
	maxScheduleAheadUrgent  = 24 * time.Hour
	videoCallReminderBefore = 1 * time.Hour
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
		return nil, h.statusConflict(ctx, params.ID.String(), "This consultation was already scheduled")
	}

	// A trigger sets the deadline for the opinion from scheduled_at.
	var opinionDeadline time.Time
	err = tx.QueryRow(ctx, `
		UPDATE video_consultations
		SET scheduled_at = @scheduled_at
		WHERE consultation_id = @id
		RETURNING expires_at_after_video
	`, pgx.NamedArgs{"scheduled_at": scheduledAt, "id": params.ID.String()}).Scan(&opinionDeadline)
	if err != nil {
		return nil, fmt.Errorf("failed to update video consultation: %w", err)
	}

	reminder, reminderOpts := workers.DoctorDeadlineReminderJob(params.ID.String(), workers.DeadlineVideoOpinion, opinionDeadline)
	if _, err = h.riverClient.InsertTx(ctx, tx, reminder, reminderOpts); err != nil {
		return nil, fmt.Errorf("failed to enqueue opinion reminder: %w", err)
	}

	// Marks the consultation not attended if the opinion isn't written in time.
	if _, err = h.riverClient.InsertTx(ctx, tx, workers.VideoScheduledNotAttendedArgs{ConsultationID: params.ID.String()}, workers.NotAttendedJobOpts(opinionDeadline)); err != nil {
		return nil, fmt.Errorf("failed to enqueue not attended marker: %w", err)
	}

	if _, err = h.riverClient.InsertTx(ctx, tx, workers.ScheduledVideoEmailArgs{ConsultationID: params.ID.String()}, &river.InsertOpts{MaxAttempts: 5}); err != nil {
		return nil, fmt.Errorf("failed to enqueue scheduled video email: %w", err)
	}

	// A call less than an hour away gets no reminder: the patient has only just
	// received the email above, and the reminder would say about one hour.
	if remindAt := scheduledAt.Add(-videoCallReminderBefore); remindAt.After(time.Now()) {
		if _, err = h.riverClient.InsertTx(ctx, tx, workers.VideoCallReminderArgs{ConsultationID: params.ID.String()}, &river.InsertOpts{
			ScheduledAt: remindAt,
			MaxAttempts: 5,
		}); err != nil {
			return nil, fmt.Errorf("failed to enqueue video call reminder: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &api.R303{Location: api.NewOptString(fmt.Sprintf("/consultations/%s", params.ID.String()))}, nil
}

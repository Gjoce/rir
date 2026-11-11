package html

import (
	"bytes"
	"context"
	"cybermedico/authorization"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/api/util"
	api "cybermedico/cmd/cybermedico/ogen"
	"cybermedico/video"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewVideoJoinAuthorizer decides who may open the signaling socket of a call: the
// assigned doctor, or whoever holds the patient's link, and only in the join window.
func NewVideoJoinAuthorizer(db *pgxpool.Pool, authService *AuthService, joinSecret []byte) video.AuthorizeFunc {
	return func(r *http.Request, consultationID uuid.UUID) (video.Role, error) {
		ctx := r.Context()

		var doctorID, consultationStatus, consultationType string
		var scheduledAt *time.Time
		err := db.QueryRow(ctx, `
			SELECT c.doctor_id, c.consultation_status, c.consultation_type, vc.scheduled_at
			FROM consultations c
			LEFT JOIN video_consultations vc ON vc.consultation_id = c.id
			WHERE c.id = @id
		`, pgx.NamedArgs{"id": consultationID.String()}).Scan(&doctorID, &consultationStatus, &consultationType, &scheduledAt)
		if errors.Is(err, pgx.ErrNoRows) {
			// Same answer as for a call the caller may not join, so ids can't be probed.
			return "", video.ErrJoinForbidden
		}
		if err != nil {
			return "", fmt.Errorf("failed to query consultation: %w", err)
		}

		target := authorization.Target{
			UserID:             doctorID,
			ConsultationStatus: consultationStatus,
			ConsultationType:   consultationType,
			WithinJoinWindow:   scheduledAt != nil && util.IsWithinJoinWindow(*scheduledAt, time.Now()),
		}

		// The patient page always sends its link token, so a token decides the side
		// even when the browser also holds a doctor's session.
		token := r.URL.Query().Get("t")
		if token != "" {
			// The link from the email is the patient's only credential, so it must be
			// signed by us and minted for this consultation.
			if _, err := video.VerifyPatientJoinToken(joinSecret, token, consultationID.String(), time.Now()); err != nil {
				return "", video.ErrJoinForbidden
			}

			allowed, err := authService.IsAllowed(ctx, "join_video_consultation_patient", target)
			if err != nil {
				return "", err
			}
			if !allowed {
				return "", video.ErrJoinForbidden
			}

			return video.RolePatient, nil
		}

		if authService.SessionManager.GetString(ctx, SessionKeyRoles) != "doctor" {
			return "", video.ErrJoinUnauthorized
		}

		allowed, err := authService.IsAllowed(ctx, "join_video_consultation", target)
		if err != nil {
			return "", err
		}
		if !allowed {
			return "", video.ErrJoinForbidden
		}

		return video.RoleDoctor, nil
	}
}

// VideoPageHandler renders the call page for the doctor and for the patient.
type VideoPageHandler struct {
	db          *pgxpool.Pool
	authService *AuthService
	joinSecret  []byte
}

type VideoPageData struct {
	ConsultationID string
	Role           string     // "doctor" | "patient"
	Token          string     // patient-only; empty for doctor
	ScheduledAt    *time.Time // the scheduled start of the call
}

// NewVideoPageHandler wires a VideoPageHandler.
func NewVideoPageHandler(pool *pgxpool.Pool, authService *AuthService, joinSecret []byte) VideoPageHandler {
	return VideoPageHandler{
		db:          pool,
		authService: authService,
		joinSecret:  joinSecret,
	}
}

type videoConsultation struct {
	DoctorID           string
	ConsultationStatus string
	ConsultationType   string
	ScheduledAt        *time.Time
}

func (h VideoPageHandler) loadVideoConsultation(ctx context.Context, id string) (videoConsultation, error) {
	var c videoConsultation
	err := h.db.QueryRow(ctx, `
		SELECT c.doctor_id, c.consultation_status, c.consultation_type, vc.scheduled_at
		FROM consultations c
		LEFT JOIN video_consultations vc ON vc.consultation_id = c.id
		WHERE c.id = @id
	`, pgx.NamedArgs{"id": id}).Scan(&c.DoctorID, &c.ConsultationStatus, &c.ConsultationType, &c.ScheduledAt)

	return c, err
}

func (c videoConsultation) target() authorization.Target {
	return authorization.Target{
		UserID:             c.DoctorID,
		ConsultationStatus: c.ConsultationStatus,
		ConsultationType:   c.ConsultationType,
		WithinJoinWindow:   c.ScheduledAt != nil && util.IsWithinJoinWindow(*c.ScheduledAt, time.Now()),
	}
}

// ConsultationsVideoForm renders /consultations/{id}/video for the assigned doctor.
func (h VideoPageHandler) ConsultationsVideoForm(ctx context.Context, params api.ConsultationsVideoFormParams) (api.ConsultationsVideoFormRes, error) {
	consultation, err := h.loadVideoConsultation(ctx, params.ID.String())
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, herrors.HtmlError{
			Code:       http.StatusNotFound,
			Msg:        "Consultation not found",
			ButtonLink: "/consultations",
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query consultation: %w", err)
	}

	allowed, err := h.authService.IsAllowed(ctx, "join_video_consultation", consultation.target())
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, herrors.HtmlError{
			Code:       http.StatusForbidden,
			Msg:        "You don't have permission to enter video call",
			ButtonLink: "/consultations",
		}
	}

	data := VideoPageData{
		ConsultationID: params.ID.String(),
		Role:           string(video.RoleDoctor),
		ScheduledAt:    consultation.ScheduledAt,
	}

	var buff bytes.Buffer

	return &api.R200{
		Data: &buff,
	}, DoctorVideoPage(data).Render(ctx, &buff)
}

// VideoPagePatient renders /video/{id} for a patient arriving from the email link.
// The ?t= token is the patient's only credential; no session is required.
// Outside the join window the patient gets a page with the appointment time
// instead of the call.
func (h VideoPageHandler) VideoPagePatient(ctx context.Context, params api.VideoPagePatientParams) (api.VideoPagePatientRes, error) {
	now := time.Now()

	// Checked first, so a caller without a valid link learns nothing about the
	// consultation. An expired link is still ours: it gets the "time has passed"
	// page below.
	_, err := video.VerifyPatientJoinToken(h.joinSecret, params.T, params.ID.String(), now)
	linkExpired := errors.Is(err, video.ErrTokenExpired)
	if err != nil && !linkExpired {
		log.Logger.Warn("video page: patient token rejected",
			"consultation_id", params.ID.String(),
			"error", err,
		)

		return nil, herrors.HtmlError{
			Code:       http.StatusForbidden,
			Msg:        "This link is invalid. Use the link from your latest email.",
			ButtonLink: "/",
		}
	}

	consultation, err := h.loadVideoConsultation(ctx, params.ID.String())
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, herrors.HtmlError{
			Code:       http.StatusNotFound,
			Msg:        "Consultation not found",
			ButtonLink: "/",
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query consultation: %w", err)
	}

	if notice, ok := patientJoinNotice(consultation, linkExpired, now); ok {
		var buff bytes.Buffer

		return &api.R200{
			Data: &buff,
		}, PatientJoinNoticePage(notice).Render(ctx, &buff)
	}

	allowed, err := h.authService.IsAllowed(ctx, "join_video_consultation_patient", consultation.target())
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, herrors.HtmlError{
			Code:       http.StatusForbidden,
			Msg:        "You don't have permission to enter video call",
			ButtonLink: "/",
		}
	}

	data := VideoPageData{
		ConsultationID: params.ID.String(),
		Role:           string(video.RolePatient),
		Token:          params.T,
		ScheduledAt:    consultation.ScheduledAt,
	}

	var buff bytes.Buffer

	return &api.R200{
		Data: &buff,
	}, PatientVideoPage(data).Render(ctx, &buff)
}

// JoinNotice is shown to a patient whose link can't open the call right now.
type JoinNotice struct {
	Title string
	Text  string
	// AppointmentTime is the formatted start of the call, or empty if there is none.
	AppointmentTime string
	// ReloadIn is set while the call hasn't opened yet: the page reloads itself
	// when it does.
	ReloadIn time.Duration
}

// maxNoticeReload is the longest wait the notice page reloads after. Browsers
// fire longer timers early, and nobody keeps a tab open that long anyway.
const maxNoticeReload = 24 * time.Hour

// patientJoinNotice decides whether the patient sees a notice instead of the
// call, and which one. ok is false when the call may be entered now.
func patientJoinNotice(c videoConsultation, linkExpired bool, now time.Time) (JoinNotice, bool) {
	if c.ConsultationStatus == "finished" {
		return JoinNotice{
			Title: "This consultation has finished",
			Text:  "The doctor's written opinion is sent to you by email.",
		}, true
	}
	if c.ConsultationStatus != "scheduled" || c.ConsultationType != "video" || c.ScheduledAt == nil {
		return JoinNotice{
			Title: "This video consultation can't be joined",
			Text:  "It isn't scheduled. The link works once the doctor sets the time of the call.",
		}, true
	}

	appointment := util.FormatAppointmentTime(*c.ScheduledAt)
	opensAt := util.JoinWindowOpensAt(*c.ScheduledAt)

	switch {
	case now.Before(opensAt):
		reloadIn := opensAt.Sub(now)
		if reloadIn > maxNoticeReload {
			reloadIn = 0
		}

		return JoinNotice{
			Title:           "Your video consultation hasn't started yet",
			Text:            fmt.Sprintf("You can join from %s, %d minutes before the start. Keep this page open and the call opens by itself.", util.FormatAppointmentTime(opensAt), int(util.JoinWindowBefore.Minutes())),
			AppointmentTime: appointment,
			ReloadIn:        reloadIn,
		}, true
	case linkExpired || !util.IsWithinJoinWindow(*c.ScheduledAt, now):
		return JoinNotice{
			Title:           "The time to join has passed",
			Text:            fmt.Sprintf("The call could be joined until %d minutes after the start.", int(util.JoinWindowAfter.Minutes())),
			AppointmentTime: appointment,
		}, true
	}

	return JoinNotice{}, false
}

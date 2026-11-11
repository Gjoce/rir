package html

import (
	"cybermedico/authorization"
	"cybermedico/cmd/cybermedico/api/util"
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

		if authService.SessionManager.GetString(ctx, SessionKeyRoles) == "doctor" {
			allowed, err := authService.IsAllowed(ctx, "join_video_consultation", target)
			if err != nil {
				return "", err
			}
			if !allowed {
				return "", video.ErrJoinForbidden
			}

			return video.RoleDoctor, nil
		}

		token := r.URL.Query().Get("t")
		if token == "" {
			return "", video.ErrJoinUnauthorized
		}

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
}

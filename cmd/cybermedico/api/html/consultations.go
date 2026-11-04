package html

import (
	"bytes"
	"context"
	"cybermedico/authorization"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	"cybermedico/cmd/cybermedico/api/util"
	api "cybermedico/cmd/cybermedico/ogen"
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
}

func NewConsultationsHandler(db *pgxpool.Pool, authService *AuthService) ConsultationsHandler {
	return ConsultationsHandler{db: db, authService: authService}
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

package html

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"cybermedico/authorization"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	api "cybermedico/cmd/cybermedico/ogen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DoctorsHandler backs the admin page that lists every doctor and switches
// their availability for new consultations (spec 001).
type DoctorsHandler struct {
	authService *AuthService
	db          *pgxpool.Pool
}

func NewDoctorsHandler(authService *AuthService, db *pgxpool.Pool) DoctorsHandler {
	return DoctorsHandler{
		authService: authService,
		db:          db,
	}
}

// DoctorRow is one doctor on the admin doctors page.
type DoctorRow struct {
	ID             string
	DisplayName    string
	Specialization string
	Workplace      string
	Available      bool
}

// availabilityLabel is the status shown next to a doctor.
func availabilityLabel(available bool) string {
	if available {
		return "Available"
	}

	return "Unavailable"
}

// availabilityAction returns the toggle button's text and the value it sends.
// The button sends the state it wants rather than asking for a flip, so a double
// click or a second tab can't switch the doctor back by accident.
func availabilityAction(available bool) (label, next string) {
	if available {
		return "Mark unavailable", "false"
	}

	return "Mark available", "true"
}

// DoctorsForm lists every doctor, available or not, ordered by name.
func (h DoctorsHandler) DoctorsForm(ctx context.Context) (api.DoctorsFormRes, error) {
	allowed, err := h.authService.IsAllowed(ctx, "doctors_form", authorization.Target{})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, herrors.HtmlError{
			Code:       http.StatusForbidden,
			Msg:        "You are not allowed to manage doctors",
			ButtonLink: "/dashboard",
		}
	}

	rows, err := h.db.Query(ctx, `
		SELECT id, display_name, specialization, workplace, available
		FROM doctor_profile
		ORDER BY display_name, id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query doctors: %w", err)
	}

	doctors, err := pgx.CollectRows(rows, pgx.RowToStructByName[DoctorRow])
	if err != nil {
		return nil, fmt.Errorf("failed to read doctors: %w", err)
	}

	var buff bytes.Buffer

	return &api.R200{Data: &buff}, DoctorsPage(doctors).Render(ctx, &buff)
}

// DoctorAvailabilitySubmit sets whether a doctor takes new consultations. The
// public list, the profile and the order forms already show only available
// doctors; existing consultations are left as they are.
func (h DoctorsHandler) DoctorAvailabilitySubmit(ctx context.Context, request *api.DoctorAvailabilitySubmitReq, params api.DoctorAvailabilitySubmitParams) (api.DoctorAvailabilitySubmitRes, error) {
	// Checked before the lookup, so a doctor can't tell which IDs exist.
	allowed, err := h.authService.IsAllowed(ctx, "change_doctor_availability", authorization.Target{})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, herrors.HtmlError{
			Code:       http.StatusForbidden,
			Msg:        "You are not allowed to change a doctor's availability",
			ButtonLink: "/dashboard",
		}
	}

	tag, err := h.db.Exec(ctx, `
		UPDATE doctor_profile
		SET available = @available
		WHERE id = @id
	`, pgx.NamedArgs{
		"id":        params.ID.String(),
		"available": request.Available,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update doctor availability: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, herrors.HtmlError{
			Code:       http.StatusNotFound,
			Msg:        "Doctor not found",
			ButtonLink: "/doctors",
		}
	}

	return &api.R303{Location: api.NewOptString("/doctors")}, nil
}

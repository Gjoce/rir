package html

import (
	"bytes"
	"context"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	api "cybermedico/cmd/cybermedico/ogen"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SpecialistsHandler struct {
	db *pgxpool.Pool
}

func NewSpecialistsHandler(db *pgxpool.Pool) SpecialistsHandler {
	return SpecialistsHandler{db: db}
}

type SpecialistProfile struct {
	ID                string
	DisplayName       string
	PhotoUrl          string
	Specialization    string
	Subspecialization string
	Bio               string
	Workplace         string
	Languages         []string
	PriceWritten      string
	PriceVideo        string
}

// Specialist is the public profile a patient reads before ordering a consultation.
func (h SpecialistsHandler) Specialist(ctx context.Context, params api.SpecialistParams) (api.SpecialistRes, error) {
	rows, err := h.db.Query(ctx, `
		SELECT
			p.id::text AS id,
			p.display_name,
			p.photo_url,
			p.specialization,
			COALESCE(p.subspecialization, '') AS subspecialization,
			p.bio,
			p.workplace,
			ARRAY(
				SELECT dl.language_id
				FROM doctor_language dl
				WHERE dl.doctor_id = p.id
				ORDER BY dl.language_id
			) AS languages,
			p.consultation_price_written::text AS price_written,
			p.consultation_price_video::text AS price_video
		FROM doctor_profile p
		WHERE p.id = @id AND p.available
	`, pgx.NamedArgs{"id": params.ID.String()})
	if err != nil {
		return nil, fmt.Errorf("failed to query doctor: %w", err)
	}

	doctor, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[SpecialistProfile])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, herrors.HtmlError{Code: http.StatusNotFound, Msg: "This doctor is not available.", ButtonText: "Show all doctors", ButtonLink: "/"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to collect doctor: %w", err)
	}

	var buff bytes.Buffer

	return &api.R200{Data: &buff}, SpecialistHtml(doctor).Render(ctx, &buff)
}

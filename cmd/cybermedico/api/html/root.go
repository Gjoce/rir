package html

import (
	"bytes"
	"context"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	"cybermedico/cmd/cybermedico/api/util"
	api "cybermedico/cmd/cybermedico/ogen"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starfederation/datastar-go/datastar"
)

type RootHandler struct {
	db *pgxpool.Pool
}

func NewRootHandler(db *pgxpool.Pool) RootHandler {
	return RootHandler{db: db}
}

type PublicDoctor struct {
	ID                string
	DisplayName       string
	PhotoUrl          string
	Specialization    string
	Subspecialization string
	Workplace         string
	Languages         []string
	PriceWritten      string
	PriceVideo        string
}

type PublicDoctorsFilters struct {
	Specialization string `json:"specialization"`
}

// Root is the patient landing page: the list of doctors who can be ordered from.
func (h RootHandler) Root(ctx context.Context, params api.RootParams) (api.RootRes, error) {
	r := util.RequestFromContext(ctx)
	w := util.ResponseFromContext(ctx)

	filters := PublicDoctorsFilters{}
	if err := datastar.ReadSignals(r, &filters); err != nil {
		return nil, herrors.HtmlError{Code: http.StatusBadRequest, Msg: "This filter is not valid", Err: err, ButtonLink: "/"}
	}

	rows, err := h.db.Query(ctx, `
		SELECT
			p.id::text AS id,
			p.display_name,
			p.photo_url,
			p.specialization,
			COALESCE(p.subspecialization, '') AS subspecialization,
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
		WHERE p.available
			AND (@specialization = '' OR p.specialization = @specialization)
		ORDER BY p.position NULLS LAST, p.display_name
	`, pgx.NamedArgs{"specialization": filters.Specialization})
	if err != nil {
		return nil, fmt.Errorf("failed to query doctors: %w", err)
	}

	doctors, err := pgx.CollectRows(rows, pgx.RowToStructByName[PublicDoctor])
	if err != nil {
		return nil, fmt.Errorf("failed to collect doctors: %w", err)
	}

	comp := PublicDoctorsSSE(doctors, filters)

	if params.DatastarRequest.Set {
		sse := datastar.NewSSE(w, r)
		if err = sse.PatchElementTempl(comp); err != nil {
			return nil, fmt.Errorf("failed to patch doctors: %w", err)
		}

		return nil, herrors.ErrNoop
	}

	// The filter offers only specializations that have an available doctor.
	specializationRows, err := h.db.Query(ctx, `
		SELECT DISTINCT specialization
		FROM doctor_profile
		WHERE available
		ORDER BY specialization
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query specializations: %w", err)
	}

	specializations, err := pgx.CollectRows(specializationRows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("failed to collect specializations: %w", err)
	}

	var buff bytes.Buffer

	return &api.R200{Data: &buff}, RootHtml(comp, specializations, filters).Render(ctx, &buff)
}

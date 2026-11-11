package html

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"cybermedico/authorization"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/api/util"
	api "cybermedico/cmd/cybermedico/ogen"
	"cybermedico/widgets"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starfederation/datastar-go/datastar"
)

type WidgetsHandler struct {
	db          *pgxpool.Pool
	authService *AuthService
}

func NewWidgetsHandler(db *pgxpool.Pool, authService *AuthService) WidgetsHandler {
	return WidgetsHandler{
		db:          db,
		authService: authService,
	}
}

type WidgetMain struct {
	ID     string
	Title  string
	Type   string
	Config widgets.Configuration
	// Layout is the raw TEXT layout column (JSON: {"x","y","w","h"}).
	Layout string
	// GridLayout is Layout parsed for rendering. It is not a DB column.
	GridLayout WidgetLayout `db:"-"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// WidgetLayout is the GridStack cell placement for a widget, stored as JSON in
// the dashboard_widgets.layout column. Fields are float64 because GridStack
// accepts fractional rows/heights (e.g. y:2.5, h:1.5) and an int would make
// json.Unmarshal fail on those values, silently collapsing the position to 0.
type WidgetLayout struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// parseLayout decodes the layout TEXT column, falling back to a sensible
// full-width default when it is empty or malformed.
func parseLayout(s string) WidgetLayout {
	var l WidgetLayout
	if s != "" {
		_ = json.Unmarshal([]byte(s), &l)
	}
	if l.W <= 0 {
		l.W = 4
	}
	if l.H <= 0 {
		l.H = 1
	}

	return l
}

func (h WidgetsHandler) Widget(ctx context.Context, params api.WidgetParams) (api.WidgetRes, error) {
	userID := h.authService.SessionManager.GetString(ctx, SessionKeyUserID)
	role := h.authService.SessionManager.GetString(ctx, SessionKeyRoles)

	allowed, err := h.authService.IsAllowed(ctx, "view_dashboard", authorization.Target{})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, herrors.HtmlError{
			Code: http.StatusForbidden,
			Msg:  "You are not allowed to view the dashboard",
		}
	}

	widget, err := h.getWidget(ctx, params.ID.String(), role)
	if err != nil {
		return nil, err
	}

	data := h.executeWidgetData(ctx, widget, userID)

	comp := widgetHtml(widget, data)

	if params.DatastarRequest.Set {
		r := util.RequestFromContext(ctx)
		w := util.ResponseFromContext(ctx)

		sse := datastar.NewSSE(w, r)
		if err := sse.PatchElementTempl(comp, datastar.WithMode(datastar.ElementPatchModeReplace)); err != nil {
			log.Logger.Error("failed to send patch", "widget_id", widget.ID, "error", err)

			return nil, err
		}

		return nil, herrors.ErrNoop
	}

	var buff bytes.Buffer

	return &api.R200{Data: &buff}, comp.Render(ctx, &buff)
}

// executeWidgetData runs the widget's stored query. The current user's id is
// always supplied as the @user_id named parameter: doctor widgets reference it
// to scope their data, while admin (aggregate) widgets simply omit it. pgx's
// non-strict NamedArgs ignores the parameter when the query doesn't use it.
func (h WidgetsHandler) executeWidgetData(ctx context.Context, widget WidgetMain, userID string) widgets.Data {
	data := widgets.Data{
		ID:    widget.ID,
		Title: widget.Title,
	}

	rows, err := h.db.Query(ctx, widget.Config.Query, pgx.NamedArgs{"user_id": userID})
	if err != nil {
		data.Error = err.Error()

		return data
	}
	defer rows.Close()

	for _, fd := range rows.FieldDescriptions() {
		data.Columns = append(data.Columns, fd.Name)
	}

	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			data.Error = err.Error()

			return data
		}

		row := make(map[string]string, len(data.Columns))
		for i, c := range data.Columns {
			row[c] = stringifyValue(vals[i])
		}

		data.Rows = append(data.Rows, row)
	}

	if err := rows.Err(); err != nil {
		data.Error = err.Error()
	}

	return data
}

// stringifyValue renders a scanned SQL value as the string the widget renderers
// consume. JSON/array columns are marshalled; everything else uses %v. Authors
// should cast exotic types (e.g. jsonb) to text in the query.
func stringifyValue(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case time.Time:
		return val.Format(time.RFC3339)
	case map[string]any, []any:
		if ba, err := json.Marshal(val); err == nil {
			return string(ba)
		}

		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}

func (h WidgetsHandler) getWidget(ctx context.Context, id, role string) (WidgetMain, error) {
	var widget WidgetMain

	err := h.db.QueryRow(ctx, `
	SELECT id, title, type, config, created_at, updated_at
	FROM dashboard_widgets
	WHERE id = @id AND role = @role
	`, pgx.NamedArgs{
		"id":   id,
		"role": role,
	}).Scan(
		&widget.ID,
		&widget.Title,
		&widget.Type,
		&widget.Config,
		&widget.CreatedAt,
		&widget.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WidgetMain{}, herrors.HtmlError{
				Code: http.StatusNotFound,
				Msg:  "Widget not found",
				Err:  err,
			}
		}

		return WidgetMain{}, err
	}

	return widget, nil
}

package html

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"

	"cybermedico/authorization"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	"cybermedico/cmd/cybermedico/api/log"
	api "cybermedico/cmd/cybermedico/ogen"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DashboardHandler struct {
	authService *AuthService
	db          *pgxpool.Pool
}

// DashboardData holds all data the dashboard templ needs.
//
// Widgets are loaded per user from the dashboard_widgets table; their data is
// fetched lazily by the browser through Datastar (see WidgetsHandler.Widget).
// The remaining fields back the simple inline summary panels.
type DashboardData struct {
	Widgets             []WidgetMain
	MonthlyRevenue      RevenueSummary
	MonthlyRevenueError string
	// SecondaryStat backs the second summary panel. Its meaning depends on the
	// viewer's role: total doctors for admins, pending consultations for doctors.
	SecondaryStatLabel string
	SecondaryStatValue string
	SecondaryStatError string
}

// RevenueSummary backs the revenue panel: the current month's total plus a
// month-over-month change indicator (trading-style up/down tick).
type RevenueSummary struct {
	// Amount is the formatted current-month revenue, e.g. "1234.00 EUR".
	Amount string
	// Delta is the absolute percentage change vs the previous month, e.g.
	// "12.3%". Empty when there's no comparable prior month (no revenue then).
	Delta string
	// Direction is "up" or "down"; empty when Delta is empty.
	Direction string
}

func NewDashboardHandler(authService *AuthService, db *pgxpool.Pool) DashboardHandler {
	return DashboardHandler{
		authService: authService,
		db:          db,
	}
}

type UpcomingConsultations struct {
	ID                string
	PatientName       string
	PatientSurname    string
	Urgent            bool
	DoctorDisplayName string
}

func (h DashboardHandler) Dashboard(ctx context.Context) (api.DashboardRes, error) {
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

	role := h.authService.SessionManager.GetString(ctx, SessionKeyRoles)
	userID := h.authService.SessionManager.GetString(ctx, SessionKeyUserID)

	query := `
	SELECT
		c.id,
		c.patient_name,
		c.patient_surname,
		c.urgent,
		dp.display_name AS doctor_display_name
	FROM consultations c
	LEFT JOIN doctor_profile dp ON c.doctor_id = dp.id
	`
	args := pgx.NamedArgs{}

	switch role {
	case "admin":
		// Platform-wide: every doctor's open consultations.
		query += " WHERE c.consultation_status IN ('pending', 'scheduled')"
	case "doctor":
		query += " WHERE c.doctor_id = @user_id AND c.consultation_status IN ('pending', 'scheduled')"
		args["user_id"] = userID
	default:
		return nil, herrors.HtmlError{
			Code: http.StatusForbidden,
			Msg:  "You are not allowed to view the dashboard",
		}
	}

	query += " ORDER BY c.created_at DESC LIMIT 5"

	rows, err := h.db.Query(ctx, query, args)
	if err != nil {
		return nil, err
	}

	table, err := pgx.CollectRows(rows, pgx.RowToStructByName[UpcomingConsultations])
	if err != nil {
		return nil, err
	}

	widgets, err := h.getRoleWidgets(ctx, role)
	if err != nil {
		log.Logger.ErrorContext(ctx, "dashboard widgets fetch error", "error", err)
	}

	monthlyRevenue, err := h.fetchMonthlyRevenue(ctx, role, userID)
	if err != nil {
		log.Logger.ErrorContext(ctx, "dashboard monthly revenue fetch error", "error", err)
	}
	monthlyRevenueErr := err

	secondaryLabel, secondaryValue, err := h.fetchSecondaryStat(ctx, role, userID)
	if err != nil {
		log.Logger.ErrorContext(ctx, "dashboard secondary stat fetch error", "error", err)
	}
	secondaryErr := err

	data := DashboardData{
		Widgets:            widgets,
		MonthlyRevenue:     monthlyRevenue,
		SecondaryStatLabel: secondaryLabel,
		SecondaryStatValue: secondaryValue,
	}
	if monthlyRevenueErr != nil {
		data.MonthlyRevenueError = monthlyRevenueErr.Error()
	}
	if secondaryErr != nil {
		data.SecondaryStatError = secondaryErr.Error()
	}

	var buff bytes.Buffer

	return &api.R200{Data: &buff}, Dashboard(data, table).Render(ctx, &buff)
}

// getRoleWidgets loads the widgets configured for a role, ordered by their grid
// position so the dashboard renders them top-to-bottom, left-to-right. Widget
// definitions are shared across every user with the role; per-user scoping
// happens at query execution time (see WidgetsHandler.executeWidgetData).
func (h DashboardHandler) getRoleWidgets(ctx context.Context, role string) ([]WidgetMain, error) {
	rows, err := h.db.Query(ctx, `
		SELECT id, title, type, config, layout, created_at, updated_at
		FROM dashboard_widgets
		WHERE role = @role
	`, pgx.NamedArgs{
		"role": role,
	})
	if err != nil {
		return nil, err
	}

	widgets, err := pgx.CollectRows(rows, pgx.RowToStructByName[WidgetMain])
	if err != nil {
		return nil, err
	}

	for i := range widgets {
		widgets[i].GridLayout = parseLayout(widgets[i].Layout)
	}

	return widgets, nil
}

// fetchMonthlyRevenue sums this month's paid consultations alongside last
// month's, so the panel can show a month-over-month change. Admins see the whole
// platform's revenue; a doctor sees only their own (doctor_id == users.id). The
// @doctor_id filter is appended for doctors and omitted for admins.
func (h DashboardHandler) fetchMonthlyRevenue(ctx context.Context, role, userID string) (RevenueSummary, error) {
	query := `
		SELECT
			COALESCE(SUM(price) FILTER (
				WHERE created_at >= DATE_TRUNC('month', NOW())
			), 0) AS current_revenue,
			COALESCE(SUM(price) FILTER (
				WHERE created_at >= DATE_TRUNC('month', NOW()) - INTERVAL '1 month'
					AND created_at < DATE_TRUNC('month', NOW())
			), 0) AS previous_revenue
		FROM consultations
		WHERE consultation_payment_status = 'paid'
			AND created_at >= DATE_TRUNC('month', NOW()) - INTERVAL '1 month'
	`
	args := pgx.NamedArgs{}

	switch role {
	case "admin":
		// Whole-platform revenue; no additional filter.
	case "doctor":
		query += " AND doctor_id = @doctor_id"
		args["doctor_id"] = userID
	default:
		return RevenueSummary{}, fmt.Errorf("monthly revenue: unauthorized role: %s", role)
	}

	var current, previous float64
	if err := h.db.QueryRow(ctx, query, args).Scan(&current, &previous); err != nil {
		return RevenueSummary{}, fmt.Errorf("monthly revenue: %w", err)
	}

	summary := RevenueSummary{Amount: fmt.Sprintf("%.2f €", current)}

	// Only show a delta when there's a prior month to compare against;
	// otherwise the percentage is undefined (division by zero).
	if previous > 0 {
		change := (current - previous) / previous * 100
		summary.Delta = fmt.Sprintf("%.1f%%", math.Abs(change))
		if change >= 0 {
			summary.Direction = "up"
		} else {
			summary.Direction = "down"
		}
	}

	return summary, nil
}

// fetchSecondaryStat backs the second summary panel. Admins see the platform's
// total doctor count; a doctor sees how many of their own consultations are
// still pending (doctor_id == users.id).
func (h DashboardHandler) fetchSecondaryStat(ctx context.Context, role, userID string) (label, value string, err error) {
	switch role {
	case "admin":
		var total int
		if err := h.db.QueryRow(ctx, `SELECT COUNT(*) FROM doctor_profile`).Scan(&total); err != nil {
			return "", "", fmt.Errorf("total doctors: %w", err)
		}

		return "Total doctors", strconv.Itoa(total), nil
	case "doctor":
		var pending int
		if err := h.db.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM consultations
			WHERE doctor_id = @user_id
				AND consultation_status = 'pending'
		`, pgx.NamedArgs{"user_id": userID}).Scan(&pending); err != nil {
			return "", "", fmt.Errorf("pending consultations: %w", err)
		}

		return "Pending consultations", strconv.Itoa(pending), nil
	default:
		return "", "", fmt.Errorf("secondary stat: unauthorized role: %s", role)
	}
}

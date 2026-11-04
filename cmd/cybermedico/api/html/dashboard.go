package html

import (
	"bytes"
	"context"
	"cybermedico/authorization"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	api "cybermedico/cmd/cybermedico/ogen"
	"net/http"
)

type DashboardHandler struct {
	authService *AuthService
}

func NewDashboardHandler(authService *AuthService) DashboardHandler {
	return DashboardHandler{authService: authService}
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

	var buff bytes.Buffer

	return &api.R200{Data: &buff}, DashboardHtml().Render(ctx, &buff)
}

package html

import (
	"bytes"
	"context"
	api "cybermedico/cmd/cybermedico/ogen"
)

type DashboardHandler struct{}

func (h DashboardHandler) Dashboard(ctx context.Context) (api.DashboardRes, error) {
	var buff bytes.Buffer

	return &api.R200{Data: &buff}, DashboardHtml().Render(ctx, &buff)
}

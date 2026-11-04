package html

import (
	"bytes"
	"context"
	api "cybermedico/cmd/cybermedico/ogen"
)

type RootHandler struct{}

func (h RootHandler) Root(ctx context.Context) (api.RootRes, error) {
	var buff bytes.Buffer

	return &api.R200{Data: &buff}, RootHtml().Render(ctx, &buff)
}

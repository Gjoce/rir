package html

import (
	"context"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	"cybermedico/cmd/cybermedico/api/util"
	api "cybermedico/cmd/cybermedico/ogen"
	"net/http"
	"net/url"
	"strings"
)

type ThemeHandler struct{}

func (h ThemeHandler) Themes(ctx context.Context, params api.ThemesParams) (api.ThemesRes, error) {
	r := util.RequestFromContext(ctx)

	if !util.IsValidTheme(params.Theme) {
		return nil, herrors.HtmlError{Code: http.StatusBadRequest, Msg: "This theme does not exist", ButtonLink: "/"}
	}

	// Keep only the path of the Referer, otherwise a link from another site
	// could use this route to redirect visitors back to that site.
	location := "/"
	if referer, err := url.Parse(r.Referer()); err == nil && strings.HasPrefix(referer.Path, "/") && !strings.HasPrefix(referer.Path, "//") {
		location = referer.RequestURI()
	}

	return &api.R303{
		Location: api.NewOptString(location),
		SetCookie: api.NewOptString((&http.Cookie{
			Name:     util.ThemeCookie,
			Value:    params.Theme,
			Path:     "/",
			MaxAge:   365 * 24 * 60 * 60,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		}).String()),
	}, nil
}

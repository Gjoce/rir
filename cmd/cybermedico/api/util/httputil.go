package util

import (
	"context"
	"net/http"
	"strings"
)

type (
	requestCtxKey  struct{}
	responseCtxKey struct{}
	contextKey     string
)

const (
	UserRoleContextKey  contextKey = "role"
	UserEmailContextKey contextKey = "email"
	UserIDContextKey    contextKey = "user_id"
)

// ThemeCookie stores the chosen theme in its own cookie because public pages
// have to remember it for patients, who never log in.
const ThemeCookie = "cybermedico_theme"

func InjectRequestToContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), requestCtxKey{}, r)
		ctx = context.WithValue(ctx, responseCtxKey{}, w)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestFromContext(ctx context.Context) *http.Request {
	r, _ := ctx.Value(requestCtxKey{}).(*http.Request)

	return r
}

func ResponseFromContext(ctx context.Context) http.ResponseWriter {
	w, _ := ctx.Value(responseCtxKey{}).(http.ResponseWriter)

	return w
}

func GetUserID(ctx context.Context) string {
	userID, _ := ctx.Value(UserIDContextKey).(string)

	return userID
}

func IsValidTheme(theme string) bool {
	return theme == "auto" || theme == "light" || theme == "dark"
}

func GetTheme(ctx context.Context) string {
	r := RequestFromContext(ctx)
	if r == nil {
		return "auto"
	}

	cookie, err := r.Cookie(ThemeCookie)
	if err != nil || !IsValidTheme(cookie.Value) {
		return "auto"
	}

	return cookie.Value
}

func GetCurrentPath(ctx context.Context) string {
	r := RequestFromContext(ctx)
	if r == nil {
		return ""
	}

	return r.URL.Path
}

func RoleIfActive(currentPath, path string) string {
	if strings.HasPrefix(currentPath, path) {
		return "button"
	}

	return ""
}

func ClassIfActive(currentPath, path string) string {
	if strings.HasPrefix(currentPath, path) {
		return "contrast"
	}

	return "secondary"
}

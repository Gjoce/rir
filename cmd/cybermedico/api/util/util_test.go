package util

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	_ "time/tzdata" // Europe/Skopje must load even where the system has no tz database.
)

var scheduled = time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)

func TestIsWithinJoinWindow(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"11 minutes before", scheduled.Add(-11 * time.Minute), false},
		{"exactly 10 minutes before", scheduled.Add(-10 * time.Minute), true},
		{"at the start", scheduled, true},
		{"59 minutes after", scheduled.Add(59 * time.Minute), true},
		{"exactly 60 minutes after", scheduled.Add(60 * time.Minute), false},
		{"a day later", scheduled.Add(24 * time.Hour), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsWithinJoinWindow(scheduled, tt.now); got != tt.want {
				t.Errorf("IsWithinJoinWindow(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}

func TestIsWithinJoinWindowZeroTime(t *testing.T) {
	if IsWithinJoinWindow(time.Time{}, scheduled) {
		t.Error("a consultation without a scheduled time must not be joinable")
	}
}

func TestJoinWindowOpensAt(t *testing.T) {
	want := time.Date(2026, 6, 1, 9, 50, 0, 0, time.UTC)
	if got := JoinWindowOpensAt(scheduled); !got.Equal(want) {
		t.Errorf("JoinWindowOpensAt = %v, want %v", got, want)
	}
}

func TestFormatAppointmentTimeUsesSkopje(t *testing.T) {
	// 10:00 UTC in June is 12:00 in Skopje (CEST, UTC+2).
	if got, want := FormatAppointmentTime(scheduled), "01.06.2026 12:00"; got != want {
		t.Errorf("FormatAppointmentTime = %q, want %q", got, want)
	}
}

func TestFormatInSkopjeWinterTime(t *testing.T) {
	// In January Skopje is on CET, UTC+1.
	winter := time.Date(2026, 1, 15, 8, 30, 0, 0, time.UTC)
	if got, want := FormatInSkopje(winter, "15:04"), "09:30"; got != want {
		t.Errorf("FormatInSkopje = %q, want %q", got, want)
	}
}

func TestDerefNil(t *testing.T) {
	if got := Deref[string](nil, "default"); got != "default" {
		t.Errorf("Deref(nil) = %q, want %q", got, "default")
	}
}

func TestDerefValue(t *testing.T) {
	v := 42
	if got := Deref(&v, 0); got != 42 {
		t.Errorf("Deref(&42) = %d, want 42", got)
	}
}

func TestIsValidTheme(t *testing.T) {
	for _, theme := range []string{"auto", "light", "dark"} {
		if !IsValidTheme(theme) {
			t.Errorf("IsValidTheme(%q) = false, want true", theme)
		}
	}
	for _, theme := range []string{"", "Dark", "blue", "light "} {
		if IsValidTheme(theme) {
			t.Errorf("IsValidTheme(%q) = true, want false", theme)
		}
	}
}

// contextWithRequest returns a context as InjectRequestToContext would build it.
func contextWithRequest(t *testing.T, r *http.Request) context.Context {
	t.Helper()

	var ctx context.Context
	handler := InjectRequestToContext(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
	}))
	handler.ServeHTTP(httptest.NewRecorder(), r)

	return ctx
}

func TestInjectRequestToContext(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/consultations", nil)
	ctx := contextWithRequest(t, r)

	if RequestFromContext(ctx) == nil {
		t.Fatal("request missing from context")
	}
	if ResponseFromContext(ctx) == nil {
		t.Fatal("response writer missing from context")
	}
	if got := GetCurrentPath(ctx); got != "/consultations" {
		t.Errorf("GetCurrentPath = %q, want /consultations", got)
	}
}

func TestEmptyContext(t *testing.T) {
	ctx := context.Background()

	if GetCurrentPath(ctx) != "" {
		t.Error("GetCurrentPath without a request should be empty")
	}
	if GetTheme(ctx) != "auto" {
		t.Error("GetTheme without a request should be auto")
	}
	if GetUserID(ctx) != "" {
		t.Error("GetUserID without a user should be empty")
	}
}

func TestGetUserID(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDContextKey, "user-123")
	if got := GetUserID(ctx); got != "user-123" {
		t.Errorf("GetUserID = %q, want user-123", got)
	}
}

func TestGetRole(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"admin", context.WithValue(context.Background(), UserRoleContextKey, "admin"), "admin"},
		{"doctor", context.WithValue(context.Background(), UserRoleContextKey, "doctor"), "doctor"},
		{"no role in context", context.Background(), ""},
		{"role of the wrong type", context.WithValue(context.Background(), UserRoleContextKey, 42), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetRole(tt.ctx); got != tt.want {
				t.Errorf("GetRole() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetThemeFromCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: ThemeCookie, Value: "dark"})

	if got := GetTheme(contextWithRequest(t, r)); got != "dark" {
		t.Errorf("GetTheme = %q, want dark", got)
	}
}

func TestGetThemeInvalidCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: ThemeCookie, Value: "neon"})

	if got := GetTheme(contextWithRequest(t, r)); got != "auto" {
		t.Errorf("GetTheme with an invalid cookie = %q, want auto", got)
	}
}

func TestGetChartTheme(t *testing.T) {
	tests := map[string]string{"dark": "dark", "light": "white", "auto": "white"}

	for uiTheme, want := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: ThemeCookie, Value: uiTheme})

		if got := GetChartTheme(contextWithRequest(t, r)); got != want {
			t.Errorf("GetChartTheme(%q) = %q, want %q", uiTheme, got, want)
		}
	}
}

func TestRoleIfActive(t *testing.T) {
	if got := RoleIfActive("/consultations/123", "/consultations"); got != "button" {
		t.Errorf("active link role = %q, want button", got)
	}
	if got := RoleIfActive("/dashboard", "/consultations"); got != "" {
		t.Errorf("inactive link role = %q, want empty", got)
	}
}

func TestClassIfActive(t *testing.T) {
	if got := ClassIfActive("/dashboard", "/dashboard"); got != "contrast" {
		t.Errorf("active link class = %q, want contrast", got)
	}
	if got := ClassIfActive("/doctors", "/dashboard"); got != "secondary" {
		t.Errorf("inactive link class = %q, want secondary", got)
	}
}

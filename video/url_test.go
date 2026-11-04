package video_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"cybermedico/video"
)

func TestURLBuilderRequiresBaseURL(t *testing.T) {
	if _, err := video.NewURLBuilder("", make([]byte, 32)); err == nil {
		t.Error("expected error for empty base URL")
	}
}

func TestURLBuilderRequiresJoinSecret(t *testing.T) {
	if _, err := video.NewURLBuilder("https://x.test", make([]byte, 16)); err == nil {
		t.Error("expected error for short join secret")
	}
}

func TestURLBuilderTrimsTrailingSlash(t *testing.T) {
	b, err := video.NewURLBuilder("https://x.test/", make([]byte, 32))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if b.BaseURL != "https://x.test" {
		t.Errorf("BaseURL = %q, want trimmed", b.BaseURL)
	}
}

func TestPatientJoinURLEmbedsValidToken(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 17)
	}
	b, err := video.NewURLBuilder("https://cybermedico.test", secret)
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	var cid string
	scheduled := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	u, err := b.PatientJoinURL(cid, scheduled)
	if err != nil {
		t.Fatalf("url: %v", err)
	}

	wantPrefix := "https://cybermedico.test/video/" + cid + "?t="
	if !strings.HasPrefix(u, wantPrefix) {
		t.Fatalf("url = %q, want prefix %q", u, wantPrefix)
	}

	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tok := parsed.Query().Get("t")
	if tok == "" {
		t.Fatal("token missing from URL")
	}

	// Verify the embedded token is genuinely accepted.
	if _, err := video.VerifyPatientJoinToken(secret, tok, cid, scheduled); err != nil {
		t.Errorf("embedded token failed verification: %v", err)
	}
}

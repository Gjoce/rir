package video_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"cybermedico/video"
)

func testSecret() []byte {
	s := make([]byte, 32)
	for i := range s {
		s[i] = byte(i + 1)
	}

	return s
}

func TestPatientJoinTokenRoundTrip(t *testing.T) {
	secret := testSecret()
	cid := "11111111-1111-1111-1111-111111111111"
	scheduled := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)

	tok, err := video.IssuePatientJoinToken(secret, cid, scheduled)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	now := scheduled.Add(10 * time.Minute)
	claims, err := video.VerifyPatientJoinToken(secret, tok, cid, now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if claims.ConsultationID != cid {
		t.Errorf("claims.ConsultationID = %v, want %v", claims.ConsultationID, cid)
	}

	wantExp := scheduled.Add(video.PatientJoinTTL).Unix()
	if claims.ExpiresAt.Unix() != wantExp {
		t.Errorf("claims.ExpiresAt unix = %d, want %d", claims.ExpiresAt.Unix(), wantExp)
	}
}

func TestPatientJoinTokenRejectsWrongConsultation(t *testing.T) {
	secret := testSecret()
	cid := "11111111-1111-1111-1111-111111111111"
	other := "22222222-2222-2222-2222-222222222222"
	scheduled := time.Now()

	tok, err := video.IssuePatientJoinToken(secret, cid, scheduled)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	_, err = video.VerifyPatientJoinToken(secret, tok, other, scheduled)
	if !errors.Is(err, video.ErrTokenMismatch) {
		t.Errorf("expected ErrTokenMismatch, got %v", err)
	}
}

func TestPatientJoinTokenRejectsExpired(t *testing.T) {
	secret := testSecret()
	cid := "11111111-1111-1111-1111-111111111111"
	scheduled := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)

	tok, err := video.IssuePatientJoinToken(secret, cid, scheduled)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	tooLate := scheduled.Add(video.PatientJoinTTL).Add(time.Second)
	_, err = video.VerifyPatientJoinToken(secret, tok, cid, tooLate)
	if !errors.Is(err, video.ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestPatientJoinTokenRejectsTamperedSignature(t *testing.T) {
	secret := testSecret()
	cid := "11111111-1111-1111-1111-111111111111"
	scheduled := time.Now()

	tok, err := video.IssuePatientJoinToken(secret, cid, scheduled)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// Flip the first character of the signature. The last one won't do: it carries
	// two unused bits, so changing it can decode to the same bytes.
	dot := strings.IndexByte(tok, '.')
	flipped := byte('A')
	if tok[dot+1] == 'A' {
		flipped = 'B'
	}
	tampered := tok[:dot+1] + string(flipped) + tok[dot+2:]

	_, err = video.VerifyPatientJoinToken(secret, tampered, cid, scheduled)
	if !errors.Is(err, video.ErrTokenSignature) {
		t.Errorf("expected ErrTokenSignature, got %v", err)
	}
}

func TestPatientJoinTokenRejectsTamperedPayload(t *testing.T) {
	secret := testSecret()
	cid := "11111111-1111-1111-1111-111111111111"
	scheduled := time.Now()

	tok, err := video.IssuePatientJoinToken(secret, cid, scheduled)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// Flip the first character of the payload segment. Not the last one: like the
	// signature's, it carries unused bits, so changing it can decode to the same bytes.
	dot := strings.IndexByte(tok, '.')
	if dot <= 0 {
		t.Fatalf("token has no '.': %q", tok)
	}

	flipIdx := 0
	orig := tok[flipIdx]
	swap := byte('A')
	if orig == 'A' {
		swap = 'B'
	}
	tampered := tok[:flipIdx] + string(swap) + tok[flipIdx+1:]

	_, err = video.VerifyPatientJoinToken(secret, tampered, cid, scheduled)
	// Either signature fails (most likely) or payload no longer decodes.
	if !errors.Is(err, video.ErrTokenSignature) && !errors.Is(err, video.ErrTokenMalformed) {
		t.Errorf("expected ErrTokenSignature or ErrTokenMalformed, got %v", err)
	}
}

func TestPatientJoinTokenRejectsMalformed(t *testing.T) {
	secret := testSecret()
	cid := "11111111-1111-1111-1111-111111111111"
	now := time.Now()

	cases := []string{
		"",
		"no-dot-here",
		".",
		"!!!.!!!",
		"abc.",
		".abc",
	}
	for _, tc := range cases {
		_, err := video.VerifyPatientJoinToken(secret, tc, cid, now)
		if err == nil {
			t.Errorf("token %q: expected error, got nil", tc)
		}
	}
}

func TestIssuePatientJoinTokenRejectsShortSecret(t *testing.T) {
	short := make([]byte, 16)
	_, err := video.IssuePatientJoinToken(short, "11111111-1111-1111-1111-111111111111", time.Now())
	if err == nil {
		t.Fatal("expected error for short secret, got nil")
	}
}

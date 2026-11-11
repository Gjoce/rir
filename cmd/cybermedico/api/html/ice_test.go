package html

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // G505: TURN REST API (use-auth-secret) mandates HMAC-SHA1 per RFC 8489; not our choice
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestIssueICEConfigUsernameAndCredentialMatchCoturnScheme(t *testing.T) {
	cfg := TURNConfig{
		Hostname:      "turn.example.com",
		SharedSecret:  []byte("super-secret"),
		CredentialTTL: 2 * time.Hour,
	}
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	sessionID := "consult-abc"

	got, err := issueICEConfig(cfg, sessionID, now)
	if err != nil {
		t.Fatalf("issueICEConfig: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(servers) = %d, want 1", len(got))
	}
	turn := got[0]

	// Username layout: "<expiry_unix>:<sessionID>".
	parts := strings.SplitN(turn.Username, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("username %q has no colon", turn.Username)
	}
	expUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		t.Fatalf("username expiry not an int: %v", err)
	}
	if expUnix != now.Add(cfg.CredentialTTL).Unix() {
		t.Errorf("expiry = %d, want %d", expUnix, now.Add(cfg.CredentialTTL).Unix())
	}
	if parts[1] != sessionID {
		t.Errorf("username session id = %q, want %q", parts[1], sessionID)
	}

	// Credential = base64(HMAC-SHA1(username, sharedSecret)).
	mac := hmac.New(sha1.New, cfg.SharedSecret)
	mac.Write([]byte(turn.Username))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if turn.Credential != want {
		t.Errorf("credential = %q, want %q", turn.Credential, want)
	}
}

func TestIssueICEConfigIncludesAllTURNURLs(t *testing.T) {
	cfg := TURNConfig{
		Hostname:      "turn.example.com",
		SharedSecret:  []byte("x"),
		CredentialTTL: time.Hour,
	}
	got, err := issueICEConfig(cfg, "s", time.Now())
	if err != nil {
		t.Fatalf("issueICEConfig: %v", err)
	}

	want := []string{
		"turn:turn.example.com:3478?transport=udp",
		"turn:turn.example.com:3478?transport=tcp",
		"turns:turn.example.com:5349?transport=tcp",
		"turns:turn.example.com:443?transport=tcp",
	}
	if len(got[0].URLs) != len(want) {
		t.Fatalf("got %d urls, want %d", len(got[0].URLs), len(want))
	}
	for i, u := range want {
		if got[0].URLs[i] != u {
			t.Errorf("urls[%d] = %q, want %q", i, got[0].URLs[i], u)
		}
	}
}

func TestIssueICEConfigPrependsSTUN(t *testing.T) {
	cfg := TURNConfig{
		Hostname:      "turn.example.com",
		SharedSecret:  []byte("x"),
		CredentialTTL: time.Hour,
		STUNURLs:      []string{"stun:stun.example.com:3478"},
	}
	got, err := issueICEConfig(cfg, "s", time.Now())
	if err != nil {
		t.Fatalf("issueICEConfig: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(servers) = %d, want 2", len(got))
	}
	if got[0].URLs[0] != "stun:stun.example.com:3478" {
		t.Errorf("stun first url = %q", got[0].URLs[0])
	}
	if got[0].Username != "" || got[0].Credential != "" {
		t.Errorf("stun entry must not carry credentials")
	}
}

func TestIssueICEConfigValidatesConfig(t *testing.T) {
	now := time.Now()
	cases := map[string]TURNConfig{
		"empty hostname": {SharedSecret: []byte("x"), CredentialTTL: time.Hour},
		"empty secret":   {Hostname: "h", CredentialTTL: time.Hour},
		"zero ttl":       {Hostname: "h", SharedSecret: []byte("x")},
		"negative ttl":   {Hostname: "h", SharedSecret: []byte("x"), CredentialTTL: -time.Second},
	}
	for name, cfg := range cases {
		_, err := issueICEConfig(cfg, "s", now)
		if err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestIssueICEConfigCredentialChangesWithTime(t *testing.T) {
	cfg := TURNConfig{
		Hostname:      "turn.example.com",
		SharedSecret:  []byte("super-secret"),
		CredentialTTL: time.Hour,
	}
	a, errA := issueICEConfig(cfg, "s", time.Unix(1_000_000, 0))
	b, errB := issueICEConfig(cfg, "s", time.Unix(2_000_000, 0))
	if errA != nil || errB != nil {
		t.Fatalf("issue: %v %v", errA, errB)
	}
	if a[0].Credential == b[0].Credential {
		t.Error("credential did not change when expiry changed")
	}
	if a[0].Username == b[0].Username {
		t.Error("username did not change when expiry changed")
	}
	// Sanity: username format.
	if !strings.Contains(a[0].Username, fmt.Sprintf("%d:", 1_000_000+3600)) {
		t.Errorf("unexpected username %q", a[0].Username)
	}
}

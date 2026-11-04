package video

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// URLBuilder constructs patient-facing join URLs that include a fresh HMAC
// token. The same instance is used everywhere an email or notification needs
// to embed a one-click join link.
type URLBuilder struct {
	BaseURL    string
	JoinSecret []byte
}

// NewURLBuilder validates inputs and returns a URLBuilder. Returns an error
// if BaseURL is empty or JoinSecret is too short for HMAC use.
func NewURLBuilder(baseURL string, joinSecret []byte) (*URLBuilder, error) {
	if baseURL == "" {
		return nil, errors.New("video: URLBuilder base URL is empty")
	}
	if len(joinSecret) < 32 {
		return nil, errors.New("video: URLBuilder join secret must be at least 32 bytes")
	}

	return &URLBuilder{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		JoinSecret: joinSecret,
	}, nil
}

// PatientJoinURL mints a token for consultationID + scheduledAt and embeds it
// in the patient-facing join URL. The token's expiry is derived from
// scheduledAt by IssuePatientJoinToken.
func (b *URLBuilder) PatientJoinURL(consultationID string, scheduledAt time.Time) (string, error) {
	token, err := IssuePatientJoinToken(b.JoinSecret, consultationID, scheduledAt)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/video/%s?t=%s", b.BaseURL, consultationID, url.QueryEscape(token)), nil
}

// DoctorJoinURL is the in-dashboard URL the doctor follows from an email.
// No token — the doctor's session cookie carries auth.
func (b *URLBuilder) DoctorJoinURL(consultationID string) string {
	return fmt.Sprintf("%s/consultations/%s/video", b.BaseURL, consultationID)
}

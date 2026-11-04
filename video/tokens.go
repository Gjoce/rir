// Package video implements the WebRTC signaling layer for doctor↔patient
// video consultations: stateless join tokens for patients, short-lived TURN
// credentials, and the in-memory signaling broker that relays SDP and ICE
// between the two peers.
//
// Patients have no session in the rest of the application — they join from a
// link in an email — so this package issues HMAC-signed tokens scoped to a
// single consultation. Doctors authenticate via the existing scs session,
// which is why no doctor-token logic lives here.
package video

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// PatientJoinTTL is how long after the scheduled start a patient join token
// remains valid. Picked to cover late arrivals and reschedule grace without
// leaving forever-valid links sitting in inboxes.
const PatientJoinTTL = 90 * time.Minute

var (
	ErrTokenMalformed = errors.New("video: join token is malformed")
	ErrTokenSignature = errors.New("video: join token signature does not match")
	ErrTokenExpired   = errors.New("video: join token expired")
	ErrTokenMismatch  = errors.New("video: join token does not match consultation")
)

// PatientJoinClaims is the verified payload of a patient join token.
type PatientJoinClaims struct {
	ConsultationID string
	ExpiresAt      time.Time
}

// IssuePatientJoinToken builds a token that authorizes a patient to join the
// signaling room for consultationID until scheduledAt + PatientJoinTTL.
//
// Wire format: base64url(payload) "." base64url(hmac-sha256(payload, secret))
// where payload is "{consultationID}:{exp_unix}". Two-segment, stateless,
// safe to put in a URL query string.
//
// secret must be at least 32 bytes. The same secret must be used by the
// verifier; rotating it invalidates every outstanding link.
func IssuePatientJoinToken(secret []byte, consultationID string, scheduledAt time.Time) (string, error) {
	if len(secret) < 32 {
		return "", errors.New("video: join token secret must be at least 32 bytes")
	}

	exp := scheduledAt.Add(PatientJoinTTL).Unix()
	payload := fmt.Sprintf("%s:%d", consultationID, exp)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	sig := mac.Sum(nil)

	enc := base64.RawURLEncoding

	return enc.EncodeToString([]byte(payload)) + "." + enc.EncodeToString(sig), nil
}

// VerifyPatientJoinToken parses a token, verifies its signature, checks
// expiration, and confirms it was minted for expectedConsultationID.
//
// Verification is constant-time with respect to the signature comparison.
// Token freshness uses now as the reference clock so callers can inject a
// clock in tests.
func VerifyPatientJoinToken(secret []byte, token string, expectedConsultationID string, now time.Time) (PatientJoinClaims, error) {
	enc := base64.RawURLEncoding

	dot := strings.IndexByte(token, '.')
	if dot < 0 {
		return PatientJoinClaims{}, ErrTokenMalformed
	}

	payloadB64, sigB64 := token[:dot], token[dot+1:]

	payload, err := enc.DecodeString(payloadB64)
	if err != nil {
		return PatientJoinClaims{}, ErrTokenMalformed
	}

	sig, err := enc.DecodeString(sigB64)
	if err != nil {
		return PatientJoinClaims{}, ErrTokenMalformed
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	expected := mac.Sum(nil)

	if subtle.ConstantTimeCompare(sig, expected) != 1 {
		return PatientJoinClaims{}, ErrTokenSignature
	}

	colon := strings.IndexByte(string(payload), ':')
	if colon < 0 {
		return PatientJoinClaims{}, ErrTokenMalformed
	}

	cidStr, expStr := string(payload[:colon]), string(payload[colon+1:])

	if cidStr != expectedConsultationID {
		return PatientJoinClaims{}, ErrTokenMismatch
	}

	expUnix, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return PatientJoinClaims{}, ErrTokenMalformed
	}

	expiresAt := time.Unix(expUnix, 0)
	if !now.Before(expiresAt) {
		return PatientJoinClaims{}, ErrTokenExpired
	}

	return PatientJoinClaims{
		ConsultationID: cidStr,
		ExpiresAt:      expiresAt,
	}, nil
}

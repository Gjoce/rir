package signing

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Algorithm names the signature scheme in a SignatureRecord.
const Algorithm = "Ed25519"

// SignableConsultation is the canonical payload that gets hashed and signed.
// Field declaration order determines JSON key order, so the hash is stable
// across verifications.
//
// Every field the doctor reviewed when forming their opinion is included:
// changing any of them after signing makes verification fail.
type SignableConsultation struct {
	// Consultation identity
	ConsultationID   string `json:"consultation_id"`
	DoctorID         string `json:"doctor_id"`
	DoctorName       string `json:"doctor_name"`       // name at time of signing
	ConsultationType string `json:"consultation_type"` // "written" | "video"

	// Patient identity
	PatientEmail   string `json:"patient_email"`
	PatientName    string `json:"patient_name"`
	PatientSurname string `json:"patient_surname"`
	PatientAge     int    `json:"patient_age"`

	// Medical intake the doctor reviewed, so the opinion can't be moved to
	// another context.
	MedicalProblem   string `json:"medical_problem"`
	PatientDiagnosis string `json:"patient_diagnosis"` // empty string if not provided
	Allergies        string `json:"allergies"`         // empty string if none
	Medication       string `json:"medication"`        // empty string if none

	// The doctor's opinion, as sent to the patient (sanitized HTML)
	Answer string `json:"answer"`

	// For video consultations: when the call was scheduled. Empty for written ones.
	ScheduledAt string `json:"scheduled_at"`

	// When this signature was created
	SignedAt string `json:"signed_at"` // RFC3339 UTC
}

// SignatureRecord is stored write-once in the signatures bucket. The public key
// in it is the one the signature is checked with.
type SignatureRecord struct {
	Algorithm      string               `json:"algorithm"`
	ConsultationID string               `json:"consultation_id"`
	DoctorID       string               `json:"doctor_id"`
	PublicKey      string               `json:"public_key"` // base64 Ed25519 public key
	SignedPayload  SignableConsultation `json:"signed_payload"`
	PayloadHash    string               `json:"payload_hash"` // hex SHA-256 of signed_payload
	Signature      string               `json:"signature"`    // base64 Ed25519 signature
}

// ErrNotFinished means the consultation has no opinion to sign yet.
var ErrNotFinished = errors.New("signing: the consultation isn't finished")

// Sign signs payload with the doctor's private key.
func Sign(priv ed25519.PrivateKey, payload SignableConsultation) (SignatureRecord, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return SignatureRecord{}, fmt.Errorf("signing: marshal payload: %w", err)
	}
	hash := sha256.Sum256(payloadBytes)

	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return SignatureRecord{}, errors.New("signing: unexpected public key type")
	}

	return SignatureRecord{
		Algorithm:      Algorithm,
		ConsultationID: payload.ConsultationID,
		DoctorID:       payload.DoctorID,
		PublicKey:      base64.StdEncoding.EncodeToString(pub),
		SignedPayload:  payload,
		PayloadHash:    hex.EncodeToString(hash[:]),
		Signature:      base64.StdEncoding.EncodeToString(ed25519.Sign(priv, hash[:])),
	}, nil
}

// Verify checks that the record is intact and signed by its public key.
func Verify(record SignatureRecord) bool {
	if record.Algorithm != Algorithm ||
		record.ConsultationID != record.SignedPayload.ConsultationID ||
		record.DoctorID != record.SignedPayload.DoctorID {
		return false
	}

	payloadBytes, err := json.Marshal(record.SignedPayload)
	if err != nil {
		return false
	}
	hash := sha256.Sum256(payloadBytes)
	if hex.EncodeToString(hash[:]) != record.PayloadHash {
		return false
	}

	pub, err := base64.StdEncoding.DecodeString(record.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(record.Signature)
	if err != nil {
		return false
	}

	return ed25519.Verify(pub, hash[:], sig)
}

// VerifyAgainst checks the record and that the consultation still holds the data
// that was signed. current is the payload built from the database now; its
// SignedAt is ignored.
func VerifyAgainst(record SignatureRecord, current SignableConsultation) bool {
	if !Verify(record) {
		return false
	}
	current.SignedAt = record.SignedPayload.SignedAt

	return current == record.SignedPayload
}

// Querier is the part of pgx used to load a payload: a pool or a transaction.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// LoadPayload builds the payload to sign, or to verify, from the database. The
// consultation must be finished. SignedAt is left empty.
func LoadPayload(ctx context.Context, db Querier, consultationID string) (SignableConsultation, error) {
	rows, err := db.Query(ctx, `
		SELECT
			c.id::text,
			c.doctor_id::text,
			p.display_name,
			c.consultation_type,
			c.consultation_status,
			c.patient_email,
			c.patient_name,
			c.patient_surname,
			c.patient_age,
			c.medical_problem,
			COALESCE(c.patient_diagnosis, ''),
			COALESCE(c.allergies, ''),
			COALESCE(c.medication, ''),
			COALESCE(wc.doctor_answer, vc.doctor_answer_after_video, ''),
			vc.scheduled_at
		FROM consultations c
		JOIN doctor_profile p ON p.id = c.doctor_id
		LEFT JOIN written_consultations wc ON wc.consultation_id = c.id
		LEFT JOIN video_consultations vc ON vc.consultation_id = c.id
		WHERE c.id = @id
	`, pgx.NamedArgs{"id": consultationID})
	if err != nil {
		return SignableConsultation{}, fmt.Errorf("signing: query consultation: %w", err)
	}

	var payload SignableConsultation
	var status string
	var scheduledAt *time.Time
	_, err = pgx.CollectExactlyOneRow(rows, func(row pgx.CollectableRow) (struct{}, error) {
		return struct{}{}, row.Scan(
			&payload.ConsultationID, &payload.DoctorID, &payload.DoctorName, &payload.ConsultationType, &status,
			&payload.PatientEmail, &payload.PatientName, &payload.PatientSurname, &payload.PatientAge,
			&payload.MedicalProblem, &payload.PatientDiagnosis, &payload.Allergies, &payload.Medication,
			&payload.Answer, &scheduledAt,
		)
	})
	if err != nil {
		return SignableConsultation{}, fmt.Errorf("signing: load consultation: %w", err)
	}
	if status != "finished" {
		return SignableConsultation{}, ErrNotFinished
	}
	if scheduledAt != nil {
		payload.ScheduledAt = scheduledAt.UTC().Format(time.RFC3339)
	}

	return payload, nil
}

package signing_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"reflect"
	"testing"

	"cybermedico/cmd/cybermedico/signing"
)

func samplePayload() signing.SignableConsultation {
	return signing.SignableConsultation{
		ConsultationID:   "11111111-1111-1111-1111-111111111111",
		DoctorID:         "22222222-2222-2222-2222-222222222222",
		DoctorName:       "Dr. Emily Carter",
		ConsultationType: "video",
		PatientEmail:     "laura@example.com",
		PatientName:      "Laura",
		PatientSurname:   "Anderson",
		PatientAge:       52,
		MedicalProblem:   "Chest pain",
		PatientDiagnosis: "Hypertension",
		Allergies:        "Penicillin",
		Medication:       "Ramipril",
		Answer:           "<p>Continue the therapy.</p>",
		ScheduledAt:      "2026-11-12T19:30:00Z",
		SignedAt:         "2026-11-12T21:00:00Z",
	}
}

func signSample(t *testing.T) signing.SignatureRecord {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	record, err := signing.Sign(priv, samplePayload())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	return record
}

func TestSignedOpinionVerifies(t *testing.T) {
	record := signSample(t)
	if !signing.Verify(record) {
		t.Fatal("a fresh signature doesn't verify")
	}

	current := samplePayload()
	current.SignedAt = "" // the database doesn't know when it was signed
	if !signing.VerifyAgainst(record, current) {
		t.Fatal("unchanged consultation data doesn't verify")
	}
}

// Changing any signed field, in the record or in the database, must make
// verification fail.
func TestChangingAnySignedFieldFailsVerification(t *testing.T) {
	fields := reflect.TypeFor[signing.SignableConsultation]()
	for i := range fields.NumField() {
		field := fields.Field(i)
		t.Run(field.Name, func(t *testing.T) {
			record := signSample(t)

			changed := samplePayload()
			v := reflect.ValueOf(&changed).Elem().Field(i)
			switch v.Kind() {
			case reflect.String:
				v.SetString(v.String() + "x")
			case reflect.Int:
				v.SetInt(v.Int() + 1)
			default:
				t.Fatalf("no change defined for %s", v.Kind())
			}

			tampered := record
			tampered.SignedPayload = changed
			if signing.Verify(tampered) {
				t.Errorf("a record with a changed %s still verifies", field.Name)
			}

			if field.Name != "SignedAt" && signing.VerifyAgainst(record, changed) {
				t.Errorf("a consultation whose %s changed in the database still verifies", field.Name)
			}
		})
	}
}

func TestVerifyRejectsForgedRecords(t *testing.T) {
	record := signSample(t)

	otherPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wrongKey := record
	wrongKey.PublicKey = base64.StdEncoding.EncodeToString(otherPub)
	if signing.Verify(wrongKey) {
		t.Error("verified with another doctor's public key")
	}

	// Changed data signed with another key doesn't pass as the doctor's.
	_, forger, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	changed := samplePayload()
	changed.Answer = "<p>Stop the therapy.</p>"
	forged, err := signing.Sign(forger, changed)
	if err != nil {
		t.Fatal(err)
	}
	forged.PublicKey = record.PublicKey
	if signing.Verify(forged) {
		t.Error("a signature made with another key verified against the doctor's key")
	}

	badSig := record
	badSig.Signature = "not base64"
	if signing.Verify(badSig) {
		t.Error("an undecodable signature verified")
	}

	wrongID := record
	wrongID.ConsultationID = "33333333-3333-3333-3333-333333333333"
	if signing.Verify(wrongID) {
		t.Error("a record for another consultation verified")
	}
}

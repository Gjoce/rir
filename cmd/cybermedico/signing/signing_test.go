package signing

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

const testSecret = "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"

func TestMatchTOTPStep(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	currentStep := now.Unix() / totpPeriodSeconds

	tests := []struct {
		name     string
		codeAt   time.Time
		wantStep int64
		wantOK   bool
	}{
		{"current code", now, currentStep, true},
		{"previous step within skew", now.Add(-30 * time.Second), currentStep - 1, true},
		{"next step within skew", now.Add(30 * time.Second), currentStep + 1, true},
		{"too old", now.Add(-60 * time.Second), 0, false},
		{"too new", now.Add(60 * time.Second), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := totp.GenerateCode(testSecret, tt.codeAt)
			if err != nil {
				t.Fatal(err)
			}

			step, ok := MatchTOTPStep(code, testSecret, now)
			if ok != tt.wantOK || step != tt.wantStep {
				t.Fatalf("got step %d ok %v, want step %d ok %v", step, ok, tt.wantStep, tt.wantOK)
			}
		})
	}
}

func TestMatchTOTPStepRejectsBadInput(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)

	code, err := totp.GenerateCode(testSecret, now)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := MatchTOTPStep(code, "JBSWY3DPEHPK3PXQJBSWY3DPEHPK3PXQ", now); ok {
		t.Fatal("code for another secret was accepted")
	}
	if _, ok := MatchTOTPStep("", testSecret, now); ok {
		t.Fatal("empty code was accepted")
	}
	if _, ok := MatchTOTPStep(code, "not base32!", now); ok {
		t.Fatal("invalid secret was accepted")
	}
	if _, ok := MatchTOTPStep(" "+code+" ", testSecret, now); !ok {
		t.Fatal("code with surrounding spaces was refused")
	}
}

func TestGenerateTOTPSecret(t *testing.T) {
	key, err := GenerateTOTPSecret("CyberMedico", "dr.carter@cybermedico.com")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	code, err := totp.GenerateCode(key.Secret, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := MatchTOTPStep(code, key.Secret, now); !ok {
		t.Fatal("code from a generated secret was refused")
	}
}

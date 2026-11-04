package signing

import (
	"crypto/subtle"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
)

const totpPeriodSeconds = 30

// TOTPKey holds the generated secret and the otpauth:// URI for authenticator apps.
type TOTPKey struct {
	Secret string // Base32-encoded, paste into any authenticator app
	URL    string // otpauth:// deep link
}

// GenerateTOTPSecret generates a new TOTP secret for the given issuer and account.
func GenerateTOTPSecret(issuer, accountName string) (TOTPKey, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
	})
	if err != nil {
		return TOTPKey{}, err
	}

	return TOTPKey{
		Secret: key.Secret(),
		URL:    key.URL(),
	}, nil
}

// MatchTOTPStep checks code against the Base32 secret, accepting one step either
// way to tolerate clock skew, and returns the time step the code belongs to.
// Callers store that step and refuse any step not newer than it. Storing the
// current step instead would let a code be used again in the next 30 seconds,
// while the skew still accepts it.
func MatchTOTPStep(code, secret string, now time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	currentStep := now.Unix() / totpPeriodSeconds

	for step := currentStep - 1; step <= currentStep+1; step++ {
		expected, err := totp.GenerateCode(secret, time.Unix(step*totpPeriodSeconds, 0))
		if err != nil {
			return 0, false
		}

		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return step, true
		}
	}

	return 0, false
}

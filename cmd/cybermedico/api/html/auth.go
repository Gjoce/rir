package html

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"time"

	"cybermedico/authorization"
	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/api/util"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	api "cybermedico/cmd/cybermedico/ogen"
	"cybermedico/cmd/cybermedico/signing"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	SessionKeyEmail             = "email"
	SessionKeyUserID            = "user_id"
	SessionKeyRoles             = "roles"
	SessionKeyPendingUserID     = "pending_user_id"
	SessionKeyPendingTOTPSecret = "pending_totp_secret" //nolint:gosec
	SessionKeyPendingExpiresAt  = "pending_expires_at"
	PendingSessionTTL           = 5 * time.Minute
)

var errNotLoggedIn = errors.New("user not found in session")

type AuthService struct {
	SessionManager *scs.SessionManager
	authorizer     authorization.Authorizer
}

func (as *AuthService) IsAllowed(ctx context.Context, operationID string, target authorization.Target) (bool, error) {
	role, ok := as.SessionManager.Get(ctx, SessionKeyRoles).(string)
	if !ok {
		role = ""
	}

	return as.authorizer.IsAllowed(ctx, operationID, authorization.Caller{
		UserID: as.SessionManager.GetString(ctx, SessionKeyUserID),
		Role:   role,
	}, target)
}

type AuthHandler struct {
	AuthService *AuthService
	db          *pgxpool.Pool
}

func NewAuthHandler(db *pgxpool.Pool, authService *AuthService) AuthHandler {
	return AuthHandler{
		AuthService: authService,
		db:          db,
	}
}

func NewAuthService(db *pgxpool.Pool, authorizer authorization.Authorizer) (*AuthService, error) {
	sessionManager := scs.New()
	sessionManager.Store = pgxstore.New(db)
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.Secure = true
	sessionManager.Cookie.Name = "cybermedico_session"

	return &AuthService{
		SessionManager: sessionManager,
		authorizer:     authorizer,
	}, nil
}

// SessionMiddleware loads session data and injects user info into the request context.
func (a *AuthService) SessionMiddleware(next http.Handler) http.Handler {
	return a.SessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := a.SessionManager.GetString(r.Context(), SessionKeyRoles)
		email := a.SessionManager.GetString(r.Context(), SessionKeyEmail)
		userId := a.SessionManager.GetString(r.Context(), SessionKeyUserID)
		ctx := r.Context()

		if role != "" {
			ctx = context.WithValue(ctx, util.UserRoleContextKey, role)
		}

		if email != "" {
			ctx = context.WithValue(ctx, util.UserEmailContextKey, email)
		}

		if userId != "" {
			ctx = context.WithValue(ctx, util.UserIDContextKey, userId)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	}))
}

func (h AuthHandler) clearAuthenticationSession(r *http.Request) {
	h.AuthService.SessionManager.Remove(r.Context(), SessionKeyEmail)
	h.AuthService.SessionManager.Remove(r.Context(), SessionKeyUserID)
	h.AuthService.SessionManager.Remove(r.Context(), SessionKeyRoles)
	h.AuthService.SessionManager.Remove(r.Context(), SessionKeyPendingUserID)
	h.AuthService.SessionManager.Remove(r.Context(), SessionKeyPendingTOTPSecret)
	h.AuthService.SessionManager.Remove(r.Context(), SessionKeyPendingExpiresAt)
}

// pendingUserID returns the user who passed the password step and has not expired,
// or "" when the visitor has to start again from the login page.
func (h AuthHandler) pendingUserID(ctx context.Context) string {
	expiresAt := h.AuthService.SessionManager.GetInt64(ctx, SessionKeyPendingExpiresAt)
	if expiresAt == 0 || time.Now().Unix() > expiresAt {
		return ""
	}

	return h.AuthService.SessionManager.GetString(ctx, SessionKeyPendingUserID)
}

// completeLogin turns the pending session into a full one. The new token means a
// session token planted before login is useless.
func (h AuthHandler) completeLogin(ctx context.Context, userID, email, role string) error {
	h.clearAuthenticationSession(util.RequestFromContext(ctx))

	if err := h.AuthService.SessionManager.RenewToken(ctx); err != nil {
		return err
	}
	h.AuthService.SessionManager.Put(ctx, SessionKeyUserID, userID)
	h.AuthService.SessionManager.Put(ctx, SessionKeyEmail, email)
	h.AuthService.SessionManager.Put(ctx, SessionKeyRoles, role)

	return nil
}

// HandleSessionCookie rejects cookies without a logged-in user, for example an
// expired session, and the error handler then redirects to the login page.
func (h AuthHandler) HandleSessionCookie(ctx context.Context, operationName api.OperationName, t api.SessionCookie) (context.Context, error) {
	user := h.AuthService.SessionManager.GetString(ctx, SessionKeyUserID)
	if user == "" {
		return ctx, errNotLoggedIn
	}

	return ctx, nil
}

// --- login page ---

type loginData struct {
	Email    string
	EmailErr string
}

func (l loginData) Valid() bool {
	return l.EmailErr == ""
}

func (h AuthHandler) Login(ctx context.Context) (api.LoginRes, error) {
	var buff bytes.Buffer

	return &api.R200{Data: &buff}, login(loginData{}).Render(ctx, &buff)
}

// LoginSubmit validates credentials and, on success, sends the user to the TOTP
// step, or to TOTP setup on the first login. No full session is granted here.
func (h AuthHandler) LoginSubmit(ctx context.Context, request *api.LoginSubmitReq) (api.LoginSubmitRes, error) {
	r := util.RequestFromContext(ctx)
	h.clearAuthenticationSession(r)

	email := request.Email
	password := request.Password

	data := loginData{Email: email}

	if email == "" {
		data.EmailErr = "Email is required"
	}

	if !data.Valid() {
		var buff bytes.Buffer

		return &api.R200{Data: &buff}, login(data).Render(ctx, &buff)
	}

	// The join leaves out accounts without a role, because only doctors and
	// administrators log in.
	query := `
	SELECT u.id, u.password_hash, u.totp_enrolled OR u.totp_pending_secret IS NOT NULL
	FROM users u
	JOIN users_roles r ON u.id = r.user_id
	WHERE u.email = @email
	`

	var userID, passwordHash string
	var hasTOTPSecret bool
	err := h.db.QueryRow(ctx, query, pgx.NamedArgs{"email": email}).Scan(&userID, &passwordHash, &hasTOTPSecret)
	if err == pgx.ErrNoRows {
		data.EmailErr = "Invalid email or password"
		var buff bytes.Buffer

		return &api.R200{Data: &buff}, login(data).Render(ctx, &buff)
	} else if err != nil {
		log.Logger.Error("database error during login", "error", err)

		return &api.R303{Location: api.NewOptString("/auth/login")}, nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		data.EmailErr = "Invalid email or password"
		var buff bytes.Buffer

		return &api.R200{Data: &buff}, login(data).Render(ctx, &buff)
	}

	// Password verified — gate on TOTP before granting a full session.
	h.AuthService.SessionManager.Put(ctx, SessionKeyPendingUserID, userID)
	h.AuthService.SessionManager.Put(ctx, SessionKeyPendingExpiresAt, time.Now().Add(PendingSessionTTL).Unix())

	if !hasTOTPSecret {
		return &api.R303{Location: api.NewOptString("/auth/2fa/setup")}, nil
	}

	return &api.R303{Location: api.NewOptString("/auth/2fa")}, nil
}

func (h AuthHandler) Logout(ctx context.Context) (api.LogoutRes, error) {
	if err := h.AuthService.SessionManager.Destroy(ctx); err != nil {
		return nil, err
	}

	return &api.R303{Location: api.NewOptString("/auth/login")}, nil
}

// --- 2FA page ---

// TwoFactorForm renders the 6-digit code entry page.
func (h AuthHandler) TwoFactorForm(ctx context.Context) (api.TwoFactorFormRes, error) {
	if h.pendingUserID(ctx) == "" {
		return &api.R303{Location: api.NewOptString("/auth/login")}, nil
	}

	var buff bytes.Buffer

	return &api.R200{Data: &buff}, twoFactor().Render(ctx, &buff)
}

// TwoFactorSubmit validates the TOTP code and, on success, promotes the
// pending session to a full authenticated session.
func (h AuthHandler) TwoFactorSubmit(ctx context.Context, req *api.TwoFactorSubmitReq) (api.TwoFactorSubmitRes, error) {
	pendingID := h.pendingUserID(ctx)
	if pendingID == "" {
		return &api.R303{Location: api.NewOptString("/auth/login")}, nil
	}

	query := `
	SELECT u.email, r.role_id,
	       u.totp_enrolled, u.totp_secret, u.totp_pending_secret,
	       u.totp_pending_secret_expires_at
	FROM users u
	JOIN users_roles r ON u.id = r.user_id
	WHERE u.id = @id
	`

	var (
		email            string
		role             string
		totpEnrolled     bool
		totpSecret       *string
		pendingSecret    *string
		pendingExpiresAt *time.Time
	)

	err := h.db.QueryRow(ctx, query, pgx.NamedArgs{"id": pendingID}).
		Scan(&email, &role, &totpEnrolled, &totpSecret, &pendingSecret, &pendingExpiresAt)
	if err != nil {
		log.Logger.Error("database error in TwoFactorSubmit", "error", err)

		return &api.R303{Location: api.NewOptString("/auth/login")}, nil
	}

	// Determine which secret is active and handle all TOTP enrollment states.
	var activeSecret string
	switch {
	case totpEnrolled && totpSecret != nil:
		activeSecret = *totpSecret

	case !totpEnrolled && pendingSecret != nil && pendingExpiresAt != nil && time.Now().Before(*pendingExpiresAt):
		// First login within the 24-hour enrollment window.
		activeSecret = *pendingSecret

	case !totpEnrolled && pendingSecret != nil:
		// Enrollment window has expired.
		return nil, herrors.HtmlError{
			Code:       http.StatusBadRequest,
			Msg:        "Your setup link has expired. Contact your administrator.",
			ButtonLink: "/auth/login",
		}

	default:
		// Data integrity issue: enrolled but no secret, or not enrolled with no pending secret
		log.Logger.Error("inconsistent TOTP state for user", "userID", pendingID,
			"totpEnrolled", totpEnrolled, "hasSecret", totpSecret != nil, "hasPending", pendingSecret != nil)

		return nil, herrors.HtmlError{
			Code:       http.StatusInternalServerError,
			Msg:        "An unexpected error occurred. Please contact your administrator.",
			ButtonLink: "/auth/login",
		}
	}

	step, ok := signing.MatchTOTPStep(req.TotpCode, activeSecret, time.Now())
	if !ok {
		return nil, herrors.HtmlError{
			Code:       http.StatusBadRequest,
			Msg:        "Invalid code. Please try again.",
			ButtonLink: "/auth/2fa",
		}
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Logger.Error("failed to rollback transaction", "error", err)
		}
	}()

	// Replay prevention in the UPDATE itself, so two requests with the same code
	// at the same moment can't both pass.
	tag, err := tx.Exec(ctx, `
		UPDATE users SET totp_last_used_step = @step
		WHERE id = @id AND (totp_last_used_step IS NULL OR totp_last_used_step < @step)`,
		pgx.NamedArgs{"step": step, "id": pendingID},
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, herrors.HtmlError{
			Code:       http.StatusBadRequest,
			Msg:        "This code was already used. Please wait for the next code and try again.",
			ButtonLink: "/auth/2fa",
		}
	}

	// First login: promote pending secret to the permanent active secret.
	if !totpEnrolled {
		if _, err := tx.Exec(ctx, `
			UPDATE users SET
				totp_secret                  = totp_pending_secret,
				totp_enrolled                = TRUE,
				totp_pending_secret          = NULL,
				totp_pending_secret_expires_at = NULL
			WHERE id = @id`,
			pgx.NamedArgs{"id": pendingID},
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	if err := h.completeLogin(ctx, pendingID, email, role); err != nil {
		return nil, err
	}

	return &api.R303{Location: api.NewOptString("/dashboard")}, nil
}

// --- 2FA setup page (for users without a TOTP secret) ---

type twoFactorSetupData struct {
	Secret string
	Err    string
}

// TwoFactorSetupForm generates a TOTP secret (if not already in session) and
// renders it as a Base32 string for the doctor to enter into an authenticator app.
func (h AuthHandler) TwoFactorSetupForm(ctx context.Context) (api.TwoFactorSetupFormRes, error) {
	pendingID := h.pendingUserID(ctx)
	if pendingID == "" {
		return &api.R303{Location: api.NewOptString("/auth/login")}, nil
	}

	var email string
	var hasTOTPSecret bool
	if err := h.db.QueryRow(ctx,
		`SELECT email, totp_enrolled OR totp_pending_secret IS NOT NULL FROM users WHERE id = @id`,
		pgx.NamedArgs{"id": pendingID},
	).Scan(&email, &hasTOTPSecret); err != nil {
		return &api.R303{Location: api.NewOptString("/auth/login")}, nil
	}

	// Only the password is proven at this point, so someone with a stolen password
	// must not be able to replace the secret and skip the second factor.
	if hasTOTPSecret {
		return &api.R303{Location: api.NewOptString("/auth/2fa")}, nil
	}

	// Reuse the secret if we already generated one this session, so refreshing
	// the page does not change the secret the doctor is trying to enroll.
	secret := h.AuthService.SessionManager.GetString(ctx, SessionKeyPendingTOTPSecret)
	if secret == "" {
		key, err := signing.GenerateTOTPSecret("CyberMedico", email)
		if err != nil {
			return nil, err
		}

		secret = key.Secret
		h.AuthService.SessionManager.Put(ctx, SessionKeyPendingTOTPSecret, secret)
	}

	var buff bytes.Buffer

	return &api.R200{Data: &buff}, twoFactorSetup(twoFactorSetupData{Secret: secret}).Render(ctx, &buff)
}

// TwoFactorSetupSubmit verifies the first TOTP code to confirm the doctor
// successfully enrolled the secret, then completes the session.
func (h AuthHandler) TwoFactorSetupSubmit(ctx context.Context, req *api.TwoFactorSetupSubmitReq) (api.TwoFactorSetupSubmitRes, error) {
	pendingID := h.pendingUserID(ctx)
	if pendingID == "" {
		return &api.R303{Location: api.NewOptString("/auth/login")}, nil
	}

	pendingSecret := h.AuthService.SessionManager.GetString(ctx, SessionKeyPendingTOTPSecret)
	if pendingSecret == "" {
		return &api.R303{Location: api.NewOptString("/auth/2fa/setup")}, nil
	}

	step, ok := signing.MatchTOTPStep(req.TotpCode, pendingSecret, time.Now())
	if !ok {
		var buff bytes.Buffer

		return &api.R200{Data: &buff}, twoFactorSetup(twoFactorSetupData{
			Secret: pendingSecret,
			Err:    "Invalid code. Add the key to your authenticator app and try again.",
		}).Render(ctx, &buff)
	}

	// The conditions stop this from replacing an existing secret, see TwoFactorSetupForm.
	tag, err := h.db.Exec(ctx, `
		UPDATE users SET totp_secret = @secret, totp_enrolled = TRUE, totp_last_used_step = @step
		WHERE id = @id AND totp_enrolled = FALSE AND totp_pending_secret IS NULL`,
		pgx.NamedArgs{"secret": pendingSecret, "step": step, "id": pendingID},
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return &api.R303{Location: api.NewOptString("/auth/2fa")}, nil
	}

	// Fetch email and role to complete the session.
	var email, role string
	if err := h.db.QueryRow(ctx, `
		SELECT u.email, r.role_id
		FROM users u
		JOIN users_roles r ON u.id = r.user_id
		WHERE u.id = @id`, pgx.NamedArgs{"id": pendingID},
	).Scan(&email, &role); err != nil {
		return nil, err
	}

	if err := h.completeLogin(ctx, pendingID, email, role); err != nil {
		return nil, err
	}

	return &api.R303{Location: api.NewOptString("/dashboard")}, nil
}

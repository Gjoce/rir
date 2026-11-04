package html

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"time"

	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/api/util"
	api "cybermedico/cmd/cybermedico/ogen"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	SessionKeyEmail  = "email"
	SessionKeyUserID = "user_id"
	SessionKeyRoles  = "roles"
)

var errNotLoggedIn = errors.New("user not found in session")

type AuthService struct {
	SessionManager *scs.SessionManager
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

func NewAuthService(db *pgxpool.Pool) *AuthService {
	sessionManager := scs.New()
	sessionManager.Store = pgxstore.New(db)
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.Secure = true
	sessionManager.Cookie.Name = "cybermedico_session"

	return &AuthService{SessionManager: sessionManager}
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

// LoginSubmit validates the credentials and starts the session.
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
	SELECT u.id, u.password_hash, r.role_id
	FROM users u
	JOIN users_roles r ON u.id = r.user_id
	WHERE u.email = @email
	`

	var userID, passwordHash, role string
	err := h.db.QueryRow(ctx, query, pgx.NamedArgs{"email": email}).Scan(&userID, &passwordHash, &role)
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

	// A new token on login means a session token planted before login is useless.
	if err := h.AuthService.SessionManager.RenewToken(ctx); err != nil {
		return nil, err
	}
	h.AuthService.SessionManager.Put(ctx, SessionKeyUserID, userID)
	h.AuthService.SessionManager.Put(ctx, SessionKeyEmail, email)
	h.AuthService.SessionManager.Put(ctx, SessionKeyRoles, role)

	return &api.R303{Location: api.NewOptString("/dashboard")}, nil
}

func (h AuthHandler) Logout(ctx context.Context) (api.LogoutRes, error) {
	if err := h.AuthService.SessionManager.Destroy(ctx); err != nil {
		return nil, err
	}

	return &api.R303{Location: api.NewOptString("/auth/login")}, nil
}

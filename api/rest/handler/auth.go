package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	apiauth "github.com/CoscaAI/cosca/api/auth"
	"github.com/CoscaAI/cosca/api/middleware"
	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/auth"
)

const (
	maxUsernameLength = 50
	maxPasswordLength = 128
)

// isSecureRequest reports whether the request arrived over a secure (TLS)
// connection, which decides whether cookies are marked Secure.
//
// When the service runs behind a TLS-terminating reverse proxy, r.TLS is
// nil for every request — the proxy is the one holding the TLS session.
// The X-Forwarded-Proto header set by the proxy is therefore honored as
// well (consistent with the CSRF cookie logic in api/middleware/csrf.go),
// so cookies are marked Secure in production behind a TLS proxy and never
// sent over plain HTTP.
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// AuthHandler handles authentication endpoints (login, register, refresh, me).
type AuthHandler struct {
	store      *auth.UserStore
	secret     []byte
	auditStore *audit.Store

	// tokenStore tracks active refresh tokens server-side so they can be
	// revoked on logout and rotated with reuse detection on refresh.
	// It is optional (nil-safe): when nil, refresh tokens remain JWT-only
	// (no server-side revocation).
	tokenStore *auth.TokenStore

	// RegistrationEnabled controls whether POST /v1/auth/register is
	// available. It defaults to false — public self-registration is
	// DISABLED by default (fail closed) so no one can self-provision an
	// account and obtain tokens without explicit operator opt-in via
	// COSCA_ENABLE_REGISTRATION=true.
	RegistrationEnabled bool
}

// NewAuthHandler creates a new AuthHandler.
// tokenStores is optional: pass a *auth.TokenStore to enable server-side
// refresh-token revocation and rotation reuse detection.
func NewAuthHandler(store *auth.UserStore, secret []byte, auditStore *audit.Store, tokenStores ...*auth.TokenStore) *AuthHandler {
	h := &AuthHandler{store: store, secret: secret, auditStore: auditStore}
	if len(tokenStores) > 0 {
		h.tokenStore = tokenStores[0]
	}
	return h
}

// loginRequest is the expected JSON body for POST /v1/auth/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// refreshRequest is the expected JSON body for POST /v1/auth/refresh.
//
// DEPRECATED: the refresh token is ONLY accepted from the cosca_refresh_token
// httpOnly cookie (M7). The body-based flow was removed — accepting the token
// in a JSON body let an XSS that could only make fetch() requests steal and
// replay it. The struct is kept only to document the removed contract.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// setAuthCookies sets the three auth cookies (access, refresh, auth_state)
// for a freshly minted token pair. The access and refresh tokens are httpOnly
// (XSS-resistant); the auth_state cookie lets JS detect the login state
// without ever reading a token.
//
// All auth cookies use SameSite=Lax. Lax still blocks cookies on CROSS-SITE
// requests (different site), which is the primary CSRF defense — a cross-site
// form post or top-level navigation from an attacker's site does not carry
// these cookies. It DOES allow the cookies on same-site requests that differ
// only by port or subdomain, which the multi-origin dev topology requires:
// the frontend (http://localhost:7000) is a different origin from the backend
// (http://localhost:14120), and SameSite=Strict would prevent the browser
// from ever sending the cookies there, breaking login entirely. In
// production behind a reverse proxy serving both on one origin, SameSite=Strict
// could be re-enabled for an even stronger cross-site stance.
func setAuthCookies(w http.ResponseWriter, r *http.Request, pair *auth.TokenPair) {
	isSecure := isSecureRequest(r)

	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_access_token",
		Value:    pair.AccessToken,
		HttpOnly: true, // JavaScript cannot read it — XSS can't steal it.
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()), // 24h
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_refresh_token",
		Value:    pair.RefreshToken,
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/v1/auth/refresh",                  // Only sent on refresh endpoint.
		MaxAge:   int((7 * 24 * time.Hour).Seconds()), // 7d
	})

	// Non-httpOnly cookie so JS can detect auth state without reading tokens.
	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_auth_state",
		Value:    "true",
		HttpOnly: false,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()),
	})
}

// Login handles POST /v1/auth/login.
// It authenticates the user and returns a token pair.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "auth system not available")
		return
	}

	var req loginRequest
	limitBody(w, r, bodyLimitSmall)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	if len(req.Username) > maxUsernameLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("username too long: max %d characters", maxUsernameLength))
		return
	}

	if len(req.Password) > maxPasswordLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("password too long: max %d characters", maxPasswordLength))
		return
	}

	// Check for account lockout before attempting authentication.
	// This provides a clear error message and avoids unnecessary bcrypt cost.
	if existingUser, err := h.store.GetByUsername(req.Username); err == nil && existingUser != nil {
		if existingUser.LockedUntil != "" {
			lockedUntil, parseErr := time.Parse(time.RFC3339, existingUser.LockedUntil)
			if parseErr == nil && time.Now().Before(lockedUntil) {
				LogEvent(h.auditStore, r, "login", "auth", audit.DetailsJSON(map[string]string{"username": req.Username}), "denied")
				writeError(w, http.StatusTooManyRequests, "account locked due to too many failed attempts — try again later")
				return
			}
		}
	}

	user, err := h.store.Authenticate(req.Username, req.Password)
	if err != nil {
		LogEvent(h.auditStore, r, "login", "auth", audit.DetailsJSON(map[string]string{"username": req.Username}), "denied")
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	LogEvent(h.auditStore, r, "login", "auth", audit.DetailsJSON(map[string]string{"username": req.Username, "role": user.Role}), "success")

	pair, err := auth.GenerateTokenPair(*user, h.secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	// Register the refresh token server-side so it can be revoked on logout
	// and rotated with reuse detection on refresh.
	h.storeRefreshToken(user.ID, pair.RefreshToken)

	// Set httpOnly cookies for XSS-resistant token storage.
	// The browser sends these automatically on every request.
	isSecure := isSecureRequest(r)

	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_access_token",
		Value:    pair.AccessToken,
		HttpOnly: true, // JavaScript cannot read it — XSS can't steal it.
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()), // 24h
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_refresh_token",
		Value:    pair.RefreshToken,
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/v1/auth/refresh",                  // Only sent on refresh endpoint.
		MaxAge:   int((7 * 24 * time.Hour).Seconds()), // 7d
	})

	// Non-httpOnly cookie so JS can detect auth state without reading tokens.
	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_auth_state",
		Value:    "true",
		HttpOnly: false,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()),
	})

	// M5: an account flagged must_change_password (e.g. the dev-mode admin,
	// whose password was auto-generated and printed once) still receives a
	// valid token pair AND the cookies above — the user must be able to
	// call /v1/auth/change-password — but the HTTP status is 403 with a
	// machine-readable body so the frontend forces the password-change
	// flow before granting full access. The success audit event was already
	// recorded above.
	if user.MustChangePassword {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{
			"error":                "password_change_required",
			"must_change_password": true,
		})
		return
	}

	writeJSON(w, http.StatusOK, pair)
}

// Refresh handles POST /v1/auth/refresh.
// It validates the refresh token and returns a new token pair.
//
// M7: the refresh token is ONLY accepted from the cosca_refresh_token
// httpOnly cookie. Accepting it in a JSON body let an XSS that could only
// make fetch() requests steal and replay it — the body-based flow was
// removed. The cookie is scoped to Path=/v1/auth/refresh so the browser
// only sends it on this endpoint. A request without the cookie is answered
// 401 (never 400) so it is not confused with a body validation error.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "auth system not available")
		return
	}

	// The refresh token must arrive in the cosca_refresh_token cookie.
	cookie, err := r.Cookie("cosca_refresh_token")
	if err != nil || cookie.Value == "" {
		LogEvent(h.auditStore, r, "auth.refresh", "auth",
			audit.DetailsJSON(map[string]string{"error": "refresh token missing"}), "denied")
		writeError(w, http.StatusUnauthorized, "refresh token missing")
		return
	}
	refreshTokenStr := cookie.Value

	// Validate the refresh token — signature, expiration, AND type "refresh".
	// An access token is rejected here (A4): the refresh endpoint only ever
	// accepts refresh tokens.
	claims, err := auth.ValidateRefreshToken(refreshTokenStr, h.secret)
	if err != nil {
		LogEvent(h.auditStore, r, "auth.refresh", "auth",
			audit.DetailsJSON(map[string]string{"error": "invalid or expired refresh token"}), "error")
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// Look up the user.
	user, err := h.store.GetByID(claims.Sub)
	if err != nil {
		LogEvent(h.auditStore, r, "auth.refresh", "auth",
			audit.DetailsJSON(map[string]string{"error": "user not found"}), "error")
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	pair, err := auth.GenerateTokenPair(*user, h.secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	// Rotate the refresh token with server-side reuse detection (A4):
	// retire the presented token and record the new one. When the presented
	// token is no longer the active one, the store treats it as a stolen/
	// reused token, revokes ALL sessions for the user, and reports
	// ErrTokenReuse.
	if h.tokenStore != nil {
		newClaims, err := auth.ValidateRefreshToken(pair.RefreshToken, h.secret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate tokens")
			return
		}
		if err := h.tokenStore.RevokeAndRotate(claims.Sub, claims.JTI, newClaims.JTI); err != nil {
			if errors.Is(err, auth.ErrTokenReuse) {
				LogEvent(h.auditStore, r, "auth.refresh", "auth",
					audit.DetailsJSON(map[string]string{"user_id": user.ID, "error": "token reuse detected — all sessions revoked"}), "denied")
				writeError(w, http.StatusUnauthorized, "token reuse detected — all sessions revoked")
				return
			}
			LogEvent(h.auditStore, r, "auth.refresh", "auth",
				audit.DetailsJSON(map[string]string{"user_id": user.ID, "error": err.Error()}), "error")
			writeError(w, http.StatusInternalServerError, "failed to rotate refresh token")
			return
		}
		_ = h.tokenStore.StoreRefresh(claims.Sub, newClaims.JTI, time.Unix(newClaims.Exp, 0))
	}

	LogEvent(h.auditStore, r, "auth.refresh", "auth",
		audit.DetailsJSON(map[string]string{"user_id": user.ID, "username": user.Username}), "success")

	// Set httpOnly cookies (same as login — rotate tokens on refresh).
	isSecure := isSecureRequest(r)

	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_access_token",
		Value:    pair.AccessToken,
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_refresh_token",
		Value:    pair.RefreshToken,
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/v1/auth/refresh",
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_auth_state",
		Value:    "true",
		HttpOnly: false,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()),
	})

	writeJSON(w, http.StatusOK, pair)
}

// changePasswordRequest is the expected JSON body for
// POST /v1/auth/change-password.
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword handles POST /v1/auth/change-password (authenticated).
// It validates the caller's CURRENT password via store.ChangePassword and
// replaces it with the new one, clearing the must_change_password flag
// (M5). On success all server-side refresh tokens for the user are revoked
// so every other session must re-authenticate — a password change ends
// sessions created under the old credential.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "auth system not available")
		return
	}

	// The auth middleware (or a test harness) injected the caller's claims.
	claims, ok := apiauth.ClaimsFromContext(r.Context())
	if !ok || claims == nil || claims.Sub == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req changePasswordRequest
	limitBody(w, r, bodyLimitSmall)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}
	if len(req.NewPassword) < 6 {
		writeError(w, http.StatusBadRequest, "new password must be at least 6 characters")
		return
	}
	if len(req.NewPassword) > maxPasswordLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("new password too long: max %d characters", maxPasswordLength))
		return
	}
	if req.NewPassword == req.CurrentPassword {
		writeError(w, http.StatusBadRequest, "new password must differ from the current password")
		return
	}

	if err := h.store.ChangePassword(claims.Sub, req.CurrentPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, "user not found")
			return
		case errors.Is(err, auth.ErrInvalidCredentials):
			LogEvent(h.auditStore, r, "auth.change_password", "auth",
				audit.DetailsJSON(map[string]string{"user_id": claims.Sub, "error": "invalid current password"}), "denied")
			writeError(w, http.StatusUnauthorized, "current password is incorrect")
			return
		default:
			LogEvent(h.auditStore, r, "auth.change_password", "auth",
				audit.DetailsJSON(map[string]string{"user_id": claims.Sub, "error": err.Error()}), "error")
			writeError(w, http.StatusInternalServerError, "failed to change password")
			return
		}
	}

	// Revoke ALL server-side refresh tokens: every session minted before
	// the password change must re-authenticate with the new credential.
	if h.tokenStore != nil {
		if err := h.tokenStore.Revoke(claims.Sub); err != nil {
			LogEvent(h.auditStore, r, "auth.change_password", "auth",
				audit.DetailsJSON(map[string]string{"user_id": claims.Sub, "error": err.Error()}), "error")
		}
	}

	LogEvent(h.auditStore, r, "auth.change_password", "auth",
		audit.DetailsJSON(map[string]string{"user_id": claims.Sub, "username": claims.Username}), "success")

	writeJSON(w, http.StatusOK, map[string]string{"message": "password changed"})
}

// Me handles GET /v1/auth/me.
// It returns the currently authenticated user from the JWT claims stored
// in the request context by Middleware.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := apiauth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "auth system not available")
		return
	}

	user, err := h.store.GetByID(claims.Sub)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// GetByID returns a defensive copy of the stored user, so sanitizing the
	// hash here only affects this response — the store's record is never
	// mutated. The User struct also tags password_hash with omitempty as a
	// second line of defense against serializing the hash.
	user.PasswordHash = ""
	writeJSON(w, http.StatusOK, user)
}

// Logout handles POST /v1/auth/logout.
// It revokes the user's server-side refresh tokens (when a token store is
// configured) and clears all auth-related httpOnly cookies by setting
// MaxAge=-1.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Revoke ALL server-side refresh tokens for the authenticated user.
	// The user is identified from the request claims when the auth
	// middleware ran; logout is a public path so it normally is not
	// authenticated — fall back to validating the refresh/access cookies.
	if h.tokenStore != nil {
		if userID := h.userIDFromRequest(r); userID != "" {
			if err := h.tokenStore.Revoke(userID); err != nil {
				LogEvent(h.auditStore, r, "auth.logout", "auth",
					audit.DetailsJSON(map[string]string{"user_id": userID, "error": err.Error()}), "error")
			} else {
				LogEvent(h.auditStore, r, "auth.logout", "auth",
					audit.DetailsJSON(map[string]string{"user_id": userID}), "success")
			}
		}
	}

	LogEvent(h.auditStore, r, "auth.logout", "auth", "{}", "success")

	isSecure := isSecureRequest(r)

	// Clear access token cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_access_token",
		Value:    "",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	})

	// Clear refresh token cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_refresh_token",
		Value:    "",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/v1/auth/refresh",
		MaxAge:   -1,
	})

	// Clear auth state cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_auth_state",
		Value:    "",
		HttpOnly: false,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// registerRequest is the expected JSON body for POST /v1/auth/register.
type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
}

// Register handles POST /v1/auth/register.
// It creates a new user account with role "viewer".
//
// Registration is DISABLED by default (fail closed): unless the operator
// explicitly enabled it via COSCA_ENABLE_REGISTRATION=true, this endpoint
// returns 403 and never creates a user or issues tokens. The disabled
// check runs before body parsing so the endpoint is fully inert when off.
// Existing users are unaffected — Login/Refresh never consult this flag.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if !h.RegistrationEnabled {
		LogEvent(h.auditStore, r, "register", "auth", "{}", "denied")
		writeError(w, http.StatusForbidden, "registration is disabled")
		return
	}

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "auth system not available")
		return
	}

	var req registerRequest
	limitBody(w, r, bodyLimitSmall)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	if len(req.Username) > maxUsernameLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("username too long: max %d characters", maxUsernameLength))
		return
	}

	if len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	if len(req.Password) > maxPasswordLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("password too long: max %d characters", maxPasswordLength))
		return
	}

	// New registrations default to "viewer" role.
	user, err := h.store.Create(req.Username, req.Password, "viewer", req.Email)
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			writeError(w, http.StatusConflict, "username already taken")
			return
		}
		LogEvent(h.auditStore, r, "register", "auth", audit.DetailsJSON(map[string]string{"error": err.Error()}), "error")
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	LogEvent(h.auditStore, r, "register", "auth", audit.DetailsJSON(map[string]string{"username": req.Username, "user_id": user.ID}), "success")

	// Automatically log in the newly registered user.
	pair, err := auth.GenerateTokenPair(*user, h.secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	// Record the refresh token server-side (same as login) so it can be
	// revoked on logout and rotated with reuse detection on refresh.
	h.storeRefreshToken(user.ID, pair.RefreshToken)

	isSecure := isSecureRequest(r)

	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_access_token",
		Value:    pair.AccessToken,
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_refresh_token",
		Value:    pair.RefreshToken,
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/v1/auth/refresh",
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "cosca_auth_state",
		Value:    "true",
		HttpOnly: false,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()),
	})

	// Return user (without password) and token pair.
	user.PasswordHash = ""
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user":   user,
		"tokens": pair,
	})
}

// CSRF handles GET /v1/csrf-token.
// It generates a CSRF token, sets it as a cookie, and returns it in the
// response body so the frontend can use it for double-submit CSRF protection.
func (h *AuthHandler) CSRF(w http.ResponseWriter, r *http.Request) {
	token, err := middleware.GenerateCSRFToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate CSRF token")
		return
	}
	middleware.SetCSRFCookie(w, r, token)
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// storeRefreshToken records a refresh token server-side so it can be revoked
// on logout and rotated with reuse detection on refresh. The token store is
// optional (nil-safe): when nil, refresh tokens remain JWT-only.
func (h *AuthHandler) storeRefreshToken(userID, refreshToken string) {
	if h.tokenStore == nil {
		return
	}
	claims, err := auth.ValidateRefreshToken(refreshToken, h.secret)
	if err != nil {
		return
	}
	_ = h.tokenStore.StoreRefresh(userID, claims.JTI, time.Unix(claims.Exp, 0))
}

// userIDFromRequest resolves the authenticated user ID from the request:
// first from the context claims injected by the auth middleware, then by
// validating the cosca_refresh_token / cosca_access_token cookies (used by
// public paths such as logout where the middleware does not run).
func (h *AuthHandler) userIDFromRequest(r *http.Request) string {
	if claims, ok := apiauth.ClaimsFromContext(r.Context()); ok && claims != nil && claims.Sub != "" {
		return claims.Sub
	}
	for _, name := range []string{"cosca_refresh_token", "cosca_access_token"} {
		cookie, err := r.Cookie(name)
		if err != nil || cookie.Value == "" {
			continue
		}
		if claims, err := auth.ValidateRefreshToken(cookie.Value, h.secret); err == nil {
			return claims.Sub
		}
		if claims, err := auth.ValidateAccessToken(cookie.Value, h.secret); err == nil {
			return claims.Sub
		}
	}
	return ""
}

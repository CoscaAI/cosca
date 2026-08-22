package auth

import (
	"net/http"

	"github.com/CoscaAI/cosca/internal/auth"
)

// Role defines an access control role.
type Role string

// Predefined access control roles.
const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// RequireRole returns middleware that checks whether the authenticated
// user (from JWT claims stored in the request context) has the required
// role or higher.
//
// Admin users bypass all role checks.
// If no claims are present in the context, a 401 is returned.
// If the user's role is insufficient, a 403 is returned.
func RequireRole(role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(ContextKeyClaims).(*auth.Claims)
			if !ok || claims == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}

			// Admin role bypasses all checks.
			if claims.Role == string(RoleAdmin) {
				next.ServeHTTP(w, r)
				return
			}

			// Check if the user's role is sufficient.
			if !hasRole(claims.Role, string(role)) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"forbidden: insufficient permissions"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// hasRole returns true if userRole is at least as privileged as requiredRole.
// Hierarchy: admin > editor > viewer.
func hasRole(userRole, requiredRole string) bool {
	rank := map[string]int{
		"admin":  3,
		"editor": 2,
		"viewer": 1,
	}

	userRank := rank[userRole]
	reqRank := rank[requiredRole]

	return userRank >= reqRank
}

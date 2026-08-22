package handler

import (
	"net/http"

	apiauth "github.com/CoscaAI/cosca/api/auth"
)

// RequireMemoryScope protects memory read endpoints.  A regular caller must
// carry both a subject and tenant; only an explicitly authenticated admin may
// intentionally use the global scope.
func RequireMemoryScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := apiauth.ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if claims.Role != string(apiauth.RoleAdmin) && (claims.Sub == "" || claims.TenantID == "") {
			http.Error(w, "owner and tenant are required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// claimsSubject returns the authenticated user ID (claims.Sub) from the
// request context, or "" when the request is not authenticated (anonymous /
// system callers). Empty is deliberately distinct from "no owner": callers
// use it to attribute ownership (StoreExecution, memory Owner) and to scope
// reads to the owner — while empty (system) callers keep full visibility.
func claimsSubject(r *http.Request) string {
	claims, ok := apiauth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		return ""
	}
	return claims.Sub
}

func claimsTenant(r *http.Request) string {
	claims, ok := apiauth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		return ""
	}
	return claims.TenantID
}

// claimsRole returns the authenticated user's role, or "" for anonymous /
// system callers. Used to propagate authorization to gRPC delegates.
func claimsRole(r *http.Request) string {
	claims, ok := apiauth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		return ""
	}
	return claims.Role
}

func isAdmin(r *http.Request) bool {
	claims, ok := apiauth.ClaimsFromContext(r.Context())
	return ok && claims != nil && claims.Role == string(apiauth.RoleAdmin)
}

func ownsMemory(r *http.Request, owner, tenant string) bool {
	if isAdmin(r) {
		return true
	}
	claims, ok := apiauth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		return owner == "" && tenant == ""
	}
	if owner != "" && owner != claims.Sub {
		return false
	}
	return tenant == "" || tenant == claims.TenantID
}

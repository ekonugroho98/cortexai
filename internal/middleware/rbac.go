package middleware

import (
	"net/http"

	"github.com/cortexai/cortexai/internal/models"
)

// RequireRole returns a middleware that only allows requests from users whose
// role matches one of the given roles. Must be applied after Auth middleware.
func RequireRole(roles ...models.Role) func(http.Handler) http.Handler {
	allowed := make(map[models.Role]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := GetCurrentUser(r.Context())
			if !ok || !allowed[user.Role] {
				models.WriteError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

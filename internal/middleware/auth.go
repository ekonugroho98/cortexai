package middleware

import (
	"context"
	"net/http"

	"github.com/cortexai/cortexai/internal/models"
)

var publicPaths = map[string]bool{
	"/":             true,
	"/health":       true,
	"/docs":         true,
	"/redoc":        true,
	"/openapi.json": true,
}

const userContextKey contextKey = "user"

// UserLookup resolves an API key to a User.
// Implemented by service.UserStore.
type UserLookup interface {
	GetByKey(apiKey string) (*models.User, bool)
}

// GetCurrentUser retrieves the authenticated User from the request context.
// Returns (nil, false) on unauthenticated paths.
func GetCurrentUser(ctx context.Context) (*models.User, bool) {
	u, ok := ctx.Value(userContextKey).(*models.User)
	return u, ok
}

// Auth validates the API key from the request header/cookie and injects the
// resolved User into the request context. Downstream handlers can retrieve
// it with GetCurrentUser(r.Context()).
func Auth(lookup UserLookup, headerName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if publicPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get(headerName)
			if key == "" {
				if c, err := r.Cookie("api_key"); err == nil {
					key = c.Value
				}
			}

			if key == "" {
				models.WriteError(w, http.StatusUnauthorized, "API key required")
				return
			}

			user, ok := lookup.GetByKey(key)
			if !ok {
				models.WriteError(w, http.StatusForbidden, "invalid API key")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

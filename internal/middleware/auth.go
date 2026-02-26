package middleware

import (
	"net/http"

	"github.com/cortexai/cortexai/internal/models"
)

var publicPaths = map[string]bool{
	"/":              true,
	"/health":        true,
	"/docs":          true,
	"/redoc":         true,
	"/openapi.json":  true,
}

func Auth(apiKeys []string, headerName string) func(http.Handler) http.Handler {
	keySet := make(map[string]bool, len(apiKeys))
	for _, k := range apiKeys {
		if k != "" {
			keySet[k] = true
		}
	}

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
			if !keySet[key] {
				models.WriteError(w, http.StatusForbidden, "invalid API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/cortexai/cortexai/internal/models"
	"github.com/rs/zerolog/log"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().
					Interface("panic", rec).
					Str("stack", string(debug.Stack())).
					Str("path", r.URL.Path).
					Msg("panic recovered")
				models.WriteError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

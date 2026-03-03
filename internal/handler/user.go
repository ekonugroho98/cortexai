package handler

import (
	"net/http"

	"github.com/cortexai/cortexai/internal/middleware"
	"github.com/cortexai/cortexai/internal/models"
)

// UserHandler handles user profile endpoints.
type UserHandler struct{}

func NewUserHandler() *UserHandler { return &UserHandler{} }

// Me handles GET /api/v1/me.
// Returns the authenticated user's profile and derived permissions.
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetCurrentUser(r.Context())
	if !ok {
		models.WriteError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	models.WriteJSON(w, http.StatusOK, user.ToResponse())
}

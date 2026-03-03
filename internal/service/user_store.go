package service

import "github.com/cortexai/cortexai/internal/models"

// UserEntry is the raw input used to build a UserStore (mirrors config.UserConfig
// but kept separate so service does not import config).
type UserEntry struct {
	ID      string
	Name    string
	Role    string // "admin" | "analyst" | "viewer"
	APIKey  string
	SquadID string
	Persona string // references persona config key; empty = "default"
}

// SquadEntry mirrors config.SquadConfig for the same reason.
type SquadEntry struct {
	ID              string
	Name            string
	Datasets        []string
	ESIndexPatterns []string
	PGDatabases     []string
}

// UserStore maps API keys to User objects.
// It implements middleware.UserLookup.
type UserStore struct {
	byKey map[string]*models.User
}

// NewUserStore builds a store from explicit user entries, squad entries, and
// optional legacy api_keys. Legacy keys get RoleViewer and no squad.
func NewUserStore(users []UserEntry, squads []SquadEntry, legacyKeys []string) *UserStore {
	store := &UserStore{byKey: make(map[string]*models.User)}

	// Build squad lookup
	squadMap := make(map[string]*models.Squad, len(squads))
	for _, se := range squads {
		squadMap[se.ID] = &models.Squad{
			ID:              se.ID,
			Name:            se.Name,
			Datasets:        se.Datasets,
			ESIndexPatterns: se.ESIndexPatterns,
			PGDatabases:     se.PGDatabases,
		}
	}

	for _, ue := range users {
		if ue.APIKey == "" {
			continue
		}
		role := models.Role(ue.Role)
		if role == "" {
			role = models.RoleViewer
		}
		u := &models.User{
			ID:      ue.ID,
			Name:    ue.Name,
			Role:    role,
			APIKey:  ue.APIKey,
			SquadID: ue.SquadID,
			Persona: ue.Persona,
		}
		if ue.SquadID != "" {
			u.Squad = squadMap[ue.SquadID] // nil if squad_id not found — treated as no restriction
		}
		store.byKey[ue.APIKey] = u
	}

	// Legacy keys: viewer role, no squad
	for _, key := range legacyKeys {
		if key == "" {
			continue
		}
		if _, exists := store.byKey[key]; !exists {
			id := key
			if len(id) > 8 {
				id = id[:8]
			}
			store.byKey[key] = &models.User{
				ID:     id,
				Name:   "API User",
				Role:   models.RoleViewer,
				APIKey: key,
			}
		}
	}

	return store
}

// GetByKey returns the User for the given API key, or (nil, false) if not found.
func (s *UserStore) GetByKey(apiKey string) (*models.User, bool) {
	u, ok := s.byKey[apiKey]
	return u, ok
}

// AllKeys returns all registered API keys.
func (s *UserStore) AllKeys() []string {
	keys := make([]string, 0, len(s.byKey))
	for k := range s.byKey {
		keys = append(keys, k)
	}
	return keys
}

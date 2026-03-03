package models

// Role represents a user's permission level.
type Role string

const (
	RoleAdmin   Role = "admin"   // full access: query, agent, datasets, cache invalidation
	RoleAnalyst Role = "analyst" // query + agent + datasets
	RoleViewer  Role = "viewer"  // read-only: datasets and tables
)

// Squad defines the data access boundaries for a team.
// A nil Squad on a User means no restrictions (e.g. admin).
type Squad struct {
	ID              string
	Name            string
	Datasets        []string // allowed BigQuery dataset IDs
	ESIndexPatterns []string // allowed Elasticsearch index patterns
	PGDatabases     []string // allowed PostgreSQL database names
}

// AllowsDataset returns true if the given dataset ID is accessible to this squad.
// An empty Datasets list means no BQ restriction for this squad.
func (s *Squad) AllowsDataset(datasetID string) bool {
	if len(s.Datasets) == 0 {
		return true
	}
	for _, d := range s.Datasets {
		if d == datasetID {
			return true
		}
	}
	return false
}

// AllowsDatabase returns true if the given PG database name is accessible to this squad.
// An empty PGDatabases list means no PG restriction for this squad.
func (s *Squad) AllowsDatabase(dbName string) bool {
	if len(s.PGDatabases) == 0 {
		return true
	}
	for _, d := range s.PGDatabases {
		if d == dbName {
			return true
		}
	}
	return false
}

// User is the authenticated principal resolved from an API key.
type User struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Role    Role   `json:"role"`
	APIKey  string `json:"-"`                  // never serialised
	SquadID string `json:"squad_id,omitempty"`
	Squad   *Squad `json:"-"`                  // resolved at startup, not serialised
	Persona string `json:"persona,omitempty"`  // references persona config; empty = "default"
}

// UserResponse is returned by GET /api/v1/me.
type UserResponse struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Role              Role     `json:"role"`
	Permissions       []string `json:"permissions"`
	SquadID           string   `json:"squad_id,omitempty"`
	SquadName         string   `json:"squad_name,omitempty"`
	AllowedDatasets   []string `json:"allowed_datasets,omitempty"`    // visible to client for BQ
	AllowedDatabases  []string `json:"allowed_databases,omitempty"`   // visible to client for PG
	Persona           string   `json:"persona,omitempty"`             // AI behavior persona
}

// ToResponse converts a User to its JSON-safe representation.
func (u *User) ToResponse() UserResponse {
	resp := UserResponse{
		ID:          u.ID,
		Name:        u.Name,
		Role:        u.Role,
		Permissions: permissionsFor(u.Role),
		Persona:     u.Persona,
	}
	if u.Squad != nil {
		resp.SquadID          = u.Squad.ID
		resp.SquadName        = u.Squad.Name
		resp.AllowedDatasets  = u.Squad.Datasets
		resp.AllowedDatabases = u.Squad.PGDatabases
	}
	return resp
}

func permissionsFor(role Role) []string {
	switch role {
	case RoleAdmin:
		return []string{"query", "agent", "datasets", "cache:invalidate"}
	case RoleAnalyst:
		return []string{"query", "agent", "datasets"}
	case RoleViewer:
		return []string{"datasets"}
	default:
		return []string{}
	}
}

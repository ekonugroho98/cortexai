package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// PersonaConfig defines the AI behavior for a named persona.
// Provider references the LLM provider ("anthropic" or "deepseek").
// SystemPromptStyle controls the response tone: "executive", "technical", or "support".
// ExcludedTools lists tool names that will not be passed to the LLM for this persona.
// Nil or empty means all tools are available (backward compatible).
// AllowedDataSources lists the data sources this persona may query ("bigquery", "elasticsearch").
// Nil or empty means all data sources are allowed (backward compatible).
type PersonaConfig struct {
	Provider           string   `json:"provider"`                        // "anthropic" | "deepseek"
	Model              string   `json:"model"`                           // e.g. "claude-sonnet-4-6", "glm-4.5-air"
	BaseURL            string   `json:"base_url,omitempty"`              // optional: override base URL for this persona
	SystemPromptStyle  string   `json:"system_prompt_style"`             // "executive" | "technical" | "support"
	MaxTokens          int      `json:"max_tokens,omitempty"`            // 0 = use agent default (4096)
	ExcludedTools      []string `json:"excluded_tools,omitempty"`        // tool names to hide from LLM; nil = all tools
	AllowedDataSources []string `json:"allowed_data_sources,omitempty"` // allowed data sources; nil = all sources
}

// PostgresConfig defines a per-squad PostgreSQL connection.
type PostgresConfig struct {
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	User      string   `json:"user"`
	Password  string   `json:"password"`
	Databases []string `json:"databases"`
	SSLMode   string   `json:"ssl_mode"`
	MaxConns  int      `json:"max_conns,omitempty"`
}

// SquadConfig defines a team's data access boundaries.
type SquadConfig struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Datasets        []string        `json:"datasets"`          // allowed BigQuery dataset IDs
	ESIndexPatterns []string        `json:"es_index_patterns"` // allowed Elasticsearch index patterns
	Postgres        *PostgresConfig `json:"postgres,omitempty"` // per-squad PG connection
}

// UserConfig defines a named user with a role and an associated API key.
type UserConfig struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Role    string `json:"role"`              // "admin" | "analyst" | "viewer"
	APIKey  string `json:"api_key"`
	SquadID string `json:"squad_id"`          // references SquadConfig.ID; empty = no squad restriction
	Persona string `json:"persona,omitempty"` // references Personas map key; empty = "default"
}

type Config struct {
	// Server
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Environment string `json:"environment"`
	APIPrefix   string `json:"api_prefix"`
	LogLevel    string `json:"log_level"`

	// CORS
	CORSOrigins []string `json:"cors_origins"`

	// Auth
	APIKeyHeader string                   `json:"api_key_header"`
	APIKeys      []string                 `json:"api_keys"`  // legacy: keys without user profiles (assigned viewer role)
	Users        []UserConfig             `json:"users"`     // keys with explicit user profiles and roles
	Squads       []SquadConfig            `json:"squads"`    // team data boundaries
	Personas     map[string]PersonaConfig `json:"personas"`  // persona name → AI behavior config
	EnableAuth   bool                     `json:"enable_auth"`

	// Rate Limiting
	RateLimitPerMinute int `json:"rate_limit_per_minute"`

	// BigQuery
	GCPProjectID                 string `json:"gcp_project_id"`
	GoogleApplicationCredentials string `json:"google_application_credentials"`
	BigQueryLocation             string `json:"bigquery_location"`

	// Security
	EnableRowLevelSecurity  bool     `json:"enable_row_level_security"`
	MaxQueryBytesProcessed  int64    `json:"max_query_bytes_processed"`
	EnableQueryCostTracking bool     `json:"enable_query_cost_tracking"`
	EnableDataMasking       bool     `json:"enable_data_masking"`
	EnablePIIDetection      bool     `json:"enable_pii_detection"`
	SensitiveColumns        []string `json:"sensitive_columns"`
	PIIKeywords             []string `json:"pii_keywords"`
	EnableAuditLogging      bool     `json:"enable_audit_logging"`

	// Elasticsearch
	ElasticsearchEnabled    bool   `json:"elasticsearch_enabled"`
	ElasticsearchHost       string `json:"elasticsearch_host"`
	ElasticsearchPort       int    `json:"elasticsearch_port"`
	ElasticsearchScheme     string `json:"elasticsearch_scheme"`
	ElasticsearchUser       string `json:"elasticsearch_user"`
	ElasticsearchPassword   string `json:"elasticsearch_password"`
	ElasticsearchVerifyCerts bool  `json:"elasticsearch_verify_certs"`
	ElasticsearchMaxRetries int    `json:"elasticsearch_max_retries"`
	ElasticsearchTimeout    int    `json:"elasticsearch_timeout"`

	// AI / LLM
	LLMProvider         string            `json:"llm_provider"`           // "anthropic" (default) | "deepseek"
	AnthropicAPIKey     string            `json:"anthropic_api_key"`
	AnthropicBaseURL    string            `json:"anthropic_base_url"`     // override for Z.ai / custom proxy
	DeepSeekAPIKey      string            `json:"deepseek_api_key"`
	DeepSeekBaseURL     string            `json:"deepseek_base_url"`      // optional override
	AgentTimeout        int               `json:"agent_timeout"`
	SchemaCacheTTL      int               `json:"schema_cache_ttl"`       // minutes; 0 = default 5 min
	ModelList           map[string]string `json:"model_list"`             // provider -> model ID

	// PostgreSQL
	PostgresEnabled    bool    `json:"postgres_enabled"`
	MaxPGQueryCost     float64 `json:"max_pg_query_cost"` // max EXPLAIN cost units

	// Elasticsearch Index Patterns
	ESAllowedPatterns []string `json:"es_allowed_patterns"`
}

func Load() (*Config, error) {
	cfg := &Config{
		Host:                   DefaultHost,
		Port:                   DefaultPort,
		Environment:            DefaultEnvironment,
		APIPrefix:              DefaultAPIPrefix,
		LogLevel:               DefaultLogLevel,
		CORSOrigins:            DefaultCORSOrigins,
		APIKeyHeader:           "X-API-Key",
		EnableAuth:             true,
		RateLimitPerMinute:     DefaultRateLimitPerMinute,
		BigQueryLocation:       DefaultBigQueryLocation,
		MaxQueryBytesProcessed: DefaultMaxQueryBytesProcessed,
		EnableRowLevelSecurity: true,
		EnableQueryCostTracking: true,
		EnableDataMasking:      true,
		EnablePIIDetection:     true,
		SensitiveColumns:       DefaultSensitiveColumns,
		PIIKeywords:            DefaultPIIKeywords,
		EnableAuditLogging:     true,
		ElasticsearchPort:      DefaultElasticsearchPort,
		ElasticsearchScheme:    DefaultElasticsearchScheme,
		ElasticsearchVerifyCerts: true,
		ElasticsearchMaxRetries: DefaultElasticsearchMaxRetries,
		ElasticsearchTimeout:   DefaultElasticsearchTimeout,
		AgentTimeout:           DefaultAgentTimeout,
		ModelList:              make(map[string]string),
	}

	// Load from JSON config file if specified
	if path := getEnv("CORTEXAI_CONFIG", ""); path != "" {
		if err := loadJSON(path, cfg); err != nil {
			return nil, err
		}
	}

	// Environment overrides
	applyEnvOverrides(cfg)

	return cfg, nil
}

func loadJSON(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, cfg)
}

func applyEnvOverrides(cfg *Config) {
	if v := getEnv("CORTEXAI_HOST", ""); v != "" {
		cfg.Host = v
	}
	if v := getEnv("CORTEXAI_PORT", ""); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := getEnv("CORTEXAI_ENV", ""); v != "" {
		cfg.Environment = v
	}
	if v := getEnv("CORTEXAI_LOG_LEVEL", ""); v != "" {
		cfg.LogLevel = v
	}
	if v := getEnv("CORTEXAI_API_KEYS", ""); v != "" {
		cfg.APIKeys = strings.Split(v, ",")
	}
	if v := getEnv("GCP_PROJECT_ID", ""); v != "" {
		cfg.GCPProjectID = v
	}
	if v := getEnv("GOOGLE_APPLICATION_CREDENTIALS", ""); v != "" {
		cfg.GoogleApplicationCredentials = v
	}
	if v := getEnv("LLM_PROVIDER", ""); v != "" {
		cfg.LLMProvider = v
	}
	if v := getEnv("ANTHROPIC_API_KEY", ""); v != "" {
		cfg.AnthropicAPIKey = v
	}
	if v := getEnv("ANTHROPIC_BASE_URL", ""); v != "" {
		cfg.AnthropicBaseURL = v
	}
	if v := getEnv("DEEPSEEK_API_KEY", ""); v != "" {
		cfg.DeepSeekAPIKey = v
	}
	if v := getEnv("DEEPSEEK_BASE_URL", ""); v != "" {
		cfg.DeepSeekBaseURL = v
	}
	if v := getEnv("POSTGRES_ENABLED", ""); v != "" {
		cfg.PostgresEnabled = v == "true" || v == "1"
	}
	if v := getEnv("ELASTICSEARCH_ENABLED", ""); v != "" {
		cfg.ElasticsearchEnabled = v == "true" || v == "1"
	}
	if v := getEnv("ELASTICSEARCH_HOST", ""); v != "" {
		cfg.ElasticsearchHost = v
	}
	if v := getEnv("ELASTICSEARCH_PORT", ""); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.ElasticsearchPort = p
		}
	}
	if v := getEnv("ELASTICSEARCH_SCHEME", ""); v != "" {
		cfg.ElasticsearchScheme = v
	}
	if v := getEnv("ELASTICSEARCH_USER", ""); v != "" {
		cfg.ElasticsearchUser = v
	}
	if v := getEnv("ELASTICSEARCH_PASSWORD", ""); v != "" {
		cfg.ElasticsearchPassword = v
	}
	if v := getEnv("RATE_LIMIT_PER_MINUTE", ""); v != "" {
		if r, err := strconv.Atoi(v); err == nil {
			cfg.RateLimitPerMinute = r
		}
	}
	if v := getEnv("ENABLE_AUTH", ""); v != "" {
		cfg.EnableAuth = v == "true" || v == "1"
	}
	if v := getEnv("MAX_QUERY_BYTES_PROCESSED", ""); v != "" {
		if b, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxQueryBytesProcessed = b
		}
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

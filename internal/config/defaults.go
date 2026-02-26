package config

import "time"

const (
	DefaultHost        = "0.0.0.0"
	DefaultPort        = 8000
	DefaultEnvironment = "development"
	DefaultAPIPrefix   = "/api/v1"
	DefaultLogLevel    = "info"

	DefaultRateLimitPerMinute = 60

	DefaultBigQueryLocation = "US"
	DefaultQueryTimeout     = 60 * time.Second
	DefaultMaxQueryTimeout  = 300 * time.Second

	DefaultMaxQueryBytesProcessed = 10_000_000_000 // 10GB

	DefaultElasticsearchPort       = 9200
	DefaultElasticsearchScheme     = "http"
	DefaultElasticsearchMaxRetries = 3
	DefaultElasticsearchTimeout    = 30

	DefaultAgentTimeout = 300 // seconds

	DefaultMaxPromptLength = 2000

	DefaultCORSMaxAge = 300
)

var DefaultCORSOrigins = []string{
	"http://localhost:3000",
	"http://localhost:8080",
}

var DefaultSensitiveColumns = []string{
	"email", "phone", "ssn", "social_security_number",
	"credit_card", "password", "secret", "token",
	"api_key", "access_key", "private_key",
}

var DefaultPIIKeywords = []string{
	"password", "ssn", "social security", "credit card",
	"bank account", "pin", "secret", "private key",
	"access token", "api key", "personal data",
}

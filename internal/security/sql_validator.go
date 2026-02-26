package security

import (
	"regexp"
	"strings"
)

// sqlDangerousPatterns mirrors Python validators.py DANGEROUS_PATTERNS exactly
var sqlDangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i);\s*DROP\s+`),
	regexp.MustCompile(`(?i);\s*DELETE\s+`),
	regexp.MustCompile(`(?i);\s*INSERT\s+`),
	regexp.MustCompile(`(?i);\s*UPDATE\s+`),
	regexp.MustCompile(`(?i);\s*ALTER\s+`),
	regexp.MustCompile(`(?i);\s*CREATE\s+`),
	regexp.MustCompile(`(?i);\s*TRUNCATE\s+`),
	regexp.MustCompile(`(?i);\s*EXEC\s*\(?`),
	regexp.MustCompile(`(?i);\s*EXECUTE\s+`),
	regexp.MustCompile(`(?i)\bUNION\s+SELECT\b`), // UNION ALL SELECT is allowed; UNION SELECT is injection
	regexp.MustCompile(`(?i)\bINTO\s+OUTFILE\b`),
	regexp.MustCompile(`(?i)\bINTO\s+DUMPFILE\b`),
	regexp.MustCompile(`(?i)\bLOAD\s+DATA\b`),
	regexp.MustCompile(`(?i)\bLOAD_FILE\s*\(`),
	regexp.MustCompile(`(?i)\bBENCHMARK\s*\(`),
	regexp.MustCompile(`(?i)\bSLEEP\s*\(`),
	regexp.MustCompile(`(?i)\bWAITFOR\s+DELAY\b`),
	regexp.MustCompile(`'.*--`),    // comment injection after string literal
	regexp.MustCompile(`;\s*--`),   // statement terminator + comment
	regexp.MustCompile(`/\*.*?\*/`),
	regexp.MustCompile(`(?i)\bor\s+1\s*=\s*1\b`),
	regexp.MustCompile(`(?i)\band\s+1\s*=\s*1\b`),
	regexp.MustCompile(`(?i)\bor\s+'1'\s*=\s*'1'`),
	regexp.MustCompile(`(?i)\band\s+'1'\s*=\s*'1'`),
}

var allowedKeywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "JOIN": true,
	"INNER": true, "LEFT": true, "RIGHT": true, "OUTER": true,
	"FULL": true, "ON": true, "AND": true, "OR": true, "NOT": true,
	"IN": true, "EXISTS": true, "BETWEEN": true, "LIKE": true,
	"IS": true, "NULL": true, "ORDER": true, "BY": true,
	"GROUP": true, "HAVING": true, "LIMIT": true, "OFFSET": true,
	"ASC": true, "DESC": true, "DISTINCT": true, "AS": true,
	"WITH": true, "CASE": true, "WHEN": true, "THEN": true,
	"ELSE": true, "END": true, "COUNT": true, "SUM": true,
	"AVG": true, "MIN": true, "MAX": true, "ARRAY_AGG": true,
	"STRING_AGG": true,
}

// SQLValidator validates SQL queries for injection and disallowed operations
type SQLValidator struct{}

func NewSQLValidator() *SQLValidator {
	return &SQLValidator{}
}

// Validate returns an error string if SQL is invalid, or empty string if OK
func (v *SQLValidator) Validate(sql string) string {
	if strings.TrimSpace(sql) == "" {
		return "SQL cannot be empty"
	}

	trimmed := strings.TrimSpace(sql)
	upperSQL := strings.ToUpper(trimmed)

	// Must start with SELECT or WITH (CTEs)
	if !strings.HasPrefix(upperSQL, "SELECT") && !strings.HasPrefix(upperSQL, "WITH") {
		return "only SELECT queries are allowed"
	}

	// Check dangerous patterns
	for _, pattern := range sqlDangerousPatterns {
		if pattern.MatchString(sql) {
			return "SQL injection pattern detected: " + pattern.String()
		}
	}

	return ""
}

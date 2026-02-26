package security

import (
	"fmt"
	"regexp"
	"strings"
)

const MaxPromptLength = 2000

// dangerousPatterns contains 30+ patterns for prompt injection and command execution
var dangerousPatterns = []*regexp.Regexp{
	// Command execution
	regexp.MustCompile(`(?i)\brm\s+-`),
	regexp.MustCompile(`(?i)\brm\s+/`),
	regexp.MustCompile(`(?i)\bcp\s+.*\s+/etc`),
	regexp.MustCompile(`(?i)\bmv\s+.*\s+/etc`),
	regexp.MustCompile(`(?i)\bcurl\s+`),
	regexp.MustCompile(`(?i)\bwget\s+`),
	regexp.MustCompile(`(?i)\bnc\s+`),
	regexp.MustCompile(`(?i)\bbash\s+-`),
	regexp.MustCompile(`(?i)\bsh\s+-`),
	regexp.MustCompile(`(?i)\bpython\s+.*\.py`),
	regexp.MustCompile(`(?i)\bnode\s+.*\.js`),
	regexp.MustCompile(`(?i)\bgit\s+`),
	regexp.MustCompile(`(?i)\bsudo\s+`),
	regexp.MustCompile(`(?i)\bsu\s+`),

	// File operations / path traversal
	regexp.MustCompile(`\.\.\/`),
	regexp.MustCompile(`/etc/passwd`),
	regexp.MustCompile(`/etc/shadow`),
	regexp.MustCompile(`/proc/`),
	regexp.MustCompile(`/sys/`),
	regexp.MustCompile(`\.env\s`),
	regexp.MustCompile(`\.env$`),
	regexp.MustCompile(`id_rsa`),
	regexp.MustCompile(`\.ssh/`),
	regexp.MustCompile(`>\s*/`),
	regexp.MustCompile(`>>\s*/`),

	// Code execution
	regexp.MustCompile(`(?i)eval\s*\(`),
	regexp.MustCompile(`(?i)exec\s*\(`),
	regexp.MustCompile(`(?i)system\s*\(`),
	regexp.MustCompile(`(?i)__import__\s*\(`),
	regexp.MustCompile(`(?i)subprocess\s*\(`),
	regexp.MustCompile(`(?i)os\.system`),
	regexp.MustCompile(`(?i)popen`),

	// Prompt injection
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous\s+instructions`),
	regexp.MustCompile(`(?i)disregard\s+(all\s+)?previous\s+instructions`),
	regexp.MustCompile(`(?i)forget\s+(all\s+)?previous\s+instructions`),
	regexp.MustCompile(`(?i)override\s+(all\s+)?previous\s+instructions`),
	regexp.MustCompile(`(?i)new\s+context\s*:`),
	regexp.MustCompile(`(?i)change\s+context\s*:`),
	regexp.MustCompile(`(?i)instead\s+of\s+the\s+above`),
}

var suspiciousIndicators = []string{
	"create file", "eval", "exec",
	"import os", "import sys", "subprocess", "__import__",
}

var dataKeywords = []string{
	// English
	"data", "table", "query", "show", "list", "get", "find",
	"log", "error", "order", "transaction", "user", "report",
	"analytics", "metrics", "search", "count", "sum", "aggregate",
	"average", "total", "revenue", "sales", "top", "bottom",
	"compare", "trend", "chart", "how many", "how much",
	"which", "what", "when", "where", "who", "based on",
	"need", "maintenance", "status", "performance", "rating",
	// Indonesian
	"berapa", "tampilkan", "tampil", "lihat", "cari", "hitung",
	"jumlah", "total", "rata-rata", "rekap", "laporan", "tabel",
	"data", "transaksi", "pengguna", "pengemudi", "kendaraan",
	"performa", "statistik", "analisis", "ringkasan", "rangkuman",
	"tertinggi", "terendah", "terbanyak", "terbesar", "terkecil",
	"per bulan", "per hari", "per minggu", "per tahun",
	"bulan ini", "tahun ini", "minggu ini", "hari ini",
}

// PromptValidator validates prompts for injection and dangerous content
type PromptValidator struct{}

func NewPromptValidator() *PromptValidator {
	return &PromptValidator{}
}

// ValidationResult contains validation outcome
type ValidationResult struct {
	Valid   bool
	Message string
}

// Validate checks a prompt for dangerous patterns
func (v *PromptValidator) Validate(prompt string) ValidationResult {
	if len(prompt) > MaxPromptLength {
		return ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("prompt too long: %d chars (max %d)", len(prompt), MaxPromptLength),
		}
	}

	if strings.TrimSpace(prompt) == "" {
		return ValidationResult{Valid: false, Message: "prompt cannot be empty"}
	}

	// Check dangerous patterns
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(prompt) {
			return ValidationResult{
				Valid:   false,
				Message: fmt.Sprintf("dangerous pattern detected: %s", pattern.String()),
			}
		}
	}

	// Check suspicious instruction chaining
	lower := strings.ToLower(prompt)
	for _, indicator := range suspiciousIndicators {
		if strings.Contains(lower, indicator) {
			return ValidationResult{
				Valid:   false,
				Message: fmt.Sprintf("suspicious instruction indicator detected: %q", indicator),
			}
		}
	}

	// Require at least one data-related keyword
	hasDataKW := false
	for _, kw := range dataKeywords {
		if strings.Contains(lower, kw) {
			hasDataKW = true
			break
		}
	}
	if !hasDataKW {
		return ValidationResult{
			Valid:   false,
			Message: "prompt must contain data-related keywords (query, show, list, etc.)",
		}
	}

	return ValidationResult{Valid: true, Message: "ok"}
}

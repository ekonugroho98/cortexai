package security

import (
	"regexp"
	"strings"
)

// identifierPatterns mirrors Python elasticsearch_prompt_validator.py IDENTIFIER_PATTERNS exactly
var identifierPatterns = map[string][]*regexp.Regexp{
	"order_id": {
		regexp.MustCompile(`(?i)\border[-_]?id\s*[:=]\s*[\w\-]+`),
		regexp.MustCompile(`(?i)\border[-_]?id\s+[\w\-]+`),
		regexp.MustCompile(`(?i)\border\s+(?:number|#)?[:=]?\s*[\w\-]+`),
	},
	"transaction_id": {
		regexp.MustCompile(`(?i)\btransaction[-_]?id\s*[:=]\s*[\w\-]+`),
		regexp.MustCompile(`(?i)\btxn[-_]?id\s*[:=]\s*[\w\-]+`),
		regexp.MustCompile(`(?i)\btransaction\s+(?:number|#)?[:=]?\s*[\w\-]+`),
	},
	"user_id": {
		regexp.MustCompile(`(?i)\buser[-_]?id\s*[:=]\s*[\w\-]+`),
		regexp.MustCompile(`(?i)\buid\s*[:=]\s*[\w\-]+`),
		regexp.MustCompile(`(?i)\bcustomer[-_]?id\s*[:=]\s*[\w\-]+`),
	},
	"booking_id": {
		regexp.MustCompile(`(?i)\bbooking[-_]?id\s*[:=]\s*[\w\-]+`),
		regexp.MustCompile(`(?i)\breservation[-_]?id\s*[:=]\s*[\w\-]+`),
	},
	"invoice_id": {
		regexp.MustCompile(`(?i)\binvoice[-_]?id\s*[:=]\s*[\w\-]+`),
		regexp.MustCompile(`(?i)\binvoice\s+(?:number|#)?[:=]?\s*[\w\-]+`),
	},
	"payment_id": {
		regexp.MustCompile(`(?i)\bpayment[-_]?id\s*[:=]\s*[\w\-]+`),
		regexp.MustCompile(`(?i)\bpayment[-_]?ref\s*[:=]\s*[\w\-]+`),
	},
	"session_id": {
		regexp.MustCompile(`(?i)\bsession[-_]?id\s*[:=]\s*[\w\-]+`),
		regexp.MustCompile(`(?i)\bsession\s+(?:id|token)?[:=]?\s*[\w\-]+`),
	},
	"request_id": {
		regexp.MustCompile(`(?i)\brequest[-_]?id\s*[:=]\s*[\w\-]+`),
		regexp.MustCompile(`(?i)\bcorrelation[-_]?id\s*[:=]\s*[\w\-]+`),
		regexp.MustCompile(`(?i)\btrace[-_]?id\s*[:=]\s*[\w\-]+`),
	},
	"email": {
		regexp.MustCompile(`(?i)\bemail\s*[:=]\s*[\w\.\-]+@[\w\.\-]+\.\w+`),
		regexp.MustCompile(`[\w\.\-]+@[\w\.\-]+\.\w+`),
	},
	"phone": {
		regexp.MustCompile(`(?i)\bphone\s*[:=]\s*[\d\-\+\(\)\s]+`),
		regexp.MustCompile(`(?i)\bmobile\s*[:=]\s*[\d\-\+\(\)\s]+`),
	},
	"ip_address": {
		regexp.MustCompile(`(?i)\bip\s*[:=]\s*\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`),
		regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`),
	},
	"time_range": {
		regexp.MustCompile(`(?i)\blast\s+\d+\s+(?:seconds?|minutes?|hours?|days?)`),
		regexp.MustCompile(`(?i)\blast\s+(?:seconds?|minutes?|hours?|days?)`),
		regexp.MustCompile(`(?i)\bfrom\s+\w+`),
		regexp.MustCompile(`(?i)\bsince\s+\w+`),
		regexp.MustCompile(`(?i)\bbetween\s+.+?\s+and\s+`),
		regexp.MustCompile(`(?i)\bpast\s+\d+\s+(?:seconds?|minutes?|hours?|days?)`),
		regexp.MustCompile(`(?i)\bpast\s+(?:seconds?|minutes?|hours?|days?)`),
		regexp.MustCompile(`(?i)\btoday\b`),
		regexp.MustCompile(`(?i)\byesterday\b`),
		regexp.MustCompile(`(?i)\bnow\s*-\s*\d+[hm]\b`),
		regexp.MustCompile(`(?i)\bgte?\s*[:=]\s*[\"']?now`),
	},
	"service_name": {
		regexp.MustCompile(`(?i)\bservice\s*[:=]\s*\w+`),
		regexp.MustCompile(`(?i)\bapp\s*[:=]\s*\w+`),
		regexp.MustCompile(`(?i)\bapplication\s*[:=]\s*\w+`),
		regexp.MustCompile(`(?i)\bmicroservice\s*[:=]\s*\w+`),
	},
	"error_code": {
		regexp.MustCompile(`(?i)\berror[-_]?code\s*[:=]\s*[\w\-]+`),
		regexp.MustCompile(`(?i)\bstatus[-_]?code\s*[:=]\s*\d{3}`),
		regexp.MustCompile(`(?i)\bhttp\s+\d{3}`),
		regexp.MustCompile(`(?i)\berr\s*[:=]\s*[\w\-]+`),
	},
	"url_path": {
		regexp.MustCompile(`/[a-zA-Z0-9_/\-]+`),
		regexp.MustCompile(`(?i)\bpath\s*[:=]\s*/[a-zA-Z0-9_/\-]+`),
		regexp.MustCompile(`(?i)\bendpoint\s*[:=]\s*/[a-zA-Z0-9_/\-]+`),
	},
	"http_method": {
		regexp.MustCompile(`\b(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\b`),
	},
	"status": {
		regexp.MustCompile(`(?i)\bstatus\s*[:=]\s*(success|failed|error|pending|timeout)`),
		regexp.MustCompile(`(?i)\bstate\s*[:=]\s*(active|inactive|blocked)`),
	},
}

// vaguePatterns: prompts matching these are rejected
var vaguePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bfind\s+all\s+errors?\b`),
	regexp.MustCompile(`(?i)\bshow\s+me\s+all\s+errors?\b`),
	regexp.MustCompile(`(?i)\blist\s+all\s+errors?\b`),
	regexp.MustCompile(`(?i)\bget\s+all\s+(logs|errors)\b`),
	regexp.MustCompile(`(?i)\ball\s+(logs|errors?)\b`),
	regexp.MustCompile(`(?i)\bshow\s+(logs|errors?)\s*(?:for\s+all|without|for\s+\w+\s*$)`),
	regexp.MustCompile(`(?i)\bdisplay\s+all\s+`),
	regexp.MustCompile(`(?i)\bwhat\s+are\s+the\s+errors?\b`),
	regexp.MustCompile(`(?i)\bany\s+errors?\b`),
}

// ESPromptValidator validates Elasticsearch prompts require at least 1 identifier
type ESPromptValidator struct{}

func NewESPromptValidator() *ESPromptValidator {
	return &ESPromptValidator{}
}

// Validate checks if prompt contains at least one specific identifier
// Returns (valid, matched_identifier_type, error_message)
func (v *ESPromptValidator) Validate(prompt string) (bool, string, string) {
	// Check vague patterns first
	for _, vp := range vaguePatterns {
		if vp.MatchString(prompt) {
			return false, "", "prompt is too vague - please include specific identifiers (order ID, user ID, time range, etc.)"
		}
	}

	// Check for at least one identifier
	for identType, patterns := range identifierPatterns {
		for _, p := range patterns {
			if p.MatchString(prompt) {
				return true, identType, ""
			}
		}
	}

	// Build helpful error message
	examples := []string{
		"order_id: 12345",
		"user_id: abc123",
		"email: user@example.com",
		"last 1 hour",
		"status: error",
	}
	msg := "prompt must include a specific identifier. Examples: " + strings.Join(examples, ", ")
	return false, "", msg
}

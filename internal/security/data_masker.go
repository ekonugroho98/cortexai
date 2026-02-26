package security

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	emailRe      = regexp.MustCompile(`(?i)email`)
	phoneRe      = regexp.MustCompile(`(?i)phone`)
	ssnRe        = regexp.MustCompile(`(?i)ssn|social_security`)
	creditCardRe = regexp.MustCompile(`(?i)credit_card|card_number`)
	fullMaskRe   = regexp.MustCompile(`(?i)password|secret|token|api_key|access_key|private_key`)
)

// DataMasker masks sensitive column values in query results
type DataMasker struct {
	sensitiveColumns []string
}

func NewDataMasker(sensitiveColumns []string) *DataMasker {
	return &DataMasker{sensitiveColumns: sensitiveColumns}
}

// MaskRows applies masking to rows returned from BigQuery
func (m *DataMasker) MaskRows(rows []map[string]interface{}) []map[string]interface{} {
	masked := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		masked[i] = m.maskRow(row)
	}
	return masked
}

func (m *DataMasker) maskRow(row map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(row))
	for col, val := range row {
		if m.isSensitive(col) {
			result[col] = m.maskValue(col, fmt.Sprintf("%v", val))
		} else {
			result[col] = val
		}
	}
	return result
}

func (m *DataMasker) isSensitive(col string) bool {
	lower := strings.ToLower(col)
	for _, s := range m.sensitiveColumns {
		if strings.Contains(lower, strings.ToLower(s)) {
			return true
		}
	}
	// Also check built-in patterns
	return emailRe.MatchString(col) || phoneRe.MatchString(col) ||
		ssnRe.MatchString(col) || creditCardRe.MatchString(col) || fullMaskRe.MatchString(col)
}

func (m *DataMasker) maskValue(col, val string) string {
	lower := strings.ToLower(col)
	switch {
	case emailRe.MatchString(lower):
		return maskEmail(val)
	case phoneRe.MatchString(lower):
		return maskPhone(val)
	case ssnRe.MatchString(lower):
		return "***-**-****"
	case creditCardRe.MatchString(lower):
		return maskCreditCard(val)
	default:
		return "***"
	}
}

// maskEmail: "john.doe@example.com" → "jo***@***.com"
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	domain := parts[1]

	// Show first 2 chars of local
	visible := 2
	if len(local) < visible {
		visible = len(local)
	}
	maskedLocal := local[:visible] + "***"

	// Mask domain, keep extension
	domainParts := strings.Split(domain, ".")
	ext := domainParts[len(domainParts)-1]
	return fmt.Sprintf("%s@***.%s", maskedLocal, ext)
}

// maskPhone: any phone → "***-***-1234" (show last 4)
func maskPhone(phone string) string {
	// Strip non-digits
	digits := ""
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			digits += string(c)
		}
	}
	if len(digits) < 4 {
		return "***-***-****"
	}
	last4 := digits[len(digits)-4:]
	return fmt.Sprintf("***-***-%s", last4)
}

// maskCreditCard: "4111111111111111" → "****-****-****-1111"
func maskCreditCard(cc string) string {
	digits := ""
	for _, c := range cc {
		if c >= '0' && c <= '9' {
			digits += string(c)
		}
	}
	if len(digits) < 4 {
		return "****-****-****-****"
	}
	last4 := digits[len(digits)-4:]
	return fmt.Sprintf("****-****-****-%s", last4)
}

package security

import (
	"strings"
)

// PIIDetector checks prompts for sensitive PII keywords
type PIIDetector struct {
	keywords []string
}

func NewPIIDetector(keywords []string) *PIIDetector {
	lower := make([]string, len(keywords))
	for i, k := range keywords {
		lower[i] = strings.ToLower(k)
	}
	return &PIIDetector{keywords: lower}
}

// Detect returns true and the matched keyword if PII is found in text
func (d *PIIDetector) Detect(text string) (bool, string) {
	lower := strings.ToLower(text)
	for _, kw := range d.keywords {
		if strings.Contains(lower, kw) {
			return true, kw
		}
	}
	return false, ""
}

package agent

import (
	"strings"
	"testing"
)

func TestSystemPromptStyle_Executive(t *testing.T) {
	s := SystemPromptStyle("executive")
	if !strings.Contains(s, "EXECUTIVE") {
		t.Error("executive style should contain EXECUTIVE marker")
	}
	if !strings.Contains(strings.ToUpper(s), "SELECT") {
		t.Error("executive style should still contain base BQ rules")
	}
}

func TestSystemPromptStyle_Technical(t *testing.T) {
	s := SystemPromptStyle("technical")
	if !strings.Contains(s, "TECHNICAL") {
		t.Error("technical style should contain TECHNICAL marker")
	}
}

func TestSystemPromptStyle_Support(t *testing.T) {
	s := SystemPromptStyle("support")
	if !strings.Contains(s, "SUPPORT") {
		t.Error("support style should contain SUPPORT marker")
	}
}

func TestSystemPromptStyle_DefaultFallback(t *testing.T) {
	for _, style := range []string{"", "unknown", "EXECUTIVE"} {
		s := SystemPromptStyle(style)
		if s != BaseSystemPrompt {
			t.Errorf("style %q: expected BaseSystemPrompt fallback", style)
		}
	}
}

func TestSystemPromptStyle_AllStylesNonEmpty(t *testing.T) {
	for _, style := range []string{"executive", "technical", "support", "", "other"} {
		if SystemPromptStyle(style) == "" {
			t.Errorf("SystemPromptStyle(%q) returned empty string", style)
		}
	}
}

func TestESSystemPromptStyle_Executive(t *testing.T) {
	s := ESSystemPromptStyle("executive")
	if !strings.Contains(s, "EXECUTIVE") {
		t.Error("ES executive style should contain EXECUTIVE marker")
	}
	if !strings.Contains(strings.ToUpper(s), "ELASTICSEARCH") {
		t.Error("ES executive style should reference Elasticsearch")
	}
}

func TestESSystemPromptStyle_Support(t *testing.T) {
	s := ESSystemPromptStyle("support")
	if !strings.Contains(s, "SUPPORT") {
		t.Error("ES support style should contain SUPPORT marker")
	}
}

func TestESSystemPromptStyle_DefaultFallback(t *testing.T) {
	for _, style := range []string{"", "unknown", "technical"} {
		s := ESSystemPromptStyle(style)
		if s != ESSystemPrompt {
			t.Errorf("ES style %q: expected ESSystemPrompt fallback", style)
		}
	}
}

func TestESSystemPromptStyle_AllStylesNonEmpty(t *testing.T) {
	for _, style := range []string{"executive", "support", "", "other"} {
		if ESSystemPromptStyle(style) == "" {
			t.Errorf("ESSystemPromptStyle(%q) returned empty string", style)
		}
	}
}

func TestAllPromptsContainLanguageRule(t *testing.T) {
	prompts := map[string]string{
		"BaseSystemPrompt":        BaseSystemPrompt,
		"executiveSystemPrompt":   SystemPromptStyle("executive"),
		"technicalSystemPrompt":   SystemPromptStyle("technical"),
		"supportSystemPrompt":     SystemPromptStyle("support"),
		"PGBaseSystemPrompt":      PGBaseSystemPrompt,
		"pgExecutiveSystemPrompt": PGSystemPromptStyle("executive"),
		"pgTechnicalSystemPrompt": PGSystemPromptStyle("technical"),
		"pgSupportSystemPrompt":   PGSystemPromptStyle("support"),
		"ESSystemPrompt":          ESSystemPrompt,
		"esExecutiveSystemPrompt": ESSystemPromptStyle("executive"),
		"esSupportSystemPrompt":   ESSystemPromptStyle("support"),
	}
	const want = "respond in the same language"
	for name, prompt := range prompts {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt %q missing language rule (expected to contain %q)", name, want)
		}
	}
}

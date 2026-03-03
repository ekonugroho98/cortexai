package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/cortexai/cortexai/internal/models"
	"github.com/cortexai/cortexai/internal/security"
	"github.com/cortexai/cortexai/internal/service"
	"github.com/cortexai/cortexai/internal/tools"
)

// ESSystemPrompt is the default Elasticsearch agent system prompt.
// Exported so system_prompts.go can use it as the fallback for unknown styles.
const ESSystemPrompt = `You are CortexAI, an expert in Elasticsearch and log analysis.

Your task is to help users investigate issues and search for data in Elasticsearch.

RULES:
1. Always use list_elasticsearch_indices first to discover available indices
2. Build precise, focused queries - never search all documents without filters
3. Use the elasticsearch_search tool to execute searches
4. Interpret results and explain findings clearly
5. Always respond in the same language as the user's prompt. If the user writes in Indonesian, respond in Indonesian. If in English, respond in English.
6. Focus on the specific identifier/time range provided by the user
7. Maximum 100 results per search

Always think step by step:
1. List available indices
2. Build appropriate query for the user's question
3. Execute the search
4. Analyze and explain the results`

// ElasticsearchHandler orchestrates the NL→ES query pipeline
type ElasticsearchHandler struct {
	agent       LLMRunner
	es          *service.ElasticsearchService
	piiDetector *security.PIIDetector
	promptVal   *security.PromptValidator
	esPromptVal *security.ESPromptValidator
	auditLogger *security.AuditLogger
}

// NewElasticsearchHandler creates a handler wired with security components
func NewElasticsearchHandler(
	agent LLMRunner,
	es *service.ElasticsearchService,
	piiDetector *security.PIIDetector,
	promptVal *security.PromptValidator,
	esPromptVal *security.ESPromptValidator,
	auditLogger *security.AuditLogger,
) *ElasticsearchHandler {
	return &ElasticsearchHandler{
		agent:       agent,
		es:          es,
		piiDetector: piiDetector,
		promptVal:   promptVal,
		esPromptVal: esPromptVal,
		auditLogger: auditLogger,
	}
}

// Handle processes an agent request for Elasticsearch.
// allowedPatterns overrides the global ES index patterns for squad isolation;
// nil means use the global patterns configured in the service.
// runner and promptStyle are resolved from the current user's persona.
func (h *ElasticsearchHandler) Handle(ctx context.Context, req *models.AgentRequest, apiKey string, allowedPatterns []string, runner LLMRunner, promptStyle string) (*models.AgentResponse, error) {
	start := time.Now()
	metadata := map[string]interface{}{
		"data_source": "elasticsearch",
		"model":       runner.Model(),
		"method":      "agent",
	}

	// 1. PII detection
	if found, kw := h.piiDetector.Detect(req.Prompt); found {
		metadata["pii_check"] = "blocked: " + kw
		return &models.AgentResponse{
			Status:        "error",
			Prompt:        req.Prompt,
			AgentMetadata: metadata,
		}, fmt.Errorf("PII detected in prompt: %s", kw)
	}
	metadata["pii_check"] = "passed"

	// 2. General prompt validation
	vr := h.promptVal.Validate(req.Prompt)
	if !vr.Valid {
		metadata["prompt_validation"] = "blocked: " + vr.Message
		return &models.AgentResponse{
			Status:        "error",
			Prompt:        req.Prompt,
			AgentMetadata: metadata,
		}, fmt.Errorf("prompt validation failed: %s", vr.Message)
	}
	metadata["prompt_validation"] = "passed"

	// 3. ES-specific identifier validation
	valid, identType, errMsg := h.esPromptVal.Validate(req.Prompt)
	if !valid {
		metadata["es_validation"] = "blocked: " + errMsg
		return &models.AgentResponse{
			Status:        "error",
			Prompt:        req.Prompt,
			AgentMetadata: metadata,
		}, fmt.Errorf("ES prompt validation failed: %s", errMsg)
	}
	metadata["es_validation"] = "passed: " + identType

	// 4. Build ES tools — use squad-scoped ES service if patterns are restricted
	esSvc := h.es
	if len(allowedPatterns) > 0 {
		esSvc = h.es.WithPatterns(allowedPatterns)
	}
	esTools := []tools.Tool{
		tools.ESListIndicesTool(esSvc),
		tools.ESSearchTool(esSvc),
	}

	// 5. Run agent loop
	agentCtx, cancel := context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
	defer cancel()

	llmStart := time.Now()
	output, toolsUsed, _, err := runner.Run(agentCtx, ESSystemPromptStyle(promptStyle), req.Prompt, esTools)
	llmMs := time.Since(llmStart).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("agent run: %w", err)
	}

	metadata["tools_used"] = toolsUsed

	execTimeMs := time.Since(start).Milliseconds()
	h.auditLogger.LogAIAgentRequest(req.Prompt, apiKey, "", true, execTimeMs)

	answerText := cleanAnswer(output)
	var answerPtr *string
	if answerText != "" {
		answerPtr = &answerText
	}

	metadata["total_time_ms"] = execTimeMs
	metadata["llm_time_ms"] = llmMs

	return &models.AgentResponse{
		Status:        "success",
		Prompt:        req.Prompt,
		AgentMetadata: metadata,
		Answer:        answerPtr,
	}, nil
}

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

const esSystemPrompt = `You are CortexAI, an expert in Elasticsearch and log analysis.

Your task is to help users investigate issues and search for data in Elasticsearch.

RULES:
1. Always use list_elasticsearch_indices first to discover available indices
2. Build precise, focused queries - never search all documents without filters
3. Use the elasticsearch_search tool to execute searches
4. Interpret results and explain findings clearly in Indonesian or English (match user's language)
5. Focus on the specific identifier/time range provided by the user
6. Maximum 100 results per search

Always think step by step:
1. List available indices
2. Build appropriate query for the user's question
3. Execute the search
4. Analyze and explain the results`

// ElasticsearchHandler orchestrates the NL→ES query pipeline
type ElasticsearchHandler struct {
	agent       *CortexAgent
	es          *service.ElasticsearchService
	piiDetector *security.PIIDetector
	promptVal   *security.PromptValidator
	esPromptVal *security.ESPromptValidator
	auditLogger *security.AuditLogger
}

// NewElasticsearchHandler creates a handler wired with security components
func NewElasticsearchHandler(
	agent *CortexAgent,
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

// Handle processes an agent request for Elasticsearch
func (h *ElasticsearchHandler) Handle(ctx context.Context, req *models.AgentRequest, apiKey string) (*models.AgentResponse, error) {
	start := time.Now()
	metadata := map[string]interface{}{
		"data_source": "elasticsearch",
		"model":       h.agent.model,
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

	// 4. Build ES tools
	esTools := []tools.Tool{
		tools.ESListIndicesTool(h.es),
		tools.ESSearchTool(h.es),
	}

	// 5. Run agent loop
	agentCtx, cancel := context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
	defer cancel()

	output, toolsUsed, _, err := h.agent.Run(agentCtx, esSystemPrompt, req.Prompt, esTools)
	if err != nil {
		return nil, fmt.Errorf("agent run: %w", err)
	}

	metadata["tools_used"] = toolsUsed

	execTimeMs := time.Since(start).Milliseconds()
	h.auditLogger.LogAIAgentRequest(req.Prompt, apiKey, "", true, execTimeMs)

	answer := truncate(output, 500)
	reasoning := output

	return &models.AgentResponse{
		Status:        "success",
		Prompt:        req.Prompt,
		AgentMetadata: metadata,
		Reasoning:     &reasoning,
		Answer:        &answer,
	}, nil
}

# CortexAI — Architecture

## Request Flow

```
HTTP Request
    ↓
Middleware Chain
    ├─ Recovery (panic handler)
    ├─ RequestID (UUID generation)
    ├─ Logging (request/response metrics)
    ├─ SecurityHeaders (HSTS, CSP)
    ├─ CORS
    ├─ Auth (API key → User injection)
    └─ RateLimit (sliding window)
    ↓
Route Handler
    ↓
Service Layer
    ├─ BigQueryService
    ├─ ElasticsearchService
    └─ IntentRouter (BQ vs ES scoring)
    ↓
Agent Layer (AI requests only)
    ├─ Security Validators
    │  ├─ PII Detector
    │  ├─ Prompt Validator (30+ patterns)
    │  ├─ ES Prompt Validator
    │  └─ SQL Validator (24+ patterns)
    ├─ Schema Cache (5min TTL, singleflight)
    ├─ LLMRunner (CortexAgent | DeepSeekAgent)
    ├─ Tool Execution Loop (max 10 iters)
    ├─ SQL Extraction (4 strategies)
    ├─ Cost Tracker (byte limits)
    ├─ Data Masker (PII columns)
    └─ Audit Logger (SHA256 hashed)
    ↓
HTTP Response (JSON or SSE)
```

## Data Isolation Model

```
Config (cortexai.json)
  └─ Squads[]
       ├─ Squad "analytics-team"
       │    ├─ Datasets: [datalake_01, datalake_02]
       │    ├─ ESIndexPatterns: [logs-*, metrics-*]
       │    └─ Users → analyst/viewer roles
       └─ Squad "finance-team"
            ├─ Datasets: [finance_db]
            ├─ ESIndexPatterns: [transactions-*]
            └─ Users → analyst/viewer roles

Admin users (squad_id="") → bypass all restrictions
```

## Agent Loop Detail

```
buildSystemPrompt()
  ├─ Check schema cache (5min TTL)
  ├─ Cache miss → singleflight.Do(datasetID)
  │    └─ Fetch all table schemas from BigQuery
  └─ Return "You are CortexAI..." + all schemas

Agent.Run(system, user, tools)
  ├─ Iteration 1-6: normal tool calling
  │    ├─ LLM responds with TextBlock + ToolUseBlock
  │    ├─ Execute tools, append results to messages
  │    └─ Track lastExecutedSQL from execute_bigquery_sql
  ├─ Iteration 7+: inject "provide final answer"
  │    └─ Run once more WITHOUT tools
  └─ Return final text + lastExecutedSQL

extractSQL(text)
  ├─ Strategy 1: ```sql...``` code block
  ├─ Strategy 2: ```...``` generic block
  ├─ Strategy 3: after ### markdown heading
  └─ Fallback: lastExecutedSQL from tool call
```

## Key Design Decisions

1. **Schema Pre-Loading** — Inject all schemas in system prompt to skip discovery iterations
2. **Singleflight Dedup** — One BQ call per dataset even under concurrent load
3. **UNION ALL SELECT Allowed** — Legitimate BigQuery pattern, only block UNION SELECT
4. **Force Answer at Iter 7** — Guaranteed termination within 10 iterations
5. **No External SDK for DeepSeek** — Pure net/http + json, minimal dependencies
6. **Squad Isolation via Config** — No database needed, config-driven access control
7. **GLM Stop Reason Handling** — Process tool calls even when stop_reason="stop"

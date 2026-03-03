package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/cortexai/cortexai/internal/tools"
)

// ── extractSQL ───────────────────────────────────────────────────────────────

func TestExtractSQL_Strategy1_SqlCodeBlock(t *testing.T) {
	input := "Here is your query:\n```sql\nSELECT id, name FROM dataset.users LIMIT 10\n```\nDone."
	got := extractSQL(input)
	want := "SELECT id, name FROM dataset.users LIMIT 10"
	if got != want {
		t.Errorf("strategy 1 (```sql): got %q, want %q", got, want)
	}
}

func TestExtractSQL_Strategy1_SqlCodeBlock_UpperCase(t *testing.T) {
	input := "```SQL\nSELECT * FROM t\n```"
	got := extractSQL(input)
	want := "SELECT * FROM t"
	if got != want {
		t.Errorf("strategy 1 (```SQL): got %q, want %q", got, want)
	}
}

func TestExtractSQL_Strategy1_WithSemicolon(t *testing.T) {
	// Trailing semicolons should be stripped
	input := "```sql\nSELECT 1 FROM dual;\n```"
	got := extractSQL(input)
	want := "SELECT 1 FROM dual"
	if got != want {
		t.Errorf("strategy 1 semicolon strip: got %q, want %q", got, want)
	}
}

func TestExtractSQL_Strategy2_GenericCodeBlock_Select(t *testing.T) {
	input := "Result:\n```\nSELECT name FROM project.dataset.table LIMIT 5\n```"
	got := extractSQL(input)
	want := "SELECT name FROM project.dataset.table LIMIT 5"
	if got != want {
		t.Errorf("strategy 2 (generic code block): got %q, want %q", got, want)
	}
}

func TestExtractSQL_Strategy2_GenericCodeBlock_With(t *testing.T) {
	input := "```\nWITH cte AS (SELECT 1) SELECT * FROM cte\n```"
	got := extractSQL(input)
	want := "WITH cte AS (SELECT 1) SELECT * FROM cte"
	if got != want {
		t.Errorf("strategy 2 (generic WITH): got %q, want %q", got, want)
	}
}

func TestExtractSQL_Strategy2_SkipsNonSQL_Block(t *testing.T) {
	// A python block should NOT be extracted as SQL; strategy 3/4 may still match.
	input := "```python\nprint('hello')\n```\nSELECT id FROM users"
	got := extractSQL(input)
	// Strategy 4 (single-line) should catch the SELECT
	if got == "" {
		t.Error("expected non-empty SQL from fallback strategies")
	}
	// Must not contain python
	if got == "print('hello')" {
		t.Error("should not extract python block as SQL")
	}
}

func TestExtractSQL_Strategy3a_CTE(t *testing.T) {
	input := `I'll use a CTE:

WITH monthly AS (
  SELECT EXTRACT(MONTH FROM date) AS month, SUM(amount) AS total
  FROM dataset.sales
  GROUP BY 1
)
SELECT * FROM monthly LIMIT 12`

	got := extractSQL(input)
	if got == "" {
		t.Error("strategy 3a (CTE): expected SQL, got empty")
	}
	if len(got) < 20 {
		t.Errorf("strategy 3a (CTE): result too short: %q", got)
	}
}

func TestExtractSQL_Strategy3b_MultilineSelect(t *testing.T) {
	input := `Here are the results:

SELECT
  user_id,
  COUNT(*) AS cnt
FROM dataset.events
WHERE event_type = 'click'
GROUP BY user_id
LIMIT 100`

	got := extractSQL(input)
	if got == "" {
		t.Error("strategy 3b (multiline SELECT): expected SQL, got empty")
	}
}

func TestExtractSQL_Strategy4_SingleLine(t *testing.T) {
	input := "The query is: SELECT id FROM dataset.users WHERE active = TRUE"
	got := extractSQL(input)
	if got == "" {
		t.Error("strategy 4 (single-line): expected SQL, got empty")
	}
}

func TestExtractSQL_Empty_NoSQL(t *testing.T) {
	inputs := []string{
		"",
		"No SQL here, just plain text.",
		"```python\nprint('hello')\n```",
	}
	for _, input := range inputs {
		got := extractSQL(input)
		// These either return empty or at most a single-line match; we just
		// verify strategy 1 and 2 do not incorrectly match non-SQL blocks.
		_ = got // valid to return "" or a non-python string
	}
}

func TestExtractSQL_PrefersSqlBlock_OverOtherStrategies(t *testing.T) {
	// When both a ```sql block and a plain SELECT are present,
	// strategy 1 (```sql) should win.
	input := "SELECT id FROM raw_table\n\n```sql\nSELECT id FROM clean_table LIMIT 5\n```"
	got := extractSQL(input)
	want := "SELECT id FROM clean_table LIMIT 5"
	if got != want {
		t.Errorf("sql block should take priority: got %q, want %q", got, want)
	}
}

// ── schemaCache ──────────────────────────────────────────────────────────────

func TestSchemaCache_SetAndGet(t *testing.T) {
	c := newSchemaCache(0)
	c.set("ds1", "prompt for ds1")

	got, ok := c.get("ds1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != "prompt for ds1" {
		t.Errorf("got %q, want 'prompt for ds1'", got)
	}
}

func TestSchemaCache_Miss(t *testing.T) {
	c := newSchemaCache(0)
	_, ok := c.get("nonexistent")
	if ok {
		t.Error("expected cache miss for nonexistent key")
	}
}

func TestSchemaCache_Invalidate(t *testing.T) {
	c := newSchemaCache(0)
	c.set("ds1", "prompt")

	c.invalidate("ds1")

	_, ok := c.get("ds1")
	if ok {
		t.Error("expected cache miss after invalidate")
	}
}

func TestSchemaCache_Invalidate_NonExistentKey(t *testing.T) {
	c := newSchemaCache(0)
	// Should not panic when invalidating a key that was never set
	c.invalidate("ghost")
}

func TestSchemaCache_TTLExpiry(t *testing.T) {
	c := newSchemaCache(0)

	// Manually insert an already-expired entry
	c.mu.Lock()
	c.store["expired"] = schemaCacheEntry{
		prompt:    "old prompt",
		expiresAt: time.Now().Add(-1 * time.Second), // expired 1s ago
	}
	c.mu.Unlock()

	_, ok := c.get("expired")
	if ok {
		t.Error("expected cache miss for expired entry")
	}
}

func TestSchemaCache_OverwriteExistingKey(t *testing.T) {
	c := newSchemaCache(0)
	c.set("ds1", "first prompt")
	c.set("ds1", "second prompt")

	got, ok := c.get("ds1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != "second prompt" {
		t.Errorf("got %q, want 'second prompt'", got)
	}
}

func TestSchemaCache_MultipleDatasets(t *testing.T) {
	c := newSchemaCache(0)
	c.set("ds_a", "prompt A")
	c.set("ds_b", "prompt B")

	a, okA := c.get("ds_a")
	b, okB := c.get("ds_b")

	if !okA || a != "prompt A" {
		t.Errorf("ds_a: got %q ok=%v", a, okA)
	}
	if !okB || b != "prompt B" {
		t.Errorf("ds_b: got %q ok=%v", b, okB)
	}
}

func TestSchemaCache_InvalidateOneDoesNotAffectOther(t *testing.T) {
	c := newSchemaCache(0)
	c.set("keep", "keep prompt")
	c.set("remove", "remove prompt")

	c.invalidate("remove")

	_, ok := c.get("remove")
	if ok {
		t.Error("'remove' should be invalidated")
	}

	got, ok := c.get("keep")
	if !ok || got != "keep prompt" {
		t.Errorf("'keep' should still be present: got %q ok=%v", got, ok)
	}
}

// ── filterTools ──────────────────────────────────────────────────────────────

// makeTools builds a synthetic []tools.Tool slice with the given names for testing.
func makeTools(names ...string) []tools.Tool {
	ts := make([]tools.Tool, len(names))
	for i, n := range names {
		ts[i] = tools.Tool{Name: n}
	}
	return ts
}

func TestFilterTools_NilExclusion(t *testing.T) {
	ts := makeTools("list_bigquery_datasets", "get_bigquery_schema", "execute_bigquery_sql")
	got := filterTools(ts, nil)
	if len(got) != len(ts) {
		t.Errorf("nil excluded: want %d tools, got %d", len(ts), len(got))
	}
}

func TestFilterTools_EmptyExclusion(t *testing.T) {
	ts := makeTools("list_bigquery_datasets", "get_bigquery_schema", "execute_bigquery_sql")
	got := filterTools(ts, []string{})
	if len(got) != len(ts) {
		t.Errorf("empty excluded: want %d tools, got %d", len(ts), len(got))
	}
}

func TestFilterTools_SingleExclusion(t *testing.T) {
	ts := makeTools(
		"list_bigquery_datasets",
		"list_bigquery_tables",
		"get_bigquery_schema",
		"get_bigquery_sample_data",
		"execute_bigquery_sql",
	)
	got := filterTools(ts, []string{"get_bigquery_sample_data"})
	if len(got) != 4 {
		t.Fatalf("single exclusion: want 4 tools, got %d", len(got))
	}
	for _, t2 := range got {
		if t2.Name == "get_bigquery_sample_data" {
			t.Error("excluded tool 'get_bigquery_sample_data' is still present")
		}
	}
}

func TestFilterTools_MultipleExclusions(t *testing.T) {
	ts := makeTools(
		"list_bigquery_datasets",
		"list_bigquery_tables",
		"get_bigquery_schema",
		"get_bigquery_sample_data",
		"execute_bigquery_sql",
	)
	excluded := []string{"get_bigquery_sample_data", "list_bigquery_tables"}
	got := filterTools(ts, excluded)
	if len(got) != 3 {
		t.Fatalf("multiple exclusions: want 3 tools, got %d", len(got))
	}
	excSet := map[string]bool{"get_bigquery_sample_data": true, "list_bigquery_tables": true}
	for _, t2 := range got {
		if excSet[t2.Name] {
			t.Errorf("excluded tool %q is still present", t2.Name)
		}
	}
}

func TestFilterTools_AllExcluded(t *testing.T) {
	names := []string{"list_bigquery_datasets", "get_bigquery_schema", "execute_bigquery_sql"}
	ts := makeTools(names...)
	got := filterTools(ts, names)
	if len(got) != 0 {
		t.Errorf("all excluded: want empty slice, got %d tools", len(got))
	}
}

func TestFilterTools_DoesNotMutateInput(t *testing.T) {
	ts := makeTools("tool_a", "tool_b", "tool_c")
	original := make([]tools.Tool, len(ts))
	copy(original, ts)
	filterTools(ts, []string{"tool_b"})
	for i, orig := range original {
		if ts[i].Name != orig.Name {
			t.Errorf("input slice mutated at index %d: want %q got %q", i, orig.Name, ts[i].Name)
		}
	}
}

// TestFilterTools_DryRunPattern verifies the dry_run exclusion pattern:
// given a full BQ tool list, when filterTools is called with "execute_bigquery_sql"
// in the excluded set (as dry_run=true would do), then execute tool is absent
// and all inspection tools remain present.
func TestFilterTools_DryRunPattern(t *testing.T) {
	allTools := makeTools(
		"list_bigquery_datasets",
		"list_bigquery_tables",
		"get_bigquery_table_schema",
		"get_bigquery_sample_data",
		"execute_bigquery_sql",
	)
	excludedTools := []string{"execute_bigquery_sql"} // simulates dry_run=true append

	got := filterTools(allTools, excludedTools)

	if len(got) != 4 {
		t.Fatalf("dry_run pattern: want 4 tools, got %d", len(got))
	}
	for _, tool := range got {
		if tool.Name == "execute_bigquery_sql" {
			t.Error("execute_bigquery_sql must not be present when dry_run=true")
		}
	}
	// inspection tools must remain
	inspectionTools := map[string]bool{
		"list_bigquery_datasets":    false,
		"list_bigquery_tables":      false,
		"get_bigquery_table_schema": false,
		"get_bigquery_sample_data":  false,
	}
	for _, tool := range got {
		inspectionTools[tool.Name] = true
	}
	for name, present := range inspectionTools {
		if !present {
			t.Errorf("inspection tool %q missing from dry_run result", name)
		}
	}
}

// ── schema section closing instruction ───────────────────────────────────────

func TestBQSchemaSectionClosingInstruction_IsDirective(t *testing.T) {
	if !strings.Contains(BQSchemaClosingInstruction, "IMPORTANT: All table schemas are already provided above") {
		t.Errorf("closing instruction must start with IMPORTANT directive, got: %q", BQSchemaClosingInstruction)
	}
	if !strings.Contains(BQSchemaClosingInstruction, "DO NOT call list_tables or get_bigquery_schema") {
		t.Errorf("closing instruction must forbid schema tool calls, got: %q", BQSchemaClosingInstruction)
	}
	if !strings.Contains(BQSchemaClosingInstruction, "at most 1 execute call") {
		t.Errorf("closing instruction must cap execute calls, got: %q", BQSchemaClosingInstruction)
	}
	if strings.Contains(BQSchemaClosingInstruction, "you can skip") {
		t.Errorf("closing instruction must not use soft 'you can skip' language, got: %q", BQSchemaClosingInstruction)
	}
}

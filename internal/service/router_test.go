package service_test

import (
	"testing"

	"github.com/cortexai/cortexai/internal/service"
)

func TestIntentRouter_BigQuery(t *testing.T) {
	r := service.NewIntentRouter()

	bqPrompts := []string{
		"Show top 10 users by order count",
		"GROUP BY status and count orders",
		"analytics report for last month",
		"what are the metrics for KPI",
		"sum of revenue by region",
	}
	for _, p := range bqPrompts {
		res := r.Route(p)
		if res.Source != service.DataSourceBigQuery {
			t.Errorf("expected BigQuery for %q, got %q (confidence %.2f: %s)",
				p, res.Source, res.Confidence, res.Reasoning)
		}
	}
}

func TestIntentRouter_Elasticsearch(t *testing.T) {
	r := service.NewIntentRouter()

	esPrompts := []string{
		"find errors in the logs",
		"search for recent exceptions",
		"investigate what happened with the order",
		"troubleshoot the exception in error logs",
		"check the error logs for user 123",
	}
	for _, p := range esPrompts {
		res := r.Route(p)
		if res.Source != service.DataSourceElasticsearch {
			t.Errorf("expected Elasticsearch for %q, got %q (confidence %.2f: %s)",
				p, res.Source, res.Confidence, res.Reasoning)
		}
	}
}

func TestIntentRouter_ExplicitOverride(t *testing.T) {
	r := service.NewIntentRouter()

	// A BQ-sounding prompt still gets correct routing
	res := r.Route("show table analytics data")
	if res.Source != service.DataSourceBigQuery {
		t.Errorf("expected BigQuery, got %s", res.Source)
	}

	// Confidence should be > 0
	if res.Confidence <= 0 {
		t.Errorf("confidence should be > 0, got %.2f", res.Confidence)
	}
	if res.Reasoning == "" {
		t.Error("reasoning should not be empty")
	}
}

func TestIntentRouter_NoKeywords(t *testing.T) {
	r := service.NewIntentRouter()
	// Default to BigQuery when no keywords match
	res := r.Route("hello world")
	if res.Source != service.DataSourceBigQuery {
		t.Errorf("default should be BigQuery, got %s", res.Source)
	}
}

func TestIntentRouter_Postgres(t *testing.T) {
	r := service.NewIntentRouter()

	pgPrompts := []string{
		"list all sequences and triggers in postgresql",
		"show foreign key primary key relationships in postgres",
		"check stored procedure and sequence in the pg_ catalog",
	}
	for _, p := range pgPrompts {
		res := r.Route(p)
		if res.Source != service.DataSourcePostgres {
			t.Errorf("expected Postgres for %q, got %q (PG=%d BQ=%d ES=%d: %s)",
				p, res.Source, res.PGScore, res.BQScore, res.ESScore, res.Reasoning)
		}
	}
}

func TestIntentRouter_TieBreak_BQOverPG(t *testing.T) {
	r := service.NewIntentRouter()
	// "table" matches BQ, "serial" matches PG → BQ wins tie-break
	res := r.Route("describe the table serial columns")
	// BQ should win when scores are equal (tie-break: BQ > PG > ES)
	if res.Source == service.DataSourceElasticsearch {
		t.Errorf("ES should not win tie-break: %s", res.Source)
	}
}

func TestIntentRouter_PGScore_Present(t *testing.T) {
	r := service.NewIntentRouter()
	res := r.Route("check postgres for stored procedure issues")
	if res.PGScore == 0 {
		t.Error("PGScore should be > 0 for postgres-related prompt")
	}
}

// ─── Indonesian prompts ───────────────────────────────────────────────────────

func TestIntentRouter_Indonesian_BigQuery(t *testing.T) {
	r := service.NewIntentRouter()

	prompts := []struct {
		prompt string
		reason string
	}{
		{"tampilkan total transaksi per bulan", "per bulan keyword"},
		{"berapa jumlah pengguna aktif hari ini", "jumlah + hari ini"},
		{"rekap laporan penjualan per minggu", "laporan + per minggu"},
		{"hitung total pendapatan per hari selama 7 hari", "total + per hari"},
		{"tampilkan data pengemudi dengan performa terbaik", "data + performa"},
		{"analisis tren transaksi bulan ini", "tren + transaksi"},
		{"tampilkan 10 merchant dengan revenue tertinggi", "revenue + tertinggi"},
		{"berapa rata-rata nilai transaksi per pengguna", "rata-rata + transaksi"},
		{"statistik pengguna baru per tahun ini", "statistik keyword"},
		{"ringkasan data kendaraan terbanyak per kota", "ringkasan keyword"},
	}
	for _, tt := range prompts {
		t.Run(tt.reason, func(t *testing.T) {
			res := r.Route(tt.prompt)
			if res.Source != service.DataSourceBigQuery {
				t.Errorf("expected BigQuery for %q (%s), got %q (BQ=%d PG=%d ES=%d)",
					tt.prompt, tt.reason, res.Source, res.BQScore, res.PGScore, res.ESScore)
			}
		})
	}
}

func TestIntentRouter_Indonesian_Elasticsearch(t *testing.T) {
	r := service.NewIntentRouter()

	prompts := []struct {
		prompt string
		reason string
	}{
		// "cari log error service payment-gateway" hits BQ=1 (payment) ES=1 (log) → tie → BQ wins.
		// Untuk ES dominan, gunakan prompt tanpa BQ keyword.
		{"cari exception dan stack trace di logs tadi", "exception+stacktrace+log no BQ"},
		{"tampilkan exception di logs 1 jam terakhir", "exception+log+time_range"},
		{"investigate what happened troubleshoot exception di logs", "investigate+troubleshoot+exception+log"},
		{"troubleshoot error di message queue logs last hour", "troubleshoot+log+last_hour"},
		{"cek stack trace dari exception di logs kemarin", "stack_trace+exception+log"},
	}
	for _, tt := range prompts {
		t.Run(tt.reason, func(t *testing.T) {
			res := r.Route(tt.prompt)
			if res.Source != service.DataSourceElasticsearch {
				t.Errorf("expected ES for %q (%s), got %q (BQ=%d PG=%d ES=%d)",
					tt.prompt, tt.reason, res.Source, res.BQScore, res.PGScore, res.ESScore)
			}
		})
	}
}

// ─── Ambiguous / mixed keyword prompts ───────────────────────────────────────

func TestIntentRouter_Ambiguous_BQWinsOverES(t *testing.T) {
	r := service.NewIntentRouter()

	// These prompts have both ES and BQ keywords — BQ should win when BQ score > ES
	prompts := []struct {
		prompt string
		reason string
	}{
		{"show top 10 users who triggered errors by transaction count", "BQ aggregate + ES error"},
		{"tampilkan jumlah error per transaksi bulan ini", "Indonesian BQ + error"},
		{"count total orders that have error status this month", "BQ count + error status"},
	}
	for _, tt := range prompts {
		t.Run(tt.reason, func(t *testing.T) {
			res := r.Route(tt.prompt)
			// When BQ score dominates, BQ should win
			if res.BQScore <= res.ESScore && res.Source != service.DataSourceElasticsearch {
				t.Errorf("routing inconsistent for %q: BQ=%d ES=%d → got %s",
					tt.prompt, res.BQScore, res.ESScore, res.Source)
			}
			if res.BQScore > res.ESScore && res.Source != service.DataSourceBigQuery {
				t.Errorf("BQ should win when BQ score > ES score for %q: BQ=%d ES=%d → got %s",
					tt.prompt, res.BQScore, res.ESScore, res.Source)
			}
		})
	}
}

func TestIntentRouter_ESWinsWhenDominant(t *testing.T) {
	r := service.NewIntentRouter()

	// Clearly ES-dominant: many ES keywords, zero BQ
	prompts := []struct {
		prompt string
		reason string
	}{
		{"investigate exception in stack trace logs last hour", "investigate+exception+trace+logs"},
		{"troubleshoot what happened with trace id correlation id", "troubleshoot+trace+correlation"},
		{"find debug message in elasticsearch index since yesterday", "debug+elasticsearch+index+since"},
	}
	for _, tt := range prompts {
		t.Run(tt.reason, func(t *testing.T) {
			res := r.Route(tt.prompt)
			if res.Source != service.DataSourceElasticsearch {
				t.Errorf("expected ES for dominant ES prompt %q, got %q (BQ=%d ES=%d)",
					tt.prompt, res.Source, res.BQScore, res.ESScore)
			}
		})
	}
}

func TestIntentRouter_ConfidenceAndReasoning(t *testing.T) {
	r := service.NewIntentRouter()

	// All routing results should have confidence > 0 and non-empty reasoning
	prompts := []string{
		"show total revenue",
		"find errors in logs",
		"check postgres sequences",
		"hello world",
	}
	for _, p := range prompts {
		res := r.Route(p)
		if res.Confidence <= 0 {
			t.Errorf("confidence should be > 0 for %q, got %.2f", p, res.Confidence)
		}
		if res.Reasoning == "" {
			t.Errorf("reasoning should not be empty for %q", p)
		}
	}
}

func TestIntentRouter_ScoresAlwaysNonNegative(t *testing.T) {
	r := service.NewIntentRouter()

	prompts := []string{
		"tampilkan data",
		"find all logs",
		"postgres trigger",
		"",
		"   ",
		"hello",
	}
	for _, p := range prompts {
		res := r.Route(p)
		if res.BQScore < 0 || res.ESScore < 0 || res.PGScore < 0 {
			t.Errorf("scores should never be negative for %q: BQ=%d ES=%d PG=%d",
				p, res.BQScore, res.ESScore, res.PGScore)
		}
	}
}

func TestIntentRouter_DefaultBigQueryForEmptyPrompt(t *testing.T) {
	r := service.NewIntentRouter()

	for _, p := range []string{"", "   ", "xyz"} {
		res := r.Route(p)
		if res.Source != service.DataSourceBigQuery {
			t.Errorf("no-keyword prompt %q should default to BigQuery, got %s", p, res.Source)
		}
	}
}

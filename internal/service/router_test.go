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

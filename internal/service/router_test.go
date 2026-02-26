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

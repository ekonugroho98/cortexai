package security

import "testing"

func TestPGCostTracker_WithinLimit(t *testing.T) {
	ct := NewPGCostTracker(1000)
	ok, msg := ct.CheckCost(500)
	if !ok {
		t.Errorf("expected ok for cost within limit, got msg: %s", msg)
	}
}

func TestPGCostTracker_ExceedsLimit(t *testing.T) {
	ct := NewPGCostTracker(1000)
	ok, msg := ct.CheckCost(1500)
	if ok {
		t.Error("expected NOT ok for cost exceeding limit")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
}

func TestPGCostTracker_ExactLimit(t *testing.T) {
	ct := NewPGCostTracker(1000)
	ok, _ := ct.CheckCost(1000)
	if !ok {
		t.Error("expected ok for cost exactly at limit")
	}
}

func TestPGCostTracker_ZeroMaxCost_NoLimit(t *testing.T) {
	ct := NewPGCostTracker(0)
	ok, _ := ct.CheckCost(999999)
	if !ok {
		t.Error("expected ok when maxCost is 0 (no limit)")
	}
}

func TestPGCostTracker_NegativeMaxCost_NoLimit(t *testing.T) {
	ct := NewPGCostTracker(-1)
	ok, _ := ct.CheckCost(999999)
	if !ok {
		t.Error("expected ok when maxCost is negative (no limit)")
	}
}

func TestPGCostTracker_LogQueryCost_NoPanic(t *testing.T) {
	ct := NewPGCostTracker(1000)
	// Should not panic
	ct.LogQueryCost("SELECT 1", 50.0, "test-key", 100)
}

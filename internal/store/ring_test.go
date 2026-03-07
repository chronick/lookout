package store

import (
	"testing"
)

func TestRingPushAndRecent(t *testing.T) {
	r := NewRing(5)

	// Empty ring
	if got := r.Recent(10); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}

	// Push 3 spans
	for i := 0; i < 3; i++ {
		r.Push(SpanRecord{SpanID: string(rune('a' + i))})
	}
	if r.Len() != 3 {
		t.Fatalf("expected len 3, got %d", r.Len())
	}

	recent := r.Recent(2)
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent, got %d", len(recent))
	}
	// Most recent first
	if recent[0].SpanID != "c" {
		t.Errorf("expected 'c', got %q", recent[0].SpanID)
	}
	if recent[1].SpanID != "b" {
		t.Errorf("expected 'b', got %q", recent[1].SpanID)
	}
}

func TestRingOverflow(t *testing.T) {
	r := NewRing(3)

	for i := 0; i < 5; i++ {
		r.Push(SpanRecord{SpanID: string(rune('a' + i))})
	}

	if r.Len() != 3 {
		t.Fatalf("expected len 3, got %d", r.Len())
	}

	recent := r.Recent(3)
	// Should have c, d, e (last 3 pushed), most recent first
	if recent[0].SpanID != "e" {
		t.Errorf("expected 'e', got %q", recent[0].SpanID)
	}
	if recent[1].SpanID != "d" {
		t.Errorf("expected 'd', got %q", recent[1].SpanID)
	}
	if recent[2].SpanID != "c" {
		t.Errorf("expected 'c', got %q", recent[2].SpanID)
	}
}

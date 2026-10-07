package internal

// GEN-001: tests for the bounded-recursive cache size estimator. The flat
// estimator undercounted tree values by 2-3 orders of magnitude, leaving the
// memory high-watermark unable to bind; the recursive core must account
// nested content while staying bounded on deep/wide inputs.

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestEstimateSizeAccountsNestedContent(t *testing.T) {
	cm := NewCacheManager(true, 16, time.Minute)

	// Flat estimate for this shape was 48 + 10*64 = 688 bytes; the recursive
	// walk must additionally account key strings and the nested 1000-byte
	// payload strings (10 * ~1.2KB total).
	v := make(map[string]any, 10)
	for i := range 10 {
		v[fmt.Sprintf("key%d", i)] = map[string]any{"payload": strings.Repeat("x", 1000)}
	}

	got := cm.estimateSize(v)
	if got <= 48+10*64 {
		t.Errorf("estimate %d must exceed the old flat estimate 688", got)
	}
	// Lower bound: 10 entries, each >= 64+16+4 (bucket+key header+key data)
	// plus the nested map's 48 header and one entry >= 64+16+7+16+1000.
	const wantLower = 10 * (64 + 16 + 4 + 48 + (64 + 16 + 7 + 16 + 1000))
	if got < wantLower {
		t.Errorf("estimate %d below expected lower bound %d", got, wantLower)
	}
}

func TestEstimateSizeDepthBounded(t *testing.T) {
	cm := NewCacheManager(true, 16, time.Minute)

	// A chain 50 levels deep — far beyond estimateMaxDepth — must terminate
	// quickly (inner levels charged flat) and stay within the cap.
	leaf := any("x")
	for range 50 {
		leaf = []any{leaf}
	}
	if got := cm.estimateSize(leaf); got <= 0 || got > 1<<30 {
		t.Errorf("deep chain estimate out of range: %d", got)
	}
}

func TestEstimateSizeBudgetBounded(t *testing.T) {
	cm := NewCacheManager(true, 16, time.Minute)

	// 5000 entries exceeds the 4096-node budget: unvisited entries get the
	// flat per-entry charge. The result must stay positive and finite.
	big := make(map[string]any, 5000)
	for i := range 5000 {
		big[strconv.Itoa(i)] = i
	}
	got := cm.estimateSize(big)
	if got <= 0 || got > 1<<30 {
		t.Errorf("wide map estimate out of range: %d", got)
	}
	// Visited portion is fully accounted (16+8 each) plus 904 flat entries.
	const wantLower = 4096*(64+16+1) + 904*64
	if got < wantLower {
		t.Errorf("wide map estimate %d below lower bound %d", got, wantLower)
	}
}

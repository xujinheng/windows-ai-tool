package main

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestHistoryBoundedNewestFirstAndPrivate(t *testing.T) {
	var h resultHistory
	for i := 0; i < 14; i++ {
		h.remember("听写结果", fmt.Sprintf("内容%02d", i))
	}
	entries := h.snapshot()
	if len(entries) != resultHistoryLimit || entries[0].Text != "内容13" || entries[9].Text != "内容04" {
		t.Fatalf("unexpected history: %#v", entries)
	}
	first, found := h.get(entries[0].ID)
	if !found || first.Text != entries[0].Text {
		t.Fatal("cannot recover latest result by ID")
	}
	if _, found := h.get(1); found {
		t.Fatal("evicted result still accessible")
	}
	h.remember("", "   ")
	if len(h.snapshot()) != resultHistoryLimit {
		t.Fatal("blank result was remembered")
	}
	h.clear()
	if len(h.snapshot()) != 0 {
		t.Fatal("clear did not erase session history")
	}
}

func TestHistoryNeverTruncatesSensitiveOutput(t *testing.T) {
	var h resultHistory
	large := strings.Repeat("x", resultHistoryMaxBytes+1)
	h.remember("大结果", large)
	if len(h.snapshot()) != 0 {
		t.Fatal("large result must not be stored partially")
	}
	h.remember("small", "hello")
	for i := 0; i < 5; i++ {
		h.remember("large", strings.Repeat("z", 1<<20))
	}
	x := h.snapshot()
	if len(x) > 4 || x[0].Text != strings.Repeat("z", 1<<20) {
		t.Fatalf("unexpected memory cap: %d", len(x))
	}
}

func TestHistoryConcurrentUse(t *testing.T) {
	var h resultHistory
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for n := 0; n < 100; n++ {
				h.remember("test", fmt.Sprintf("%d:%d", i, n))
				_ = h.snapshot()
			}
		}(i)
	}
	wg.Wait()
	if len(h.snapshot()) != resultHistoryLimit {
		t.Fatal("concurrent writers lost history capacity")
	}
}

package main

import "testing"

func TestTaskEpochRejectsOldTask(t *testing.T) {
	var g taskEpoch
	first := g.start()
	if !g.valid(first) {
		t.Fatal("first task should be current")
	}
	next := g.start()
	if g.valid(first) || !g.valid(next) {
		t.Fatal("a new task must fence old workers")
	}
	g.shutdown()
	if g.valid(next) || g.start() != 0 {
		t.Fatal("shutdown must fence every task")
	}
}

func TestTextClipboardRestoreOwnership(t *testing.T) {
	cases := []struct {
		old       bool
		last, now uintptr
		want      bool
	}{
		{true, 11, 11, true}, {true, 11, 12, false},
		{true, 0, 0, false}, {false, 11, 11, false},
	}
	for _, tc := range cases {
		got := mayRestoreTextClipboard(tc.old, tc.last, tc.now)
		if got != tc.want {
			t.Fatalf("%+v got=%v", tc, got)
		}
	}
}

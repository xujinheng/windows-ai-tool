package main

import (
	"testing"
	"time"
)

func TestV14CompactGeometry(t *testing.T) {
	if handyStatusHeight != 50 {
		t.Fatalf("height %d", handyStatusHeight)
	}
	if handyWaveformX(8)+handyWaveBarWidth+8 >= handyClockX {
		t.Fatal("wave/clock collision")
	}
	if handyClockX+handyClockWidth+6 >= handyRecordingWidth-handyCancelGlyphInset {
		t.Fatal("clock/close collision")
	}
	if handyWorkingTextX+148 >= handyWorkingWidth-handyWorkingTextRightGap {
		t.Fatal("work status too narrow")
	}
	for _, dpi := range []float64{1, 1.25, 1.5, 1.75, 2} {
		for _, width := range []int{handyRecordingWidth, handyWorkingWidth} {
			w, h := handyPhysicalSize(width, handyStatusHeight, dpi)
			if w < width || h < handyStatusHeight {
				t.Fatalf("dpi=%f size=%d,%d", dpi, w, h)
			}
		}
	}
}
func TestV14OpacityForInterruptedSessions(t *testing.T) {
	start := time.Unix(100, 0)
	if got := handyOpacity(start, start, start, statusRecording); got != 0 {
		t.Fatalf("first frame alpha=%d", got)
	}
	if got := handyOpacity(start, start, start.Add(handyIntroDuration), statusRecording); got != 255 {
		t.Fatalf("steady alpha=%d", got)
	}
	change := start.Add(time.Second)
	if got := handyOpacity(start, change, change, statusProcessing); got != 225 {
		t.Fatalf("transition alpha=%d", got)
	}
	if got := handyOpacity(start, change, change.Add(handyPhaseEaseDuration), statusProcessing); got != 255 {
		t.Fatalf("transition end=%d", got)
	}
	end := start.Add(2 * time.Second)
	if got := handyOpacity(start, end, end.Add(handyTerminalDuration), statusSuccess); got != 0 {
		t.Fatalf("terminal alpha=%d", got)
	}
	// The next task gets a clean initial fade, independent of any prior ending.
	if got := handyOpacity(end.Add(time.Second), end.Add(time.Second), end.Add(time.Second+handyIntroDuration/2), statusRecording); got < 100 || got > 155 {
		t.Fatalf("new session alpha=%d", got)
	}
}
func TestV14MicSmoothing(t *testing.T) {
	level := 0.0
	for i := 0; i < 20; i++ {
		level = handySmoothedLevel(level, 1)
	}
	if level < 0.95 || level > 1 {
		t.Fatalf("slow ramp %f", level)
	}
	next := handySmoothedLevel(level, 0)
	if next >= level || next < 0.5 {
		t.Fatalf("unstable decay %f", next)
	}
}

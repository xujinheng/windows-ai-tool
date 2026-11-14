package main

import (
	"testing"
	"time"
)

func TestPolishLayoutWithoutTimeCrossOverlap(t *testing.T) {
	if handyWaveformX(8)+handyWaveBarWidth >= handyClockX {
		t.Fatal("waveform overlaps timer")
	}
	if handyClockX+handyClockWidth >= handyRecordingWidth-handyCancelGlyphInset-6 {
		t.Fatal("timer overlaps the close icon")
	}
	if !handyCancelHit(handyRecordingWidth-10, handyRecordingWidth) {
		t.Fatal("cancel icon cannot be clicked")
	}
	if handyCancelHit(handyWaveformX(5), handyRecordingWidth) {
		t.Fatal("waveform treated as cancel")
	}
}
func TestPolishDuration(t *testing.T) {
	started := time.Unix(0, 0)
	for _, tt := range []struct {
		elapsed time.Duration
		want    string
	}{
		{2 * time.Second, "00:02"}, {12 * time.Second, "00:12"}, {61 * time.Second, "01:01"}, {-5 * time.Second, "00:00"},
	} {
		got := handyDurationText(started, started.Add(tt.elapsed))
		if got != tt.want {
			t.Errorf("got %q want %q", got, tt.want)
		}
	}
}
func TestPolishBarsRoundedAndBounded(t *testing.T) {
	for i := 0; i < 9; i++ {
		for _, l := range []float64{-1, 0, 0.1, 0.5, 1, 3} {
			h := handyWaveHeight(i, l, 120)
			if h < 4 || h > 18 {
				t.Fatalf("bar %d height %d", i, h)
			}
		}
	}
}

func TestPhysicalSizeNeverUsedAsLogicalLayout(t *testing.T) {
	for _, scale := range []float64{1, 1.25, 1.5, 1.75, 2} {
		for _, logical := range []int{handyRecordingWidth, handyWorkingWidth} {
			w, h := handyPhysicalSize(logical, handyStatusHeight, scale)
			if w < logical || h < handyStatusHeight {
				t.Fatalf("bad scale=%v: %dx%d", scale, w, h)
			}
			if w != int(float64(logical)*scale+0.5) {
				t.Fatalf("bad width at scale %v: %d", scale, w)
			}
		}
		if handyCancelHit(handyRecordingWidth-handyCancelWidth-1, handyRecordingWidth) {
			t.Fatalf("cancel hit area crosses content at %v", scale)
		}
		if !handyCancelHit(handyRecordingWidth-handyCancelWidth, handyRecordingWidth) {
			t.Fatalf("missing hit area at %v", scale)
		}
	}
}
func TestStatusResponsiveWidths(t *testing.T) {
	// Every element is expressed in logical units; all lanes fit in the pill.
	if handyWaveformX(8)+handyWaveBarWidth+8 >= handyClockX {
		t.Fatal("waveform and clock too close")
	}
	if handyClockX+handyClockWidth+6 >= handyRecordingWidth-handyCancelGlyphInset {
		t.Fatal("clock and x too close")
	}
	if handyWorkingTextX+130 >= handyWorkingWidth-handyWorkingTextRightGap {
		t.Fatal("working label too narrow")
	}
}

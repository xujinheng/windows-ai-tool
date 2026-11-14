package main

import (
	"fmt"
	"math"
	"time"
)

// The geometry is shared by both Windows status bar flows. Keep it isolated
// so layout regressions are exercised by ordinary (including Linux) Go tests.
const (
	handyStatusHeight   = 58
	handyRecordingWidth = 250
	handyWorkingWidth   = 268
	handyCancelWidth    = 36
)

func handyCancelHit(x, width int) bool { return x >= width-handyCancelWidth && x < width }

// Keep every drawing rectangle in logical 96-DPI units. Physical pixel widths
// are calculated ONLY at the final bitmap/window boundary. v12 mixed physical
// window widths into these coordinates, causing right-end clipping at 125%+ DPI.
func handyPhysicalSize(logicalWidth, logicalHeight int, scale float64) (int, int) {
	if scale <= 0 {
		scale = 1
	}
	return int(math.Round(float64(logicalWidth) * scale)), int(math.Round(float64(logicalHeight) * scale))
}

const (
	handyWaveBarWidth        = 4
	handyClockX              = 166
	handyClockWidth          = 49
	handyCancelGlyphInset    = 22
	handyIconCenterY         = 28
	handyWorkingTextX        = 56
	handyWorkingTextRightGap = 46
)

func handyDurationText(started time.Time, now time.Time) string {
	sec := int(now.Sub(started).Seconds())
	if sec < 0 {
		sec = 0
	}
	return fmt.Sprintf("%02d:%02d", sec/60, sec%60)
}

func handyWaveformX(i int) int {
	return 57 + i*9
}

func handyWaveHeight(i int, level float64, tick uint32) int {
	if level < 0 {
		level = 0
	}
	if level > 1 {
		level = 1
	}
	envelope := 0.44 + 0.56*math.Abs(math.Sin(float64(i)*0.75+float64(tick)*0.24))
	h := int(math.Round(4 + 17*math.Pow(level, 0.7)*envelope))
	if h < 4 {
		h = 4
	}
	if h > 21 {
		h = 21
	}
	return h
}

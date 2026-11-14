package main

import (
	"fmt"
	"math"
	"time"
)

// Shared state identifiers, used by both Windows native painting and portable tests.
const (
	statusRecording  uintptr = 1
	statusProcessing uintptr = 2
	statusSuccess    uintptr = 3
	statusError      uintptr = 4
	statusRewriting  uintptr = 5
	statusApplying   uintptr = 6
	statusAnswer     uintptr = 7
	statusSelection  uintptr = 8
	statusFallback   uintptr = 9
)

// The geometry is shared by both Windows status bar flows. Keep it isolated
// so layout regressions are exercised by ordinary (including Linux) Go tests.
const (
	handyStatusHeight   = 50
	handyRecordingWidth = 232
	handyWorkingWidth   = 252
	handyCancelWidth    = 36
)

func handyCancelHit(x, width int) bool { return x >= width-handyCancelWidth && x < width }

// Keep geometry in logical 96-DPI units and scale only at the bitmap/window boundary.
func handyPhysicalSize(logicalWidth, logicalHeight int, scale float64) (int, int) {
	if scale <= 0 {
		scale = 1
	}
	return int(math.Round(float64(logicalWidth) * scale)), int(math.Round(float64(logicalHeight) * scale))
}

const (
	handyWaveBarWidth        = 3
	handyClockX              = 145
	handyClockWidth          = 50
	handyCancelGlyphInset    = 19
	handyIconCenterY         = 25
	handyWorkingTextX        = 53
	handyWorkingTextRightGap = 38
)

func handyDurationText(started time.Time, now time.Time) string {
	sec := int(now.Sub(started).Seconds())
	if sec < 0 {
		sec = 0
	}
	return fmt.Sprintf("%02d:%02d", sec/60, sec%60)
}

func handyWaveformX(i int) int {
	return 52 + i*8
}

func handyWaveHeight(i int, level float64, tick uint32) int {
	if level < 0 {
		level = 0
	}
	if level > 1 {
		level = 1
	}
	envelope := 0.44 + 0.56*math.Abs(math.Sin(float64(i)*0.75+float64(tick)*0.24))
	h := int(math.Round(4 + 14*math.Pow(level, 0.7)*envelope))
	if h < 4 {
		h = 4
	}
	if h > 18 {
		h = 18
	}
	return h
}

// The status pill should communicate state changes without flashing or
// stealing focus. Opacity is a single native layered-window alpha value,
// so the same behavior works on any Windows DPI scaling.
const (
	handyIntroDuration     = 170 * time.Millisecond
	handyPhaseEaseDuration = 160 * time.Millisecond
	handyTerminalDuration  = 1750 * time.Millisecond
	handyOutroDuration     = 240 * time.Millisecond
)

func handyOpacity(started, changed, now time.Time, phase uintptr) uint8 {
	if now.Before(started) {
		return 0
	}
	age := now.Sub(started)
	if age < handyIntroDuration {
		return uint8(math.Round(255 * float64(age) / float64(handyIntroDuration)))
	}
	if phase == statusSuccess || phase == statusError || phase == statusAnswer || phase == statusFallback {
		age = now.Sub(changed)
		if age >= handyTerminalDuration {
			return 0
		}
		if age > handyTerminalDuration-handyOutroDuration {
			left := handyTerminalDuration - age
			return uint8(math.Round(255 * float64(left) / float64(handyOutroDuration)))
		}
	}
	if elapsed := now.Sub(changed); elapsed >= 0 && elapsed < handyPhaseEaseDuration {
		// Small brightness dip when switching state; no hard visual jump.
		return uint8(225 + math.Round(30*float64(elapsed)/float64(handyPhaseEaseDuration)))
	}
	return 255
}

// EMA makes the nine bars follow a voice naturally instead of jumping frame-to-frame.
func handySmoothedLevel(previous, incoming float64) float64 {
	if incoming < 0 {
		incoming = 0
	}
	if incoming > 1 {
		incoming = 1
	}
	if previous < 0 {
		previous = 0
	}
	if previous > 1 {
		previous = 1
	}
	return previous*0.72 + incoming*0.28
}

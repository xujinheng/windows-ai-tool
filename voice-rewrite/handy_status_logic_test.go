//go:build windows

package main

import "testing"

// Both mouse flows use one overlay state machine. Only text and phases differ.
func TestUnifiedStatusPhases(t *testing.T) {
	for _, tt := range []struct {
		phase      uintptr
		mode, want string
	}{
		{statusProcessing, "听写", "正在转写…"},
		{statusProcessing, "改写", "识别指令…"},
		{statusSelection, "改写", "读取选区…"},
		{statusRewriting, "改写", "AI 处理中…"},
		{statusApplying, "改写", "正在替换…"},
		{statusSuccess, "改写", "已发送改写"},
		{statusSuccess, "听写", "已发送文字"},
		{statusAnswer, "改写", "已显示结果"},
		{statusError, "改写", "处理失败 · 查看日志"},
	} {
		if !handyPhaseAllowed(tt.phase) {
			t.Errorf("phase=%d should be allowed", tt.phase)
		}
		if got := handyStatusLabel(tt.phase, tt.mode); got != tt.want {
			t.Errorf("phase=%d mode=%s got=%q want=%q", tt.phase, tt.mode, got, tt.want)
		}
	}
	if handyPhaseAllowed(999) {
		t.Fatal("unknown phase accepted")
	}
	if !isHandyTerminalPhase(statusAnswer) || isHandyTerminalPhase(statusRewriting) {
		t.Fatal("terminal states invalid")
	}
	if handyStatusWidth(statusRecording) != handyRecordingWidth || handyStatusWidth(statusRewriting) != handyWorkingWidth {
		t.Fatal("overlay geometry unexpectedly changed")
	}
}

func TestPolishedStatusNoOverlappingLanes(t *testing.T) {
	if handyWaveformX(8)+handyWaveBarWidth+8 >= handyClockX || handyClockX+handyClockWidth+6 >= handyRecordingWidth-handyCancelGlyphInset {
		t.Fatal("recording visual lanes can overlap")
	}
	if !handyCancelHit(handyRecordingWidth-12, handyRecordingWidth) || handyCancelHit(100, handyRecordingWidth) {
		t.Fatal("cancel hitbox has wrong geometry")
	}
	if got := handyWaveHeight(4, -1, 0); got < 4 {
		t.Fatalf("height %d", got)
	}
	if got := handyWaveHeight(4, 2, 70); got > 21 {
		t.Fatalf("height %d", got)
	}
}

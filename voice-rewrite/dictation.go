package main

import "math"

// Stable mode mapping prevents clipboard-based guesses from misrouting
// VS Code's Copy Line behavior to a selection rewrite.
func sideButtonFlow(button uint16, rewriteButton, dictationButton uint16) string {
	switch button {
	case dictationButton:
		if button != 0 {
			return "dictation"
		}
	case rewriteButton:
		if button != 0 {
			return "rewrite"
		}
	}
	return "ignore"
}

// PCM16 mono level for Handy-style recording indicator; scale 0..1000.
func pcmRMSLevel(pcm []byte) uint32 {
	samples := len(pcm) / 2
	if samples == 0 {
		return 0
	}
	var energy float64
	for i := 0; i+1 < len(pcm); i += 4 { // subset for less CPU
		sample := int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8)
		v := float64(sample) / 32768
		energy += v * v
	}
	n := float64((samples + 1) / 2)
	rms := math.Sqrt(energy / n)
	level := math.Min(1.0, rms*5.0)
	return uint32(level * 1000)
}

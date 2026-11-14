package main

import (
	"encoding/binary"
	"strings"
	"unicode"
	"unicode/utf8"
)

const pcmBytesPerSecond = 16000 * 2
const asrChunkSeconds = 25

// splitPCMForASR preserves every sample. For long dictations, it places cuts
// near low-energy regions in the last four seconds of each 25-second window.
// Server-specific maximum audio durations are not assumed by the UI.
func splitPCMForASR(pcm []byte) [][]byte {
	if len(pcm) <= asrChunkSeconds*pcmBytesPerSecond || len(pcm)%2 != 0 {
		return [][]byte{pcm}
	}
	var parts [][]byte
	maxBytes := asrChunkSeconds * pcmBytesPerSecond
	for start := 0; start < len(pcm); {
		end := start + maxBytes
		if end >= len(pcm) {
			parts = append(parts, pcm[start:])
			break
		}
		// Only seek for quieter boundaries if a meaningful tail remains.
		if len(pcm)-end > 3*pcmBytesPerSecond {
			from := end - 4*pcmBytesPerSecond
			if from < start+10*pcmBytesPerSecond {
				from = start + 10*pcmBytesPerSecond
			}
			end = quietBoundary(pcm, from, end)
		}
		end &^= 1 // PCM16 sample alignment
		if end <= start {
			end = start + maxBytes
		}
		parts = append(parts, pcm[start:end])
		start = end
	}
	return parts
}

func quietBoundary(pcm []byte, from, until int) int {
	const window = pcmBytesPerSecond / 5 // 200 ms
	const step = pcmBytesPerSecond / 10  // 100 ms
	best, bestEnergy := until, uint64(^uint64(0))
	for x := from; x+window <= until; x += step {
		var energy uint64
		for i := x; i < x+window; i += 4 { // deterministic sub-sampling
			sample := int64(int16(binary.LittleEndian.Uint16(pcm[i : i+2])))
			if sample < 0 {
				sample = -sample
			}
			energy += uint64(sample)
		}
		if energy < bestEnergy {
			best, bestEnergy = x+window/2, energy
		}
	}
	return best
}

func joinASRChunks(parts []string) string {
	var out strings.Builder
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if out.Len() > 0 {
			left, _ := utf8.DecodeLastRuneInString(out.String())
			right, _ := utf8.DecodeRuneInString(part)
			if !unicode.Is(unicode.Han, left) && !unicode.Is(unicode.Han, right) &&
				!strings.ContainsRune("，。！？；：、,.!?;:)]}", right) &&
				!strings.ContainsRune("([{", left) {
				out.WriteByte(' ')
			}
		}
		out.WriteString(part)
	}
	return out.String()
}

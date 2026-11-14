package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Match OneClick's metadata delimiter, not arbitrary characters between "language"
// and <asr_text>. Multiple delimiters may occur in one transcript.
var asrMarker = regexp.MustCompile(`(?i)language[ \t]+[a-z][a-z0-9 ,_+\-/]{0,79}[ \t]*<asr_text>`)
var standaloneMarker = regexp.MustCompile(`(?i)^\s*<asr_text>\s*`)
var thinkingTag = regexp.MustCompile(`(?s)<think>.*?</think>\s*`)

func cleanASR(s string) string {
	s = asrMarker.ReplaceAllString(s, "")
	s = standaloneMarker.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "<|endoftext|>", "")
	return strings.TrimSpace(s)
}

// OneClick and some llama.cpp/faster-whisper endpoints return JSON, while
// other OpenAI-compatible servers return plain text. Never silently treat
// malformed JSON as a dictated sentence.
func parseASRResponse(data []byte, contentType string) (string, error) {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &payload); err == nil {
		return payload.Text, nil
	}
	var rawText string
	if err := json.Unmarshal(data, &rawText); err == nil {
		return rawText, nil
	}
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(strings.ToLower(contentType), "text/plain") &&
		!strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") &&
		len(data) <= (3<<20)-1 {
		return trimmed, nil
	}
	return "", errors.New("ASR 响应不是有效的 JSON {text:...} 或 text/plain")
}

func encodeWAV(pcm []byte) ([]byte, error) {
	if len(pcm) == 0 || len(pcm)%2 != 0 {
		return nil, errors.New("录音为空或 PCM 长度无效")
	}
	if len(pcm) > 16000*2*180 {
		return nil, errors.New("录音超出时长限制")
	}
	b := make([]byte, 44+len(pcm))
	copy(b[0:4], "RIFF")
	binary.LittleEndian.PutUint32(b[4:8], uint32(len(b)-8))
	copy(b[8:12], "WAVE")
	copy(b[12:16], "fmt ")
	binary.LittleEndian.PutUint32(b[16:20], 16)
	binary.LittleEndian.PutUint16(b[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(b[22:24], 1) // mono
	binary.LittleEndian.PutUint32(b[24:28], 16000)
	binary.LittleEndian.PutUint32(b[28:32], 32000)
	binary.LittleEndian.PutUint16(b[32:34], 2)
	binary.LittleEndian.PutUint16(b[34:36], 16)
	copy(b[36:40], "data")
	binary.LittleEndian.PutUint32(b[40:44], uint32(len(pcm)))
	copy(b[44:], pcm)
	return b, nil
}

// transcribe sends the exact multipart request expected by Handy-compatible ASR.
func transcribe(ctx context.Context, cfg Config, pcm []byte) (string, error) {
	chunks := splitPCMForASR(pcm)
	results := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		part, err := transcribeChunk(ctx, cfg, chunk)
		if err != nil {
			if len(chunks) > 1 {
				return "", fmt.Errorf("语音第 %d/%d 段识别失败: %w", i+1, len(chunks), err)
			}
			return "", err
		}
		results = append(results, part)
	}
	return joinASRChunks(results), nil
}

// transcribeChunk is one HTTP request; taskEpoch cancellation propagates
// through the request context to stop outstanding network I/O immediately.
func newASRRequest(ctx context.Context, cfg Config, pcm []byte) (*http.Request, error) {
	wav, err := encodeWAV(pcm)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	w := multipart.NewWriter(&out)
	f, e := w.CreateFormFile("file", "voice.wav")
	if e != nil {
		return nil, e
	}
	if _, e = f.Write(wav); e != nil {
		return nil, e
	}
	if e = w.WriteField("model", cfg.ASRModel); e != nil {
		return nil, e
	}
	if e = w.WriteField("response_format", "json"); e != nil {
		return nil, e
	}
	if e = w.Close(); e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", endpoint(cfg.ASRBaseURL, "/audio/transcriptions"), &out)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if cfg.ASRAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.ASRAPIKey)
	}
	return req, nil
}

func transcribeChunk(ctx context.Context, cfg Config, pcm []byte) (string, error) {
	req, err := newASRRequest(ctx, cfg, pcm)
	if err != nil {
		return "", err
	}
	resp, e := requestClient(90 * time.Second).Do(req)
	if e != nil {
		return "", fmt.Errorf("语音转写请求失败（请检查 ASR API 地址及网络）: %w", e)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", providerHTTPError("ASR", resp.StatusCode)
	}
	response, e := io.ReadAll(io.LimitReader(resp.Body, 3<<20))
	if e != nil {
		return "", fmt.Errorf("读取 ASR 响应失败: %w", e)
	}
	transcript, e := parseASRResponse(response, resp.Header.Get("Content-Type"))
	if e != nil {
		return "", e
	}
	result := cleanASR(transcript)
	if result == "" {
		return "", errors.New("ASR 没有识别到语音，请检查麦克风输入设备")
	}
	return result, nil
}

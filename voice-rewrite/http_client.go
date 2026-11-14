package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var providerTransport = func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 32
	t.MaxIdleConnsPerHost = 8
	t.IdleConnTimeout = 90 * time.Second
	t.ResponseHeaderTimeout = 75 * time.Second
	return t
}()

// Redirecting a speech or text request to an unconfigured host can leak
// private voice recordings, text and credentials. Treat redirects as errors.
func requestClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: providerTransport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func endpoint(base, suffix string) string { return strings.TrimRight(base, "/") + suffix }

// Only a concise, non-sensitive status goes to UI or logs. Provider error
// bodies can echo selected text, API keys or request transcripts.
func providerHTTPError(service string, status int) error {
	return fmt.Errorf("%s HTTP %d", service, status)
}

func validateServerBaseURL(base string) error {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return errors.New("服务地址必须是无用户名、查询参数或锚点的 http(s) API 基础 URL")
	}
	return nil
}

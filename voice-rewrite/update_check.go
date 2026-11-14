package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	releasesAPI  = "https://api.github.com/repos/xujinheng/windows-ai-tool/releases?per_page=15"
	releasesPage = "https://github.com/xujinheng/windows-ai-tool/releases"
)

var versionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

type appVersion struct {
	major, minor, patch int64
	pre                 string
}

func parseAppVersion(s string) (appVersion, bool) {
	m := versionPattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return appVersion{}, false
	}
	a, ea := strconv.ParseInt(m[1], 10, 64)
	b, eb := strconv.ParseInt(m[2], 10, 64)
	c, ec := strconv.ParseInt(m[3], 10, 64)
	if ea != nil || eb != nil || ec != nil {
		return appVersion{}, false
	}
	return appVersion{a, b, c, m[4]}, true
}
func versionCompare(a, b appVersion) int {
	left := []int64{a.major, a.minor, a.patch}
	right := []int64{b.major, b.minor, b.patch}
	for i := range left {
		if left[i] < right[i] {
			return -1
		}
		if left[i] > right[i] {
			return 1
		}
	}
	if a.pre == b.pre {
		return 0
	}
	if a.pre == "" {
		return 1
	}
	if b.pre == "" {
		return -1
	}
	la, lb := strings.Split(a.pre, "."), strings.Split(b.pre, ".")
	for i := 0; i < len(la) && i < len(lb); i++ {
		if la[i] == lb[i] {
			continue
		}
		an, ae := strconv.ParseUint(la[i], 10, 64)
		bn, be := strconv.ParseUint(lb[i], 10, 64)
		if ae == nil && be == nil {
			if an < bn {
				return -1
			}
			return 1
		}
		if ae == nil {
			return -1
		}
		if be == nil {
			return 1
		}
		if la[i] < lb[i] {
			return -1
		}
		return 1
	}
	if len(la) < len(lb) {
		return -1
	}
	return 1
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}
type updateNotice struct {
	latest string
	newer  bool
	found  bool
	err    error
}

func selectUpdate(current string, releases []githubRelease) updateNotice {
	installed, ok := parseAppVersion(current)
	if !ok {
		return updateNotice{err: fmt.Errorf("无法解析当前版本 %q", current)}
	}
	var latest appVersion
	notice := updateNotice{}
	for _, release := range releases {
		if release.Draft {
			continue
		}
		v, valid := parseAppVersion(release.TagName)
		if !valid {
			continue
		}
		if !notice.found || versionCompare(v, latest) > 0 {
			latest = v
			notice.found = true
			notice.latest = release.TagName
		}
	}
	notice.newer = notice.found && versionCompare(latest, installed) > 0
	return notice
}
func fetchUpdate(ctx context.Context, installed string, client *http.Client) updateNotice {
	if _, ok := parseAppVersion(installed); !ok {
		return updateNotice{err: fmt.Errorf("未识别的版本 %q", installed)}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPI, nil)
	if err != nil {
		return updateNotice{err: err}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "VoiceRewrite-update-check/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return updateNotice{err: errors.New("无法连接 GitHub，请检查网络或稍后再试")}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return updateNotice{err: fmt.Errorf("GitHub 更新服务返回 HTTP %d", resp.StatusCode)}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024+1))
	if err != nil || len(data) > 512*1024 {
		return updateNotice{err: errors.New("更新信息超出安全大小限制")}
	}
	var releases []githubRelease
	if err := json.Unmarshal(data, &releases); err != nil {
		return updateNotice{err: errors.New("GitHub 发布信息格式不正确")}
	}
	return selectUpdate(installed, releases)
}
func checkForUpdate(current string) updateNotice {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client := &http.Client{
		Timeout:       8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return fetchUpdate(ctx, current, client)
}

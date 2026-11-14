package main

import "testing"

func TestVersionOrdering(t *testing.T) {
	for _, item := range []struct {
		a, b string
		want int
	}{
		{"0.25.0", "0.24.9", 1},
		{"v0.25.0", "0.25.0", 0},
		{"0.25.0-dev", "0.25.0", -1},
		{"0.25.0-rc.10", "0.25.0-rc.9", 1},
		{"1.0.0-alpha.1", "1.0.0-alpha", 1},
		{"1.0.0+build.1", "v1.0.0", 0},
	} {
		a, ok := parseAppVersion(item.a)
		if !ok {
			t.Fatalf("parse %s", item.a)
		}
		b, ok := parseAppVersion(item.b)
		if !ok {
			t.Fatalf("parse %s", item.b)
		}
		if got := versionCompare(a, b); got != item.want {
			t.Errorf("%s vs %s = %d, want %d", item.a, item.b, got, item.want)
		}
	}
}
func TestUpdateSelection(t *testing.T) {
	releases := []githubRelease{
		{TagName: "v0.24.0"}, {TagName: "v0.30.0", Draft: true},
		{TagName: "v0.26.0-beta.2", Prerelease: true}, {TagName: "v0.25.0"},
	}
	r := selectUpdate("0.25.0-dev", releases)
	if r.err != nil || !r.found || !r.newer || r.latest != "v0.26.0-beta.2" {
		t.Fatalf("result: %#v", r)
	}
}

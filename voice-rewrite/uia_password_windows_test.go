//go:build windows

package main

import "testing"

func TestBrowserLikeWindow(t *testing.T) {
	for _, test := range []struct {
		class string
		want  bool
	}{
		{"Chrome_WidgetWin_1", true},
		{"MozillaWindowClass", true},
		{"CEFCLIENTWINDOW", true},
		{"Electron_Window", true},
		{"Notepad", false},
		{"Edit", false},
	} {
		if got := browserLikeWindow(test.class); got != test.want {
			t.Errorf("browserLikeWindow(%q) = %v, want %v", test.class, got, test.want)
		}
	}
}

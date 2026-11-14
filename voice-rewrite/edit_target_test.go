package main

import "testing"

func TestNativePasswordControl(t *testing.T) {
	for _, tc := range []struct {
		class string
		style uintptr
		want  bool
	}{
		{"Edit", 0x0020, true},
		{"RichEdit50W", 0x0020, true},
		{"WindowsForms10.EDIT.app.0", 0x0020, true},
		{"Edit", 0, false},
		{"Chrome_RenderWidgetHostHWND", 0x0020, false},
		{"Static", 0x0020, false},
	} {
		if got := nativePasswordControl(tc.class, tc.style); got != tc.want {
			t.Errorf("nativePasswordControl(%q,%#x)=%t want %t", tc.class, tc.style, got, tc.want)
		}
	}
}

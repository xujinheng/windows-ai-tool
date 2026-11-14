package main

import "testing"

func TestDictationShortcutValidation(t *testing.T) {
	for _, tc := range []struct {
		candidate, want string
		valid           bool
	}{
		{"Mouse4", "Mouse4", true}, {"xbutton1", "Mouse4", true},
		{"f8", "F8", true}, {"F24", "F24", true},
		{"f0", "", false}, {"F25", "", false},
		{"Ctrl+Alt+F8", "", false}, {"Mouse5", "", false}, {"F9", "", false},
	} {
		got, err := uxDictationShortcut(tc.candidate, "F9", "Ctrl+Alt+F10")
		if tc.candidate == "Mouse5" {
			got, err = uxDictationShortcut(tc.candidate, "Mouse5", "Ctrl+Alt+F10")
		}
		if tc.valid && (err != nil || got != tc.want) {
			t.Errorf("%q: got %q, err %v", tc.candidate, got, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("%q unexpectedly allowed", tc.candidate)
		}
	}
}

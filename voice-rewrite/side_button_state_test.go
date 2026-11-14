package main

import "testing"

func TestSideButtonShortPressReplaysNavigation(t *testing.T) {
	var s sideButtonState
	token, ok := s.press()
	if !ok || !s.canActivate(token) {
		t.Fatal("initial press must begin pending hold")
	}
	if held, active := s.release(); !held || active {
		t.Fatalf("short release: held=%t active=%t", held, active)
	}
	if s.activate(token) || s.canActivate(token) {
		t.Fatal("expired timer activated a released button")
	}
}

func TestSideButtonLongPressStartsOnce(t *testing.T) {
	var s sideButtonState
	token, ok := s.press()
	if !ok || !s.activate(token) {
		t.Fatal("hold must activate")
	}
	if s.canActivate(token) || s.activate(token) {
		t.Fatal("hold activated more than once")
	}
	if held, active := s.release(); !held || !active {
		t.Fatalf("long release: held=%t active=%t", held, active)
	}
}

func TestSideButtonDuplicateAndStaleEvents(t *testing.T) {
	var s sideButtonState
	if held, _ := s.release(); held {
		t.Fatal("release without a press was handled")
	}
	first, _ := s.press()
	if second, ok := s.press(); ok || first != second {
		t.Fatal("duplicate down changed active press")
	}
	s.release()
	next, ok := s.press()
	if !ok || next == first || s.activate(first) {
		t.Fatal("old timer activated new press")
	}
	if !s.activate(next) {
		t.Fatal("current press did not activate")
	}
}

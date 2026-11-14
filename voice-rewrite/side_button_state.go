package main

// sideButtonState is accessed under the mouse-hook state lock.
type sideButtonState struct {
	down   bool
	active bool
	token  uint64
}

func (s *sideButtonState) press() (uint64, bool) {
	if s.down {
		return s.token, false
	}
	s.token++
	s.down = true
	s.active = false
	return s.token, true
}

func (s *sideButtonState) canActivate(token uint64) bool {
	return s.down && !s.active && s.token == token
}

func (s *sideButtonState) activate(token uint64) bool {
	if !s.canActivate(token) {
		return false
	}
	s.active = true
	return true
}

func (s *sideButtonState) release() (wasHeld, active bool) {
	if !s.down {
		return false, false
	}
	active = s.active
	s.down = false
	s.active = false
	s.token++
	return true, active
}

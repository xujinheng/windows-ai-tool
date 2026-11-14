package main

func main() { runApp() }

// releaseDebounced needs two consecutive polls with the recording key up.
// This prevents transient keyboard-state gaps from cutting off dictation.
func releaseDebounced(keyDown bool, upPolls *int) bool {
	if keyDown {
		*upPolls = 0
		return false
	}
	*upPolls++
	return *upPolls >= 2
}

// trayMenuRequested recognizes both pre-v4 notification callbacks and
// NOTIFYICON_VERSION_4 WM_CONTEXTMENU events.
func trayMenuRequested(event uint32) bool {
	return event == 0x0205 || event == 0x007b
}

//go:build windows

package main

import "testing"

func TestClipboardGDIHandlesNotCopiedAsHGlobal(t *testing.T) {
	for _, id := range []uintptr{2, 3, 9, 14, 0x80, 0x82, 0x83, 0x8e} {
		if !clipboardHandleFormat(id) {
			t.Errorf("format %#x requires special Windows ownership handling", id)
		}
	}
	for _, id := range []uintptr{1, 7, 8, 13, 15, 0xc000} {
		if clipboardHandleFormat(id) {
			t.Errorf("format %#x normally uses HGLOBAL", id)
		}
	}
}

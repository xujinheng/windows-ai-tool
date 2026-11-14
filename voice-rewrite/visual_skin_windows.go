//go:build windows

package main

import (
	"runtime"
	"unsafe"
)

// Paint a native GDI card with a single fill, outline and radius.
func uiRoundSurface(hdc uintptr, r rect32, radius int32, fill, outline uintptr) {
	b, _, _ := procCreateSolidBrush.Call(fill)
	pen, _, _ := procCreatePen.Call(0, 1, outline)
	if b == 0 || pen == 0 {
		if b != 0 {
			procDeleteObject.Call(b)
		}
		if pen != 0 {
			procDeleteObject.Call(pen)
		}
		return
	}
	oldB, _, _ := procSelectObject.Call(hdc, b)
	oldP, _, _ := procSelectObject.Call(hdc, pen)
	procRoundRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius), uintptr(radius))
	procSelectObject.Call(hdc, oldB)
	procSelectObject.Call(hdc, oldP)
	procDeleteObject.Call(b)
	procDeleteObject.Call(pen)
}

// Owner-draw keeps keyboard focus, tab navigation, access keys and real HWNDs.
func uiDrawActionButton(item *drawItemStruct, palette uiPalette, font uintptr, label string, primary bool) bool {
	if item == nil {
		return false
	}
	fill, outline, ink := palette.secondary, palette.border, palette.text
	if primary {
		fill, outline, ink = palette.primary, palette.primary, palette.primaryText
	} else if item.ItemState&ODS_SELECTED != 0 {
		fill = palette.secondaryPressed
	}
	if primary && item.ItemState&ODS_SELECTED != 0 {
		if palette.dark {
			fill = 0xC44D81
		} else {
			fill = 0x93365C
		}
		outline = fill
	}
	uiRoundSurface(item.Hdc, item.Rect, 12, palette.color(fill), palette.color(outline))
	procSetBkMode.Call(item.Hdc, TRANSPARENT)
	procSetTextColor.Call(item.Hdc, palette.color(ink))
	if font != 0 {
		old, _, _ := procSelectObject.Call(item.Hdc, font)
		defer procSelectObject.Call(item.Hdc, old)
	}
	ptr := uxPtr(label)
	procDrawText.Call(item.Hdc, uintptr(unsafe.Pointer(ptr)), ^uintptr(0), uintptr(unsafe.Pointer(&item.Rect)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	runtime.KeepAlive(ptr)
	if item.ItemState&0x0010 != 0 { // ODS_FOCUS
		focus := item.Rect
		focus.Left += 4
		focus.Top += 4
		focus.Right -= 4
		focus.Bottom -= 4
		user32.NewProc("DrawFocusRect").Call(item.Hdc, uintptr(unsafe.Pointer(&focus)))
	}
	return true
}

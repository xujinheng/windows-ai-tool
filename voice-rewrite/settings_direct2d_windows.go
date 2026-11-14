//go:build windows

package main

import "unsafe"

// drawSettings reuses the same Direct2D and DirectWrite resources as the result
// card. Native Windows EDIT, COMBOBOX and BUTTON children remain interactive;
// only noninteractive surfaces and the compact window heading are painted.
func (d *resultRenderer) drawSettings(scrollX, scrollY int32) bool {
	if d == nil {
		return false
	}
	if d.target == 0 {
		if err := d.createTarget(); err != nil {
			return false
		}
	}
	transform := resultD2DMatrix{M11: d.scale, M22: d.scale}
	resultCOM(d.target, 30, uintptr(unsafe.Pointer(&transform)))
	resultCOM(d.target, 48)
	bg := resultColor(d.colors[0])
	resultCOM(d.target, 47, uintptr(unsafe.Pointer(&bg)))
	x, y := float32(scrollX), float32(scrollY)
	// No brand panel, tagline, decorative badges or unused right-column gap.
	d.text("设置", 20-x, 13-y, 260, 34, 2, 4)
	d.round(14-x, 54-y, 323, 301, 13, 2)
	d.outline(14-x, 54-y, 323, 301, 13, 6)
	d.round(348-x, 54-y, 331, 301, 13, 2)
	d.outline(348-x, 54-y, 331, 301, 13, 6)
	d.round(14-x, 366-y, 665, 96, 13, 2)
	d.outline(14-x, 366-y, 665, 96, 13, 6)
	d.fill(18-x, 470-y, 657, 1, 6)
	hr := resultCOM(d.target, 49, 0, 0)
	if uint32(hr) == 0x8899000c { // D2DERR_RECREATE_TARGET
		d.disposeTarget()
	}
	return int32(hr) >= 0
}

// The GDI fallback uses exactly the same geometry. A machine without usable
// Direct2D retains every editable field, model probe and Save action.
func (p *uxSettings) drawSettingsFallback(hdc uintptr) {
	sc := func(v int32) int32 { return int32(float64(v)*p.scale + 0.5) }
	ox, oy := p.scrollX, p.scrollY
	for _, r := range []rect32{
		{14, 54, 337, 355},
		{348, 54, 679, 355},
		{14, 366, 679, 462},
	} {
		uiRoundSurface(hdc, rect32{
			sc(r.Left - ox), sc(r.Top - oy),
			sc(r.Right - ox), sc(r.Bottom - oy),
		}, sc(13), p.palette.color(p.palette.surface), p.palette.color(p.palette.border))
	}
	procSetBkMode.Call(hdc, TRANSPARENT)
	procSetTextColor.Call(hdc, p.palette.color(p.palette.text))
	if p.titleFont != 0 {
		old, _, _ := procSelectObject.Call(hdc, p.titleFont)
		defer procSelectObject.Call(hdc, old)
	}
	txt := uxPtr("设置")
	title := rect32{sc(20 - ox), sc(12 - oy), sc(320 - ox), sc(47 - oy)}
	procDrawText.Call(hdc, uintptr(unsafe.Pointer(txt)), ^uintptr(0), uintptr(unsafe.Pointer(&title)), 0)
}

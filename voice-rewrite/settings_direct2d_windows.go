//go:build windows

package main

import "unsafe"

// These are deliberately *not* raised cards. A quiet left navigation rail,
// understated dividers and one focused settings pane follow Raycast/Linear's
// navigational hierarchy, while the application's identity stays original.
var settingsHeadings = [3]string{"语音识别", "AI 写作", "快捷键与设备"}
var settingsSubtitles = [3]string{
	"连接语音识别服务，将录音转换为文字。",
	"配置用于改写、翻译与问答的文本模型。",
	"选择听写快捷键和录音输入设备。",
}
var settingsSectionNames = [3]string{"连接配置", "模型配置", "输入偏好"}

func (d *resultRenderer) drawSettings(page int, scrollX, scrollY int32, focusRow int) bool {
	if d == nil {
		return false
	}
	if d.target == 0 {
		if err := d.createTarget(); err != nil {
			return false
		}
	}
	var rc rect32
	procGetClientRect.Call(d.hwnd, uintptr(unsafe.Pointer(&rc)))
	w, h := float32(rc.Right)/d.scale, float32(rc.Bottom)/d.scale
	transform := resultD2DMatrix{M11: d.scale, M22: d.scale}
	resultCOM(d.target, 30, uintptr(unsafe.Pointer(&transform)))
	resultCOM(d.target, 48)
	bg := resultColor(d.colors[0])
	resultCOM(d.target, 47, uintptr(unsafe.Pointer(&bg)))
	// Persistent left navigation is intentionally less contrasty than content.
	d.fill(0, 0, 194, h, 1)
	d.fill(193, 0, 1, h, 6)
	d.text("VoiceRewrite", 20, 23, 167, 27, 3, 4)
	d.text("设置分类", 22, 75, 150, 20, 0, 5)
	d.fill(20, 246, 154, 1, 6)
	// No decorative footer copy: navigation is the only purpose of the rail.
	// Main pane. No brand banner, stacked white cards or redundant headings.
	if page < 0 || page >= len(settingsHeadings) {
		page = 0
	}
	d.text(settingsHeadings[page], 220, 35, w-245, 39, 2, 4)
	d.text(settingsSubtitles[page], 222, 79, w-244, 23, 0, 5)
	d.fill(219, 121, w-239, 1, 6)
	d.text(settingsSectionNames[page], 224, 140, 235, 30, 3, 4)
	// Horizontal rules align with the three native field rows.
	if page == 0 || page == 1 {
		for i, y := range []float32{169, 239, 309} {
			rectY := y-float32(scrollY)
			rectX := float32(345)-float32(scrollX)
			d.round(rectX, rectY, 447, 42, 8, 2)
			outline := 6
			if i == focusRow { outline = 7 }
			d.outline(rectX, rectY, 447, 42, 8, outline)
		}
		for _, y := range []float32{213, 283, 353} {
			d.fill(224-float32(scrollX), y-float32(scrollY), w-253, 1, 6)
		}
	} else {
		rectX, rectY := float32(345)-float32(scrollX), float32(169)-float32(scrollY)
		d.round(rectX, rectY, 226, 42, 8, 2)
		outline := 6
		if focusRow == 0 { outline = 7 }
		d.outline(rectX, rectY, 226, 42, 8, outline)
		d.fill(224-float32(scrollX), 216-float32(scrollY), w-253, 1, 6)
		d.fill(224-float32(scrollX), 290-float32(scrollY), w-253, 1, 6)
	}
	d.fill(194, h-77, w-194, 1, 6)
	hr := resultCOM(d.target, 49, 0, 0)
	if uint32(hr) == 0x8899000c {
		d.disposeTarget()
	}
	return int32(hr) >= 0
}

// GDI paints the same overall navigation and main hierarchy if Direct2D is
// unavailable, without sacrificing Win32 fields or keyboard access.
func (p *uxSettings) drawSettingsFallback(hdc uintptr) {
	sc := func(n int32) int32 { return int32(float64(n)*p.scale + 0.5) }
	var rc rect32
	procGetClientRect.Call(p.window, uintptr(unsafe.Pointer(&rc)))
	side := rect32{0, 0, sc(194), rc.Bottom}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&side)), p.cardBrush)
	line := rect32{sc(193), 0, sc(194), rc.Bottom}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&line)), p.editBrush)
	for i, y := range []int32{169, 239, 309} {
		if p.page == 2 && i > 0 { break }
		w := int32(447)
		if p.page == 2 { w = 226 }
		x := int32(345)
		outline := p.palette.color(p.palette.border)
		if p.focusRow == i { outline = p.palette.color(p.palette.primary) }
		uiRoundSurface(hdc, rect32{sc(x-p.scrollX), sc(y-p.scrollY),
			sc(x+w-p.scrollX), sc(y+42-p.scrollY)}, sc(9), p.palette.color(p.palette.field), outline)
	}
	procSetBkMode.Call(hdc, TRANSPARENT)
	procSetTextColor.Call(hdc, p.palette.color(p.palette.text))
	head, _, _ := procSelectObject.Call(hdc, p.titleFont)
	title := uxPtr(settingsHeadings[p.page])
	bounds := rect32{sc(220), sc(34), rc.Right - sc(15), sc(80)}
	procDrawText.Call(hdc, uintptr(unsafe.Pointer(title)), ^uintptr(0), uintptr(unsafe.Pointer(&bounds)), 0)
	procSelectObject.Call(hdc, head)
	procSetTextColor.Call(hdc, p.palette.color(p.palette.muted))
	sub := uxPtr(settingsSubtitles[p.page])
	bounds = rect32{sc(222), sc(80), rc.Right - sc(15), sc(110)}
	procDrawText.Call(hdc, uintptr(unsafe.Pointer(sub)), ^uintptr(0), uintptr(unsafe.Pointer(&bounds)), 0)
}

// Owner-drawn native sidebar BUTTON: focusable, keyboard-activatable, no
// browser renderer. Only the selected item receives a tinted background.
func (p *uxSettings) drawSidebarItem(item *drawItemStruct, index int) bool {
	if p == nil || item == nil || index < 0 || index >= len(settingsHeadings) {
		return false
	}
	r := item.Rect
	// Inactive items are transparent relative to the sidebar, not three
	// separately filled cards.
	sideRGB := uint32(0x181b23)
	if !p.palette.dark { sideRGB = 0xf0f1f6 }
	side, _, _ := procCreateSolidBrush.Call(p.palette.color(sideRGB))
	if side != 0 {
		procFillRect.Call(item.Hdc, uintptr(unsafe.Pointer(&r)), side)
		procDeleteObject.Call(side)
	}
	if index == p.page {
		uiRoundSurface(item.Hdc, r, 9, p.palette.color(p.palette.secondary), p.palette.color(p.palette.secondary))
		accent, _, _ := procCreateSolidBrush.Call(p.palette.color(p.palette.primary))
		if accent != 0 {
			mark := rect32{r.Left + 2, r.Top + 9, r.Left + 5, r.Bottom - 9}
			procFillRect.Call(item.Hdc, uintptr(unsafe.Pointer(&mark)), accent)
			procDeleteObject.Call(accent)
		}
	}
	
	procSetBkMode.Call(item.Hdc, TRANSPARENT)
	ink := p.palette.muted
	if index == p.page {
		ink = p.palette.text
	}
	procSetTextColor.Call(item.Hdc, p.palette.color(ink))
	if p.font != 0 {
		old, _, _ := procSelectObject.Call(item.Hdc, p.font)
		defer procSelectObject.Call(item.Hdc, old)
	}
	label := uxPtr(settingsHeadings[index])
	r.Left += 18
	procDrawText.Call(item.Hdc, uintptr(unsafe.Pointer(label)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_VCENTER|DT_SINGLELINE)
	if item.ItemState&0x0010 != 0 {
		focus := item.Rect
		focus.Left += 8
		focus.Top += 4
		focus.Right -= 6
		focus.Bottom -= 4
		user32.NewProc("DrawFocusRect").Call(item.Hdc, uintptr(unsafe.Pointer(&focus)))
	}
	return true
}

//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// A small, native settings surface; it does not host a browser or intercept
// the editor focus until deliberately opened from the tray menu.
const (
	uxWMCommand       = 0x0111
	uxWMSize          = 0x0005
	uxWMVScroll       = 0x0115
	uxWMHScroll       = 0x0114
	uxWMMouseWheel    = 0x020A
	uxSIFRange        = 1
	uxSIFPage         = 2
	uxSIFPos          = 4
	uxSIFTrackPos     = 16
	uxSBHorz          = 0
	uxSBVert          = 1
	uxWMGetMinMaxInfo = 0x0024
	uxWSThickFrame    = 0x00040000
	uxWMSettingChange = 0x001A
	uxWMThemeChanged  = 0x031A
	uxWMClose         = 0x0010
	uxWMEraseBkgnd    = 0x0014
	uxWMPaint         = 0x000F
	uxWMNCDestroy     = 0x0082
	uxWMColorStatic   = 0x0138
	uxWMColorEdit     = 0x0133
	uxWMDrawItem      = 0x002B
	uxBSOwnerDraw     = 0x000B
	uxEMSetMargins    = 0x00D3
	uxWMProbeDone     = 0x8000 + 104
	uxSaveButton      = 9101
	uxCloseButton     = 9102
	uxAdvancedButton  = 9103
	uxProbeASRButton  = 9104
	uxProbeLLMButton  = 9105
	uxShortcutInput   = 9106
	uxMicrophoneInput = 9107
	uxTabSpeech       = 9120
	uxTabWriting      = 9121
	uxTabInput        = 9122
	uxRoleOther       = iota + 100
	uxRolePageTitle
	uxRolePageDescription
	uxRoleLabel
	uxRoleField
	uxRoleProbe
	uxRoleStatus
	uxCBAddString     = 0x0143
	uxCBSetCurSel     = 0x014E
	uxCBGetCurSel     = 0x0147
	uxCBSDropDownList = 0x0003
	uxWSChild         = 0x40000000
	uxWSVisible       = 0x10000000
	uxWSTabStop       = 0x00010000
	uxESAutoScroll    = 0x0080
	uxESPassword      = 0x0020
	uxWSBorder        = 0x00800000
	uxWSCaption       = 0x00C00000
	uxWSSysMenu       = 0x00080000
	uxWSMinimizeBox   = 0x00020000
)

var (
	uxGetWindowText     = user32.NewProc("GetWindowTextW")
	uxSetTextColor      = gdi32.NewProc("SetTextColor")
	uxSelectObject      = gdi32.NewProc("SelectObject")
	uxRoundRect         = gdi32.NewProc("RoundRect")
	uxGetStockObject    = gdi32.NewProc("GetStockObject")
	uxSetBkMode         = gdi32.NewProc("SetBkMode")
	uxWindowCallback    = syscall.NewCallback(uxSettingsWndProc)
	procShowScrollBarUX = user32.NewProc("ShowScrollBar")
	procSaveDCPrefs     = gdi32.NewProc("SaveDC")
	procRestoreDCPrefs  = gdi32.NewProc("RestoreDC")
	procClipPrefs       = gdi32.NewProc("IntersectClipRect")
	uxActiveSettings    *uxSettings
)

type uxSettings struct {
	window                                  uintptr
	app                                     *appState
	inputs                                  [6]uintptr
	shortcut                                uintptr
	microphone                              uintptr
	microphoneIDs                           []int
	hint                                    uintptr
	font, titleFont, sectionFont, mutedFont uintptr
	background                              uintptr
	cardBrush                               uintptr
	editBrush                               uintptr
	palette                                 uiPalette
	cardStatics                             map[uintptr]bool
	probeMu                                 sync.Mutex
	probeResult                             [2]string
	probeStatus                             [2]uintptr
	probeSerial                             [2]uint64
	controls                                []uxControl
	fieldFrames                             []uxControl
	activeTab                               int
	dirty                                   bool
	initializing                            bool
	tabs                                    [3]uintptr
	scale                                   float64
	scrollX, scrollY                        int32
}

// Every child control is reflowed after a resize. No hard-coded physical
// coordinates may escape uxLayoutRect; the data is in 96-DPI logical units.
type uxControl struct {
	hwnd       uintptr
	x, y, w, h int32
	stretch    bool
	right      bool
	id         uintptr
	tab        int
	role       int
	row        int
	inset      bool
}

func uxLayoutRect(c uxControl, clientWidth int32) (x, y, w, h int32) {
	return layoutSettingsRect(uxLayoutSpec{x: c.x, y: c.y, w: c.w, h: c.h, stretch: c.stretch, right: c.right}, clientWidth)
}

// 96-DPI logical coordinate scrolling avoids inaccessible buttons on small
// work areas and 125%–200% display scaling.
const uxContentWidth, uxContentHeight int32 = 694, 689

type uxScrollInfo struct {
	Size, Mask    uint32
	Min, Max      int32
	Page          uint32
	Pos, TrackPos int32
}

func uxScrollClamp(n, limit int32) int32 {
	if n < 0 {
		return 0
	}
	if n > limit {
		return limit
	}
	return n
}

// preferenceRect keeps a compact, macOS-inspired preferences toolbar while
// moving labels above fields on narrow/high-DPI windows. The form is one
// page at a time; no control is placed outside the usable horizontal viewport.
func preferenceRect(c uxControl, viewW, viewH, scrollY int32) (x, y, w, h int32) {
	x, y, w, h = c.x, c.y, c.w, c.h
	narrow := viewW < 640
	compact := viewH < 460
	if c.id >= uxTabSpeech && c.id <= uxTabInput {
		width := (viewW - 64 - 16) / 3
		left := int32(32)
		if viewW >= 680 {
			width = 134
			left = (viewW - (3*width + 16)) / 2
		}
		if width < 90 {
			width = 90
		}
		x = left + (int32(c.id-uxTabSpeech))*(width+8)
		if compact {
			return x, 66, width, 38
		}
		return x, 115, width, 40
	}
	footerY := viewH - 64
	if compact {
		footerY = viewH - 54
	}
	if c.id == uxSaveButton {
		return viewW - 188, footerY, 156, 36
	}
	if c.id == uxCloseButton {
		return viewW - 296, footerY, 94, 36
	}
	if c.id == uxAdvancedButton {
		return 32, footerY, 130, 36
	}
	if c.tab < 0 {
		if c.y >= 600 {
			return 34, viewH - 22, viewW - 68, 18
		}
		if compact && c.y < 100 {
			if c.x == 78 {
				y = 13
			}
			if c.x == 80 && c.y == 25 {
				y = 13
			}
		}
		if c.x >= 510 {
			return viewW - 220, y, 184, h
		}
		if c.x == 80 || c.x == 78 {
			w = viewW - x - 38
		}
		return
	}
	switch c.role {
	case uxRolePageTitle:
		x, y, w = 46, 186, viewW-92
		if compact {
			x, y, w = 42, 115, viewW-84
		}
	case uxRolePageDescription:
		x, y, w = 46, 222, viewW-92
		if compact {
			x, y, w = 42, 152, viewW-84
		}
	case uxRoleLabel:
		if narrow {
			x, y, w = 42, 270+int32(c.row)*88, viewW-84
		} else {
			x, y, w = 48, 297+int32(c.row)*68, 98
		}
	case uxRoleField:
		if narrow {
			x, y, w, h = 42, 294+int32(c.row)*88, viewW-84, 35
			if c.row == 0 && c.tab != 2 {
				w -= 110
			}
		} else {
			x, y, w, h = 164, 287+int32(c.row)*68, viewW-200, 38
			if c.row == 0 && c.tab != 2 {
				w -= 116
			}
		}
	case uxRoleProbe:
		if narrow {
			x, y, w, h = viewW-144, 294, 102, 35
		} else {
			x, y, w, h = viewW-148, 287, 112, 38
		}
	case uxRoleStatus:
		x, y, w = 48, 502, viewW-96
		if narrow {
			x, y, w = 42, 562, viewW-84
		}
	}
	if compact {
		switch c.role {
		case uxRoleLabel:
			x, y, w = 42, 197+int32(c.row)*82, viewW-84
		case uxRoleField:
			x, y, w, h = 42, 222+int32(c.row)*82, viewW-84, 35
			if c.row == 0 && c.tab != 2 {
				w -= 110
			}
		case uxRoleProbe:
			x, y, w, h = viewW-144, 222, 102, 35
		case uxRoleStatus:
			x, y, w = 42, 476, viewW-84
		}
	}
	if c.role != uxRolePageTitle && c.role != uxRolePageDescription {
		y -= scrollY
	}
	if c.inset {
		x += 6
		y += 2
		w -= 12
		h -= 4
	}
	return
}
func (p *uxSettings) layout() {
	if p == nil || p.window == 0 {
		return
	}
	var r rect32
	procGetClientRect.Call(p.window, uintptr(unsafe.Pointer(&r)))
	viewW := int32(float64(r.Right-r.Left) / p.scale)
	viewH := int32(float64(r.Bottom-r.Top) / p.scale)
	if viewW < 1 {
		viewW = 1
	}
	if viewH < 1 {
		viewH = 1
	}
	compact := viewH < 460
	footerTop := viewH - 82
	bottom := int32(550)
	bodyTop := int32(176)
	if viewW < 640 {
		bottom = 604
	}
	if compact {
		footerTop = viewH - 66
		bottom = 514
		bodyTop = 179
	}
	maxScroll := bottom - footerTop
	if maxScroll < 0 {
		maxScroll = 0
	}
	p.scrollX = 0
	p.scrollY = uxScrollClamp(p.scrollY, maxScroll)
	// Hide the native scrollbar on normal monitors; use it as an explicit
	// accessibility fallback on vertically constrained high-DPI work areas.
	if maxScroll > 0 {
		info := uxScrollInfo{Size: uint32(unsafe.Sizeof(uxScrollInfo{})), Mask: uxSIFRange | uxSIFPage | uxSIFPos,
			Min: 0, Max: bottom - bodyTop, Page: uint32(footerTop - bodyTop), Pos: p.scrollY}
		if footerTop <= bodyTop {
			info.Page = 1
		}
		procSetScrollInfoUX.Call(p.window, uxSBVert, uintptr(unsafe.Pointer(&info)), 1)
	}
	if maxScroll > 0 {
		procShowScrollBarUX.Call(p.window, uxSBVert, 1)
	} else {
		procShowScrollBarUX.Call(p.window, uxSBVert, 0)
	}
	px := func(v int32) uintptr { return uintptr(int32(float64(v)*p.scale + 0.5)) }
	for _, c := range p.controls {
		x, y, w, h := preferenceRect(c, viewW, viewH, p.scrollY)
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		procMoveWindow.Call(c.hwnd, px(x), px(y), px(w), px(h), 1)
		visible := c.tab < 0 || c.tab == p.activeTab
		if c.tab >= 0 {
			if c.role != uxRolePageTitle && c.role != uxRolePageDescription && (y < bodyTop || y+h > footerTop) {
				visible = false
			}
		}
		if compact && c.tab < 0 && (c.y == 65 || (c.x >= 510 && c.id == 0 && c.y < 100) || c.y >= 600) {
			visible = false
		}
		if visible {
			procShowWindow.Call(c.hwnd, SW_SHOW)
		} else {
			procShowWindow.Call(c.hwnd, 0)
		}
	}
	procInvalidateRectSettings.Call(p.window, 0, 1)
}
func (p *uxSettings) scroll(axis, request uintptr) {
	var rect rect32
	procGetClientRect.Call(p.window, uintptr(unsafe.Pointer(&rect)))
	viewW := int32(float64(rect.Right-rect.Left) / p.scale)
	viewH := int32(float64(rect.Bottom-rect.Top) / p.scale)
	bottom := int32(550)
	if viewW < 640 {
		bottom = 604
	}
	footerTop := viewH - 82
	if viewH < 460 {
		bottom = 514
		footerTop = viewH - 66
	}
	max := bottom - footerTop
	if max < 0 {
		max = 0
	}
	view := viewH - 258
	if viewH < 460 {
		view = viewH - 245
	}
	if view < 36 {
		view = 36
	}
	pos := &p.scrollY
	if axis == uxSBHorz {
		return
	}
	var delta int32
	switch request & 0xffff {
	case 0:
		delta = -24
	case 1:
		delta = 24
	case 2:
		delta = -view + 48
	case 3:
		delta = view - 48
	case 6:
		*pos = 0
	case 7:
		*pos = max
	case 4, 5:
		info := uxScrollInfo{Size: uint32(unsafe.Sizeof(uxScrollInfo{})), Mask: uxSIFTrackPos}
		procGetScrollInfoUX.Call(p.window, axis, uintptr(unsafe.Pointer(&info)))
		*pos = info.TrackPos
	}
	*pos = uxScrollClamp(*pos+delta, max)
	p.layout()
}

var (
	procMoveWindow             = user32.NewProc("MoveWindow")
	procSetScrollInfoUX        = user32.NewProc("SetScrollInfo")
	procGetScrollInfoUX        = user32.NewProc("GetScrollInfo")
	procInvalidateRectSettings = user32.NewProc("InvalidateRect")
)

type uxMinMaxInfo struct{ Reserved, MaxSize, MaxPos, MinSize, MaxTrack trayPoint }

func uxPtr(s string) *uint16 { ptr, _ := syscall.UTF16PtrFromString(s); return ptr }

func uxWinText(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLength.Call(hwnd)
	if n > 4096 {
		n = 4096
	}
	buf := make([]uint16, n+1)
	uxGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}
func uxSetText(hwnd uintptr, text string) {
	if hwnd == 0 {
		return
	}
	ptr := uxPtr(text)
	procSetWindowText.Call(hwnd, uintptr(unsafe.Pointer(ptr)))
	runtime.KeepAlive(ptr)
}

func uxSettingsKeyMessage(msg winMSG) bool {
	p := uxActiveSettings
	if p == nil || p.window == 0 {
		return false
	}
	if msg.Message != WM_KEYDOWN || msg.WParam != VK_ESCAPE {
		return false
	}
	if msg.Hwnd != p.window {
		child, _, _ := procIsChild.Call(p.window, msg.Hwnd)
		if child == 0 {
			return false
		}
	}
	procSendMessage.Call(p.window, uxWMClose, 0, 0)
	return true
}

func (p *uxSettings) refreshTheme() {
	if p == nil || p.window == 0 {
		return
	}
	palette := systemPalette()
	if palette == p.palette {
		return
	}
	p.background = refreshPaletteBrush(p.background, palette, palette.background)
	p.cardBrush = refreshPaletteBrush(p.cardBrush, palette, palette.surface)
	p.editBrush = refreshPaletteBrush(p.editBrush, palette, palette.field)
	p.palette = palette
	applyWindowPalette(p.window, palette)
	refreshNativeWindow(p.window)
	for _, c := range p.controls {
		refreshNativeWindow(c.hwnd)
	}
}

// Feedback colors communicate probe state without overstating compatibility:
// reachable does not necessarily mean a model can execute the requested task.
func uxSettingsFeedbackColor(p uiPalette, status string) uint32 {
	switch {
	case strings.Contains(status, "接受 WAV") ||
		strings.Contains(status, "已连接") ||
		strings.Contains(status, "已在服务中发现"):
		if p.dark {
			return 0x7BD6B1
		}
		return 0x237A58
	case strings.Contains(status, "无效") ||
		strings.Contains(status, "拒绝") ||
		strings.Contains(status, "无法连接") ||
		strings.Contains(status, "错误") ||
		strings.Contains(status, "不可用"):
		if p.dark {
			return 0xF39B9B
		}
		return 0xB54353
	case strings.Contains(status, "可达") || strings.Contains(status, "不提供模型列表"):
		if p.dark {
			return 0xE8BD78
		}
		return 0x896012
	default:
		return p.muted
	}
}

// markDirty is shared by the real edit notifications and preview fixture.
func (p *uxSettings) markDirty() {
	if p==nil || p.window==0 || p.initializing {return}
	p.dirty=true
	if p.hint!=0 {uxSetText(p.hint,"● 有未保存的更改 · 点击右侧保存")}
	refreshNativeWindow(p.window)
	for _,ctrl:=range p.controls {
		if ctrl.id==uxSaveButton {refreshNativeWindow(ctrl.hwnd);break}
	}
}

func uxSettingsWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	p := uxActiveSettings
	switch uint32(msg) {
	case uxWMSettingChange, uxWMThemeChanged:
		if p != nil && p.window == hwnd {
			p.refreshTheme()
		}
		return 0
	case uxWMVScroll, uxWMHScroll:
		if p != nil && p.window == hwnd {
			axis := uintptr(uxSBVert)
			if uint32(msg) == uxWMHScroll {
				axis = uxSBHorz
			}
			p.scroll(axis, wParam)
		}
		return 0
	case uxWMMouseWheel:
		if p != nil && p.window == hwnd {
			delta := int32(int16((wParam >> 16) & 0xffff))
			p.scrollY -= delta / 120 * 72
			p.layout()
		}
		return 0
	case uxWMSize:
		if p != nil && p.window == hwnd {
			p.layout()
		}
		return 0
	case uxWMGetMinMaxInfo:
		if p != nil && lParam != 0 {
			bounds := (*uxMinMaxInfo)(unsafe.Pointer(lParam))
			work := uxWorkAreaAtCursor()
			minW, minH := int32(480*p.scale), int32(440*p.scale)
			if minW > work.right-work.left-12 {
				minW = work.right - work.left - 12
			}
			if minH > work.bottom-work.top-12 {
				minH = work.bottom - work.top - 12
			}
			bounds.MinSize = trayPoint{X: minW, Y: minH}
		}
		return 0
	case uxWMClose:
		if p != nil && p.window == hwnd && p.dirty {
			question := uxPtr("当前有未保存的设置。确定放弃这些更改吗？")
			title := uxPtr("VoiceRewrite · 放弃更改")
			// Yes/No with No as the safe default; do not silently discard credentials.
			choice, _, _ := user32.NewProc("MessageBoxW").Call(hwnd,
				uintptr(unsafe.Pointer(question)), uintptr(unsafe.Pointer(title)),
				0x00000004|0x00000030|0x00000100)
			if choice != 6 {
				return 0
			} // IDYES
		}
		procDestroyWindow.Call(hwnd)
		return 0
	case uxWMNCDestroy:
		if p != nil && p.window == hwnd {
			uxActiveSettings = nil
			if p.font != 0 {
				procDeleteObject.Call(p.font)
			}
			if p.titleFont != 0 {
				procDeleteObject.Call(p.titleFont)
			}
			if p.sectionFont != 0 {
				procDeleteObject.Call(p.sectionFont)
			}
			if p.mutedFont != 0 {
				procDeleteObject.Call(p.mutedFont)
			}
			if p.background != 0 {
				procDeleteObject.Call(p.background)
			}
			if p.cardBrush != 0 {
				procDeleteObject.Call(p.cardBrush)
			}
			if p.editBrush != 0 {
				procDeleteObject.Call(p.editBrush)
			}
		}
		return 0
	case uxWMEraseBkgnd:
		if p != nil && p.window == hwnd && p.background != 0 {
			var r rect32
			procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
			procFillRect.Call(wParam, uintptr(unsafe.Pointer(&r)), p.background)
			return 1
		}
	case uxWMPaint:
		if p != nil && p.window == hwnd {
			var ps handyPaintStruct
			hdc, _, _ := procBeginPaintHandy.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
			if hdc != 0 {
				sc := func(v int32) int32 { return int32(float64(v)*p.scale + 0.5) }
				var rc rect32
				procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
				viewW := int32(float64(rc.Right) / p.scale)
				viewH := int32(float64(rc.Bottom) / p.scale)
				compact := viewH < 460
				footerTop := viewH - 82
				bodyTop := int32(176)
				cardTop := int32(265)
				cardBottom := int32(548)
				if viewW < 640 {
					cardBottom = 604
				}
				if compact {
					footerTop = viewH - 66
					bodyTop = 179
					cardTop = 186
					cardBottom = 515
				}
				// The preference switcher is a compact, centered toolbar at normal
				// widths, with a full-width adaptive layout when space is limited.
				navW := int32(3*134 + 16)
				navX := (viewW - navW) / 2
				if viewW < 680 {
					navW = viewW - 64
					navX = 32
				}
				navY := int32(109)
				if compact {
					navY = 61
				}
				uiRoundSurface(hdc, rect32{sc(navX - 4), sc(navY), sc(navX + navW + 4), sc(navY + 52)},
					sc(15), p.palette.color(p.palette.secondary), p.palette.color(p.palette.secondary))
				// The content surface scrolls under stable navigation and action bars.
				saved, _, _ := procSaveDCPrefs.Call(hdc)
				procClipPrefs.Call(hdc, 0, uintptr(sc(bodyTop)), uintptr(rc.Right), uintptr(sc(footerTop)))
				uiRoundSurface(hdc, rect32{sc(26), sc(cardTop - p.scrollY), rc.Right - sc(26), sc(cardBottom - p.scrollY)},
					sc(18), p.palette.color(p.palette.surface), p.palette.color(p.palette.border))
				contentW := viewW
				for _, field := range p.fieldFrames {
					if field.tab != p.activeTab {
						continue
					}
					x, y, w, h := preferenceRect(field, contentW, viewH, p.scrollY)
					uiRoundSurface(hdc, rect32{sc(x), sc(y), sc(x + w), sc(y + h)}, sc(10),
						p.palette.color(p.palette.field), p.palette.color(p.palette.border))
				}
				if saved != 0 {
					procRestoreDCPrefs.Call(hdc, saved)
				}
				// One restrained accent mark anchors the brand; avoid decorating every group.
				brandY := int32(30)
				if compact {
					brandY = 13
				}
				uiRoundSurface(hdc, rect32{sc(32), sc(brandY), sc(66), sc(brandY + 34)}, sc(11),
					p.palette.color(p.palette.primary), p.palette.color(p.palette.primary))
				uxSetBkMode.Call(hdc, TRANSPARENT)
				uxSetTextColor.Call(hdc, p.palette.color(p.palette.primaryText))
				oldFont, _, _ := uxSelectObject.Call(hdc, p.sectionFont)
				monogram := uxPtr("V")
				brandRect := rect32{sc(32), sc(brandY), sc(66), sc(brandY + 34)}
				procDrawText.Call(hdc, uintptr(unsafe.Pointer(monogram)), ^uintptr(0), uintptr(unsafe.Pointer(&brandRect)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
				uxSelectObject.Call(hdc, oldFont)
				// Hairline above stable footer: quiet, native macOS preferences rhythm.
				lineBrush, _, _ := procCreateSolidBrush.Call(p.palette.color(p.palette.border))
				if lineBrush != 0 {
					line := rect32{0, sc(footerTop), rc.Right, sc(footerTop + 1)}
					procFillRect.Call(hdc, uintptr(unsafe.Pointer(&line)), lineBrush)
					procDeleteObject.Call(lineBrush)
				}
				procEndPaintHandy.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
			}
			return 0
		}
	case uxWMDrawItem:
		if p != nil && p.window == hwnd && lParam != 0 {
			item := (*drawItemStruct)(unsafe.Pointer(lParam))
			if item.CtlID >= uxTabSpeech && item.CtlID <= uxTabInput {
				labels := [3]string{"语音输入", "AI 写作", "快捷键"}
				selected := int(item.CtlID-uxTabSpeech) == p.activeTab
				fill, border, ink := p.palette.background, p.palette.background, p.palette.muted
				if selected {
					fill, border, ink = p.palette.surface, p.palette.border, p.palette.text
				}
				if item.ItemState&ODS_SELECTED != 0 {
					fill = p.palette.secondaryPressed
				}
				base, _, _ := procCreateSolidBrush.Call(p.palette.color(p.palette.secondary))
				if base != 0 {
					procFillRect.Call(item.Hdc, uintptr(unsafe.Pointer(&item.Rect)), base)
					procDeleteObject.Call(base)
				}
				uiRoundSurface(item.Hdc, item.Rect, 13, p.palette.color(fill), p.palette.color(border))
				uxSetBkMode.Call(item.Hdc, TRANSPARENT)
				uxSetTextColor.Call(item.Hdc, p.palette.color(ink))
				old, _, _ := uxSelectObject.Call(item.Hdc, p.font)
				ptr := uxPtr(labels[int(item.CtlID-uxTabSpeech)])
				procDrawText.Call(item.Hdc, uintptr(unsafe.Pointer(ptr)), ^uintptr(0), uintptr(unsafe.Pointer(&item.Rect)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
				uxSelectObject.Call(item.Hdc, old)
				if item.ItemState&0x0010 != 0 { // ODS_FOCUS
					focus := item.Rect
					focus.Left += 4
					focus.Top += 4
					focus.Right -= 4
					focus.Bottom -= 4
					user32.NewProc("DrawFocusRect").Call(item.Hdc, uintptr(unsafe.Pointer(&focus)))
				}
				return 1
			}
			label := ""
			primary := false
			switch item.CtlID {
			case uxSaveButton:
				label, primary = "保存设置", true
				if p.dirty {
					label = "保存更改"
				}
			case uxCloseButton:
				label = "关闭"
			case uxAdvancedButton:
				label = "高级设置"
			case uxProbeASRButton, uxProbeLLMButton:
				label = "测试连接"
			}
			if label != "" && uiDrawActionButton(item, p.palette, p.font, label, primary) {
				return 1
			}
		}
	case uxWMColorStatic:
		if p != nil && p.window == hwnd && p.background != 0 {
			color := p.palette.text
			if lParam == p.hint {
				color = p.palette.muted
				if p.dirty {
					color = p.palette.primary
				} else if status:=uxWinText(p.hint); strings.HasPrefix(status,"✓") || strings.HasPrefix(status,"设置已保存") {
					if p.palette.dark {color=0x7BD6B1} else {color=0x237A58}
				}
			} else if lParam == p.probeStatus[0] || lParam == p.probeStatus[1] {
				color = uxSettingsFeedbackColor(p.palette, uxWinText(lParam))
			}
			uxSetTextColor.Call(wParam, p.palette.color(color))
			uxSetBkMode.Call(wParam, TRANSPARENT)
			if p.cardStatics[lParam] {
				return p.cardBrush
			}
			return p.background
		}
	case uxWMColorEdit:
		if p != nil && p.window == hwnd && p.editBrush != 0 {
			uxSetTextColor.Call(wParam, p.palette.color(p.palette.text))
			procSetBkColor.Call(wParam, p.palette.color(p.palette.field))
			return p.editBrush
		}
	case uxWMProbeDone:
		if p != nil && p.window == hwnd && wParam < 2 {
			idx := int(wParam)
			p.probeMu.Lock()
			serial := p.probeSerial[idx]
			result := p.probeResult[idx]
			p.probeMu.Unlock()
			if serial == uint64(lParam) {
				uxSetText(p.probeStatus[idx], result)
			}
		}
		return 0
	case uxWMCommand:
		if p != nil && p.window == hwnd {
			if !p.initializing && (((wParam>>16)&0xffff == 0x0300) || // EN_CHANGE
				((wParam>>16)&0xffff == 1 && (wParam&0xffff) == uxMicrophoneInput)) { // CBN_SELCHANGE
				p.markDirty()
			}
			if !p.initializing && (wParam>>16)&0xffff == 0x0300 { // EN_CHANGE
				for idx, controls := range [2][3]uintptr{{p.inputs[0], p.inputs[1], p.inputs[2]}, {p.inputs[3], p.inputs[4], p.inputs[5]}} {
					for _, edit := range controls {
						if lParam == edit {
							p.probeMu.Lock()
							p.probeSerial[idx]++ // reject in-flight stale response
							p.probeMu.Unlock()
							uxSetText(p.probeStatus[idx], "连接配置已修改 · 请重新检测")
							break
						}
					}
				}
			}
			switch wParam & 0xffff {
			case uxTabSpeech, uxTabWriting, uxTabInput:
				p.activeTab = int((wParam & 0xffff) - uxTabSpeech)
				p.scrollY = 0
				p.layout()
				return 0
			case uxSaveButton:
				if err := p.save(); err != nil {
					uxSetText(p.hint, "保存失败："+err.Error())
				} else {
					// save() provides the precise message, including restart notices.
				}
				return 0
			case uxCloseButton:
				procSendMessage.Call(hwnd, uxWMClose, 0, 0)
				return 0
			case uxAdvancedButton:
				if err := shellOpenLocalFile(configPath()); err != nil {
					uxSetText(p.hint, "打开高级配置失败："+err.Error())
				}
				return 0
			case uxProbeASRButton:
				p.check("ASR", uxWinText(p.inputs[0]), uxWinText(p.inputs[2]), uxWinText(p.inputs[1]))
				return 0
			case uxProbeLLMButton:
				p.check("LLM", uxWinText(p.inputs[3]), uxWinText(p.inputs[5]), uxWinText(p.inputs[4]))
				return 0
			}
		}
	}
	v, _, _ := procDefWindowProc.Call(hwnd, msg, wParam, lParam)
	return v
}

func (p *uxSettings) save() error {
	if p.app.busy.Load() {
		return errors.New("请先完成当前录音／改写，再保存")
	}
	var fields [6]string
	for i, h := range p.inputs {
		fields[i] = uxWinText(h)
	}
	cfg, err := uxUpdatedConfig(p.app.cfg, fields)
	if err != nil {
		return err
	}
	newShortcut := strings.TrimSpace(uxWinText(p.shortcut))
	var shortcutErr error
	newShortcut, shortcutErr = uxDictationShortcut(newShortcut, cfg.HotkeyRecord, cfg.HotkeyExit)
	if shortcutErr != nil {
		return shortcutErr
	}
	changedShortcut := cfg.DictationHotkey != newShortcut
	cfg.DictationHotkey = newShortcut
	selected, _, _ := procSendMessage.Call(p.microphone, uxCBGetCurSel, 0, 0)
	if p.microphone != 0 && selected != ^uintptr(0) && selected < uintptr(len(p.microphoneIDs)) {
		cfg.AudioDeviceID = p.microphoneIDs[selected]
	}
	changedAudioDevice := cfg.AudioDeviceID != p.app.cfg.AudioDeviceID
	if err = saveConfig(cfg); err != nil {
		return err
	}
	p.app.cfg = cfg
	p.dirty = false
	for _, ctrl := range p.controls {
		if ctrl.id == uxSaveButton {
			refreshNativeWindow(ctrl.hwnd)
		}
	}
	if changedShortcut || changedAudioDevice {
		uxSetText(p.hint, "设置已保存。快捷键和录音设备将在重启 VoiceRewrite 后生效。")
	} else {
		uxSetText(p.hint, "✓ 已保存，API 地址和模型立即生效")
	}
	p.app.logger.Print("settings updated: connection configuration saved (endpoint/key values redacted)")
	return nil
}

func (p *uxSettings) check(kind, base, key, model string) {
	if p == nil || p.window == 0 {
		return
	}
	idx := 0
	if kind == "LLM" {
		idx = 1
	}
	p.probeMu.Lock()
	p.probeSerial[idx]++
	serial := p.probeSerial[idx]
	p.probeMu.Unlock()
	uxSetText(p.probeStatus[idx], "正在检测服务…")
	h := p.window
	go func() {
		result := uxProbeModel(context.Background(), kind, base, key, model)
		p.probeMu.Lock()
		if serial == p.probeSerial[idx] {
			p.probeResult[idx] = result
		}
		p.probeMu.Unlock()
		procPostMessage.Call(h, uxWMProbeDone, uintptr(idx), uintptr(serial))
	}()
}

func uxCreateControl(parent uintptr, class, text string, style uintptr, x, y, w, h int32, id uintptr, font uintptr) uintptr {
	cn := uxPtr(class)
	val := uxPtr(text)
	handle, _, _ := procCreateWindow.Call(0, uintptr(unsafe.Pointer(cn)), uintptr(unsafe.Pointer(val)), style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, id, 0, 0)
	runtime.KeepAlive(cn)
	runtime.KeepAlive(val)
	if font != 0 {
		setFont(handle, font)
	}
	return handle
}

// Center a settings window inside the monitor work area; the recording
// pill remains bottom-centered, but a full settings window should not be.
func uxCenteredWindow(w, h int32) (int32, int32) {
	pt := currentCursorPoint()
	packed := uint64(uint32(pt.X)) | uint64(uint32(pt.Y))<<32
	mon, _, _ := procMonitorFromPointHandy.Call(uintptr(packed), MONITOR_DEFAULTTONEAREST)
	if mon != 0 {
		info := handyMonitorInfo{Size: uint32(unsafe.Sizeof(handyMonitorInfo{}))}
		if ok, _, _ := procGetMonitorInfoHandy.Call(mon, uintptr(unsafe.Pointer(&info))); ok != 0 {
			area := info.Work
			x := area.Left + (area.Right-area.Left-w)/2
			y := area.Top + (area.Bottom-area.Top-h)/2
			if x < area.Left {
				x = area.Left
			}
			if y < area.Top {
				y = area.Top
			}
			return x, y
		}
	}
	width, _, _ := procGetSystemMetrics.Call(0)
	height, _, _ := procGetSystemMetrics.Call(1)
	return (int32(width) - w) / 2, (int32(height) - h) / 2
}

func uxWorkAreaAtCursor() desktopWorkArea { return uxWorkAreaForPoint(currentCursorPoint()) }

func showNativeSettings(app *appState) error {
	restoreDPI := handyAwareWindowThread()
	defer restoreDPI()
	if uxActiveSettings != nil && uxActiveSettings.window != 0 {
		procShowWindow.Call(uxActiveSettings.window, SW_SHOW)
		procSetForegroundWindow.Call(uxActiveSettings.window)
		return nil
	}
	className := uxPtr("VoiceRewrite.Settings.v20")
	title := uxPtr("VoiceRewrite · 偏好设置")
	module, _, _ := procGetModuleHandle.Call(0)
	klass := trayWindowClass{Size: uint32(unsafe.Sizeof(trayWindowClass{})), WndProc: uxWindowCallback, Instance: module, ClassName: className}
	atom, _, regErr := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&klass)))
	if atom == 0 && regErr != syscall.Errno(1410) {
		return fmt.Errorf("无法注册设置窗口: %w", regErr)
	}
	// 96-DPI logical layout; all children use the same scale as the parent.
	scale := handyMonitorScale()
	if scale < 1 {
		scale = 1
	}
	sc := func(v int32) uintptr { return uintptr(int32(float64(v)*scale + 0.5)) }
	work := uxWorkAreaAtCursor()
	winW, winH := desktopFitSize(int32(sc(760)), int32(sc(690)), work, 12)
	x, y := uxCenteredWindow(winW, winH)
	h, _, createErr := procCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)),
		uxWSCaption|uxWSSysMenu|uxWSMinimizeBox|uxWSThickFrame|WS_VSCROLL, uintptr(x), uintptr(y), uintptr(winW), uintptr(winH), 0, 0, module, 0)
	if h == 0 {
		return fmt.Errorf("创建设置窗口失败: %w", createErr)
	}
	p := &uxSettings{window: h, app: app, scale: scale, cardStatics: make(map[uintptr]bool), palette: systemPalette(), initializing: true}
	if os.Getenv("VOICEREWRITE_VISUAL_PREVIEW") != "" {
		switch os.Getenv("VOICEREWRITE_VISUAL_TAB") {
		case "1":
			p.activeTab = 1
		case "2":
			p.activeTab = 2
		}
	}
	uxActiveSettings = p
	p.background, _, _ = procCreateSolidBrush.Call(p.palette.color(p.palette.background))
	p.cardBrush, _, _ = procCreateSolidBrush.Call(p.palette.color(p.palette.surface))
	p.editBrush, _, _ = procCreateSolidBrush.Call(p.palette.color(p.palette.field))
	applyWindowPalette(h, p.palette)
	fontName := uxPtr("Microsoft YaHei UI")
	p.font, _, _ = procCreateFont.Call(sc(-16), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	p.titleFont, _, _ = procCreateFont.Call(sc(-26), 0, 0, 0, 650, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	p.sectionFont, _, _ = procCreateFont.Call(sc(-20), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	p.mutedFont, _, _ = procCreateFont.Call(sc(-14), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	// Three independent preferences pages avoid the former 700px wall of
	// server fields. All controls remain real HWNDs and retain their values
	// while their page is hidden.
	tabGroup := -1
	makeOne := func(cls, t string, style uintptr, x, y, w, hh int32, id uintptr, head bool) uintptr {
		useFont := p.font
		if head {
			useFont = p.sectionFont
		}
		role, row := uxRoleOther, 0
		if tabGroup >= 0 {
			switch {
			case strings.HasPrefix(t, "●") || strings.HasPrefix(t, "修改快捷键"):
				role = uxRoleStatus
			case cls == "EDIT" || cls == "COMBOBOX":
				role = uxRoleField
			case id == uxProbeASRButton || id == uxProbeLLMButton:
				role = uxRoleProbe
			case y < 208:
				role = uxRolePageTitle
			case y < 260:
				role = uxRolePageDescription
			default:
				role = uxRoleLabel
			}
			if role == uxRoleLabel || role == uxRoleField {
				if y >= 410 {
					row = 2
				} else if y >= 345 {
					row = 1
				}
			}
		}
		inset := cls == "EDIT"
		if inset {
			p.fieldFrames = append(p.fieldFrames, uxControl{x: x, y: y, w: w, h: hh, tab: tabGroup, role: uxRoleField, row: row})
		}
		control := uxCreateControl(h, cls, t, style, int32(sc(x)), int32(sc(y)), int32(sc(w)), int32(sc(hh)), id, useFont)
		if control == 0 {
			return 0
		}
		if inset {
			procSendMessage.Call(control, uxEMSetMargins, 3, uintptr(7|(7<<16)))
		}
		if role == uxRoleLabel || role == uxRoleStatus {
			p.cardStatics[control] = true
		}
		p.controls = append(p.controls, uxControl{hwnd: control, x: x, y: y, w: w, h: hh, id: id, tab: tabGroup, role: role, row: row, inset: inset})
		return control
	}
	stat := uintptr(uxWSChild | uxWSVisible)
	edit := uintptr(uxWSChild | uxWSVisible | uxWSTabStop | uxESAutoScroll)
	btn := uintptr(uxWSChild | uxWSVisible | uxWSTabStop | uxBSOwnerDraw)
	heading := makeOne("STATIC", "VoiceRewrite", stat, 80, 25, 410, 35, 0, false)
	setFont(heading, p.titleFont)
	subtitle := makeOne("STATIC", "说话，文字就位。", stat, 80, 65, 410, 23, 0, false)
	setFont(subtitle, p.mutedFont)
	badge := makeOne("STATIC", "专注写作 · 轻松表达", stat, 552, 41, 184, 22, 0, false)
	setFont(badge, p.mutedFont)
	p.tabs[0] = makeOne("BUTTON", "语音输入", btn, 32, 115, 154, 40, uxTabSpeech, false)
	p.tabs[1] = makeOne("BUTTON", "AI 写作", btn, 195, 115, 154, 40, uxTabWriting, false)
	p.tabs[2] = makeOne("BUTTON", "快捷键", btn, 358, 115, 154, 40, uxTabInput, false)

	tabGroup = 0
	makeOne("STATIC", "语音输入", stat, 46, 186, 540, 32, 0, true)
	desc := makeOne("STATIC", "选择识别服务，让每一句话自然成为文字。", stat, 46, 222, 650, 24, 0, false)
	setFont(desc, p.mutedFont)
	makeOne("STATIC", "服务地址", stat, 48, 297, 100, 24, 0, false)
	p.inputs[0] = makeOne("EDIT", app.cfg.ASRBaseURL, edit, 164, 287, 440, 38, 0, false)
	makeOne("BUTTON", "测试连接", btn, 612, 287, 112, 38, uxProbeASRButton, false)
	makeOne("STATIC", "模型 ID", stat, 48, 365, 100, 24, 0, false)
	p.inputs[1] = makeOne("EDIT", app.cfg.ASRModel, edit, 164, 355, 556, 38, 0, false)
	makeOne("STATIC", "API 密钥", stat, 48, 433, 100, 24, 0, false)
	p.inputs[2] = makeOne("EDIT", app.cfg.ASRAPIKey, edit|uxESPassword, 164, 423, 556, 38, 0, false)
	p.probeStatus[0] = makeOne("STATIC", "● 尚未检测 · 配置好服务后测试连接", stat, 48, 502, 660, 26, 0, false)
	setFont(p.probeStatus[0], p.mutedFont)

	tabGroup = 1
	makeOne("STATIC", "AI 写作", stat, 46, 186, 540, 32, 0, true)
	desc = makeOne("STATIC", "让你的文字更准确、更流畅，也更符合表达意图。", stat, 46, 222, 650, 24, 0, false)
	setFont(desc, p.mutedFont)
	makeOne("STATIC", "服务地址", stat, 48, 297, 100, 24, 0, false)
	p.inputs[3] = makeOne("EDIT", app.cfg.LLMBaseURL, edit, 164, 287, 440, 38, 0, false)
	makeOne("BUTTON", "测试连接", btn, 612, 287, 112, 38, uxProbeLLMButton, false)
	makeOne("STATIC", "模型 ID", stat, 48, 365, 100, 24, 0, false)
	p.inputs[4] = makeOne("EDIT", app.cfg.TextModel, edit, 164, 355, 556, 38, 0, false)
	makeOne("STATIC", "API 密钥", stat, 48, 433, 100, 24, 0, false)
	p.inputs[5] = makeOne("EDIT", app.cfg.LLMAPIKey, edit|uxESPassword, 164, 423, 556, 38, 0, false)
	p.probeStatus[1] = makeOne("STATIC", "● 尚未检测 · 支持改写、翻译与问答", stat, 48, 502, 660, 26, 0, false)
	setFont(p.probeStatus[1], p.mutedFont)

	tabGroup = 2
	makeOne("STATIC", "输入习惯", stat, 46, 186, 540, 32, 0, true)
	desc = makeOne("STATIC", "让快捷键和麦克风配合你的工作方式。", stat, 46, 222, 650, 24, 0, false)
	setFont(desc, p.mutedFont)
	makeOne("STATIC", "按住说话", stat, 48, 297, 110, 24, 0, false)
	p.shortcut = makeOne("EDIT", app.cfg.DictationHotkey, edit, 164, 287, 556, 38, uxShortcutInput, false)
	makeOne("STATIC", "麦克风", stat, 48, 365, 110, 24, 0, false)
	p.microphone = makeOne("COMBOBOX", "", uxWSChild|uxWSVisible|uxWSTabStop|uxCBSDropDownList|WS_VSCROLL, 164, 355, 556, 200, uxMicrophoneInput, false)
	p.populateMicrophones(app.cfg.AudioDeviceID)
	makeOne("STATIC", "修改快捷键后，重启 VoiceRewrite 即可生效。", stat, 48, 502, 660, 26, 0, false)

	tabGroup = -1
	makeOne("BUTTON", "高级设置", btn, 32, 578, 130, 36, uxAdvancedButton, false)
	makeOne("BUTTON", "关闭", btn, 458, 578, 94, 36, uxCloseButton, false)
	makeOne("BUTTON", "保存设置", btn, 572, 578, 156, 36, uxSaveButton, false)
	p.hint = makeOne("STATIC", "服务由你选择 · API 密钥已加密保护", stat, 34, 635, 640, 18, 0, false)
	setFont(p.hint, p.mutedFont)
	p.initializing = false
	// CI uses this preview-only knob to render the same dirty state reached
	// through real EN_CHANGE/CBN_SELCHANGE notifications, without a provider.
	if os.Getenv("VOICEREWRITE_VISUAL_PREVIEW")!="" && os.Getenv("VOICEREWRITE_VISUAL_DIRTY")=="1" {
		p.markDirty()
	}
	p.layout()
	// Windows 11 DWM-rounded frame and non-activating status pill are separate.
	rounding := uint32(2)
	procDwmSetWindowAttribute.Call(h, DWMWA_WINDOW_CORNER_PREFERENCE, uintptr(unsafe.Pointer(&rounding)), 4)
	procShowWindow.Call(h, SW_SHOW)
	procSetForegroundWindow.Call(h)
	return nil
}
func closeNativeSettings() {
	if uxActiveSettings != nil && uxActiveSettings.window != 0 {
		procDestroyWindow.Call(uxActiveSettings.window)
	}
}

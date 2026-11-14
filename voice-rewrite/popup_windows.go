//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

// The result card is an ordinary HWND with a complete Win32 message pump.
// DWM supplies Windows 11 rounded corners; native controls keep it accessible.
const (
	WM_COMMAND                     = 0x0111
	WM_CLOSE                       = 0x0010
	WM_NCDESTROY                   = 0x0082
	WM_SETFONT                     = 0x0030
	WM_KEYDOWN                     = 0x0100
	WM_ERASEBKGND                  = 0x0014
	WM_CTLCOLOREDIT                = 0x0133
	WM_CTLCOLORSTATIC              = 0x0138
	WM_DRAWITEM                    = 0x002B
	VK_ESCAPE                      = 0x1b
	WS_OVERLAPPED                  = 0
	WS_CAPTION                     = 0x00c00000
	WS_SYSMENU                     = 0x00080000
	WS_CHILD                       = 0x40000000
	WS_VISIBLE                     = 0x10000000
	WS_BORDER                      = 0x00800000
	WS_VSCROLL                     = 0x00200000
	WS_EX_TOPMOST                  = 0x00000008
	ES_MULTILINE                   = 0x0004
	ES_AUTOVSCROLL                 = 0x0040
	ES_READONLY                    = 0x0800
	EM_LIMITTEXT                   = 0x00C5
	BS_OWNERDRAW                   = 0x0000000B
	SW_SHOWNOACTIVATE              = 4
	SWP_NOACTIVATE                 = 0x0010
	HWND_TOPMOST                   = ^uintptr(0)
	BUTTON_COPY                    = 431
	BUTTON_CLOSE                   = 432
	BUTTON_APPLY                   = 433
	GWL_STYLE                      = ^uintptr(15)
	WS_DISABLED                    = 0x08000000
	GA_ROOT                        = 2
	SM_XVIRTUALSCREEN              = 76
	SM_YVIRTUALSCREEN              = 77
	SM_CXVIRTUALSCREEN             = 78
	SM_CYVIRTUALSCREEN             = 79
	DT_CENTER                      = 1
	DT_VCENTER                     = 4
	DT_SINGLELINE                  = 0x20
	TRANSPARENT                    = 1
	ODS_SELECTED                   = 1
	DWMWA_WINDOW_CORNER_PREFERENCE = 33
	DWMWA_BORDER_COLOR             = 34
	DWMWA_CAPTION_COLOR            = 35
	DWMWA_TEXT_COLOR               = 36
)

type popupMessage struct {
	title, body string
	anchor      trayPoint
	app         *appState
	selection   *selectedText
	allowApply  bool
	generation  uint64
}
type answerPopup struct {
	window, edit, note uintptr
	fonts              []uintptr
	bgBrush            uintptr
	body               string
	app                *appState
	selection          *selectedText
	allowApply         bool
	applyResult        chan error
	generation         uint64
	canceled           atomic.Bool
}

var activeAnswer *answerPopup
var (
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procGetGUIThreadInfo         = user32.NewProc("GetGUIThreadInfo")
	procGetClassName             = user32.NewProc("GetClassNameW")
	procGetWindowLongPtr         = user32.NewProc("GetWindowLongPtrW")
	procGetAncestor              = user32.NewProc("GetAncestor")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procSetWindowText            = user32.NewProc("SetWindowTextW")
	procSendMessage              = user32.NewProc("SendMessageW")
	procCreateFont               = gdi32.NewProc("CreateFontW")
	procGetClientRect            = user32.NewProc("GetClientRect")
	procSetTextColor             = gdi32.NewProc("SetTextColor")
	procSetBkColor               = gdi32.NewProc("SetBkColor")
	procSetBkMode                = gdi32.NewProc("SetBkMode")
	procCreateSolidBrush         = gdi32.NewProc("CreateSolidBrush")
	procCreatePen                = gdi32.NewProc("CreatePen")
	procFillRect                 = user32.NewProc("FillRect")
	procRoundRect                = gdi32.NewProc("RoundRect")
	procSelectObject             = gdi32.NewProc("SelectObject")
	procDrawText                 = user32.NewProc("DrawTextW")
	procIsChild                  = user32.NewProc("IsChild")
	procGetWindowTextLength      = user32.NewProc("GetWindowTextLengthW")
	dwmapi                       = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttribute    = dwmapi.NewProc("DwmSetWindowAttribute")
)

type guiThreadInfo struct {
	Size, Flags                                 uint32
	Active, Focus, Capture, MenuOwner, MoveSize uintptr
	Caret                                       rect32
}
type rect32 struct{ Left, Top, Right, Bottom int32 }
type drawItemStruct struct {
	CtlType, CtlID, ItemID, ItemAction, ItemState uint32
	HwndItem, Hdc                                 uintptr
	Rect                                          rect32
	ItemData                                      uintptr
}

func foregroundFocusedClass(target uintptr) string {
	if target == 0 || !isSameForegroundWindow(target) {
		return ""
	}
	tid, _, _ := procGetWindowThreadProcessId.Call(target, 0)
	if tid == 0 {
		return ""
	}
	info := guiThreadInfo{Size: uint32(unsafe.Sizeof(guiThreadInfo{}))}
	ok, _, _ := procGetGUIThreadInfo.Call(tid, uintptr(unsafe.Pointer(&info)))
	if ok == 0 || info.Focus == 0 {
		return ""
	}
	root, _, _ := procGetAncestor.Call(info.Focus, GA_ROOT)
	if root != target {
		return ""
	}
	return windowClassName(info.Focus)
}
func windowClassName(hwnd uintptr) string {
	if hwnd == 0 {
		return ""
	}
	chars := make([]uint16, 128)
	n, _, _ := procGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&chars[0])), uintptr(len(chars)))
	if n == 0 {
		return ""
	}
	return strings.ToLower(syscall.UTF16ToString(chars[:n]))
}
func isSafelyEditableControl(target uintptr) bool {
	focusClass := foregroundFocusedClass(target)
	if focusClass == "" {
		return false
	}
	if focusClass != "edit" && !strings.HasPrefix(focusClass, "richedit") && !strings.HasPrefix(focusClass, "windowsforms10.edit") {
		return false
	}
	tid, _, _ := procGetWindowThreadProcessId.Call(target, 0)
	info := guiThreadInfo{Size: uint32(unsafe.Sizeof(guiThreadInfo{}))}
	if ok, _, _ := procGetGUIThreadInfo.Call(tid, uintptr(unsafe.Pointer(&info))); ok == 0 || info.Focus == 0 {
		return false
	}
	style, _, _ := procGetWindowLongPtr.Call(info.Focus, GWL_STYLE)
	return style&ES_READONLY == 0 && style&WS_DISABLED == 0
}

// Chrome/Edge/Electron and Firefox expose a renderer HWND, not an EDIT HWND.
// A positive value only authorizes a guarded PASTE PROBE for explicit edits,
// never an unconditional replacement or general-purpose write permission.
func isBrowserEditorSurface(target uintptr) bool {
	focus := foregroundFocusedClass(target)
	root := windowClassName(target)
	if focus == "" {
		return false
	}
	return (strings.Contains(root, "chrome_widgetwin") || strings.Contains(root, "mozillawindowclass")) && (strings.Contains(focus, "chrome") || strings.Contains(focus, "mozilla") || strings.Contains(focus, "renderwidget"))
}
func currentCursorPoint() trayPoint {
	var p trayPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}
func colorRGB(r, g, b uint32) uintptr { return uintptr(r | (g << 8) | (b << 16)) }

func (p *answerPopup) setNote(s string) {
	if p == nil || p.note == 0 {
		return
	}
	label, _ := syscall.UTF16PtrFromString(s)
	procSetWindowText.Call(p.note, uintptr(unsafe.Pointer(label)))
	runtime.KeepAlive(label)
}
func (p *answerPopup) apply() {
	if p == nil || p.app == nil || p.selection == nil || !p.allowApply || p.canceled.Load() || !p.app.epoch.valid(p.generation) {
		return
	}
	if !p.app.busy.CompareAndSwap(false, true) {
		p.setNote("任务尚未结束，请稍后点击应用")
		return
	}
	app := p.app
	sel := *p.selection
	body := p.body
	hwnd := p.window
	p.applyResult = make(chan error, 1)
	p.setNote("正在核验原选区并应用…")
	if valid, _, _ := procIsWindow.Call(sel.TargetWindow); valid == 0 {
		p.applyResult <- errors.New("原编辑窗口已关闭")
		procPostMessage.Call(app.Window, WM_APPLY_RESULT, 0, 0)
		app.busy.Store(false)
		return
	}
	procSetForegroundWindow.Call(sel.TargetWindow)
	go func() {
		defer app.busy.Store(false)
		time.Sleep(90 * time.Millisecond)
		err := replaceSelectionWithGuard(app.Window, sel, body, app.cfg, !isSafelyEditableControl(sel.TargetWindow), func() bool { return !p.canceled.Load() && app.epoch.valid(p.generation) })
		if err != nil {
			app.logger.Printf("manual apply failed: %v", err)
		} else {
			app.logger.Print("manual apply succeeded")
		}
		// The HWND and channel belong to this particular visible card. Avoid
		// changing another card if the user closed it while the worker was running.
		if ok, _, _ := procIsWindow.Call(hwnd); ok != 0 {
			p.applyResult <- err
			procPostMessage.Call(app.Window, WM_APPLY_RESULT, uintptr(hwnd), 0)
		}
	}()
}
func popupWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch uint32(msg) {
	case WM_ERASEBKGND:
		if activeAnswer != nil && activeAnswer.window == hwnd && activeAnswer.bgBrush != 0 {
			var r rect32
			procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
			procFillRect.Call(wParam, uintptr(unsafe.Pointer(&r)), activeAnswer.bgBrush)
			return 1
		}
	case WM_CTLCOLORSTATIC, WM_CTLCOLOREDIT:
		if activeAnswer != nil && activeAnswer.window == hwnd {
			procSetBkMode.Call(wParam, TRANSPARENT)
			if uint32(msg) == WM_CTLCOLORSTATIC && activeAnswer.note == lParam {
				procSetTextColor.Call(wParam, colorRGB(119, 128, 144))
			} else {
				procSetTextColor.Call(wParam, colorRGB(37, 45, 60))
			}
			procSetBkColor.Call(wParam, colorRGB(255, 255, 255))
			return activeAnswer.bgBrush
		}
	case WM_DRAWITEM:
		if lParam != 0 {
			item := (*drawItemStruct)(unsafe.Pointer(lParam))
			if drawCardButton(item) {
				return 1
			}
		}
	case WM_COMMAND:
		if activeAnswer != nil && activeAnswer.window == hwnd {
			switch uint16(wParam & 0xffff) {
			case BUTTON_COPY:
				if err := setClipboardText(hwnd, activeAnswer.body); err == nil {
					activeAnswer.setNote("已复制结果到剪贴板")
				} else {
					activeAnswer.setNote("复制失败，请尝试选中正文复制")
				}
				return 0
			case BUTTON_CLOSE:
				procDestroyWindow.Call(hwnd)
				return 0
			case BUTTON_APPLY:
				activeAnswer.apply()
				return 0
			}
		}
	case WM_CLOSE:
		procDestroyWindow.Call(hwnd)
		return 0
	case WM_NCDESTROY:
		if activeAnswer != nil && activeAnswer.window == hwnd {
			activeAnswer.canceled.Store(true)
			for _, f := range activeAnswer.fonts {
				if f != 0 {
					procDeleteObject.Call(f)
				}
			}
			if activeAnswer.bgBrush != 0 {
				procDeleteObject.Call(activeAnswer.bgBrush)
			}
			activeAnswer = nil
		}
	}
	result, _, _ := procDefWindowProc.Call(hwnd, msg, wParam, lParam)
	return result
}

func drawCardButton(item *drawItemStruct) bool {
	if item == nil || (item.CtlID != BUTTON_COPY && item.CtlID != BUTTON_APPLY && item.CtlID != BUTTON_CLOSE) {
		return false
	}
	primary := item.CtlID == BUTTON_APPLY
	var fill, border, ink uintptr
	if primary {
		fill = colorRGB(190, 75, 123)
		border = fill
		ink = colorRGB(255, 255, 255)
	} else {
		fill = colorRGB(247, 246, 250)
		border = colorRGB(232, 228, 236)
		ink = colorRGB(45, 56, 75)
	}
	if item.ItemState&ODS_SELECTED != 0 {
		if primary {
			fill = colorRGB(155, 51, 99)
		} else {
			fill = colorRGB(227, 232, 241)
		}
	}
	b, _, _ := procCreateSolidBrush.Call(fill)
	pen, _, _ := procCreatePen.Call(0, 1, border)
	if b == 0 || pen == 0 {
		if b != 0 {
			procDeleteObject.Call(b)
		}
		if pen != 0 {
			procDeleteObject.Call(pen)
		}
		return false
	}
	oldB, _, _ := procSelectObject.Call(item.Hdc, b)
	oldP, _, _ := procSelectObject.Call(item.Hdc, pen)
	r := item.Rect
	procRoundRect.Call(item.Hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), 16, 16)
	procSelectObject.Call(item.Hdc, oldB)
	procSelectObject.Call(item.Hdc, oldP)
	procDeleteObject.Call(b)
	procDeleteObject.Call(pen)
	procSetBkMode.Call(item.Hdc, TRANSPARENT)
	procSetTextColor.Call(item.Hdc, ink)
	text := "关闭"
	if item.CtlID == BUTTON_COPY {
		text = "复制结果"
	}
	if item.CtlID == BUTTON_APPLY {
		text = "应用到原文"
	}
	p, _ := syscall.UTF16PtrFromString(text)
	if activeAnswer != nil && len(activeAnswer.fonts) > 2 {
		old, _, _ := procSelectObject.Call(item.Hdc, activeAnswer.fonts[2])
		defer procSelectObject.Call(item.Hdc, old)
	}
	procDrawText.Call(item.Hdc, uintptr(unsafe.Pointer(p)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	runtime.KeepAlive(p)
	return true
}

var answerWndProcCallback = syscall.NewCallback(popupWndProc)

func shouldClosePopupForKey(msg winMSG) bool {
	if activeAnswer == nil || activeAnswer.window == 0 || msg.Message != WM_KEYDOWN || msg.WParam != VK_ESCAPE {
		return false
	}
	if msg.Hwnd == activeAnswer.window {
		return true
	}
	child, _, _ := procIsChild.Call(activeAnswer.window, msg.Hwnd)
	return child != 0
}
func closeActiveAnswer() {
	if activeAnswer != nil && activeAnswer.window != 0 {
		procDestroyWindow.Call(activeAnswer.window)
	}
}
func makeChild(className, text string, style uintptr, x, y, w, h int32, parent, id uintptr) (uintptr, error) {
	class, _ := syscall.UTF16PtrFromString(className)
	caption, _ := syscall.UTF16PtrFromString(text)
	handle, _, err := procCreateWindow.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(caption)), style, uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, id, 0, 0)
	runtime.KeepAlive(class)
	runtime.KeepAlive(caption)
	if handle == 0 {
		return 0, fmt.Errorf("CreateWindowExW child %s: %w", className, err)
	}
	return handle, nil
}
func popupFont(height, weight int32) uintptr {
	name, _ := syscall.UTF16PtrFromString("Segoe UI")
	font, _, _ := procCreateFont.Call(uintptr(height), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(name)))
	runtime.KeepAlive(name)
	return font
}
func setFont(hwnd, font uintptr) {
	if hwnd != 0 && font != 0 {
		procSendMessage.Call(hwnd, WM_SETFONT, font, 1)
	}
}
func showAnswerPopup(msg popupMessage) error {
	closeActiveAnswer()
	if strings.TrimSpace(msg.body) == "" {
		return errors.New("AI 未返回可显示的文字")
	}
	// Never truncate a model response: users must be able to review and copy
	// the full answer, including large translations and generated documents.
	n := utf8.RuneCountInString(msg.body)
	width, height := v14PopupSize(n)
	x, y := msg.anchor.X+18, msg.anchor.Y+22
	left, _, _ := procGetSystemMetrics.Call(SM_XVIRTUALSCREEN)
	top, _, _ := procGetSystemMetrics.Call(SM_YVIRTUALSCREEN)
	ww, _, _ := procGetSystemMetrics.Call(SM_CXVIRTUALSCREEN)
	hh, _, _ := procGetSystemMetrics.Call(SM_CYVIRTUALSCREEN)
	vx, vy := int32(left), int32(top)
	if x > vx+int32(ww)-width {
		x = vx + int32(ww) - width
	}
	if y > vy+int32(hh)-height {
		y = vy + int32(hh) - height
	}
	if x < vx {
		x = vx
	}
	if y < vy {
		y = vy
	}
	class, _ := syscall.UTF16PtrFromString("VoiceRewrite.ResultCard.v20")
	instance, _, _ := procGetModuleHandle.Call(0)
	wc := trayWindowClass{Size: uint32(unsafe.Sizeof(trayWindowClass{})), WndProc: answerWndProcCallback, Instance: instance, ClassName: class}
	atom, _, regErr := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	runtime.KeepAlive(wc)
	if atom == 0 && regErr != syscall.Errno(1410) {
		return fmt.Errorf("register card class: %w", regErr)
	}
	caption, _ := syscall.UTF16PtrFromString("VoiceRewrite")
	hwnd, _, err := procCreateWindow.Call(WS_EX_TOPMOST|WS_EX_TOOLWINDOW, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(caption)),
		WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0, instance, 0)
	runtime.KeepAlive(class)
	runtime.KeepAlive(caption)
	if hwnd == 0 {
		return fmt.Errorf("create result card: %w", err)
	}
	popup := &answerPopup{window: hwnd, body: msg.body, app: msg.app, selection: msg.selection, allowApply: msg.allowApply, generation: msg.generation}
	activeAnswer = popup
	popup.bgBrush, _, _ = procCreateSolidBrush.Call(colorRGB(255, 255, 255))
	var bounds rect32
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&bounds)))
	cw, ch := bounds.Right-bounds.Left, bounds.Bottom-bounds.Top
	title, err := makeChild("STATIC", msg.title, WS_CHILD|WS_VISIBLE, 22, 16, cw-44, 28, hwnd, 0)
	if err != nil {
		closeActiveAnswer()
		return err
	}
	summary := "基于选中文字 · 未修改原文"
	if msg.selection == nil {
		summary = "原输入位置不可用 · 完整结果保留在此，可复制"
	} else if !msg.allowApply {
		summary = "基于选中文字 · AI 回答"
	}
	note, err := makeChild("STATIC", summary, WS_CHILD|WS_VISIBLE, 23, 46, cw-46, 21, hwnd, 0)
	if err != nil {
		closeActiveAnswer()
		return err
	}
	popup.note = note
	bodyStyle := uintptr(WS_CHILD | WS_VISIBLE | ES_MULTILINE | ES_AUTOVSCROLL | ES_READONLY)
	if n > 400 {
		bodyStyle |= WS_VSCROLL
	}
	editorText := strings.ReplaceAll(strings.ReplaceAll(msg.body, "\r\n", "\n"), "\n", "\r\n")
	editor, err := makeChild("EDIT", "", bodyStyle, 22, 77, cw-44, ch-132, hwnd, 0)
	if err != nil {
		closeActiveAnswer()
		return err
	}
	popup.edit = editor
	// Multiline EDIT controls default to a 32K character limit. Raise it
	// before setting the text, otherwise the UI silently drops the tail.
	procSendMessage.Call(editor, EM_LIMITTEXT, uintptr(len(utf16.Encode([]rune(editorText)))+1), 0)
	uxSetText(editor, editorText)
	if size, _, _ := procGetWindowTextLength.Call(editor); size == 0 {
		closeActiveAnswer()
		return errors.New("卡片正文控件未收到模型结果")
	}
	bottom := ch - 43
	if msg.allowApply {
		if _, err = makeChild("BUTTON", "应用到原文", WS_CHILD|WS_VISIBLE|BS_OWNERDRAW, cw-342, bottom, 122, 34, hwnd, BUTTON_APPLY); err != nil {
			closeActiveAnswer()
			return err
		}
	}
	if _, err = makeChild("BUTTON", "复制结果", WS_CHILD|WS_VISIBLE|BS_OWNERDRAW, cw-209, bottom, 102, 34, hwnd, BUTTON_COPY); err != nil {
		closeActiveAnswer()
		return err
	}
	if _, err = makeChild("BUTTON", "关闭", WS_CHILD|WS_VISIBLE|BS_OWNERDRAW, cw-96, bottom, 74, 34, hwnd, BUTTON_CLOSE); err != nil {
		closeActiveAnswer()
		return err
	}
	popup.fonts = []uintptr{popupFont(-21, 600), popupFont(-18, 400), popupFont(-15, 550), popupFont(-13, 400)}
	setFont(title, popup.fonts[0])
	setFont(editor, popup.fonts[1])
	setFont(note, popup.fonts[3])
	rounded := uint32(2)
	procDwmSetWindowAttribute.Call(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, uintptr(unsafe.Pointer(&rounded)), unsafe.Sizeof(rounded))
	captionColor := uint32(colorRGB(250, 249, 251))
	procDwmSetWindowAttribute.Call(hwnd, DWMWA_CAPTION_COLOR, uintptr(unsafe.Pointer(&captionColor)), unsafe.Sizeof(captionColor))
	captionText := uint32(colorRGB(45, 56, 73))
	procDwmSetWindowAttribute.Call(hwnd, DWMWA_TEXT_COLOR, uintptr(unsafe.Pointer(&captionText)), unsafe.Sizeof(captionText))
	borderColor := uint32(colorRGB(228, 232, 238))
	procDwmSetWindowAttribute.Call(hwnd, DWMWA_BORDER_COLOR, uintptr(unsafe.Pointer(&borderColor)), unsafe.Sizeof(borderColor))
	procShowWindow.Call(hwnd, SW_SHOWNOACTIVATE)
	procSetWindowPos.Call(hwnd, HWND_TOPMOST, uintptr(x), uintptr(y), uintptr(width), uintptr(height), SWP_NOACTIVATE)
	return nil
}

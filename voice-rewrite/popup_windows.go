//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf8"
	"unsafe"
)

// Native, non-modal answer window. Its HWND and child controls are created and
// destroyed only on the UI thread. The mouse-hook and HTTP workers never touch
// its Win32 controls.
const (
	WM_COMMAND         = 0x0111
	WM_CLOSE           = 0x0010
	WM_NCDESTROY       = 0x0082
	WM_SETFONT         = 0x0030
	WS_OVERLAPPED      = 0
	WM_KEYDOWN         = 0x0100
	VK_ESCAPE          = 0x1b
	WS_CAPTION         = 0x00c00000
	WS_SYSMENU         = 0x00080000
	WS_CHILD           = 0x40000000
	WS_VISIBLE         = 0x10000000
	WS_BORDER          = 0x00800000
	WS_VSCROLL         = 0x00200000
	WS_EX_TOPMOST      = 0x00000008
	ES_MULTILINE       = 0x0004
	ES_AUTOVSCROLL     = 0x0040
	ES_READONLY        = 0x0800
	BS_PUSHBUTTON      = 0
	SW_SHOWNOACTIVATE  = 4
	SWP_NOACTIVATE     = 0x0010
	HWND_TOPMOST       = ^uintptr(0)
	BUTTON_COPY        = 431
	BUTTON_CLOSE       = 432
	GWL_STYLE          = ^uintptr(15) // -16 as uintptr
	WS_DISABLED        = 0x08000000
	GA_ROOT            = 2
	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79
)

type popupMessage struct {
	title, body string
	anchor      trayPoint
}
type answerPopup struct {
	window uintptr
	edit   uintptr
	font   uintptr
	body   string
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
	procIsChild                  = user32.NewProc("IsChild")
	procGetWindowTextLength      = user32.NewProc("GetWindowTextLengthW")
)

type guiThreadInfo struct {
	Size      uint32
	Flags     uint32
	Active    uintptr
	Focus     uintptr
	Capture   uintptr
	MenuOwner uintptr
	MoveSize  uintptr
	Caret     rect32
}
type rect32 struct{ Left, Top, Right, Bottom int32 }

// This narrow, deliberately conservative check considers native Win32 EDIT and
// RichEdit controls only. Unknown web and UI frameworks get a safe preview.
func isSafelyEditableControl(target uintptr) bool {
	if target == 0 || !isSameForegroundWindow(target) {
		return false
	}
	tid, _, _ := procGetWindowThreadProcessId.Call(target, 0)
	if tid == 0 {
		return false
	}
	info := guiThreadInfo{Size: uint32(unsafe.Sizeof(guiThreadInfo{}))}
	ok, _, _ := procGetGUIThreadInfo.Call(tid, uintptr(unsafe.Pointer(&info)))
	if ok == 0 || info.Focus == 0 {
		return false
	}
	root, _, _ := procGetAncestor.Call(info.Focus, GA_ROOT)
	if root != target {
		return false
	}
	chars := make([]uint16, 128)
	n, _, _ := procGetClassName.Call(info.Focus, uintptr(unsafe.Pointer(&chars[0])), uintptr(len(chars)))
	if n == 0 {
		return false
	}
	name := strings.ToLower(syscall.UTF16ToString(chars[:n]))
	if name != "edit" && !strings.HasPrefix(name, "richedit") && !strings.HasPrefix(name, "windowsforms10.edit") {
		return false
	}
	style, _, _ := procGetWindowLongPtr.Call(info.Focus, GWL_STYLE)
	return style&ES_READONLY == 0 && style&WS_DISABLED == 0
}

func currentCursorPoint() trayPoint {
	var p trayPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}

func popupWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch uint32(msg) {
	case WM_COMMAND:
		if activeAnswer != nil && activeAnswer.window == hwnd {
			switch uint16(wParam & 0xffff) {
			case BUTTON_COPY:
				// Copy is non-destructive; close after successful copying.
				if err := setClipboardText(hwnd, activeAnswer.body); err == nil {
					procDestroyWindow.Call(hwnd)
				}
				return 0
			case BUTTON_CLOSE:
				procDestroyWindow.Call(hwnd)
				return 0
			}
		}
	case WM_CLOSE:
		procDestroyWindow.Call(hwnd)
		return 0
	case WM_NCDESTROY:
		if activeAnswer != nil && activeAnswer.window == hwnd {
			if activeAnswer.font != 0 {
				procDeleteObject.Call(activeAnswer.font)
			}
			activeAnswer = nil
		}
	}
	result, _, _ := procDefWindowProc.Call(hwnd, msg, wParam, lParam)
	return result
}

var answerWndProcCallback = syscall.NewCallback(popupWndProc)

// Escape is handled in the UI thread message pump because the keyboard
// target is usually the read-only EDIT child rather than the parent window.
func shouldClosePopupForKey(msg winMSG) bool {
	if activeAnswer == nil || activeAnswer.window == 0 || msg.Message != WM_KEYDOWN || msg.WParam != VK_ESCAPE {
		return false
	}
	if msg.Hwnd == activeAnswer.window {
		return true
	}
	isChild, _, _ := procIsChild.Call(activeAnswer.window, msg.Hwnd)
	return isChild != 0
}

func closeActiveAnswer() {
	if activeAnswer != nil && activeAnswer.window != 0 {
		procDestroyWindow.Call(activeAnswer.window)
	}
}
func makeChild(className, text string, style uintptr, x, y, w, h int32, parent, id uintptr) (uintptr, error) {
	class, _ := syscall.UTF16PtrFromString(className)
	caption, _ := syscall.UTF16PtrFromString(text)
	handle, _, err := procCreateWindow.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(caption)), style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, id, 0, 0)
	runtime.KeepAlive(class)
	runtime.KeepAlive(caption)
	if handle == 0 {
		return 0, fmt.Errorf("CreateWindowExW child %s: %w", className, err)
	}
	return handle, nil
}

func showAnswerPopup(msg popupMessage) error {
	closeActiveAnswer()
	if strings.TrimSpace(msg.body) == "" {
		return errors.New("AI 未返回可显示的文字")
	}
	if utf8.RuneCountInString(msg.body) > 5500 {
		runes := []rune(msg.body)
		msg.body = string(runes[:5500]) + "\n\n（显示已截断）"
	}
	// Compact cards for short translations; expand for long explanations.
	const width = 440
	height := int32(190)
	if utf8.RuneCountInString(msg.body) > 90 {
		height = 255
	}
	if utf8.RuneCountInString(msg.body) > 230 {
		height = 350
	}
	if utf8.RuneCountInString(msg.body) > 750 {
		height = 435
	}
	x, y := msg.anchor.X+16, msg.anchor.Y+20
	left, _, _ := procGetSystemMetrics.Call(SM_XVIRTUALSCREEN)
	top, _, _ := procGetSystemMetrics.Call(SM_YVIRTUALSCREEN)
	ww, _, _ := procGetSystemMetrics.Call(SM_CXVIRTUALSCREEN)
	hh, _, _ := procGetSystemMetrics.Call(SM_CYVIRTUALSCREEN)
	vx, vy := int32(left), int32(top)
	if vx+int32(ww)-width < x {
		x = vx + int32(ww) - width
	}
	if vy+int32(hh)-height < y {
		y = vy + int32(hh) - height
	}
	if x < vx {
		x = vx
	}
	if y < vy {
		y = vy
	}
	className, _ := syscall.UTF16PtrFromString("VoiceRewrite.AnswerPopup.v6")
	instance, _, _ := procGetModuleHandle.Call(0)
	klass := trayWindowClass{Size: uint32(unsafe.Sizeof(trayWindowClass{})), WndProc: answerWndProcCallback, Instance: instance, ClassName: className}
	atom, _, regErr := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&klass)))
	runtime.KeepAlive(klass)
	// Error 1410 means the window class is already registered, which is okay.
	if atom == 0 && regErr != syscall.Errno(1410) {
		return fmt.Errorf("register popup class: %w", regErr)
	}
	title, _ := syscall.UTF16PtrFromString("VoiceRewrite · " + msg.title)
	hwnd, _, createErr := procCreateWindow.Call(WS_EX_TOPMOST|WS_EX_TOOLWINDOW, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)),
		WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU, uintptr(x), uintptr(y), width, uintptr(height), 0, 0, instance, 0)
	runtime.KeepAlive(className)
	runtime.KeepAlive(title)
	if hwnd == 0 {
		return fmt.Errorf("create answer popup: %w", createErr)
	}
	popup := &answerPopup{window: hwnd, body: msg.body}
	activeAnswer = popup
	edit, err := makeChild("EDIT", msg.body, WS_CHILD|WS_VISIBLE|WS_BORDER|WS_VSCROLL|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY, 12, 12, width-24, height-105, hwnd, 0)
	if err != nil {
		closeActiveAnswer()
		return err
	}
	popup.edit = edit
	// ShowWindow requests actual painting; the main GetMessage loop must call
	// DispatchMessage so WM_PAINT/WM_COMMAND/WM_CLOSE reach their WndProcs.
	if textSize, _, _ := procGetWindowTextLength.Call(edit); textSize == 0 {
		closeActiveAnswer()
		return errors.New("弹窗文本控件未接收 AI 返回内容")
	}
	copyButton, err := makeChild("BUTTON", "复制并关闭", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, width-244, height-78, 118, 32, hwnd, BUTTON_COPY)
	if err != nil {
		closeActiveAnswer()
		return err
	}
	closeButton, err := makeChild("BUTTON", "关闭", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, width-118, height-78, 94, 32, hwnd, BUTTON_CLOSE)
	if err != nil {
		closeActiveAnswer()
		return err
	}
	fontName, _ := syscall.UTF16PtrFromString("Microsoft YaHei UI")
	font, _, _ := procCreateFont.Call(18, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	runtime.KeepAlive(fontName)
	if font != 0 {
		popup.font = font
		procSendMessage.Call(edit, WM_SETFONT, font, 1)
		procSendMessage.Call(copyButton, WM_SETFONT, font, 1)
		procSendMessage.Call(closeButton, WM_SETFONT, font, 1)
	}
	procShowWindow.Call(hwnd, SW_SHOWNOACTIVATE)
	procSetWindowPos.Call(hwnd, HWND_TOPMOST, uintptr(x), uintptr(y), width, uintptr(height), SWP_NOACTIVATE)
	return nil
}

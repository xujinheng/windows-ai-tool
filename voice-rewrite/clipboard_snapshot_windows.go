//go:build windows

package main

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

// clipboardSnapshot is an in-memory copy of the clipboard formats whose data
// is a movable HGLOBAL. It is deliberately conservative: GDI/owner-display
// handles and uncopyable formats make capture fail BEFORE Ctrl+C or Ctrl+V can
// destroy the user's clipboard. Clipboard contents are never logged or saved.
const (
	clipboardFormatMaxBytes = 16 << 20
	clipboardTotalMaxBytes  = 48 << 20
)

type savedClipboardFormat struct {
	id   uintptr
	data []byte
}

type clipboardSnapshot struct {
	mu       sync.Mutex
	formats  []savedClipboardFormat
	lastOwn  uintptr
	finished bool
}

var procEnumClipboardFormats = user32.NewProc("EnumClipboardFormats")

func clipboardHandleFormat(id uintptr) bool {
	switch id {
	case 2, 3, 9, 14, 0x80, 0x82, 0x83, 0x8e: // bitmap, palette, metafile, owner-display
		return true
	}
	return false
}

func snapshotClipboard(hwnd uintptr) (*clipboardSnapshot, error) {
	if err := openClipboard(hwnd); err != nil {
		return nil, err
	}
	defer procCloseClipboard.Call()
	snap := &clipboardSnapshot{}
	total := uintptr(0)
	previous := uintptr(0)
	for {
		id, _, _ := procEnumClipboardFormats.Call(previous)
		if id == 0 {
			break
		}
		previous = id
		if clipboardHandleFormat(id) {
			return nil, fmt.Errorf("剪贴板包含图像或系统对象（格式 %d），为避免丢失数据已取消自动操作", id)
		}
		h, _, _ := procGetClipboardData.Call(id)
		if h == 0 {
			return nil, fmt.Errorf("剪贴板格式 %d 无法复制，已取消自动操作", id)
		}
		size, _, _ := procGlobalSize.Call(h)
		if size == 0 || size > clipboardFormatMaxBytes || total+size > clipboardTotalMaxBytes {
			return nil, fmt.Errorf("剪贴板格式 %d 不是可安全复制的内存数据，已取消自动操作", id)
		}
		p, _, _ := procGlobalLock.Call(h)
		if p == 0 {
			return nil, fmt.Errorf("无法锁定剪贴板格式 %d，已取消自动操作", id)
		}
		buf := make([]byte, size)
		copy(buf, unsafe.Slice((*byte)(unsafe.Pointer(p)), int(size)))
		procGlobalUnlock.Call(h)
		snap.formats = append(snap.formats, savedClipboardFormat{id: id, data: buf})
		total += size
	}
	return snap, nil
}

func (s *clipboardSnapshot) markOwned(sequence uintptr) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.finished {
		s.lastOwn = sequence
	}
}

func (s *clipboardSnapshot) isFinished() bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finished
}

func (s *clipboardSnapshot) ownsCurrent() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.finished && s.lastOwn != 0 && clipboardSeq() == s.lastOwn
}

// abandon leaves an explicitly user-requested clipboard value untouched.
func (s *clipboardSnapshot) abandon() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.finished = true
	s.mu.Unlock()
}

// restoreIfOwned has a sequence fence: another application's clipboard update
// always wins. The check occurs while the clipboard is open, so a second
// process cannot race between our ownership check and EmptyClipboard.
func (s *clipboardSnapshot) restoreIfOwned(hwnd uintptr) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished || s.lastOwn == 0 {
		return nil
	}
	if err := openClipboard(hwnd); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	if clipboardSeq() != s.lastOwn {
		s.finished = true
		return nil
	}
	// Allocate every block before emptying the clipboard. Ownership of a block
	// transfers to Windows only once SetClipboardData succeeds.
	handles := make([]uintptr, len(s.formats))
	for i, f := range s.formats {
		h, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, uintptr(len(f.data)))
		if h == 0 {
			freeSnapshotHandles(handles)
			return errors.New("剪贴板恢复失败：内存分配失败，原内容尚未更改")
		}
		handles[i] = h
		p, _, _ := procGlobalLock.Call(h)
		if p == 0 {
			freeSnapshotHandles(handles)
			return errors.New("剪贴板恢复失败：无法锁定内存，原内容尚未更改")
		}
		copy(unsafe.Slice((*byte)(unsafe.Pointer(p)), len(f.data)), f.data)
		procGlobalUnlock.Call(h)
	}
	if ok, _, _ := procEmptyClipboard.Call(); ok == 0 {
		freeSnapshotHandles(handles)
		return errors.New("剪贴板恢复失败：无法清空临时内容")
	}
	s.finished = true
	var failed int
	for i, f := range s.formats {
		if out, _, _ := procSetClipboardData.Call(f.id, handles[i]); out == 0 {
			failed++
		} else {
			handles[i] = 0 // Windows owns this HGLOBAL now
		}
	}
	freeSnapshotHandles(handles)
	if failed > 0 {
		return fmt.Errorf("剪贴板恢复部分失败：%d 个格式未能写回", failed)
	}
	return nil
}

func freeSnapshotHandles(handles []uintptr) {
	for _, h := range handles {
		if h != 0 {
			procGlobalFree.Call(h)
		}
	}
}

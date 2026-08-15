//go:build windows

package main

import (
	"context"
	"errors"
	"syscall"
	"time"
	"unsafe"
)

const (
	cfUnicodeText = 13
	cfHDrop       = 15
	gmemMoveable  = 0x0002
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	shell32          = syscall.NewLazyDLL("shell32.dll")
	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	emptyClipboard   = user32.NewProc("EmptyClipboard")
	getClipboardData = user32.NewProc("GetClipboardData")
	setClipboardData = user32.NewProc("SetClipboardData")
	dragQueryFileW   = shell32.NewProc("DragQueryFileW")
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	globalAlloc      = kernel32.NewProc("GlobalAlloc")
	globalLock       = kernel32.NewProc("GlobalLock")
	globalUnlock     = kernel32.NewProc("GlobalUnlock")
	globalFree       = kernel32.NewProc("GlobalFree")
)

func readWindowsClipboardFiles(ctx context.Context) ([]uploadCandidate, error) {
	paths, err := readWindowsClipboardFilePaths(ctx)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errors.New("no file references in clipboard")
	}
	candidates := make([]uploadCandidate, 0, len(paths))
	for _, file := range paths {
		candidate, err := candidateFromFile(file)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func readWindowsClipboardFilePaths(ctx context.Context) ([]string, error) {
	if err := openWindowsClipboard(ctx); err != nil {
		return nil, err
	}
	defer closeClipboard.Call()

	hdrop, _, _ := getClipboardData.Call(uintptr(cfHDrop))
	if hdrop == 0 {
		return nil, errors.New("clipboard does not contain CF_HDROP")
	}

	count, _, _ := dragQueryFileW.Call(hdrop, 0xFFFFFFFF, 0, 0)
	if count == 0 {
		return nil, errors.New("clipboard file list is empty")
	}

	paths := make([]string, 0, count)
	for i := uintptr(0); i < count; i++ {
		length, _, _ := dragQueryFileW.Call(hdrop, i, 0, 0)
		if length == 0 {
			continue
		}
		buf := make([]uint16, length+1)
		dragQueryFileW.Call(hdrop, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		path := syscall.UTF16ToString(buf)
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func writeWindowsClipboardText(ctx context.Context, text string) error {
	if err := openWindowsClipboard(ctx); err != nil {
		return err
	}
	defer closeClipboard.Call()

	if ok, _, err := emptyClipboard.Call(); ok == 0 {
		if err != syscall.Errno(0) {
			return err
		}
		return errors.New("empty clipboard failed")
	}

	utf16Text, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}
	size := uintptr(len(utf16Text) * 2)
	hmem, _, err := globalAlloc.Call(uintptr(gmemMoveable), size)
	if hmem == 0 {
		if err != syscall.Errno(0) {
			return err
		}
		return errors.New("global alloc failed")
	}

	ptr, _, err := globalLock.Call(hmem)
	if ptr == 0 {
		globalFree.Call(hmem)
		if err != syscall.Errno(0) {
			return err
		}
		return errors.New("global lock failed")
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16Text)), utf16Text)
	globalUnlock.Call(hmem)

	if ok, _, err := setClipboardData.Call(uintptr(cfUnicodeText), hmem); ok == 0 {
		globalFree.Call(hmem)
		if err != syscall.Errno(0) {
			return err
		}
		return errors.New("set clipboard data failed")
	}
	return nil
}

func openWindowsClipboard(ctx context.Context) error {
	deadline := time.Now().Add(2 * time.Second)
	for {
		ok, _, err := openClipboard.Call(0)
		if ok != 0 {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			if err != syscall.Errno(0) {
				return err
			}
			return errors.New("open clipboard failed")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

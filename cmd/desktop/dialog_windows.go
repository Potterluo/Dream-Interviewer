//go:build windows

package main

// A native message box, so a startup failure is not silent.
//
// A windowsgui build has no console and therefore no stderr: without this, a
// crash looks identical to "I double-clicked and nothing happened". Implemented
// with user32 directly to avoid pulling in CGO or another dependency.

import (
	"syscall"
	"unsafe"
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

const (
	mbOK            = 0x00000000
	mbIconError     = 0x00000010
	mbSetForeground = 0x00010000
	mbTopmost       = 0x00040000
)

// showError displays a modal error dialog. It is a no-op if the call fails —
// a failed error report must never itself take down the process.
func showError(title, message string) {
	t, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	m, err := syscall.UTF16PtrFromString(message)
	if err != nil {
		return
	}
	_, _, _ = procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(m)),
		uintptr(unsafe.Pointer(t)),
		uintptr(mbOK|mbIconError|mbSetForeground|mbTopmost),
	)
}

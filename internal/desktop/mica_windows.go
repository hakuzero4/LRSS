//go:build windows

package desktop

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modDwmapi                 = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute = modDwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	dwmwaCaptionColor              = 35
	dwmwaSystemBackdropType        = 38
	dwmwaColorDefault       uint32 = 0xFFFFFFFF
	backdropNone            uint32 = 1
	backdropMica            uint32 = 2
)

func dwmSetUint(hwnd uintptr, attr uint32, value uint32) {
	if hwnd == 0 || procDwmSetWindowAttribute.Find() != nil {
		return
	}
	_, _, _ = procDwmSetWindowAttribute.Call(
		hwnd,
		uintptr(attr),
		uintptr(unsafe.Pointer(&value)),
		unsafe.Sizeof(value),
	)
}

// ApplyWindowMica sets or clears the DWM Mica backdrop and caption fill.
func ApplyWindowMica(hwnd uintptr, enabled bool) {
	if hwnd == 0 {
		return
	}
	if enabled {
		dwmSetUint(hwnd, dwmwaSystemBackdropType, backdropMica)
		dwmSetUint(hwnd, dwmwaCaptionColor, dwmwaColorNone)
		return
	}
	dwmSetUint(hwnd, dwmwaSystemBackdropType, backdropNone)
	dwmSetUint(hwnd, dwmwaCaptionColor, dwmwaColorDefault)
}

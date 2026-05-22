package windowsapi

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/zyzil/win-pasterer/internal/core"
	"golang.org/x/sys/windows"
)

const (
	vkControl = 0x11
	maxVKCode = 0xFE

	cfUnicodeText = 13
	gmemMoveable  = 0x0002

	processQueryLimitedInformation = 0x1000
	maxPath                        = 260

	clipboardOpenRetryAttempts = 3
	clipboardOpenRetryDelay    = 5 * time.Millisecond

	errNoForegroundWindow  = "no foreground window"
	errNoPID               = "no pid"
	errOpenClipboardFailed = "OpenClipboard failed"
	errGlobalAllocFailed   = "GlobalAlloc failed"
	errGlobalLockFailed    = "GlobalLock failed"
	errSetClipboardFailed  = "SetClipboardData failed"
	errNilGlobalMemoryPtr  = "nil global memory pointer"
	errInvalidMaxUnits     = "invalid max units"
	errClipboardUnterm     = "clipboard text is not null-terminated within limit"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetAsyncKeyState           = user32.NewProc("GetAsyncKeyState")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procEmptyClipboard             = user32.NewProc("EmptyClipboard")
	procSetClipboardData           = user32.NewProc("SetClipboardData")

	procGlobalLock                 = kernel32.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32.NewProc("GlobalUnlock")
	procGlobalAlloc                = kernel32.NewProc("GlobalAlloc")
	procGlobalFree                 = kernel32.NewProc("GlobalFree")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
)

func IsCtrlPressed() bool {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vkControl))
	return (uint16(ret) & 0x8000) != 0
}

func KeyboardVKCode(lParam uintptr) (uint32, bool) {
	if lParam == 0 {
		return 0, false
	}
	type kbdllhookstruct struct {
		VkCode      uint32
		ScanCode    uint32
		Flags       uint32
		Time        uint32
		DwExtraInfo uintptr
	}
	k := (*kbdllhookstruct)(unsafe.Pointer(lParam))
	if k.VkCode > maxVKCode {
		return 0, false
	}
	return k.VkCode, true
}

func GetForegroundProcessImageName() (string, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return "", fmt.Errorf(errNoForegroundWindow)
	}

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return "", fmt.Errorf(errNoPID)
	}

	hProc, _, err := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if hProc == 0 {
		return "", err
	}
	defer procCloseHandle.Call(hProc)

	buf := make([]uint16, maxPath)
	size := uint32(len(buf))
	ret, _, callErr := procQueryFullProcessImageNameW.Call(
		hProc,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return "", callErr
	}

	full := windows.UTF16ToString(buf[:size])
	return strings.ToLower(filepath.Base(full)), nil
}

func NormalizeClipboardCRLFToLF() error {
	original, err := readClipboardUnicodeText()
	if err != nil {
		return err
	}
	if original == "" {
		return nil
	}

	converted := core.ConvertCRLFToLF(original)
	if converted == original {
		return nil
	}

	return writeClipboardUnicodeText(converted)
}

func readClipboardUnicodeText() (string, error) {
	if !openClipboardWithRetry(clipboardOpenRetryAttempts) {
		return "", fmt.Errorf(errOpenClipboardFailed)
	}
	defer procCloseClipboard.Call()

	available, _, _ := procIsClipboardFormatAvailable.Call(cfUnicodeText)
	if available == 0 {
		return "", nil
	}

	hData, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if hData == 0 {
		return "", nil
	}

	pData, _, _ := procGlobalLock.Call(hData)
	if pData == 0 {
		return "", nil
	}

	text, err := safeUTF16FromGlobalMemory(pData, core.MaxClipboardUTF16Units)
	procGlobalUnlock.Call(hData)
	if err != nil {
		return "", err
	}
	return text, nil
}

func writeClipboardUnicodeText(text string) error {
	if !openClipboardWithRetry(clipboardOpenRetryAttempts) {
		return fmt.Errorf(errOpenClipboardFailed)
	}
	defer procCloseClipboard.Call()

	utf16 := windows.StringToUTF16(text)
	bytesNeeded := uintptr(len(utf16) * 2)

	hNew, _, _ := procGlobalAlloc.Call(gmemMoveable, bytesNeeded)
	if hNew == 0 {
		return fmt.Errorf(errGlobalAllocFailed)
	}

	pNew, _, _ := procGlobalLock.Call(hNew)
	if pNew == 0 {
		procGlobalFree.Call(hNew)
		return fmt.Errorf(errGlobalLockFailed)
	}

	dst := unsafe.Slice((*byte)(unsafe.Pointer(pNew)), bytesNeeded)
	src := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(utf16))), bytesNeeded)
	copy(dst, src)
	procGlobalUnlock.Call(hNew)

	procEmptyClipboard.Call()
	setRet, _, _ := procSetClipboardData.Call(cfUnicodeText, hNew)
	if setRet == 0 {
		procGlobalFree.Call(hNew)
		return fmt.Errorf(errSetClipboardFailed)
	}

	return nil
}

func openClipboardWithRetry(maxAttempts int) bool {
	for i := 0; i < maxAttempts; i++ {
		ret, _, _ := procOpenClipboard.Call(0)
		if ret != 0 {
			return true
		}
		time.Sleep(clipboardOpenRetryDelay)
	}
	return false
}

func safeUTF16FromGlobalMemory(ptr uintptr, maxUnits int) (string, error) {
	if ptr == 0 {
		return "", fmt.Errorf(errNilGlobalMemoryPtr)
	}
	if maxUnits <= 0 {
		return "", fmt.Errorf(errInvalidMaxUnits)
	}
	buf := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), maxUnits)
	for i, r := range buf {
		if r == 0 {
			return windows.UTF16ToString(buf[:i]), nil
		}
	}
	return "", fmt.Errorf(errClipboardUnterm)
}

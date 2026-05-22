package main

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/zyzil/win-pasterer/internal/core"
	"golang.org/x/sys/windows"
)

type SettingsResult struct {
	Processes    []string
	RunAtStartup bool
}

type SettingsDialog interface {
	Show(owner uintptr, processes []string, runAtStartup bool) (SettingsResult, bool)
}

type winSettingsDialog struct{}

type settingsDialogState struct {
	hwnd             uintptr
	owner            uintptr
	editHwnd         uintptr
	startupCheckHwnd uintptr
	accepted         bool
	result           []string
	runAtStartup     bool
}

var settingsByHwnd sync.Map

func newSettingsDialog() SettingsDialog {
	return &winSettingsDialog{}
}

func (d *winSettingsDialog) Show(owner uintptr, processes []string, runAtStartup bool) (SettingsResult, bool) {
	title, _ := windows.UTF16PtrFromString(settingsWindowTitle)
	className, _ := windows.UTF16PtrFromString(settingsClassName)
	dpi := getDPIForWindow(owner)

	state := &settingsDialogState{owner: owner, result: processes, runAtStartup: runAtStartup}

	hwnd, _, _ := procCreateWindowExW.Call(
		uintptr(wsExAppWindow|wsExDlgModalFrame),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		uintptr(wsOverlapped|wsCaption|wsSysMenu|wsVisible|wsClipSiblings),
		uintptr(scaleDPI(settingsWindowX, dpi)),
		uintptr(scaleDPI(settingsWindowY, dpi)),
		uintptr(scaleDPI(settingsWindowWidth, dpi)),
		uintptr(scaleDPI(settingsWindowHeight, dpi)),
		owner,
		0,
		0,
		0,
	)
	if hwnd == 0 {
		return SettingsResult{Processes: processes, RunAtStartup: runAtStartup}, false
	}
	state.hwnd = hwnd
	settingsByHwnd.Store(hwnd, state)
	initSettingsDialogControls(hwnd, state)

	procEnableWindow.Call(owner, 0)
	defer procEnableWindow.Call(owner, 1)

	procShowWindow.Call(hwnd, swShow)

	for {
		if !isWindow(hwnd) {
			break
		}
		var m msg
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	return SettingsResult{Processes: state.result, RunAtStartup: state.runAtStartup}, state.accepted
}

func settingsWindowProc(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
	switch message {
	case wmCommand:
		cmdID := uint16(wParam & 0xffff)
		raw, ok := settingsByHwnd.Load(hwnd)
		if !ok {
			return 0
		}
		state := raw.(*settingsDialogState)

		switch cmdID {
		case idSettingsSave:
			text := getWindowText(state.editHwnd)
			state.result = core.ParseProcessEditorInput(text)
			state.runAtStartup = isButtonChecked(state.startupCheckHwnd)
			state.accepted = true
			procDestroyWindow.Call(hwnd)
			return 0
		case idSettingsCancel:
			state.accepted = false
			procDestroyWindow.Call(hwnd)
			return 0
		}

	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0

	case wmDestroy:
		settingsByHwnd.Delete(hwnd)
		return 0
	}

	return defWindowProc(hwnd, message, wParam, lParam)
}

func initSettingsDialogControls(hwnd uintptr, state *settingsDialogState) {
	dpi := getDPIForWindow(hwnd)
	labelClass, _ := windows.UTF16PtrFromString("STATIC")
	labelText, _ := windows.UTF16PtrFromString(settingsExecutablesHelp)
	procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(labelClass)),
		uintptr(unsafe.Pointer(labelText)),
		uintptr(wsChild|wsVisible),
		uintptr(scaleDPI(settingsLabelX, dpi)),
		uintptr(scaleDPI(settingsLabelY, dpi)),
		uintptr(scaleDPI(settingsLabelWidth, dpi)),
		uintptr(scaleDPI(settingsLabelHeight, dpi)),
		hwnd,
		0,
		0,
		0,
	)

	editClass, _ := windows.UTF16PtrFromString("EDIT")
	currentText := strings.Join(state.result, "\r\n")
	currentPtr, _ := windows.UTF16PtrFromString(currentText)
	editHwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(editClass)),
		uintptr(unsafe.Pointer(currentPtr)),
		uintptr(wsChild|wsVisible|wsBorder|wsVScroll|wsTabStop|esLeft|esMultiline|esAutoVScroll|esWantReturn),
		uintptr(scaleDPI(settingsEditX, dpi)),
		uintptr(scaleDPI(settingsEditY, dpi)),
		uintptr(scaleDPI(settingsEditWidth, dpi)),
		uintptr(scaleDPI(settingsEditHeight, dpi)),
		hwnd,
		uintptr(idSettingsEdit),
		0,
		0,
	)
	state.editHwnd = editHwnd

	buttonClass, _ := windows.UTF16PtrFromString("BUTTON")
	startupText, _ := windows.UTF16PtrFromString(settingsStartupLabel)
	startupCheckHwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonClass)),
		uintptr(unsafe.Pointer(startupText)),
		uintptr(wsChild|wsVisible|wsTabStop|bsAutoCheckbox),
		uintptr(scaleDPI(settingsStartupX, dpi)),
		uintptr(scaleDPI(settingsStartupY, dpi)),
		uintptr(scaleDPI(settingsStartupWidth, dpi)),
		uintptr(scaleDPI(settingsStartupHeight, dpi)),
		hwnd,
		uintptr(idSettingsStartup),
		0,
		0,
	)
	state.startupCheckHwnd = startupCheckHwnd
	if state.runAtStartup {
		procSendMessageW.Call(startupCheckHwnd, bmSetCheck, bstChecked, 0)
	} else {
		procSendMessageW.Call(startupCheckHwnd, bmSetCheck, bstUnchecked, 0)
	}

	saveText, _ := windows.UTF16PtrFromString(settingsSaveLabel)
	procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonClass)),
		uintptr(unsafe.Pointer(saveText)),
		uintptr(wsChild|wsVisible|wsTabStop),
		uintptr(scaleDPI(settingsSaveX, dpi)),
		uintptr(scaleDPI(settingsSaveY, dpi)),
		uintptr(scaleDPI(settingsSaveWidth, dpi)),
		uintptr(scaleDPI(settingsSaveHeight, dpi)),
		hwnd,
		uintptr(idSettingsSave),
		0,
		0,
	)

	cancelText, _ := windows.UTF16PtrFromString(settingsCancelLabel)
	procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonClass)),
		uintptr(unsafe.Pointer(cancelText)),
		uintptr(wsChild|wsVisible|wsTabStop),
		uintptr(scaleDPI(settingsCancelX, dpi)),
		uintptr(scaleDPI(settingsCancelY, dpi)),
		uintptr(scaleDPI(settingsCancelWidth, dpi)),
		uintptr(scaleDPI(settingsCancelHeight, dpi)),
		hwnd,
		uintptr(idSettingsCancel),
		0,
		0,
	)
}

func getWindowText(hwnd uintptr) string {
	if hwnd == 0 {
		return ""
	}
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return windows.UTF16ToString(buf)
}

func isWindow(hwnd uintptr) bool {
	ret, _, _ := procIsWindow.Call(hwnd)
	return ret != 0
}

func isButtonChecked(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	state, _, _ := procSendMessageW.Call(hwnd, bmGetCheck, 0, 0)
	return state == bstChecked
}

func getDPIForWindow(hwnd uintptr) int32 {
	if hwnd != 0 {
		if dpi, _, _ := procGetDpiForWindow.Call(hwnd); dpi != 0 {
			return int32(dpi)
		}
	}
	if dpi, _, _ := procGetDpiForSystem.Call(); dpi != 0 {
		return int32(dpi)
	}
	return defaultDPI
}

func scaleDPI(value int32, dpi int32) int32 {
	if dpi <= 0 {
		dpi = defaultDPI
	}
	return int32((int64(value)*int64(dpi) + (defaultDPI / 2)) / defaultDPI)
}

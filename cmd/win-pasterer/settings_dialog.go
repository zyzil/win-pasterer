package main

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/zyzil/win-pasterer/internal/core"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type SettingsResult struct {
	Enabled      bool
	Processes    []string
	RunAtStartup bool
}

type SettingsDialog interface {
	Show(owner uintptr, enabled bool, processes []string, runAtStartup bool) (SettingsResult, bool, error)
}

type winSettingsDialog struct{}

type settingsDialogState struct {
	hwnd             uintptr
	labelHwnd        uintptr
	editHwnd         uintptr
	enabledCheckHwnd uintptr
	enabledLabelHwnd uintptr
	startupCheckHwnd uintptr
	startupLabelHwnd uintptr
	saveHwnd         uintptr
	cancelHwnd       uintptr
	font             uintptr
	theme            settingsTheme
	accepted         bool
	enabled          bool
	result           []string
	runAtStartup     bool
}

type settingsTheme struct {
	dark         bool
	windowColor  uint32
	controlColor uint32
	buttonColor  uint32
	pressedColor uint32
	borderColor  uint32
	textColor    uint32
	windowBrush  uintptr
	controlBrush uintptr
	buttonBrush  uintptr
	pressedBrush uintptr
	borderBrush  uintptr
}

var settingsByHwnd sync.Map

func newSettingsDialog() SettingsDialog {
	return &winSettingsDialog{}
}

func (d *winSettingsDialog) Show(owner uintptr, enabled bool, processes []string, runAtStartup bool) (SettingsResult, bool, error) {
	defer func() {
		if r := recover(); r != nil {
			if globalApp != nil {
				globalApp.logError("settings dialog panic: " + strings.TrimSpace(toString(r)))
			}
			panic(r)
		}
	}()
	title, _ := windows.UTF16PtrFromString(settingsWindowTitle)
	className, _ := windows.UTF16PtrFromString(settingsClassName)
	dpi := getDPIForWindow(owner)
	x, y, width, height := centeredSettingsRect(dpi)

	state := &settingsDialogState{enabled: enabled, result: processes, runAtStartup: runAtStartup}
	state.theme = newSettingsTheme(appsUseDarkTheme())

	hwnd, _, _ := procCreateWindowExW.Call(
		uintptr(wsExAppWindow|wsExDlgModalFrame),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		uintptr(wsOverlapped|wsCaption|wsSysMenu|wsClipChildren|wsClipSiblings),
		uintptr(x),
		uintptr(y),
		uintptr(width),
		uintptr(height),
		owner,
		0,
		0,
		0,
	)
	if hwnd == 0 {
		state.theme.destroy()
		return SettingsResult{Enabled: enabled, Processes: processes, RunAtStartup: runAtStartup}, false, fmt.Errorf("CreateWindowExW failed for settings dialog")
	}
	state.hwnd = hwnd
	settingsByHwnd.Store(hwnd, state)
	applyThemeToWindow(hwnd, state.theme)
	applyWindowFrameTheme(hwnd, state.theme)
	if err := initSettingsDialogControls(hwnd, state); err != nil {
		settingsByHwnd.Delete(hwnd)
		state.theme.destroy()
		procDestroyWindow.Call(hwnd)
		return SettingsResult{Enabled: enabled, Processes: processes, RunAtStartup: runAtStartup}, false, err
	}

	procShowWindow.Call(hwnd, swShow)

	for {
		if !isWindow(hwnd) {
			break
		}
		var m msg
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			// This nested loop may receive WM_QUIT before the outer app loop.
			// Re-post it so choosing Exit while settings is open still exits.
			if ret == 0 {
				procPostQuitMessage.Call(uintptr(m.WParam))
			}
			break
		}
		if handled, _, _ := procIsDialogMessageW.Call(hwnd, uintptr(unsafe.Pointer(&m))); handled != 0 {
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	return SettingsResult{Enabled: state.enabled, Processes: state.result, RunAtStartup: state.runAtStartup}, state.accepted, nil
}

func settingsWindowProc(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) (ret uintptr) {
	defer recoverCallback("settings window", hwnd, &ret)
	switch message {
	case wmEraseBkgnd:
		raw, ok := settingsByHwnd.Load(hwnd)
		if !ok {
			return 0
		}
		state := raw.(*settingsDialogState)
		paintSettingsBackground(hwnd, wParam, state.theme)
		return 1

	case wmCtlColorStatic:
		raw, ok := settingsByHwnd.Load(hwnd)
		if !ok {
			return 0
		}
		state := raw.(*settingsDialogState)
		return applyControlColors(wParam, state.theme, false)

	case wmCtlColorEdit:
		raw, ok := settingsByHwnd.Load(hwnd)
		if !ok {
			return 0
		}
		state := raw.(*settingsDialogState)
		return applyControlColors(wParam, state.theme, true)

	case wmCtlColorBtn:
		raw, ok := settingsByHwnd.Load(hwnd)
		if !ok {
			return 0
		}
		state := raw.(*settingsDialogState)
		return applyControlColors(wParam, state.theme, false)

	case wmDrawItem:
		raw, ok := settingsByHwnd.Load(hwnd)
		if !ok {
			return 0
		}
		state := raw.(*settingsDialogState)
		if drawSettingsButton(lParam, state.theme) {
			return 1
		}
		return 0

	case wmThemeChanged, wmSettingChange:
		raw, ok := settingsByHwnd.Load(hwnd)
		if !ok {
			return 0
		}
		state := raw.(*settingsDialogState)
		updateSettingsTheme(state, appsUseDarkTheme())
		return 0

	case wmDPIChanged:
		raw, ok := settingsByHwnd.Load(hwnd)
		if !ok {
			return 0
		}
		state := raw.(*settingsDialogState)
		dpi := int32(wParam >> 16)
		if dpi <= 0 {
			dpi = getDPIForWindow(hwnd)
		}
		if lParam != 0 {
			r := (*rect)(unsafe.Pointer(lParam))
			procSetWindowPos.Call(
				hwnd,
				0,
				uintptr(r.Left),
				uintptr(r.Top),
				uintptr(r.Right-r.Left),
				uintptr(r.Bottom-r.Top),
				0,
			)
		}
		updateSettingsDialogLayout(state, dpi)
		return 0

	case wmCommand:
		cmdID := uint16(wParam & 0xffff)
		raw, ok := settingsByHwnd.Load(hwnd)
		if !ok {
			return 0
		}
		state := raw.(*settingsDialogState)

		switch cmdID {
		case idSettingsEnabledLabel:
			toggleButtonChecked(state.enabledCheckHwnd)
			return 0
		case idSettingsStartupLabel:
			toggleButtonChecked(state.startupCheckHwnd)
			return 0
		case idSettingsSave:
			text := getWindowText(state.editHwnd)
			state.enabled = isButtonChecked(state.enabledCheckHwnd)
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
		if raw, ok := settingsByHwnd.Load(hwnd); ok {
			state := raw.(*settingsDialogState)
			if state.font != 0 {
				procDeleteObject.Call(state.font)
				state.font = 0
			}
			state.theme.destroy()
		}
		settingsByHwnd.Delete(hwnd)
		return 0
	}

	return defWindowProc(hwnd, message, wParam, lParam)
}

func closeSettingsDialogs() {
	settingsByHwnd.Range(func(key any, value any) bool {
		hwnd, ok := key.(uintptr)
		if !ok || hwnd == 0 {
			return true
		}
		if state, ok := value.(*settingsDialogState); ok {
			state.accepted = false
		}
		if isWindow(hwnd) {
			procDestroyWindow.Call(hwnd)
		}
		return true
	})
}

func activateSettingsDialog() bool {
	activated := false
	settingsByHwnd.Range(func(key any, _ any) bool {
		hwnd, ok := key.(uintptr)
		if !ok || hwnd == 0 || !isWindow(hwnd) {
			return true
		}
		procShowWindow.Call(hwnd, swShow)
		procSetForegroundWindow.Call(hwnd)
		activated = true
		return false
	})
	return activated
}

func initSettingsDialogControls(hwnd uintptr, state *settingsDialogState) error {
	dpi := getDPIForWindow(hwnd)
	state.font = createSettingsFont(dpi)
	if state.font == 0 {
		return fmt.Errorf("CreateFontW failed for settings dialog")
	}
	labelClass, _ := windows.UTF16PtrFromString("STATIC")
	labelText, _ := windows.UTF16PtrFromString(settingsExecutablesHelp)
	labelHwnd, _, _ := procCreateWindowExW.Call(
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
	if labelHwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed for settings label")
	}
	state.labelHwnd = labelHwnd
	setControlFont(labelHwnd, state.font)
	applyThemeToWindow(labelHwnd, state.theme)

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
	if editHwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed for settings editor")
	}
	state.editHwnd = editHwnd
	setControlFont(editHwnd, state.font)
	applyThemeToWindow(editHwnd, state.theme)

	buttonClass, _ := windows.UTF16PtrFromString("BUTTON")
	emptyText, _ := windows.UTF16PtrFromString("")
	enabledCheckHwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonClass)),
		uintptr(unsafe.Pointer(emptyText)),
		uintptr(wsChild|wsVisible|wsTabStop|bsAutoCheckbox),
		uintptr(scaleDPI(settingsEnabledX, dpi)),
		uintptr(scaleDPI(settingsEnabledY, dpi)),
		uintptr(scaleDPI(settingsEnabledWidth, dpi)),
		uintptr(scaleDPI(settingsEnabledHeight, dpi)),
		hwnd,
		uintptr(idSettingsEnabled),
		0,
		0,
	)
	if enabledCheckHwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed for enabled checkbox")
	}
	state.enabledCheckHwnd = enabledCheckHwnd
	setControlFont(enabledCheckHwnd, state.font)
	applyThemeToWindow(enabledCheckHwnd, state.theme)

	enabledLabelText, _ := windows.UTF16PtrFromString(settingsEnabledLabel)
	enabledLabelHwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(labelClass)),
		uintptr(unsafe.Pointer(enabledLabelText)),
		uintptr(wsChild|wsVisible|ssNotify|ssCenterImage),
		uintptr(scaleDPI(settingsEnabledLabelX, dpi)),
		uintptr(scaleDPI(settingsEnabledLabelY, dpi)),
		uintptr(scaleDPI(settingsEnabledLabelWidth, dpi)),
		uintptr(scaleDPI(settingsEnabledLabelHeight, dpi)),
		hwnd,
		uintptr(idSettingsEnabledLabel),
		0,
		0,
	)
	if enabledLabelHwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed for enabled label")
	}
	state.enabledLabelHwnd = enabledLabelHwnd
	setControlFont(enabledLabelHwnd, state.font)
	applyThemeToWindow(enabledLabelHwnd, state.theme)
	if state.enabled {
		procSendMessageW.Call(enabledCheckHwnd, bmSetCheck, bstChecked, 0)
	} else {
		procSendMessageW.Call(enabledCheckHwnd, bmSetCheck, bstUnchecked, 0)
	}

	startupCheckHwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonClass)),
		uintptr(unsafe.Pointer(emptyText)),
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
	if startupCheckHwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed for startup checkbox")
	}
	state.startupCheckHwnd = startupCheckHwnd
	setControlFont(startupCheckHwnd, state.font)
	applyThemeToWindow(startupCheckHwnd, state.theme)

	startupLabelText, _ := windows.UTF16PtrFromString(settingsStartupLabel)
	startupLabelHwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(labelClass)),
		uintptr(unsafe.Pointer(startupLabelText)),
		uintptr(wsChild|wsVisible|ssNotify|ssCenterImage),
		uintptr(scaleDPI(settingsStartupLabelX, dpi)),
		uintptr(scaleDPI(settingsStartupLabelY, dpi)),
		uintptr(scaleDPI(settingsStartupLabelWidth, dpi)),
		uintptr(scaleDPI(settingsStartupLabelHeight, dpi)),
		hwnd,
		uintptr(idSettingsStartupLabel),
		0,
		0,
	)
	if startupLabelHwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed for startup label")
	}
	state.startupLabelHwnd = startupLabelHwnd
	setControlFont(startupLabelHwnd, state.font)
	applyThemeToWindow(startupLabelHwnd, state.theme)
	if state.runAtStartup {
		procSendMessageW.Call(startupCheckHwnd, bmSetCheck, bstChecked, 0)
	} else {
		procSendMessageW.Call(startupCheckHwnd, bmSetCheck, bstUnchecked, 0)
	}

	saveText, _ := windows.UTF16PtrFromString(settingsSaveLabel)
	saveHwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonClass)),
		uintptr(unsafe.Pointer(saveText)),
		uintptr(wsChild|wsVisible|wsTabStop|bsOwnerDraw|bsDefPushButton),
		uintptr(scaleDPI(settingsSaveX, dpi)),
		uintptr(scaleDPI(settingsSaveY, dpi)),
		uintptr(scaleDPI(settingsSaveWidth, dpi)),
		uintptr(scaleDPI(settingsSaveHeight, dpi)),
		hwnd,
		uintptr(idSettingsSave),
		0,
		0,
	)
	if saveHwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed for save button")
	}
	state.saveHwnd = saveHwnd
	setControlFont(saveHwnd, state.font)
	applyThemeToWindow(saveHwnd, state.theme)

	cancelText, _ := windows.UTF16PtrFromString(settingsCancelLabel)
	cancelHwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonClass)),
		uintptr(unsafe.Pointer(cancelText)),
		uintptr(wsChild|wsVisible|wsTabStop|bsOwnerDraw),
		uintptr(scaleDPI(settingsCancelX, dpi)),
		uintptr(scaleDPI(settingsCancelY, dpi)),
		uintptr(scaleDPI(settingsCancelWidth, dpi)),
		uintptr(scaleDPI(settingsCancelHeight, dpi)),
		hwnd,
		uintptr(idSettingsCancel),
		0,
		0,
	)
	if cancelHwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed for cancel button")
	}
	state.cancelHwnd = cancelHwnd
	setControlFont(cancelHwnd, state.font)
	applyThemeToWindow(cancelHwnd, state.theme)
	return nil
}

func updateSettingsDialogLayout(state *settingsDialogState, dpi int32) {
	if state == nil {
		return
	}
	if state.font != 0 {
		procDeleteObject.Call(state.font)
	}
	state.font = createSettingsFont(dpi)

	moveControl(state.labelHwnd, settingsLabelX, settingsLabelY, settingsLabelWidth, settingsLabelHeight, dpi)
	moveControl(state.editHwnd, settingsEditX, settingsEditY, settingsEditWidth, settingsEditHeight, dpi)
	moveControl(state.enabledCheckHwnd, settingsEnabledX, settingsEnabledY, settingsEnabledWidth, settingsEnabledHeight, dpi)
	moveControl(state.enabledLabelHwnd, settingsEnabledLabelX, settingsEnabledLabelY, settingsEnabledLabelWidth, settingsEnabledLabelHeight, dpi)
	moveControl(state.startupCheckHwnd, settingsStartupX, settingsStartupY, settingsStartupWidth, settingsStartupHeight, dpi)
	moveControl(state.startupLabelHwnd, settingsStartupLabelX, settingsStartupLabelY, settingsStartupLabelWidth, settingsStartupLabelHeight, dpi)
	moveControl(state.saveHwnd, settingsSaveX, settingsSaveY, settingsSaveWidth, settingsSaveHeight, dpi)
	moveControl(state.cancelHwnd, settingsCancelX, settingsCancelY, settingsCancelWidth, settingsCancelHeight, dpi)

	setControlFont(state.labelHwnd, state.font)
	setControlFont(state.editHwnd, state.font)
	setControlFont(state.enabledCheckHwnd, state.font)
	setControlFont(state.enabledLabelHwnd, state.font)
	setControlFont(state.startupCheckHwnd, state.font)
	setControlFont(state.startupLabelHwnd, state.font)
	setControlFont(state.saveHwnd, state.font)
	setControlFont(state.cancelHwnd, state.font)
	invalidateWindow(state.hwnd)
}

func updateSettingsTheme(state *settingsDialogState, dark bool) {
	if state == nil {
		return
	}
	state.theme.destroy()
	state.theme = newSettingsTheme(dark)
	applyThemeToWindow(state.hwnd, state.theme)
	applyWindowFrameTheme(state.hwnd, state.theme)
	applyThemeToWindow(state.labelHwnd, state.theme)
	applyThemeToWindow(state.editHwnd, state.theme)
	applyThemeToWindow(state.enabledCheckHwnd, state.theme)
	applyThemeToWindow(state.enabledLabelHwnd, state.theme)
	applyThemeToWindow(state.startupCheckHwnd, state.theme)
	applyThemeToWindow(state.startupLabelHwnd, state.theme)
	applyThemeToWindow(state.saveHwnd, state.theme)
	applyThemeToWindow(state.cancelHwnd, state.theme)
	invalidateWindow(state.hwnd)
	invalidateWindow(state.labelHwnd)
	invalidateWindow(state.editHwnd)
	invalidateWindow(state.enabledCheckHwnd)
	invalidateWindow(state.enabledLabelHwnd)
	invalidateWindow(state.startupCheckHwnd)
	invalidateWindow(state.startupLabelHwnd)
	invalidateWindow(state.saveHwnd)
	invalidateWindow(state.cancelHwnd)
}

func newSettingsTheme(dark bool) settingsTheme {
	theme := settingsThemeForMode(dark)
	theme.windowBrush = createBrush(theme.windowColor)
	theme.controlBrush = createBrush(theme.controlColor)
	theme.buttonBrush = createBrush(theme.buttonColor)
	theme.pressedBrush = createBrush(theme.pressedColor)
	theme.borderBrush = createBrush(theme.borderColor)
	return theme
}

func settingsThemeForMode(dark bool) settingsTheme {
	if dark {
		return settingsTheme{
			dark:         true,
			windowColor:  rgb(32, 32, 32),
			controlColor: rgb(45, 45, 45),
			buttonColor:  rgb(58, 58, 58),
			pressedColor: rgb(72, 72, 72),
			borderColor:  rgb(104, 104, 104),
			textColor:    rgb(245, 245, 245),
		}
	}
	return settingsTheme{
		windowColor:  rgb(240, 240, 240),
		controlColor: rgb(255, 255, 255),
		buttonColor:  rgb(245, 245, 245),
		pressedColor: rgb(230, 230, 230),
		borderColor:  rgb(150, 150, 150),
		textColor:    rgb(0, 0, 0),
	}
}

func (t settingsTheme) destroy() {
	if t.windowBrush != 0 {
		procDeleteObject.Call(t.windowBrush)
	}
	if t.controlBrush != 0 && t.controlBrush != t.windowBrush {
		procDeleteObject.Call(t.controlBrush)
	}
	if t.buttonBrush != 0 {
		procDeleteObject.Call(t.buttonBrush)
	}
	if t.pressedBrush != 0 {
		procDeleteObject.Call(t.pressedBrush)
	}
	if t.borderBrush != 0 {
		procDeleteObject.Call(t.borderBrush)
	}
}

func rgb(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

func createBrush(color uint32) uintptr {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(color))
	return brush
}

func paintSettingsBackground(hwnd uintptr, hdc uintptr, theme settingsTheme) {
	if hwnd == 0 || hdc == 0 || theme.windowBrush == 0 {
		return
	}
	var r rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), theme.windowBrush)
}

func applyControlColors(hdc uintptr, theme settingsTheme, edit bool) uintptr {
	if hdc == 0 {
		return 0
	}
	procSetTextColor.Call(hdc, uintptr(theme.textColor))
	if edit {
		procSetBkColor.Call(hdc, uintptr(theme.controlColor))
		return theme.controlBrush
	}
	procSetBkColor.Call(hdc, uintptr(theme.windowColor))
	return theme.windowBrush
}

func drawSettingsButton(lParam uintptr, theme settingsTheme) bool {
	if lParam == 0 {
		return false
	}
	item := (*drawItemStruct)(unsafe.Pointer(lParam))
	if item.CtlType != odtButton {
		return false
	}
	label := ""
	switch item.CtlID {
	case idSettingsSave:
		label = settingsSaveLabel
	case idSettingsCancel:
		label = settingsCancelLabel
	default:
		return false
	}

	brush := theme.buttonBrush
	if item.ItemState&odsSelected != 0 && theme.pressedBrush != 0 {
		brush = theme.pressedBrush
	}
	if brush == 0 {
		brush = theme.controlBrush
	}
	procFillRect.Call(item.HDC, uintptr(unsafe.Pointer(&item.RcItem)), brush)
	if theme.borderBrush != 0 {
		procFrameRect.Call(item.HDC, uintptr(unsafe.Pointer(&item.RcItem)), theme.borderBrush)
	}

	textRect := item.RcItem
	if item.ItemState&odsSelected != 0 {
		textRect.Left++
		textRect.Top++
	}
	text, _ := windows.UTF16PtrFromString(label)
	procSetBkMode.Call(item.HDC, transparent)
	procSetTextColor.Call(item.HDC, uintptr(theme.textColor))
	procDrawTextW.Call(
		item.HDC,
		uintptr(unsafe.Pointer(text)),
		uintptr(^uint32(0)),
		uintptr(unsafe.Pointer(&textRect)),
		uintptr(dtCenter|dtVCenter|dtSingleLine),
	)
	return true
}

func applyThemeToWindow(hwnd uintptr, theme settingsTheme) {
	if hwnd == 0 {
		return
	}
}

func applyWindowFrameTheme(hwnd uintptr, theme settingsTheme) {
	if hwnd == 0 || procDwmSetWindowAttribute.Find() != nil {
		return
	}
	dark := int32(0)
	if theme.dark {
		dark = 1
	}
	// DWMWA_USE_IMMERSIVE_DARK_MODE is documented as attribute 20.
	procDwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(dark))
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case error:
		return x.Error()
	default:
		return "unknown error"
	}
}

func invalidateWindow(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	procInvalidateRect.Call(hwnd, 0, 1)
}

func appsUseDarkTheme() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return false
	}
	return v == 0
}

func moveControl(hwnd uintptr, x, y, width, height, dpi int32) {
	if hwnd == 0 {
		return
	}
	procMoveWindow.Call(
		hwnd,
		uintptr(scaleDPI(x, dpi)),
		uintptr(scaleDPI(y, dpi)),
		uintptr(scaleDPI(width, dpi)),
		uintptr(scaleDPI(height, dpi)),
		1,
	)
}

func setControlFont(hwnd uintptr, font uintptr) {
	if hwnd == 0 || font == 0 {
		return
	}
	procSendMessageW.Call(hwnd, wmSetFont, font, 1)
}

func createSettingsFont(dpi int32) uintptr {
	if dpi <= 0 {
		dpi = defaultDPI
	}
	name, _ := windows.UTF16PtrFromString(settingsFontName)
	height := -int32((9*int64(dpi) + 36) / 72)
	font, _, _ := procCreateFontW.Call(
		uintptr(height),
		0,
		0,
		0,
		uintptr(fwNormal),
		0,
		0,
		0,
		uintptr(defaultCharset),
		uintptr(outDefaultPrecis),
		uintptr(clipDefaultPrecis),
		uintptr(defaultQuality),
		uintptr(defaultPitch),
		uintptr(unsafe.Pointer(name)),
	)
	return font
}

func centeredSettingsRect(dpi int32) (int32, int32, int32, int32) {
	width := scaleDPI(settingsWindowWidth, dpi)
	height := scaleDPI(settingsWindowHeight, dpi)
	screenWidth := systemMetricForDPI(smCxScreen, dpi)
	screenHeight := systemMetricForDPI(smCyScreen, dpi)
	x := (screenWidth - width) / 2
	y := (screenHeight - height) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y, width, height
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

func toggleButtonChecked(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	next := uintptr(bstChecked)
	if isButtonChecked(hwnd) {
		next = bstUnchecked
	}
	procSendMessageW.Call(hwnd, bmSetCheck, next, 0)
	invalidateWindow(hwnd)
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

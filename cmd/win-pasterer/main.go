package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/zyzil/win-pasterer/internal/core"
	"github.com/zyzil/win-pasterer/internal/platform/startup"
	"github.com/zyzil/win-pasterer/internal/platform/windowsapi"
	"golang.org/x/sys/windows"
)

const (
	// Window and tray constants.
	appName                  = "win-pasterer"
	hiddenClassName          = "WinPastererHiddenWindow"
	settingsClassName        = "WinPastererSettingsWindow"
	wmTrayIcon        uint32 = 0x0400 + 1 // WM_APP + 1

	// Menu command IDs.
	idMenuToggleEnabled = 1001
	idMenuSettings      = 1002
	idMenuExit          = 1003

	// Settings control IDs.
	idSettingsEdit    = 2001
	idSettingsSave    = 2002
	idSettingsCancel  = 2003
	idSettingsStartup = 2004

	// Win32 constants.
	whKeyboardLL   = 13
	hcAction       = 0
	wmDestroy      = 0x0002
	wmClose        = 0x0010
	wmCommand      = 0x0111
	wmKeyDown      = 0x0100
	wmRButtonUp    = 0x0205
	wmContextMenu  = 0x007B
	swShow         = 5
	wsOverlapped   = 0x00000000
	wsCaption      = 0x00C00000
	wsSysMenu      = 0x00080000
	wsVisible      = 0x10000000
	wsChild        = 0x40000000
	wsBorder       = 0x00800000
	wsVScroll      = 0x00200000
	wsTabStop      = 0x00010000
	wsClipSiblings = 0x04000000
	bsAutoCheckbox = 0x00000003

	wsExToolWindow    = 0x00000080
	wsExAppWindow     = 0x00040000
	wsExDlgModalFrame = 0x00000001

	esLeft        = 0x0000
	esMultiline   = 0x0004
	esAutoVScroll = 0x0040
	esWantReturn  = 0x1000

	mfString    = 0x00000000
	mfSeparator = 0x00000800
	mfChecked   = 0x00000008
	mfUnchecked = 0x00000000

	tpmRightButton = 0x0002
	tpmRetCmd      = 0x0100

	// Tray icon flags.
	nimAdd     = 0x00000000
	nimDelete  = 0x00000002
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	// Keyboard constants.
	vkV = 0x56

	// Misc constants.
	idcArrow       = 32512
	idiApplication = 32512
	imageIcon      = 1
	lrDefaultSize  = 0x00000040
	lrShared       = 0x00008000

	bmGetCheck   = 0x00F0
	bmSetCheck   = 0x00F1
	bstChecked   = 1
	bstUnchecked = 0
	mbIconError  = 0x00000010
)

type point struct {
	X int32
	Y int32
}

type msg struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

type wndClassExW struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type notifyIconDataW struct {
	CbSize            uint32
	HWnd              uintptr
	UID               uint32
	UFlags            uint32
	UCallbackMessage  uint32
	HIcon             uintptr
	SzTip             [128]uint16
	State             uint32
	StateMask         uint32
	SzInfo            [256]uint16
	UTimeoutOrVersion uint32
	SzInfoTitle       [64]uint16
	DWInfoFlags       uint32
	GuidItem          windows.GUID
	HBalloonIcon      uintptr
}

type app struct {
	hwnd           uintptr
	hook           uintptr
	hookProc       uintptr
	enabled        bool
	runAtStartup   bool
	processes      map[string]struct{}
	configPath     string
	taskbarCreated uint32
	hiddenProc     uintptr
	settingsProc   uintptr
	settingsUI     SettingsDialog

	mu sync.RWMutex
}

var (
	user32  = windows.NewLazySystemDLL("user32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procRegisterClassExW       = user32.NewProc("RegisterClassExW")
	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procDestroyWindow          = user32.NewProc("DestroyWindow")
	procPostQuitMessage        = user32.NewProc("PostQuitMessage")
	procGetMessageW            = user32.NewProc("GetMessageW")
	procTranslateMessage       = user32.NewProc("TranslateMessage")
	procDispatchMessageW       = user32.NewProc("DispatchMessageW")
	procSetWindowsHookExW      = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx         = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx    = user32.NewProc("UnhookWindowsHookEx")
	procCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	procAppendMenuW            = user32.NewProc("AppendMenuW")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procTrackPopupMenu         = user32.NewProc("TrackPopupMenu")
	procGetCursorPos           = user32.NewProc("GetCursorPos")
	procSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	procLoadCursorW            = user32.NewProc("LoadCursorW")
	procLoadIconW              = user32.NewProc("LoadIconW")
	procRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	procSendMessageW           = user32.NewProc("SendMessageW")
	procGetWindowTextLengthW   = user32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW         = user32.NewProc("GetWindowTextW")
	procEnableWindow           = user32.NewProc("EnableWindow")
	procShowWindow             = user32.NewProc("ShowWindow")
	procIsWindow               = user32.NewProc("IsWindow")
	procLoadImageW             = user32.NewProc("LoadImageW")
	procMessageBoxW            = user32.NewProc("MessageBoxW")
	procGetDpiForWindow        = user32.NewProc("GetDpiForWindow")
	procGetDpiForSystem        = user32.NewProc("GetDpiForSystem")

	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")

	globalApp *app
)

func startupValueName() string {
	return appName
}

func main() {
	runtime.LockOSThread()

	cfgPath, err := core.ConfigPath(appName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config path error: %v\n", err)
		return
	}

	loaded := core.LoadConfig(cfgPath)
	globalApp = &app{
		enabled:      loaded.Enabled,
		runAtStartup: loaded.RunAtStartup,
		processes:    core.ToProcessSet(loaded.Processes),
		configPath:   cfgPath,
		settingsUI:   newSettingsDialog(),
	}

	if currentStartup, err := startup.IsEnabled(startupValueName()); err == nil {
		globalApp.runAtStartup = currentStartup
		if currentStartup != loaded.RunAtStartup {
			_ = core.SaveConfig(cfgPath, core.Config{Enabled: loaded.Enabled, Processes: loaded.Processes, RunAtStartup: currentStartup})
		}
	} else {
		showErrorMessage(0, fmt.Sprintf("Startup status check failed: %v", err))
	}

	globalApp.hiddenProc = syscall.NewCallback(hiddenWindowProc)
	globalApp.settingsProc = syscall.NewCallback(settingsWindowProc)

	if err := registerWindowClass(hiddenClassName, globalApp.hiddenProc); err != nil {
		fmt.Fprintf(os.Stderr, "hidden class register failed: %v\n", err)
		return
	}
	if err := registerWindowClass(settingsClassName, globalApp.settingsProc); err != nil {
		fmt.Fprintf(os.Stderr, "settings class register failed: %v\n", err)
		return
	}

	hwnd, err := createHiddenWindow(hiddenClassName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create hidden window failed: %v\n", err)
		return
	}
	globalApp.hwnd = hwnd

	globalApp.taskbarCreated = registerTaskbarCreated()

	if err := globalApp.installKeyboardHook(); err != nil {
		fmt.Fprintf(os.Stderr, "keyboard hook failed: %v\n", err)
	}
	if err := globalApp.addTrayIcon(); err != nil {
		fmt.Fprintf(os.Stderr, "tray icon failed: %v\n", err)
		globalApp.uninstallKeyboardHook()
		return
	}

	runMessageLoop()

	globalApp.removeTrayIcon()
	globalApp.uninstallKeyboardHook()
}

func registerWindowClass(className string, proc uintptr) error {
	cname, _ := windows.UTF16PtrFromString(className)
	hCursor, _, _ := procLoadCursorW.Call(0, uintptr(idcArrow))
	hIcon, _, _ := procLoadIconW.Call(0, uintptr(idiApplication))

	wc := wndClassExW{
		Size:      uint32(unsafe.Sizeof(wndClassExW{})),
		WndProc:   proc,
		Instance:  0,
		Cursor:    hCursor,
		Icon:      hIcon,
		IconSm:    hIcon,
		ClassName: cname,
	}

	ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		// Ignore already-exists class registration errors.
		if err != nil && err != syscall.Errno(1410) {
			return err
		}
	}
	return nil
}

func createHiddenWindow(className string) (uintptr, error) {
	cname, _ := windows.UTF16PtrFromString(className)
	wname, _ := windows.UTF16PtrFromString(appName)

	hwnd, _, err := procCreateWindowExW.Call(
		uintptr(wsExToolWindow),
		uintptr(unsafe.Pointer(cname)),
		uintptr(unsafe.Pointer(wname)),
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
	)
	if hwnd == 0 {
		return 0, err
	}
	return hwnd, nil
}

func registerTaskbarCreated() uint32 {
	msg, _ := windows.UTF16PtrFromString("TaskbarCreated")
	ret, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(msg)))
	return uint32(ret)
}

func runMessageLoop() {
	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (a *app) installKeyboardHook() error {
	a.hookProc = syscall.NewCallback(lowLevelKeyboardProc)
	h, _, err := procSetWindowsHookExW.Call(
		uintptr(whKeyboardLL),
		a.hookProc,
		0,
		0,
	)
	if h == 0 {
		return err
	}
	a.hook = h
	return nil
}

func (a *app) uninstallKeyboardHook() {
	if a.hook == 0 {
		return
	}
	procUnhookWindowsHookEx.Call(a.hook)
	a.hook = 0
	a.hookProc = 0
}

func (a *app) addTrayIcon() error {
	hIcon := loadTrayIconHandle()

	nid := notifyIconDataW{}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = a.hwnd
	nid.UID = 1
	nid.UFlags = nifMessage | nifIcon | nifTip
	nid.UCallbackMessage = wmTrayIcon
	nid.HIcon = hIcon

	tip := windows.StringToUTF16(trayTipText)
	copy(nid.SzTip[:], tip)

	ret, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	if ret == 0 {
		return err
	}
	return nil
}

func loadTrayIconHandle() uintptr {
	// Resource icon ID 1 from winres manifest/resources.
	hIcon, _, _ := procLoadImageW.Call(
		0,
		uintptr(1),
		uintptr(imageIcon),
		0,
		0,
		uintptr(lrDefaultSize|lrShared),
	)
	if hIcon != 0 {
		return hIcon
	}
	fallback, _, _ := procLoadIconW.Call(0, uintptr(idiApplication))
	return fallback
}

func (a *app) removeTrayIcon() {
	nid := notifyIconDataW{}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = a.hwnd
	nid.UID = 1
	_ = nid
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
}

func (a *app) showTrayMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	enabled := a.isEnabled()
	toggleLabel, _ := windows.UTF16PtrFromString(menuEnabledLabel)
	toggleFlags := uintptr(mfString)
	if enabled {
		toggleFlags |= mfChecked
	} else {
		toggleFlags |= mfUnchecked
	}
	procAppendMenuW.Call(menu, toggleFlags, uintptr(idMenuToggleEnabled), uintptr(unsafe.Pointer(toggleLabel)))

	procAppendMenuW.Call(menu, uintptr(mfSeparator), 0, 0)

	settingsLabel, _ := windows.UTF16PtrFromString(menuSettingsLabel)
	procAppendMenuW.Call(menu, uintptr(mfString), uintptr(idMenuSettings), uintptr(unsafe.Pointer(settingsLabel)))

	exitLabel, _ := windows.UTF16PtrFromString(menuExitLabel)
	procAppendMenuW.Call(menu, uintptr(mfString), uintptr(idMenuExit), uintptr(unsafe.Pointer(exitLabel)))

	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	procSetForegroundWindow.Call(a.hwnd)

	cmd, _, _ := procTrackPopupMenu.Call(
		menu,
		uintptr(tpmRightButton|tpmRetCmd),
		uintptr(p.X),
		uintptr(p.Y),
		0,
		a.hwnd,
		0,
	)
	if cmd != 0 {
		procSendMessageW.Call(a.hwnd, wmCommand, cmd, 0)
	}
}

func (a *app) isEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabled
}

func (a *app) setEnabled(v bool) {
	a.mu.Lock()
	a.enabled = v
	cfg := core.Config{Enabled: a.enabled, Processes: core.FromProcessSet(a.processes), RunAtStartup: a.runAtStartup}
	a.mu.Unlock()

	_ = core.SaveConfig(a.configPath, cfg)
}

func (a *app) shouldMonitor(processName string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.processes[strings.ToLower(processName)]
	return ok
}

func (a *app) getProcesses() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return core.FromProcessSet(a.processes)
}

func (a *app) isRunAtStartup() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.runAtStartup
}

func (a *app) applySettings(processes []string, runAtStartup bool) error {
	if err := startup.SetEnabled(startupValueName(), runAtStartup); err != nil {
		return err
	}

	normalized := core.NormalizeProcesses(processes)
	newSet := core.ToProcessSet(normalized)

	a.mu.Lock()
	a.processes = newSet
	a.runAtStartup = runAtStartup
	cfg := core.Config{Enabled: a.enabled, Processes: normalized, RunAtStartup: runAtStartup}
	a.mu.Unlock()

	return core.SaveConfig(a.configPath, cfg)
}

func lowLevelKeyboardProc(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	if nCode != hcAction {
		return callNextHook(nCode, wParam, lParam)
	}
	if wParam != wmKeyDown {
		return callNextHook(nCode, wParam, lParam)
	}

	vkCode, ok := windowsapi.KeyboardVKCode(lParam)
	if !ok || vkCode != vkV || !windowsapi.IsCtrlPressed() {
		return callNextHook(nCode, wParam, lParam)
	}

	if globalApp == nil || !globalApp.isEnabled() {
		return callNextHook(nCode, wParam, lParam)
	}

	procName, err := windowsapi.GetForegroundProcessImageName()
	if err != nil || procName == "" {
		return callNextHook(nCode, wParam, lParam)
	}

	if !globalApp.shouldMonitor(procName) {
		return callNextHook(nCode, wParam, lParam)
	}

	_ = windowsapi.NormalizeClipboardCRLFToLF()
	return callNextHook(nCode, wParam, lParam)
}

func callNextHook(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

func hiddenWindowProc(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
	if globalApp == nil {
		return defWindowProc(hwnd, message, wParam, lParam)
	}

	if message == globalApp.taskbarCreated {
		_ = globalApp.addTrayIcon()
		return 0
	}

	switch message {
	case wmTrayIcon:
		if uint32(lParam) == wmRButtonUp || uint32(lParam) == wmContextMenu {
			globalApp.showTrayMenu()
		}
		return 0

	case wmCommand:
		cmdID := uint16(wParam & 0xffff)
		switch cmdID {
		case idMenuToggleEnabled:
			globalApp.setEnabled(!globalApp.isEnabled())
		case idMenuSettings:
			result, ok := globalApp.settingsUI.Show(hwnd, globalApp.getProcesses(), globalApp.isRunAtStartup())
			if ok {
				if err := globalApp.applySettings(result.Processes, result.RunAtStartup); err != nil {
					fmt.Fprintf(os.Stderr, "apply settings failed: %v\n", err)
					showErrorMessage(hwnd, fmt.Sprintf("Unable to apply settings: %v", err))
				}
			}
		case idMenuExit:
			procDestroyWindow.Call(hwnd)
		}
		return 0

	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}

	return defWindowProc(hwnd, message, wParam, lParam)
}

func defWindowProc(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func showErrorMessage(owner uintptr, message string) {
	msg, _ := windows.UTF16PtrFromString(message)
	title, _ := windows.UTF16PtrFromString(errorDialogTitle)
	procMessageBoxW.Call(owner, uintptr(unsafe.Pointer(msg)), uintptr(unsafe.Pointer(title)), mbIconError)
}

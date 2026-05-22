package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	appName                    = "win-pasterer"
	appIconResourceName        = "APP"
	hiddenClassName            = "WinPastererHiddenWindow"
	settingsClassName          = "WinPastererSettingsWindow"
	wmTrayIcon          uint32 = 0x0400 + 1 // WM_APP + 1

	// Menu command IDs.
	idMenuToggleEnabled = 1001
	idMenuSettings      = 1002
	idMenuExit          = 1003

	// Settings control IDs.
	idSettingsEdit         = 2001
	idSettingsSave         = 2002
	idSettingsCancel       = 2003
	idSettingsStartup      = 2004
	idSettingsStartupLabel = 2005
	idSettingsEnabled      = 2006
	idSettingsEnabledLabel = 2007

	// Win32 constants.
	whKeyboardLL     = 13
	hcAction         = 0
	wmDestroy        = 0x0002
	wmEraseBkgnd     = 0x0014
	wmSettingChange  = 0x001A
	wmDrawItem       = 0x002B
	wmClose          = 0x0010
	wmCommand        = 0x0111
	wmSetFont        = 0x0030
	wmCtlColorEdit   = 0x0133
	wmCtlColorBtn    = 0x0135
	wmCtlColorStatic = 0x0138
	wmDPIChanged     = 0x02E0
	wmThemeChanged   = 0x031A
	wmKeyDown        = 0x0100
	wmRButtonUp      = 0x0205
	wmContextMenu    = 0x007B
	swShow           = 5
	wsOverlapped     = 0x00000000
	wsCaption        = 0x00C00000
	wsSysMenu        = 0x00080000
	wsVisible        = 0x10000000
	wsChild          = 0x40000000
	wsBorder         = 0x00800000
	wsVScroll        = 0x00200000
	wsTabStop        = 0x00010000
	wsClipChildren   = 0x02000000
	wsClipSiblings   = 0x04000000
	bsAutoCheckbox   = 0x00000003
	bsDefPushButton  = 0x00000001
	bsOwnerDraw      = 0x0000000B

	ssNotify      = 0x00000100
	ssCenterImage = 0x00000200

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
	mfGrayed    = 0x00000001

	tpmRightButton = 0x0002
	tpmRetCmd      = 0x0100

	// Tray icon flags.
	nimAdd             = 0x00000000
	nimDelete          = 0x00000002
	nimSetVersion      = 0x00000004
	nifMessage         = 0x00000001
	nifIcon            = 0x00000002
	nifTip             = 0x00000004
	nifGuid            = 0x00000020
	notifyIconVersion4 = 4

	// Misc constants.
	idcArrow       = 32512
	idiApplication = 32512
	imageIcon      = 1
	lrDefaultSize  = 0x00000040
	lrShared       = 0x00008000
	smCxIcon       = 11
	smCxSmallIcon  = 49
	smCxScreen     = 0
	smCyScreen     = 1

	defaultCharset    = 1
	outDefaultPrecis  = 0
	clipDefaultPrecis = 0
	defaultQuality    = 0
	defaultPitch      = 0
	fwNormal          = 400
	transparent       = 1

	odtButton    = 4
	odsSelected  = 0x0001
	dtCenter     = 0x00000001
	dtVCenter    = 0x00000004
	dtSingleLine = 0x00000020

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

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type drawItemStruct struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   uintptr
	HDC        uintptr
	RcItem     rect
	ItemData   uintptr
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

type desktopServices interface {
	ForegroundProcessImageName() (string, error)
	NormalizeClipboardCRLFToLF() error
}

type keyboardStateReader interface {
	ModifierState() core.ModifierState
}

type configSaver interface {
	Save(core.Config) error
}

type startupController interface {
	IsEnabled(valueName string) (bool, error)
	SetEnabled(valueName string, enable bool) error
}

type windowsDesktopServices struct{}

func (windowsDesktopServices) ForegroundProcessImageName() (string, error) {
	return windowsapi.GetForegroundProcessImageName()
}

func (windowsDesktopServices) NormalizeClipboardCRLFToLF() error {
	return windowsapi.NormalizeClipboardCRLFToLF()
}

type windowsKeyboardState struct{}

func (windowsKeyboardState) ModifierState() core.ModifierState {
	return windowsapi.ModifierState()
}

type fileConfigSaver struct {
	path string
}

func (s fileConfigSaver) Save(cfg core.Config) error {
	return core.SaveConfig(s.path, cfg)
}

type registryStartupController struct{}

func (registryStartupController) IsEnabled(valueName string) (bool, error) {
	return startup.IsEnabled(valueName)
}

func (registryStartupController) SetEnabled(valueName string, enable bool) error {
	return startup.SetEnabled(valueName, enable)
}

type app struct {
	hwnd           uintptr
	hook           uintptr
	hookProc       uintptr
	enabled        bool
	runAtStartup   bool
	processes      map[string]struct{}
	configPath     string
	logPath        string
	taskbarCreated uint32
	hiddenProc     uintptr
	settingsProc   uintptr
	settingsUI     SettingsDialog
	desktop        desktopServices
	keyboard       keyboardStateReader
	config         configSaver
	startup        startupController

	mu sync.RWMutex
}

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")

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
	procShowWindow             = user32.NewProc("ShowWindow")
	procIsWindow               = user32.NewProc("IsWindow")
	procLoadImageW             = user32.NewProc("LoadImageW")
	procMessageBoxW            = user32.NewProc("MessageBoxW")
	procGetDpiForWindow        = user32.NewProc("GetDpiForWindow")
	procGetDpiForSystem        = user32.NewProc("GetDpiForSystem")
	procGetSystemMetrics       = user32.NewProc("GetSystemMetrics")
	procGetSystemMetricsForDpi = user32.NewProc("GetSystemMetricsForDpi")
	procIsDialogMessageW       = user32.NewProc("IsDialogMessageW")
	procMoveWindow             = user32.NewProc("MoveWindow")
	procSetWindowPos           = user32.NewProc("SetWindowPos")
	procGetClientRect          = user32.NewProc("GetClientRect")
	procInvalidateRect         = user32.NewProc("InvalidateRect")
	procDrawTextW              = user32.NewProc("DrawTextW")
	procFrameRect              = user32.NewProc("FrameRect")

	procShellNotifyIconW      = shell32.NewProc("Shell_NotifyIconW")
	procCreateFontW           = gdi32.NewProc("CreateFontW")
	procDeleteObject          = gdi32.NewProc("DeleteObject")
	procGetModuleHandleW      = kernel32.NewProc("GetModuleHandleW")
	procCreateSolidBrush      = gdi32.NewProc("CreateSolidBrush")
	procFillRect              = user32.NewProc("FillRect")
	procSetTextColor          = gdi32.NewProc("SetTextColor")
	procSetBkColor            = gdi32.NewProc("SetBkColor")
	procSetBkMode             = gdi32.NewProc("SetBkMode")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")

	globalApp *app
)

var appVersion = "dev"

var trayIconGUID = windows.GUID{
	Data1: 0x2eab7ff9,
	Data2: 0x7f2d,
	Data3: 0x4f4c,
	Data4: [8]byte{0x85, 0x9b, 0xc4, 0x69, 0xfb, 0xc7, 0x87, 0x62},
}

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
	startupCtl := registryStartupController{}
	globalApp = &app{
		enabled:      loaded.Enabled,
		runAtStartup: loaded.RunAtStartup,
		processes:    core.ToProcessSet(loaded.Processes),
		configPath:   cfgPath,
		logPath:      filepath.Join(filepath.Dir(cfgPath), "win-pasterer.log"),
		settingsUI:   newSettingsDialog(),
		desktop:      windowsDesktopServices{},
		keyboard:     windowsKeyboardState{},
		config:       fileConfigSaver{path: cfgPath},
		startup:      startupCtl,
	}

	if currentStartup, err := startupCtl.IsEnabled(startupValueName()); err == nil {
		globalApp.runAtStartup = currentStartup
		if currentStartup != loaded.RunAtStartup {
			_ = globalApp.config.Save(core.Config{Enabled: loaded.Enabled, Processes: loaded.Processes, RunAtStartup: currentStartup})
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
	hIcon := loadAppIconHandle(systemMetricForDPI(smCxIcon, getDPIForSystem()))
	hIconSm := loadAppIconHandle(systemMetricForDPI(smCxSmallIcon, getDPIForSystem()))

	wc := wndClassExW{
		Size:      uint32(unsafe.Sizeof(wndClassExW{})),
		WndProc:   proc,
		Instance:  0,
		Cursor:    hCursor,
		Icon:      hIcon,
		IconSm:    hIconSm,
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
	hIcon := loadAppIconHandle(systemMetricForDPI(smCxSmallIcon, getDPIForWindow(a.hwnd)))

	nid := notifyIconDataW{}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = a.hwnd
	nid.UID = 1
	nid.UFlags = nifMessage | nifIcon | nifTip | nifGuid
	nid.UCallbackMessage = wmTrayIcon
	nid.HIcon = hIcon
	nid.GuidItem = trayIconGUID

	tip := windows.StringToUTF16(trayTipText)
	copy(nid.SzTip[:], tip)

	ret, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	if ret == 0 {
		return err
	}
	nid.UTimeoutOrVersion = notifyIconVersion4
	procShellNotifyIconW.Call(nimSetVersion, uintptr(unsafe.Pointer(&nid)))
	return nil
}

func loadAppIconHandle(size int32) uintptr {
	if size <= 0 {
		size = systemMetricForDPI(smCxSmallIcon, getDPIForSystem())
	}
	hModule, _, _ := procGetModuleHandleW.Call(0)
	resourceName, _ := windows.UTF16PtrFromString(appIconResourceName)
	hIcon, _, _ := procLoadImageW.Call(
		hModule,
		uintptr(unsafe.Pointer(resourceName)),
		uintptr(imageIcon),
		uintptr(size),
		uintptr(size),
		0,
	)
	if hIcon != 0 {
		return hIcon
	}
	hIcon, _, _ = procLoadImageW.Call(
		hModule,
		uintptr(1),
		uintptr(imageIcon),
		uintptr(size),
		uintptr(size),
		0,
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
	nid.UFlags = nifGuid
	nid.GuidItem = trayIconGUID
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

	procAppendMenuW.Call(menu, uintptr(mfSeparator), 0, 0)

	hotkeyLabel, _ := windows.UTF16PtrFromString(menuHotkeyLabel)
	procAppendMenuW.Call(menu, uintptr(mfString|mfGrayed), 0, uintptr(unsafe.Pointer(hotkeyLabel)))

	procAppendMenuW.Call(menu, uintptr(mfSeparator), 0, 0)

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

func isTrayContextMenuEvent(lParam uintptr) bool {
	event := uint32(lParam) & 0xffff
	return event == wmRButtonUp || event == wmContextMenu
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

	_ = a.saveConfig(cfg)
}

func (a *app) shouldMonitor(processName string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return core.IsProcessMonitored(processName, a.processes)
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

func (a *app) applySettings(enabled bool, processes []string, runAtStartup bool) error {
	if a.startup == nil {
		a.startup = registryStartupController{}
	}
	if err := a.startup.SetEnabled(startupValueName(), runAtStartup); err != nil {
		return err
	}

	normalized := core.NormalizeProcesses(processes)
	newSet := core.ToProcessSet(normalized)

	a.mu.Lock()
	a.enabled = enabled
	a.processes = newSet
	a.runAtStartup = runAtStartup
	cfg := core.Config{Enabled: a.enabled, Processes: normalized, RunAtStartup: runAtStartup}
	a.mu.Unlock()

	return a.saveConfig(cfg)
}

func (a *app) saveConfig(cfg core.Config) error {
	if a.config != nil {
		return a.config.Save(cfg)
	}
	return core.SaveConfig(a.configPath, cfg)
}

func lowLevelKeyboardProc(nCode int32, wParam uintptr, lParam uintptr) (ret uintptr) {
	defer recoverCallback("keyboard hook", 0, &ret)
	if nCode != hcAction {
		return callNextHook(nCode, wParam, lParam)
	}
	if wParam != wmKeyDown {
		return callNextHook(nCode, wParam, lParam)
	}

	vkCode, ok := windowsapi.KeyboardVKCode(lParam)
	if !ok {
		return callNextHook(nCode, wParam, lParam)
	}

	if globalApp != nil {
		modifiers := windowsapi.ModifierState()
		if globalApp.keyboard != nil {
			modifiers = globalApp.keyboard.ModifierState()
		}
		globalApp.handleKeyDown(core.KeyEvent{VKCode: vkCode, Modifiers: modifiers})
	}
	return callNextHook(nCode, wParam, lParam)
}

func (a *app) handleKeyDown(event core.KeyEvent) {
	a.mu.RLock()
	enabled := a.enabled
	processes := make(map[string]struct{}, len(a.processes))
	for p := range a.processes {
		processes[p] = struct{}{}
	}
	desktop := a.desktop
	a.mu.RUnlock()

	if desktop == nil || !enabled || !core.IsPasteHotkey(event) {
		return
	}

	procName, err := desktop.ForegroundProcessImageName()
	if err != nil || procName == "" {
		return
	}

	if !core.ShouldNormalizePaste(enabled, procName, processes, event) {
		return
	}

	_ = desktop.NormalizeClipboardCRLFToLF()
}

func getDPIForSystem() int32 {
	if dpi, _, _ := procGetDpiForSystem.Call(); dpi != 0 {
		return int32(dpi)
	}
	return defaultDPI
}

func systemMetricForDPI(metric int32, dpi int32) int32 {
	if dpi <= 0 {
		dpi = defaultDPI
	}
	if procGetSystemMetricsForDpi.Find() == nil {
		if v, _, _ := procGetSystemMetricsForDpi.Call(uintptr(metric), uintptr(dpi)); v != 0 {
			return int32(v)
		}
	}
	if v, _, _ := procGetSystemMetrics.Call(uintptr(metric)); v != 0 {
		return int32(v)
	}
	return scaleDPI(16, dpi)
}

func callNextHook(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

func hiddenWindowProc(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) (ret uintptr) {
	defer recoverCallback("hidden window", hwnd, &ret)
	if globalApp == nil {
		return defWindowProc(hwnd, message, wParam, lParam)
	}

	if message == globalApp.taskbarCreated {
		_ = globalApp.addTrayIcon()
		return 0
	}

	switch message {
	case wmTrayIcon:
		if isTrayContextMenuEvent(lParam) {
			globalApp.showTrayMenu()
		}
		return 0

	case wmCommand:
		cmdID := uint16(wParam & 0xffff)
		switch cmdID {
		case idMenuToggleEnabled:
			globalApp.setEnabled(!globalApp.isEnabled())
		case idMenuSettings:
			if activateSettingsDialog() {
				return 0
			}
			result, ok, err := globalApp.showSettings(hwnd)
			if err != nil {
				reportUserVisibleError(hwnd, "Unable to open settings", err)
				return 0
			}
			if ok {
				if err := globalApp.applySettings(result.Enabled, result.Processes, result.RunAtStartup); err != nil {
					reportUserVisibleError(hwnd, "Unable to apply settings", err)
				}
			}
		case idMenuExit:
			closeSettingsDialogs()
			procDestroyWindow.Call(hwnd)
		}
		return 0

	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}

	return defWindowProc(hwnd, message, wParam, lParam)
}

func (a *app) showSettings(owner uintptr) (result SettingsResult, ok bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("settings dialog failed: %v", r)
		}
	}()
	if a.settingsUI == nil {
		return SettingsResult{}, false, fmt.Errorf("settings UI is not initialized")
	}
	return a.settingsUI.Show(owner, a.isEnabled(), a.getProcesses(), a.isRunAtStartup())
}

func defWindowProc(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func recoverCallback(context string, owner uintptr, ret *uintptr) {
	if r := recover(); r != nil {
		err := fmt.Errorf("%s callback failed: %v", context, r)
		reportUserVisibleError(owner, "win-pasterer encountered an internal error", err)
		if ret != nil {
			// A zero callback result is the least surprising fail-open value for
			// window procs and lets the keyboard hook continue to the next hook.
			*ret = 0
		}
	}
}

func reportUserVisibleError(owner uintptr, title string, err error) {
	msg := err.Error()
	if title != "" {
		msg = title + ": " + msg
	}
	if globalApp != nil {
		globalApp.logError(msg)
	}
	fmt.Fprintln(os.Stderr, msg)
	showErrorMessage(owner, msg)
}

func (a *app) logError(message string) {
	if a == nil || a.logPath == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(a.logPath), 0o755)
	f, err := os.OpenFile(a.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", message)
}

func showErrorMessage(owner uintptr, message string) {
	msg, _ := windows.UTF16PtrFromString(message)
	title, _ := windows.UTF16PtrFromString(errorDialogTitle)
	procMessageBoxW.Call(owner, uintptr(unsafe.Pointer(msg)), uintptr(unsafe.Pointer(title)), mbIconError)
}

package main

import (
	"errors"
	"testing"

	"github.com/zyzil/win-pasterer/internal/core"
)

type fakeDesktop struct {
	processName string
	processErr  error
	normalizeN  int
}

type fakeConfigSaver struct {
	cfg     core.Config
	saveN   int
	saveErr error
}

type fakeStartupController struct {
	enabled bool
	setN    int
	setErr  error
}

func (f *fakeDesktop) ForegroundProcessImageName() (string, error) {
	return f.processName, f.processErr
}

func (f *fakeDesktop) NormalizeClipboardCRLFToLF() error {
	f.normalizeN++
	return nil
}

func (f *fakeConfigSaver) Save(cfg core.Config) error {
	f.cfg = cfg
	f.saveN++
	return f.saveErr
}

func (f *fakeStartupController) IsEnabled(valueName string) (bool, error) {
	return f.enabled, nil
}

func (f *fakeStartupController) SetEnabled(valueName string, enable bool) error {
	f.enabled = enable
	f.setN++
	return f.setErr
}

func TestHandleKeyDownNormalizesForConfiguredHotkeyAndProcess(t *testing.T) {
	desktop := &fakeDesktop{processName: "ALACRITTY.EXE"}
	a := &app{
		enabled:   true,
		processes: core.ToProcessSet([]string{"alacritty.exe"}),
		desktop:   desktop,
	}
	a.handleKeyDown(core.KeyEvent{VKCode: core.VKV, Modifiers: core.ModifierState{Ctrl: true, Shift: true}})
	if desktop.normalizeN != 1 {
		t.Fatalf("NormalizeClipboardCRLFToLF called %d times, want 1", desktop.normalizeN)
	}
}

func TestHandleKeyDownIgnoresPlainCtrlV(t *testing.T) {
	desktop := &fakeDesktop{processName: "alacritty.exe"}
	a := &app{
		enabled:   true,
		processes: core.ToProcessSet([]string{"alacritty.exe"}),
		desktop:   desktop,
	}
	a.handleKeyDown(core.KeyEvent{VKCode: core.VKV, Modifiers: core.ModifierState{Ctrl: true}})
	if desktop.normalizeN != 0 {
		t.Fatalf("NormalizeClipboardCRLFToLF called %d times, want 0", desktop.normalizeN)
	}
}

func TestHandleKeyDownFailsOpenOnProcessLookupError(t *testing.T) {
	desktop := &fakeDesktop{processErr: errors.New("lookup failed")}
	a := &app{
		enabled:   true,
		processes: core.ToProcessSet([]string{"alacritty.exe"}),
		desktop:   desktop,
	}
	a.handleKeyDown(core.KeyEvent{VKCode: core.VKV, Modifiers: core.ModifierState{Ctrl: true, Shift: true}})
	if desktop.normalizeN != 0 {
		t.Fatalf("NormalizeClipboardCRLFToLF called %d times, want 0", desktop.normalizeN)
	}
}

func TestSetEnabledPersistsConfig(t *testing.T) {
	config := &fakeConfigSaver{}
	a := &app{
		enabled:      false,
		processes:    core.ToProcessSet([]string{"alacritty.exe"}),
		runAtStartup: true,
		config:       config,
	}
	a.setEnabled(true)
	if !a.isEnabled() {
		t.Fatal("expected app to be enabled")
	}
	if config.saveN != 1 {
		t.Fatalf("config saves = %d, want 1", config.saveN)
	}
	if !config.cfg.Enabled || !config.cfg.RunAtStartup || len(config.cfg.Processes) != 1 || config.cfg.Processes[0] != "alacritty.exe" {
		t.Fatalf("unexpected saved config: %#v", config.cfg)
	}
}

func TestApplySettingsUsesStartupAndConfigServices(t *testing.T) {
	config := &fakeConfigSaver{}
	startup := &fakeStartupController{}
	a := &app{
		enabled:   true,
		processes: core.ToProcessSet([]string{"alacritty.exe"}),
		config:    config,
		startup:   startup,
	}
	if err := a.applySettings(false, []string{"NOTEPAD.EXE", "bad"}, true); err != nil {
		t.Fatalf("applySettings failed: %v", err)
	}
	if startup.setN != 1 || !startup.enabled {
		t.Fatalf("startup not enabled as expected: %#v", startup)
	}
	if config.saveN != 1 || !config.cfg.RunAtStartup || len(config.cfg.Processes) != 1 || config.cfg.Processes[0] != "notepad.exe" {
		t.Fatalf("unexpected saved config: %#v", config.cfg)
	}
	if config.cfg.Enabled {
		t.Fatal("enabled setting was not persisted")
	}
	if !a.shouldMonitor("notepad.exe") {
		t.Fatal("updated process set was not applied")
	}
}

func TestIsTrayContextMenuEvent(t *testing.T) {
	if !isTrayContextMenuEvent(uintptr(wmRButtonUp)) {
		t.Fatal("expected legacy right-button event to show menu")
	}
	if !isTrayContextMenuEvent(uintptr(wmContextMenu) | (uintptr(1) << 16)) {
		t.Fatal("expected NOTIFYICON_VERSION_4 context-menu event to show menu")
	}
	if isTrayContextMenuEvent(0x0200) {
		t.Fatal("mouse move should not show menu")
	}
}

func TestTrayTooltipTextIncludesEnabledState(t *testing.T) {
	if got := trayTooltipText(true); got != "win-pasterer: enabled" {
		t.Fatalf("enabled tooltip = %q", got)
	}
	if got := trayTooltipText(false); got != "win-pasterer: disabled" {
		t.Fatalf("disabled tooltip = %q", got)
	}
}

func TestSettingsThemeForMode(t *testing.T) {
	light := settingsThemeForMode(false)
	if light.dark || light.windowColor != rgb(240, 240, 240) || light.controlColor != rgb(255, 255, 255) || light.textColor != rgb(0, 0, 0) {
		t.Fatalf("unexpected light theme: %#v", light)
	}
	dark := settingsThemeForMode(true)
	if !dark.dark || dark.windowColor == light.windowColor || dark.controlColor == light.controlColor || dark.textColor == light.textColor {
		t.Fatalf("unexpected dark theme: %#v", dark)
	}
}

# win-pasterer

A lightweight Windows tray utility written in Go that intercepts `Ctrl+V` and, for configured target apps (default `alacritty.exe`), normalizes clipboard text line endings from CRLF to LF before paste.

## Features

- Global low-level keyboard hook (`WH_KEYBOARD_LL`) for `Ctrl+V`
- Foreground process filtering by image name (case-insensitive)
- Clipboard normalization for `CF_UNICODETEXT` only (`\r\n` -> `\n`)
- Tray icon with menu:
  - `Enabled` toggle
  - `Settings`
  - `Exit`
- Settings dialog for monitored executable list
- `Run at startup` option (per-user `HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run`)
- Embedded icon + manifest resources via `go-winres`
- Modern Windows visual styles enabled (`use-common-controls-v6`)
- DPI awareness enabled in manifest (`per monitor v2`) with runtime DPI-scaled settings layout

## Project Layout

- [cmd/win-pasterer/main.go](cmd/win-pasterer/main.go): application entrypoint, window/tray event loop
- [cmd/win-pasterer/settings_dialog.go](cmd/win-pasterer/settings_dialog.go): interface-based settings dialog component
- [cmd/win-pasterer/ui_constants.go](cmd/win-pasterer/ui_constants.go): centralized UI labels and layout constants
- [internal/core/logic.go](internal/core/logic.go): pure logic helpers and normalization
- [internal/core/config_store.go](internal/core/config_store.go): config persistence and defaults
- [internal/platform/startup/registry_windows.go](internal/platform/startup/registry_windows.go): startup registry integration
- [internal/platform/windowsapi/input_clipboard_windows.go](internal/platform/windowsapi/input_clipboard_windows.go): WinAPI process/clipboard helpers
- [winres/winres.json](winres/winres.json): icon + manifest resource config
- [build.ps1](build.ps1): release-oriented Windows build script
- [scripts/generate_icons.ps1](scripts/generate_icons.ps1): placeholder icon generation

## Requirements

- Windows
- Go 1.26+
- PowerShell

## Run

```powershell
go run ./cmd/win-pasterer
```

## Build

```powershell
./build.ps1 -Clean
```

This will:

1. Generate `.syso` resources from [winres/winres.json](winres/winres.json)
2. Build `win-pasterer.exe` from `./cmd/win-pasterer`

## Tests

Unit tests only (default):

```powershell
go test ./...
```

Short mode:

```powershell
go test -short ./...
```

Integration tests (Windows side effects: clipboard + registry):

```powershell
go test -tags integration ./...
```

## Verification

Use the helper script:

```powershell
./scripts/verify.ps1
```

Optional integration pass:

```powershell
./scripts/verify.ps1 -IncludeIntegration
```

## Configuration

Runtime config is stored at:

- `%APPDATA%\\win-pasterer\\config.json`

Schema:

```json
{
  "enabled": true,
  "processes": ["alacritty.exe"],
  "runAtStartup": false
}
```

## DPI and Theming Notes

- App opts into modern visual styles via manifest (`comctl32 v6`)
- App opts into per-monitor-v2 DPI awareness via manifest
- Settings dialog creation now scales control/window coordinates based on current monitor DPI

## Security and Safety Notes

- Unsafe pointer usage is intentionally isolated to WinAPI-boundary code paths.
- Clipboard processing is limited to `CF_UNICODETEXT`.
- Hook path returns quickly and fails open (passes to next hook) on parse/lookup errors.
- Elevated target applications may still require running this app elevated for consistent behavior across integrity levels.

## Known Limitations

- Clipboard conversion currently targets only Unicode text format.
- Dynamic control re-layout after live monitor-DPI switches is currently limited (dialog is DPI-scaled at creation time).
- `go vet` may emit warnings around unavoidable Win32 `uintptr`/`unsafe.Pointer` interop in callback/syscall boundaries.

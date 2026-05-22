# win-pasterer

A lightweight Windows tray utility written in Go that intercepts `Ctrl+Shift+V` and, for configured target apps (default `alacritty.exe`), normalizes clipboard text line endings from CRLF to LF before paste.

> **Disclaimer:** This is a vibe-coded personal utility that hooks keyboard input and rewrites clipboard contents. Use it at your own risk, review the code before running it, and test it with your workflow before relying on it.

## Features

- Global low-level keyboard hook (`WH_KEYBOARD_LL`) for `Ctrl+Shift+V`
- Foreground process filtering by image name (case-insensitive)
- Clipboard normalization for `CF_UNICODETEXT` only (`\r\n` -> `\n`)
- Tray icon with menu:
  - `Enabled` toggle
  - `Settings...`
  - `Hotkey: Ctrl+Shift+V`
  - `Exit`
- Settings dialog for monitored executable list, app enabled state, and startup preference
- `Run at startup` option (per-user `HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run`)
- Embedded icon + manifest resources via `go-winres`
- Modern Windows visual styles enabled (`use-common-controls-v6`)
- DPI awareness enabled in manifest (`per monitor v2`) with runtime DPI-scaled settings layout
- Settings dialog follows Windows light/dark app theme where raw Win32 controls can be themed reliably

## Project Layout

- [cmd/win-pasterer/main.go](cmd/win-pasterer/main.go): application entrypoint, window/tray event loop
- [cmd/win-pasterer/settings_dialog.go](cmd/win-pasterer/settings_dialog.go): interface-based settings dialog component
- [cmd/win-pasterer/ui_constants.go](cmd/win-pasterer/ui_constants.go): centralized UI labels and layout constants
- [internal/core/logic.go](internal/core/logic.go): pure logic helpers and normalization
- [internal/core/config_store.go](internal/core/config_store.go): config persistence and defaults
- [internal/platform/startup/registry_windows.go](internal/platform/startup/registry_windows.go): startup registry integration
- [internal/platform/windowsapi/input_clipboard_windows.go](internal/platform/windowsapi/input_clipboard_windows.go): WinAPI process/clipboard helpers
- [winres/winres.json](winres/winres.json): icon + manifest resource config
- [VERSION](VERSION): four-part Windows version used by release builds
- [build.ps1](build.ps1): release-oriented Windows build script
- [scripts/generate_icons.ps1](scripts/generate_icons.ps1): PNG icon generation from [icon.ico](icon.ico)

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
2. Apply the version from [VERSION](VERSION) to the Windows resources and Go build metadata
3. Build a GUI-subsystem `win-pasterer.exe` from `./cmd/win-pasterer`

To override the version for a one-off build:

```powershell
./build.ps1 -Clean -Version 0.1.1.0
```

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

## Versioning and Release Build

Windows resources use a four-part version number (`major.minor.patch.build`). The checked-in version source is [VERSION](VERSION), and [build.ps1](build.ps1) passes that value to `go-winres` as both file version and product version.

To prepare a new version:

1. Update [VERSION](VERSION), for example `0.1.1.0`
2. Update the manifest identity version in [winres/winres.json](winres/winres.json) to match
3. Run `./scripts/verify.ps1 -IncludeIntegration`
4. Run `./build.ps1 -Clean`
5. Test the resulting `win-pasterer.exe` manually from the tray and settings dialog
6. Commit the version change and create a matching git tag, for example `v0.1.1`

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
- Settings dialog uses Segoe UI, tab navigation, centered placement, and DPI-scaled layout including `WM_DPICHANGED` relayout
- Settings window frame uses documented DWM immersive dark mode where available
- Tray context menu uses the native Windows menu; fully custom dark tray menus are out of scope

## Security and Safety Notes

- Unsafe pointer usage is intentionally isolated to WinAPI-boundary code paths.
- Clipboard processing is limited to `CF_UNICODETEXT`.
- Clipboard reads are bounded by the source `HGLOBAL` size and an app-level maximum before scanning UTF-16 text.
- Hook path returns quickly and fails open (passes to next hook) on parse/lookup errors.
- The app runs as invoker and uses per-user startup registration only.
- Elevated target applications may still require running this app elevated for consistent behavior across integrity levels.

## Known Limitations

- Clipboard conversion currently targets only Unicode text format.
- Clipboard conversion keeps normalized Unicode text on the clipboard and does not preserve other clipboard formats when conversion occurs.
- `go vet` may emit warnings around unavoidable Win32 `uintptr`/`unsafe.Pointer` interop in callback/syscall boundaries.

## License

MIT. See [LICENSE](LICENSE).

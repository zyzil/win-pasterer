package core

import (
	"sort"
	"strings"
)

const MaxClipboardUTF16Units = 5_000_000

type Config struct {
	Enabled      bool     `json:"enabled"`
	Processes    []string `json:"processes"`
	RunAtStartup bool     `json:"runAtStartup"`
}

type ModifierState struct {
	Ctrl  bool
	Shift bool
	Alt   bool
}

type KeyEvent struct {
	VKCode    uint32
	Modifiers ModifierState
}

const VKV uint32 = 0x56

func DefaultConfig() Config {
	return Config{Enabled: true, Processes: []string{"alacritty.exe"}, RunAtStartup: false}
}

func NormalizeProcesses(in []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" {
			continue
		}
		if !strings.HasSuffix(v, ".exe") {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return DefaultConfig().Processes
	}
	sort.Strings(out)
	return out
}

func ToProcessSet(processes []string) map[string]struct{} {
	set := make(map[string]struct{}, len(processes))
	for _, p := range NormalizeProcesses(processes) {
		set[p] = struct{}{}
	}
	return set
}

func FromProcessSet(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	if len(out) == 0 {
		return DefaultConfig().Processes
	}
	sort.Strings(out)
	return out
}

func ConvertCRLFToLF(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

func ParseProcessEditorInput(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	return NormalizeProcesses(strings.Split(normalized, "\n"))
}

func IsPasteHotkey(event KeyEvent) bool {
	return event.VKCode == VKV &&
		event.Modifiers.Ctrl &&
		event.Modifiers.Shift &&
		!event.Modifiers.Alt
}

func ShouldNormalizePaste(enabled bool, processName string, monitored map[string]struct{}, event KeyEvent) bool {
	if !enabled || !IsPasteHotkey(event) {
		return false
	}
	return IsProcessMonitored(processName, monitored)
}

func IsProcessMonitored(processName string, monitored map[string]struct{}) bool {
	_, ok := monitored[strings.ToLower(strings.TrimSpace(processName))]
	return ok
}

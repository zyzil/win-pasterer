package core

import "strings"

const MaxClipboardUTF16Units = 5_000_000

type Config struct {
	Enabled      bool     `json:"enabled"`
	Processes    []string `json:"processes"`
	RunAtStartup bool     `json:"runAtStartup"`
}

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
	return out
}

func ConvertCRLFToLF(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

func ParseProcessEditorInput(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	return NormalizeProcesses(strings.Split(normalized, "\n"))
}

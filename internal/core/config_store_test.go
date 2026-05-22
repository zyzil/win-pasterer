package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigMissingCreatesDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	got := LoadConfig(path)
	if !got.Enabled || got.RunAtStartup || len(got.Processes) != 1 || got.Processes[0] != "alacritty.exe" {
		t.Fatalf("unexpected default config: %#v", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}
}

func TestLoadConfigMalformedFallsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadConfig(path)
	if !got.Enabled || len(got.Processes) != 1 || got.Processes[0] != "alacritty.exe" {
		t.Fatalf("unexpected fallback config: %#v", got)
	}
}

func TestSaveConfigNormalizesDeterministically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	cfg := Config{Enabled: true, Processes: []string{"ZED.EXE", "bad", " alacritty.exe ", "zed.exe"}, RunAtStartup: true}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Config
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("saved invalid json: %v", err)
	}
	want := []string{"alacritty.exe", "zed.exe"}
	if len(got.Processes) != len(want) || got.Processes[0] != want[0] || got.Processes[1] != want[1] {
		t.Fatalf("processes not normalized deterministically: %#v", got.Processes)
	}
	if !got.RunAtStartup {
		t.Fatal("runAtStartup not persisted")
	}
}

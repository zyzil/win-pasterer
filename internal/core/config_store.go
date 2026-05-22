package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ConfigPath(appName string) (string, error) {
	appData := os.Getenv("APPDATA")
	if strings.TrimSpace(appData) == "" {
		return "", fmt.Errorf("APPDATA not set")
	}
	dir := filepath.Join(appData, appName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LoadConfig(path string) Config {
	cfg := DefaultConfig()
	b, err := os.ReadFile(path)
	if err != nil {
		_ = SaveConfig(path, cfg)
		return cfg
	}

	var parsed Config
	if err := json.Unmarshal(b, &parsed); err != nil {
		_ = SaveConfig(path, cfg)
		return cfg
	}

	cfg.Enabled = parsed.Enabled
	cfg.Processes = NormalizeProcesses(parsed.Processes)
	cfg.RunAtStartup = parsed.RunAtStartup
	_ = SaveConfig(path, cfg)
	return cfg
}

func SaveConfig(path string, cfg Config) error {
	cfg.Processes = NormalizeProcesses(cfg.Processes)
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTmp = false
	return nil
}

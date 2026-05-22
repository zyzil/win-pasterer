package startup

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const runKeyPath = `Software\\Microsoft\\Windows\\CurrentVersion\\Run`

func QuoteForCmd(path string) string {
	if strings.HasPrefix(path, `"`) && strings.HasSuffix(path, `"`) {
		return path
	}
	return `"` + path + `"`
}

func commandForCurrentExe() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return "", fmt.Errorf("empty executable path")
	}
	return QuoteForCmd(exePath), nil
}

func SetEnabled(valueName string, enable bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if !enable {
		err = k.DeleteValue(valueName)
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}

	cmd, err := commandForCurrentExe()
	if err != nil {
		return err
	}
	return k.SetStringValue(valueName, cmd)
}

func IsEnabled(valueName string) (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	defer k.Close()

	v, _, err := k.GetStringValue(valueName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(v) != "", nil
}

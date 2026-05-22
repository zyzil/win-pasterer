//go:build windows && integration
// +build windows,integration

package startup

import (
	"fmt"
	"testing"
	"time"
)

func TestSetEnabledRoundTrip(t *testing.T) {
	name := fmt.Sprintf("win-pasterer-test-%d", time.Now().UnixNano())
	defer func() {
		_ = SetEnabled(name, false)
	}()

	if err := SetEnabled(name, true); err != nil {
		t.Fatalf("SetEnabled(true) failed: %v", err)
	}
	enabled, err := IsEnabled(name)
	if err != nil {
		t.Fatalf("IsEnabled after set failed: %v", err)
	}
	if !enabled {
		t.Fatalf("expected startup value %q to be enabled", name)
	}

	if err := SetEnabled(name, false); err != nil {
		t.Fatalf("SetEnabled(false) failed: %v", err)
	}
	enabled, err = IsEnabled(name)
	if err != nil {
		t.Fatalf("IsEnabled after delete failed: %v", err)
	}
	if enabled {
		t.Fatalf("expected startup value %q to be disabled", name)
	}
}

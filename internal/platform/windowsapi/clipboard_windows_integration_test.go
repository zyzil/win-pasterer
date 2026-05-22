//go:build windows && integration
// +build windows,integration

package windowsapi

import "testing"

func TestNormalizeClipboardCRLFToLFIntegration(t *testing.T) {
	original, readErr := readClipboardUnicodeText()
	hadOriginal := readErr == nil && original != ""
	defer func() {
		if hadOriginal {
			_ = writeClipboardUnicodeText(original)
		}
	}()

	if err := writeClipboardUnicodeText("line1\r\nline2\r\n"); err != nil {
		t.Fatalf("writeClipboardUnicodeText failed: %v", err)
	}
	if err := NormalizeClipboardCRLFToLF(); err != nil {
		t.Fatalf("NormalizeClipboardCRLFToLF failed: %v", err)
	}

	got, err := readClipboardUnicodeText()
	if err != nil {
		t.Fatalf("readClipboardUnicodeText failed: %v", err)
	}
	if got != "line1\nline2\n" {
		t.Fatalf("unexpected clipboard value: %q", got)
	}
}

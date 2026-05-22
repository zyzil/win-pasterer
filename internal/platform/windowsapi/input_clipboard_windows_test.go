package windowsapi

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/zyzil/win-pasterer/internal/core"
)

func TestSafeUTF16FromGlobalMemory(t *testing.T) {
	data := []uint16{'a', '\r', '\n', 'b', 0}
	got, err := safeUTF16FromGlobalMemory(uintptr(unsafe.Pointer(&data[0])), len(data))
	if err != nil {
		t.Fatalf("safeUTF16FromGlobalMemory failed: %v", err)
	}
	if got != "a\r\nb" {
		t.Fatalf("unexpected text: %q", got)
	}
}

func TestSafeUTF16FromGlobalMemoryRejectsInvalidInputs(t *testing.T) {
	if _, err := safeUTF16FromGlobalMemory(0, 1); err == nil {
		t.Fatal("expected nil pointer error")
	}
	data := []uint16{'a'}
	if _, err := safeUTF16FromGlobalMemory(uintptr(unsafe.Pointer(&data[0])), 0); err == nil {
		t.Fatal("expected invalid max units error")
	}
	if _, err := safeUTF16FromGlobalMemory(uintptr(unsafe.Pointer(&data[0])), len(data)); err == nil || !strings.Contains(err.Error(), errClipboardUnterm) {
		t.Fatalf("expected unterminated error, got %v", err)
	}
}

func TestClipboardUTF16UnitLimitUsesGlobalSize(t *testing.T) {
	hSmall, _, _ := procGlobalAlloc.Call(gmemMoveable, 10)
	if hSmall == 0 {
		t.Fatal("GlobalAlloc failed")
	}
	defer procGlobalFree.Call(hSmall)

	got, err := clipboardUTF16UnitLimit(hSmall, 100)
	if err != nil {
		t.Fatalf("clipboardUTF16UnitLimit failed: %v", err)
	}
	if got != 5 {
		t.Fatalf("units = %d, want 5", got)
	}

	hLarge, _, _ := procGlobalAlloc.Call(gmemMoveable, 20)
	if hLarge == 0 {
		t.Fatal("GlobalAlloc failed")
	}
	defer procGlobalFree.Call(hLarge)

	got, err = clipboardUTF16UnitLimit(hLarge, 3)
	if err != nil {
		t.Fatalf("clipboardUTF16UnitLimit failed: %v", err)
	}
	if got != 3 {
		t.Fatalf("units = %d, want app limit 3", got)
	}
}

func TestClipboardUTF16UnitLimitRejectsInvalidInputs(t *testing.T) {
	if _, err := clipboardUTF16UnitLimit(0, 100); err == nil || !strings.Contains(err.Error(), errInvalidGlobalSize) {
		t.Fatalf("expected invalid global size error, got %v", err)
	}
	if _, err := clipboardUTF16UnitLimit(1, 0); err == nil || !strings.Contains(err.Error(), errInvalidMaxUnits) {
		t.Fatalf("expected invalid max units error, got %v", err)
	}
}

func TestModifierStateTypeContract(t *testing.T) {
	state := core.ModifierState{Ctrl: true, Shift: true}
	if !state.Ctrl || !state.Shift || state.Alt {
		t.Fatalf("unexpected modifier state: %#v", state)
	}
}

package core

import "testing"

func TestNormalizeProcesses(t *testing.T) {
	got := NormalizeProcesses([]string{"NOTEPAD.EXE", "ALACRITTY.EXE", " alacritty.exe ", "bad", ""})
	if len(got) != 2 {
		t.Fatalf("expected 2 processes, got %d: %#v", len(got), got)
	}
	if got[0] != "alacritty.exe" || got[1] != "notepad.exe" {
		t.Fatalf("unexpected normalized order/content: %#v", got)
	}
}

func TestNormalizeProcessesFallback(t *testing.T) {
	got := NormalizeProcesses([]string{"bad", "", "foo"})
	if len(got) != 1 || got[0] != "alacritty.exe" {
		t.Fatalf("expected default fallback, got %#v", got)
	}
}

func TestConvertCRLFToLF(t *testing.T) {
	input := "a\r\nb\r\n"
	got := ConvertCRLFToLF(input)
	if got != "a\nb\n" {
		t.Fatalf("unexpected conversion result: %q", got)
	}
}

func TestParseProcessEditorInput(t *testing.T) {
	in := "alacritty.exe\r\n\r\nNOTEPAD.EXE\ninvalid"
	got := ParseProcessEditorInput(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %#v", got)
	}
	if got[0] != "alacritty.exe" || got[1] != "notepad.exe" {
		t.Fatalf("unexpected entries: %#v", got)
	}
}

func TestIsPasteHotkey(t *testing.T) {
	tests := []struct {
		name  string
		event KeyEvent
		want  bool
	}{
		{
			name:  "ctrl shift v",
			event: KeyEvent{VKCode: VKV, Modifiers: ModifierState{Ctrl: true, Shift: true}},
			want:  true,
		},
		{
			name:  "ctrl v",
			event: KeyEvent{VKCode: VKV, Modifiers: ModifierState{Ctrl: true}},
			want:  false,
		},
		{
			name:  "shift v",
			event: KeyEvent{VKCode: VKV, Modifiers: ModifierState{Shift: true}},
			want:  false,
		},
		{
			name:  "alt ctrl shift v",
			event: KeyEvent{VKCode: VKV, Modifiers: ModifierState{Ctrl: true, Shift: true, Alt: true}},
			want:  false,
		},
		{
			name:  "wrong key",
			event: KeyEvent{VKCode: 'C', Modifiers: ModifierState{Ctrl: true, Shift: true}},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPasteHotkey(tt.event); got != tt.want {
				t.Fatalf("IsPasteHotkey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldNormalizePaste(t *testing.T) {
	monitored := ToProcessSet([]string{"alacritty.exe"})
	event := KeyEvent{VKCode: VKV, Modifiers: ModifierState{Ctrl: true, Shift: true}}
	if !ShouldNormalizePaste(true, "ALACRITTY.EXE", monitored, event) {
		t.Fatal("expected monitored process and hotkey to normalize")
	}
	if ShouldNormalizePaste(false, "alacritty.exe", monitored, event) {
		t.Fatal("disabled app should not normalize")
	}
	if ShouldNormalizePaste(true, "notepad.exe", monitored, event) {
		t.Fatal("unmonitored process should not normalize")
	}
}

func TestIsProcessMonitored(t *testing.T) {
	monitored := ToProcessSet([]string{"alacritty.exe"})
	if !IsProcessMonitored(" ALACRITTY.EXE ", monitored) {
		t.Fatal("expected process match to trim and lowercase")
	}
	if IsProcessMonitored("notepad.exe", monitored) {
		t.Fatal("unexpected process match")
	}
}

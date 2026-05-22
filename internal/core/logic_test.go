package core

import "testing"

func TestNormalizeProcesses(t *testing.T) {
	got := NormalizeProcesses([]string{"ALACRITTY.EXE", " alacritty.exe ", "notepad.exe", "bad", ""})
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

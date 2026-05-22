package startup

import "testing"

func TestQuoteForCmd(t *testing.T) {
	got := QuoteForCmd(`C:\\Program Files\\App\\app.exe`)
	if got != `"C:\\Program Files\\App\\app.exe"` {
		t.Fatalf("unexpected quoted command: %s", got)
	}
}

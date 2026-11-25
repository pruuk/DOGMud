package refusal

import "testing"

func TestStaleButtonLine(t *testing.T) {
	if StaleButtonLine != `That button was out of date. Try again.` {
		t.Fatalf("StaleButtonLine = %q", StaleButtonLine)
	}
}

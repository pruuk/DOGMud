package gmcp

import (
	"strings"
	"testing"
)

// #555: on a GMCP cache miss the Mudlet-only refusal read "You are currently
// using: " with nothing after it. It names the client only when it is known.
func TestMudletOnlyRefusal_NamesOnlyAKnownClient(t *testing.T) {
	if got := mudletOnlyRefusal("TinTin++", true); !strings.Contains(got, "You are currently using: TinTin++") {
		t.Errorf("a known client is named: %q", got)
	}
	for _, c := range []struct {
		name  string
		known bool
	}{{"", false}, {"", true}, {"Stale", false}} {
		got := mudletOnlyRefusal(c.name, c.known)
		if strings.Contains(got, "currently using") {
			t.Errorf("name %q known %v: %q names no client and must not say so", c.name, c.known, got)
		}
		if !strings.Contains(got, "only available for Mudlet clients") {
			t.Errorf("the refusal itself is kept: %q", got)
		}
	}
}

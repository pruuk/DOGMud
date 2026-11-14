package main

import (
	"os"
	"regexp"
	"testing"
)

// Source guards for #868: the web client's password field follows the prompt
// only if the world goroutine syncs it.
func TestWorldSyncsTextMask(t *testing.T) {
	raw, err := os.ReadFile(`world.go`)
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)

	// processInput defers the sync right after the nil-user return.
	re := regexp.MustCompile(`(?s)func \(w \*World\) processInput\(.*?\n\tuser := users\.GetByUserId\(userId\)\n\tif user == nil \{[^}]*\n\t\treturn\n\t\}\n(?:\s*//[^\n]*\n)*\s*defer user\.SyncTextMask\(\)`)
	if !re.MatchString(src) {
		t.Error(`processInput must "defer user.SyncTextMask()" right after the nil-user return`)
	}

	// enterWorld (every login and reconnect) syncs once.
	re = regexp.MustCompile(`(?s)func \(w \*World\) enterWorld\(.*?userInfo\.SyncTextMask\(\)`)
	if !re.MatchString(src) {
		t.Error(`enterWorld must call userInfo.SyncTextMask()`)
	}
}

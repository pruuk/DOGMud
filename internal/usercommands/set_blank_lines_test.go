package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// TestSetStatusScreenKeepsItsBlankLines pins #610 on one real screen. The
// `set` status screen spaces each setting with a deliberate blank line.
// Since the categorized pipeline (T9, May 2026) a literal empty SendText
// delivers nothing, so the screen ran together; the blank lines now go
// through UserRecord.SendBlankLine, which delivers exactly one empty line.
func TestSetStatusScreenKeepsItsBlankLines(t *testing.T) {
	u := users.NewTestUser(8802, "spacer", "Spacer", 18802)
	events.DrainQueuedMessagesForTest(u.UserId)

	displaySetStatus(u)
	msgs := events.DrainQueuedMessagesForTest(u.UserId)

	blank := 0
	for _, m := range msgs {
		if m == "\n" {
			blank++
		}
	}
	// displaySetStatus sends nine blank lines itself, one after each of
	// description, ScreenReader, charset, prompt, fprompt, wimpy, submission,
	// surrender and combatverbosity, and displayBoolSetting sends one after
	// each of its four settings (auction, shortadjectives, tinymap, tips).
	if blank != 13 {
		t.Fatalf("the set status screen delivered %d blank lines, want 13: %q", blank, msgs)
	}
}

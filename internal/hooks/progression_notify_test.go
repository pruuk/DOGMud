package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// fixedText is a progression line that reads no width, as the regen, crit
// and fumble lines are.
func fixedText(text string) func(int) string {
	return func(int) string { return text }
}

func TestProgressionNotifyCallback_SkillProgressCategory(t *testing.T) {
	u := users.NewTestUser(901, "prognotify", "Prognotify", 1901)
	restore := users.SeedUsersForTest(map[int]*users.UserRecord{901: u})
	defer restore()
	events.DrainQueuedMessagesForTest(901)

	ProgressionNotifyCallback(901, fixedText("BANNER"))

	got := events.DrainQueuedMessagesForTest(901)
	want := `<ansi fg="skill-progress">BANNER</ansi>` + "\n"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("queued %q, want exactly [%q]", got, want)
	}
}

// The line is laid out for the reader's own width: the callback holds the
// user, so it is the one place that knows it (characters cannot import
// users).
func TestProgressionNotifyCallback_RendersAtTheReadersWidth(t *testing.T) {
	u := users.NewTestUser(903, "prognarrow", "Prognarrow", 1903)
	u.LineWidth = 30
	restore := users.SeedUsersForTest(map[int]*users.UserRecord{903: u})
	defer restore()
	events.DrainQueuedMessagesForTest(903)

	ProgressionNotifyCallback(903, func(width int) string { return strings.Repeat("━", width) })

	got := events.DrainQueuedMessagesForTest(903)
	want := `<ansi fg="skill-progress">` + strings.Repeat("━", 30) + `</ansi>` + "\n"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("queued %q, want exactly [%q]", got, want)
	}
}

func TestProgressionNotifyCallback_UnknownUserSendsNothing(t *testing.T) {
	restore := users.SeedUsersForTest(map[int]*users.UserRecord{})
	defer restore()
	events.DrainQueuedMessagesForTest(902)

	ProgressionNotifyCallback(902, fixedText("BANNER"))

	if got := events.DrainQueuedMessagesForTest(902); len(got) != 0 {
		t.Fatalf("queued %q for a user who is not online, want nothing", got)
	}
}

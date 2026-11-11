package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/require"
)

// The room view as look draws it (plan 3b-2): the title, the prose and the
// minimap rooms.GetDetails lays out, the alerts, the roster and the exits,
// for a telnet reader at 30, 42 and 80 columns. On a screen 42 columns wide
// or narrower the minimap goes above the prose (wider, it stays beside it:
// rooms.minimapAboveWidth); at every width each alert sits between
// two rules as wide as the screen (owner, 2026-11-10).

// thornwallBankDescription is the bank's prose as the 2026-11-10 capture
// showed it.
const thornwallBankDescription = "A solid stone building with iron-banded doors and narrow windows. Inside, a long counter of dark wood separates the public lobby from the vault area behind. Heavy ledgers line the shelves, and the faint clink of counted coins drifts from a back room. The air smells of ink, old paper, and the metallic tang of gold. A large iron strongbox sits against the far wall, its lock the size of a fist."

// roomView runs look on user 1's room, made the Thornwall Bank with both
// alerts, at a reader width, and returns each send.
func roomView(t *testing.T, width int) []string {
	t.Helper()
	user, room := getTestUserAndRoom(t)
	prev := *room
	prevWidth := user.LineWidth
	t.Cleanup(func() {
		room.Title, room.Zone, room.Description = prev.Title, prev.Zone, prev.Description
		room.IsBank, room.IsStorage = prev.IsBank, prev.IsStorage
		user.LineWidth = prevWidth
	})
	room.Title, room.Zone, room.Description = "Thornwall Bank", "Thornwall City", thornwallBankDescription
	room.IsBank, room.IsStorage = true, true
	user.LineWidth = width

	events.DrainQueuedMessagesForTest(user.UserId)
	lookRoom(user, room.RoomId, true)
	return events.DrainQueuedMessagesForTest(user.UserId)
}

func TestRoomViewFitsEveryWidth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	for _, width := range narrowWidths {
		out := strings.Join(roomView(t, width), "\n")
		require.Contains(t, out, "This is a bank!", "at %d", width)
		require.Contains(t, out, "╔═════╗", "a telnet reader keeps the minimap at %d", width)
		require.LessOrEqual(t, templates.WidestLine(out), width, "room view at %d columns:\n%s", width, out)

		lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
		rules := 0
		for i, line := range lines {
			if strings.Contains(line, "─") {
				rules++
				require.Equal(t, width, templates.WidestLine(line), "alert rule %q at %d", line, width)
			}
			if strings.Contains(line, "╔═════╗") {
				// Beside the prose at 80, alone above it at 42 and 30.
				require.Equal(t, width > 42, templates.WidestLine(line) > 7, "minimap top at %d: %q", width, line)
				if width <= 42 {
					require.Contains(t, strings.Join(lines[i+7:], " "), "A solid stone building", "the prose follows the map at %d", width)
				}
			}
		}
		require.Equal(t, 4, rules, "two alerts, two rules each, at %d", width)
	}
}

// The 80-column room view is pinned: unchanged from master but for the
// alerts, which the owner ruled become full-width rules at every width.
func TestRoomViewKeepsIts80ColumnOutput(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	checkGolden(t, "room_view", strings.Join(roomView(t, 80), "\n"))
}

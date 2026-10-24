package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hintNoQuestWords is what a player with no active quest must be told (owner
// ruling 2026-10-21, #375): there are no current quests, a [!] mark beside a
// name means that person has a quest, and `talk <name>` speaks with them.
func assertHintPointsToTalk(t *testing.T, told string) {
	t.Helper()
	assert.Contains(t, told, "You don't have any current quests.")
	assert.Contains(t, told, "[!]")
	// The angle brackets travel escaped, as in every other usage line.
	assert.Contains(t, told, "talk &lt;name&gt;")
	assert.NotContains(t, told, "active quests")
}

// hintTold runs `hint` with rest for user 1 and returns what the player read.
func hintTold(t *testing.T, user *users.UserRecord, room *rooms.Room, rest string) string {
	t.Helper()
	events.DrainQueuedMessagesForTest(1)
	_, err := Hint(rest, user, room, events.EventFlag(0))
	require.NoError(t, err)
	return aimTold(1)
}

// withHintQuests loads two finished-able quests: 900003 "Lost Lamb" and
// 900004 "Broken Wheel".
func withHintQuests(t *testing.T) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	for id, name := range map[int]string{900003: "Lost Lamb", 900004: "Broken Wheel"} {
		questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
			QuestId: id,
			Name:    name,
			Steps:   []questengine.QuestStep{{Id: "start", Hint: "Find the thing."}},
		})
	}
}

// TestHint_NamedFinishedQuestSaysComplete keeps `hint <name>` and
// `hint <number>` on a finished quest reporting it complete: only the bare
// `hint` with nothing in progress points to finding a new quest.
func TestHint_NamedFinishedQuestSaysComplete(t *testing.T) {
	user, room := aimScene(t, aimFull)
	withHintQuests(t)
	user.Character.QuestProgress = map[int]string{900003: "end", 900004: "start"}
	user.Character.LastQuestId = 900003
	for _, rest := range []string{"lamb", "900003"} {
		told := hintTold(t, user, room, rest)
		assert.Contains(t, told, "Lost Lamb", rest)
		assert.Contains(t, told, "This quest is complete!", rest)
	}
	// Bare hint with one quest still in progress hints at that one.
	told := hintTold(t, user, room, "")
	assert.Contains(t, told, "Broken Wheel")
	assert.Contains(t, told, "Find the thing.")
}

func TestHint_NoActiveQuestPointsToMarkerAndTalk(t *testing.T) {
	// Path 1: the player has no quest progress at all.
	t.Run("no progress at all", func(t *testing.T) {
		user, room := aimScene(t, aimFull)
		user.Character.QuestProgress = nil
		assertHintPointsToTalk(t, hintTold(t, user, room, ""))
	})

	// Path 2: progress exists but every quest is finished, so none is active
	// and there is no focus quest.
	t.Run("every quest finished", func(t *testing.T) {
		user, room := aimScene(t, aimFull)
		user.Character.QuestProgress = map[int]string{900001: "end"}
		user.Character.LastQuestId = 900001
		assertHintPointsToTalk(t, hintTold(t, user, room, ""))
	})

	// Path 2b: the same with the finished quest's definition loaded (the
	// playtest's case, seat E). The focus falls back to the finished quest,
	// and hint used to answer "This quest is complete!" instead of saying how
	// to find a new one (R23).
	t.Run("every quest finished, definitions loaded", func(t *testing.T) {
		user, room := aimScene(t, aimFull)
		withHintQuests(t)
		user.Character.QuestProgress = map[int]string{900003: "end", 900004: "end"}
		user.Character.LastQuestId = 900004
		assertHintPointsToTalk(t, hintTold(t, user, room, ""))
	})

	// Path 3: the focused quest id has no definition loaded.
	t.Run("focused quest has no definition", func(t *testing.T) {
		user, room := aimScene(t, aimFull)
		user.Character.QuestProgress = map[int]string{900002: "somestep"}
		user.Character.LastQuestId = 900002
		assertHintPointsToTalk(t, hintTold(t, user, room, ""))
	})
}

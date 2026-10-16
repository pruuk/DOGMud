package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
)

// #371: the unknown-spell refusal echoes only the spell word the lookup fell
// back to, never the target the player typed after it (a name the player may
// only know by looking).
func TestCast_UnknownSpellEchoesOnlyTheSpellWord(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)

	prev, had := user.Character.Skills["spellcasting"]
	user.Character.SetSkill("spellcasting", 4)
	t.Cleanup(func() {
		if had {
			user.Character.Skills["spellcasting"] = prev
		} else {
			delete(user.Character.Skills, "spellcasting")
		}
	})

	events.DrainQueuedMessagesForTest(user.UserId)
	handled, err := Cast("flarbzap shadowy figure", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	got := hoodTestText(user.UserId)
	require.Contains(t, got, "There is no spell called 'flarbzap'.")
	require.NotContains(t, got, "shadowy")
	require.NotContains(t, got, "figure")
}

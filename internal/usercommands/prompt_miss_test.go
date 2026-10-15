package usercommands

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// M7 slice 2: a typed name that matches nothing reads the slice 1
// not-here line, never a question back at the player.
func TestPromptMisses_ReadNotHere(t *testing.T) {
	cases := []struct {
		name string
		cmd  UserCommand
		rest string
		want string
	}{
		{"ask", Ask, "zzyzx to sit", refusal.NotHereCreatureLine},
		{"talk", Talk, "zzyzx", refusal.NotHereCreatureLine},
		{"look", Look, "zzyzx", refusal.NotHereThingLine},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user, room := aimScene(t, aimFull)
			handled, err := c.cmd(c.rest, user, room, 0)
			require.NoError(t, err)
			require.True(t, handled)
			out := aimTold(1)
			assert.Contains(t, out, c.want)
			assert.NotContains(t, out, "zzyzx", "no echo")
			assert.NotContains(t, out, "?", "a miss is not a question")
		})
	}
}

// Bobrick (user 2) is a player in the fixture room.
func TestTalkToAPlayerSaysUseSay(t *testing.T) {
	user, room := aimScene(t, aimFull)
	handled, err := Talk("bobrick", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)
	// Raw, not aimTold: that helper strips anything shaped <...>, which
	// would eat the "<message>" placeholder in the line.
	raw := strings.Join(events.DrainQueuedMessagesForTest(1), "")
	assert.Contains(t, raw, refusal.TalkToPlayerLine)
}

// #454: a creature hidden from the viewer reads the same as a name that
// matches nothing, and never leaks its own name.
func TestPromptMisses_HiddenCreatureReadsLikeNoMatch(t *testing.T) {
	cases := []struct {
		name string
		cmd  UserCommand
		rest string // %s is the typed name
	}{
		{"ask", Ask, "%s to sit"},
		{"talk", Talk, "%s"},
		{"look", Look, "%s"},
		{"steal", Steal, "%s"},
	}
	run := func(c struct {
		name string
		cmd  UserCommand
		rest string
	}, typed string, hide bool) string {
		user, room := aimScene(t, aimFull)
		user.Character.Skills = map[string]int{string(skills.Skullduggery): 3}
		if hide {
			hideForAmbushTest(t, &mobs.GetInstance(100).Character)
		}
		_, err := c.cmd(strings.Replace(c.rest, "%s", typed, 1), user, room, 0)
		require.NoError(t, err)
		return aimTold(1)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hidden := run(c, "skeleton", true)
			nomatch := run(c, "zzyzx", false)
			require.NotEmpty(t, nomatch, "the miss must say something, or equality is vacuous")
			// A found steal target starts a pickpocket in its pause; flush it
			// so it cannot leak into the cooldown tests.
			found := run(c, "skeleton", false)
			if c.name == "steal" {
				util.LockMud()
				actions.FlushPocketAttempts()
				util.UnlockMud()
				// The attempt's goroutine untracks itself once it sees the
				// flush (as actions' waitSettled waits for).
				require.Eventually(t, func() bool { return actions.PendingPocketAttempts() == 0 },
					5*time.Second, 5*time.Millisecond, "the control's pickpocket is flushed")
			}
			assert.NotEqual(t, nomatch, found, "control: an unhidden skeleton is found, so the comparison can fail")
			assert.Equal(t, nomatch, hidden, "a hidden creature reads like no match")
			assert.NotContains(t, strings.ToLower(hidden), "skeleton")
		})
	}
}

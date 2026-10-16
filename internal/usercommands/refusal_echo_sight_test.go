package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refusalEchoRun is one scene: a fresh aimScene at band, the actor put in
// combat with the Skeleton when the command needs a current target (target
// refuses everyone who is not in combat), the command run once, and the actor
// taken back out of combat when the scene ends. Nothing is shared between
// scenes, so the order they run in cannot matter.
func refusalEchoRun(t *testing.T, band aimBand, needsCombat bool, run func(string, *users.UserRecord, *rooms.Room, events.EventFlag) (bool, error), rest string) string {
	t.Helper()
	user, room := aimScene(t, band)
	targeting.Release(user.Character, targeting.ReasonAttack)
	if needsCombat {
		require.True(t, targeting.Commit(user.Character, state.ActorRef{MobInstanceId: 100}, targeting.ReasonAttack))
		require.True(t, user.Character.IsInCombat(), "the scene needs the actor in combat")
	}
	events.DrainQueuedMessagesForTest(1)
	_, _ = run(rest, user, room, events.EventFlag(0))
	told := aimTold(1)
	targeting.Release(user.Character, targeting.ReasonAttack)
	return told
}

// partyInvite drives `party invite <rest>` as a one-word command for the
// sight tables below.
func partyInvite(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	return Party("invite "+rest, user, room, flags)
}

// M7 spec section 2 (#454): a creature that is there but unseen and a name
// that matches nothing read the same, and neither echoes what was typed.
// Each command is driven in the dark at the Skeleton (there) and at a name
// that matches nothing, and under full light at the name that matches nothing.
func TestRefusalNeverEchoesThroughSight(t *testing.T) {
	cmds := []struct {
		name        string
		run         func(string, *users.UserRecord, *rooms.Room, events.EventFlag) (bool, error)
		needsCombat bool
	}{
		{"target", Target, true},
		{"consider", Consider, false},
		{"party invite", partyInvite, false},
	}
	for _, c := range cmds {
		there := refusalEchoRun(t, aimDark, c.needsCombat, c.run, "skeleton")
		nothing := refusalEchoRun(t, aimDark, c.needsCombat, c.run, "zzyzx")
		assert.Equal(t, there, nothing, "%s: an unseen creature and no match must read the same", c.name)
		assert.Contains(t, there, refusal.NotHereCreatureLine, c.name)
		assert.NotContains(t, strings.ToLower(there), "skeleton", c.name)
		litNothing := refusalEchoRun(t, aimFull, c.needsCombat, c.run, "zzyzx")
		assert.Contains(t, litNothing, refusal.NotHereCreatureLine, "%s in light", c.name)
		assert.NotContains(t, strings.ToLower(litNothing), "zzyzx", "%s echoed a name the player looked for", c.name)
	}
}

// The shapes-only band: a typed name gets the shapes hint, and a creature
// that is there ("skeleton") and a name that matches nothing ("zzyzx") read
// identically and never echo what was typed.
func TestRefusalNeverEchoesAtShapes(t *testing.T) {
	cmds := []struct {
		name        string
		run         func(string, *users.UserRecord, *rooms.Room, events.EventFlag) (bool, error)
		needsCombat bool
	}{
		{"target", Target, true},
		{"consider", Consider, false},
		{"party invite", partyInvite, false},
	}
	for _, c := range cmds {
		there := refusalEchoRun(t, aimShapes, c.needsCombat, c.run, "skeleton")
		nothing := refusalEchoRun(t, aimShapes, c.needsCombat, c.run, "zzyzx")
		assert.Equal(t, there, nothing, "%s: a creature that is there and no match must read the same at shapes", c.name)
		assert.Contains(t, there, aimTagPattern.ReplaceAllString(actions.AimShapesHint(c.name), ``), "%s: the shapes hint", c.name)
		assert.NotContains(t, strings.ToLower(there), "skeleton", c.name)
		assert.NotContains(t, strings.ToLower(nothing), "zzyzx", "%s echoed a typed name at shapes", c.name)
	}
}

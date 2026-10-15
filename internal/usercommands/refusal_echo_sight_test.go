package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// M7 spec section 2 (#454): a creature that is there but unseen and a name
// that matches nothing read the same, and neither echoes what was typed.
// Each command is driven in the dark at the Skeleton (there) and at a name
// that matches nothing, and under full light at the name that matches nothing.
func TestRefusalNeverEchoesThroughSight(t *testing.T) {
	cmds := map[string]func(string) string{
		"target": func(rest string) string {
			user, room := aimScene(t, aimDark)
			require.True(t, targeting.Commit(user.Character, state.ActorRef{MobInstanceId: 100}, targeting.ReasonAttack))
			_, _ = Target(rest, user, room, events.EventFlag(0))
			return aimTold(1)
		},
		"consider": func(rest string) string {
			user, room := aimScene(t, aimDark)
			_, _ = Consider(rest, user, room, events.EventFlag(0))
			return aimTold(1)
		},
	}
	lit := map[string]func(string) string{
		"target": func(rest string) string {
			user, room := aimScene(t, aimFull)
			require.True(t, targeting.Commit(user.Character, state.ActorRef{MobInstanceId: 100}, targeting.ReasonAttack))
			_, _ = Target(rest, user, room, events.EventFlag(0))
			return aimTold(1)
		},
		"consider": func(rest string) string {
			user, room := aimScene(t, aimFull)
			_, _ = Consider(rest, user, room, events.EventFlag(0))
			return aimTold(1)
		},
	}
	for name, run := range cmds {
		there, nothing := run("skeleton"), run("zzyzx")
		assert.Equal(t, there, nothing, "%s: an unseen creature and no match must read the same", name)
		assert.Contains(t, there, refusal.NotHereCreatureLine, name)
		assert.NotContains(t, strings.ToLower(there), "skeleton", name)
		litNothing := lit[name]("zzyzx")
		assert.Contains(t, litNothing, refusal.NotHereCreatureLine, "%s in light", name)
		assert.NotContains(t, litNothing, "zzyzx", "%s echoed a name the player looked for", name)
	}
}

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #606: a looker whose name shares a word with a mob in the room looks at the
// mob, not at themselves. ResolveTargetActor prefers players over mobs, so the
// looker used to win. Looking at yourself by a word only you match still works.
func TestResolveLook_TheLookerDoesNotOutrankAMobByTheirOwnName(t *testing.T) {
	s := newGateScene(t, gateLit)
	s.user.Character.Name = "Gatemob Pathfinder"
	me := s.actor("player")

	res := ResolveLook(me, "gatemob")
	require.Equal(t, LookCreature, res.Kind)
	assert.False(t, res.Target.IsPlayer(),
		"look gatemob matched the looker \"Gatemob Pathfinder\" ahead of the mob Gatemob")

	self := ResolveLook(me, "pathfinder")
	require.Equal(t, LookCreature, self.Kind, "a word only the looker matches still finds the looker")
	assert.True(t, self.Target.IsPlayer())
	assert.Equal(t, gateUserId, self.Target.GetUserId())
}

package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #454: a typed name resolves only at full sight. The scene is
// seedAllRegistries' room 1: Aliceia (1, the actor), Bobrick (2) and the
// Skeleton (mob 100). Aliceia's figures at shapes are Bobrick (shape 1) and
// the Skeleton (shape 2).

const aimInfraredConditionId = 9454

type aimBand int

const (
	aimFull aimBand = iota
	aimShapes
	aimDark
)

// aimScene seeds the registries and sets room 1 to the band asked for:
// lit (lamp 60), or pitch dark with Aliceia given infrared (shapes) or not.
func aimScene(t *testing.T, band aimBand) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		aimInfraredConditionId: {ConditionId: aimInfraredConditionId, Name: "Test Infrared",
			RoundInterval: 1, TriggerCount: 1, Flags: []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))
	user := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	if band != aimFull {
		room.Lamp = nil
		room.Biome = "cave"
		require.Equal(t, 0, room.LightLevel(), "the dark bands need a pitch-dark room")
	}
	if band == aimShapes {
		require.True(t, user.Character.Conditions.AddCondition(aimInfraredConditionId, true))
	}
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
	return user, room
}

var aimTagPattern = regexp.MustCompile(`<[^>]*>`)

// aimTold is everything userId was sent since the last drain, tags stripped.
func aimTold(userId int) string {
	return aimTagPattern.ReplaceAllString(strings.Join(events.DrainQueuedMessagesForTest(userId), ""), "")
}

func TestAttackSight_ClearSightResolvesAName(t *testing.T) {
	user, room := aimScene(t, aimFull)
	_, err := Attack("skeleton", user, room, events.EventFlag(0))
	require.NoError(t, err)
	assert.Equal(t, 100, user.Character.CurrentCombatTarget().MobInstanceId)
}

func TestAttackSight_ShapesHintsANameAndTakesAShape(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	_, _ = Attack("skeleton", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "You can only make out shapes here.")
	assert.Contains(t, told, "attack shape")
	assert.Equal(t, 0, user.Character.CurrentCombatTarget().MobInstanceId, "a typed name engaged at shapes")

	_, _ = Attack("2.shape", user, room, events.EventFlag(0))
	assert.Equal(t, 100, user.Character.CurrentCombatTarget().MobInstanceId, "shape 2 is the Skeleton")
	assert.NotContains(t, aimTold(1), "Skeleton", "the attacker who aimed at a shape read its name")
}

func TestAttackSight_NoSightResolvesNothing(t *testing.T) {
	for _, rest := range []string{"skeleton", "shape", "*"} {
		user, room := aimScene(t, aimDark)
		_, _ = Attack(rest, user, room, events.EventFlag(0))
		assert.Equal(t, 0, user.Character.CurrentCombatTarget().MobInstanceId, "%q engaged with no sight", rest)
		told := aimTold(1)
		if rest == "skeleton" {
			assert.Contains(t, told, actions.AimNotHereLine)
		} else {
			assert.Contains(t, told, actions.AimNothingLine, "%q", rest)
		}
	}
}

// Every melee special stages its target through StageMeleeTarget.
func TestMeleeStageSight_KickAtEachBand(t *testing.T) {
	user, room := aimScene(t, aimFull)
	_, handled := actions.StageMeleeTarget(user, room, "skeleton", actions.MeleeTargetOpts{Verb: "kick"})
	assert.False(t, handled, "clear sight stages a named target")

	user, room = aimScene(t, aimShapes)
	_, handled = actions.StageMeleeTarget(user, room, "skeleton", actions.MeleeTargetOpts{Verb: "kick"})
	assert.True(t, handled)
	assert.Contains(t, aimTold(1), "kick 2.shape")
	_, handled = actions.StageMeleeTarget(user, room, "2.shape", actions.MeleeTargetOpts{Verb: "kick"})
	assert.False(t, handled, "a shape stages at shapes")

	user, room = aimScene(t, aimDark)
	_, handled = actions.StageMeleeTarget(user, room, "skeleton", actions.MeleeTargetOpts{Verb: "kick"})
	assert.True(t, handled, "no sight stages nothing")
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
}

// target is attack's sibling: attack hands a target switch to it.
func TestTargetSight_NoSightResolvesNothing(t *testing.T) {
	user, room := aimScene(t, aimDark)
	require.True(t, targeting.Commit(user.Character, state.ActorRef{MobInstanceId: 100}, targeting.ReasonAttack))
	_, _ = Target("bobrick", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
	assert.Equal(t, 100, user.Character.CurrentCombatTarget().MobInstanceId, "the target did not change")
}

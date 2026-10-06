package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Condition room lines describe what the room SEES ("A warm glow surrounds Alice"),
// but went out on the audio channel, which is never sight-gated, so blind and
// unsighted observers received them. M2 fixed the same defect for
// a spell's cast_observer line; these three condition phases were never touched.

// expire sets a condition's remaining triggers to the pruning threshold, so the next
// PruneConditions removes it and sends its end text. Deterministic, unlike counting
// ticks.
func expire(t *testing.T, list []*conditions.Condition, conditionId int) {
	t.Helper()
	for _, b := range list {
		if b.ConditionId == conditionId {
			b.TriggersLeft = conditions.TriggersLeftExpired
			return
		}
	}
	t.Fatalf("condition %d not found to expire", conditionId)
}

// rawLineContaining returns the first raw (still tagged) line whose plain text
// contains want, or "" if none does.
func rawLineContaining(raw []string, want string) string {
	for _, line := range raw {
		if strings.Contains(plainText(line), want) {
			return line
		}
	}
	return ""
}

func TestConditionStartRoomText_SightedObserverSeesIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	// Pins room 1 fully lit regardless of the ambient test round (see
	// combat_blind_warning_test.go).
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	drainPlain(2)

	assert.Equal(t, events.Continue, ApplyConditions(events.Condition{UserId: 1, ConditionId: glowConditionId}))
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia glows."))
}

func TestConditionStartRoomText_UnsightedObserverInTheDarkGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	drainPlain(2)

	ApplyConditions(events.Condition{UserId: 1, ConditionId: glowConditionId})
	assert.Equal(t, 0, countContaining(drainPlain(2), "glows."),
		"an observer who cannot see must not be told what a condition looks like")
}

// GRADED LIGHTING PLAN 2 renamed and flipped this test. NightVision no
// longer grants sight outright; it shifts the observer's usable band, and a
// shifted window is still blind below its floor at light 0 no matter how
// strong the shift (internal/messaging/window.go). So a nightvision-only
// observer in a pitch dark room now reads nothing, the same as an unsighted
// one; only actual room light or an infra reach change the answer.
func TestConditionStartRoomText_NightVisionAloneStillGetsNothingInTheDark(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	require.True(t, users.GetByUserId(2).Character.Conditions.AddCondition(nightEyesConditionId, true))
	drainPlain(2)

	ApplyConditions(events.Condition{UserId: 1, ConditionId: glowConditionId})
	assert.Equal(t, 0, countContaining(drainPlain(2), "glows."),
		"nightvision alone must not see room text in true darkness")
}

func TestConditionStartRoomText_MobHolderUsesTheMobTag(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	// Pins room 1 fully lit regardless of the ambient test round (see
	// combat_blind_warning_test.go).
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	events.DrainQueuedMessagesForTest(2)

	ApplyConditions(events.Condition{MobInstanceId: 100, ConditionId: glowConditionId})
	line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "Skeleton glows.")
	require.NotEmpty(t, line, "the observer must receive the mob's start text")
	assert.Contains(t, line, `fg="mobname`)
	assert.NotContains(t, line, `fg="username`,
		"a mob holder was tagged with the player colour")
}

func TestConditionTriggerRoomText_UnsightedObserverInTheDarkGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	require.True(t, users.GetByUserId(1).Character.Conditions.AddCondition(shiverConditionId, false))
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	assert.Equal(t, 0, countContaining(drainPlain(2), "shivers."))
}

func TestConditionTriggerRoomText_SightedObserverSeesIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	// Pins room 1 fully lit regardless of the ambient test round (see
	// combat_blind_warning_test.go).
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	require.True(t, users.GetByUserId(1).Character.Conditions.AddCondition(shiverConditionId, false))
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia shivers."))
}

func TestConditionEndRoomText_UnsightedObserverInTheDarkGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(fadeConditionId, false))
	expire(t, holder.Character.Conditions.List, fadeConditionId)
	drainPlain(2)

	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 0, countContaining(drainPlain(2), "fades."))
}

func TestConditionEndRoomText_SightedObserverSeesIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	// Pins room 1 fully lit regardless of the ambient test round (see
	// combat_blind_warning_test.go).
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(fadeConditionId, false))
	expire(t, holder.Character.Conditions.List, fadeConditionId)
	drainPlain(2)

	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia fades."))
}

func TestConditionEndRoomText_MobHolderIsVisualAndUsesTheMobTag(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	// Pins room 1 fully lit for the first (sighted) phase below, regardless
	// of the ambient test round (see combat_blind_warning_test.go). Cleared
	// before the second phase's darken(t, 1): Room.Lamp is a room-level
	// override that takes priority over the biome darken() sets, so leaving
	// it set would keep the room lit through the biome switch.
	room1 := rooms.LoadRoom(1)
	room1.Lamp = rooms.LampPtr(90)
	mob := mobs.GetInstance(100)
	require.True(t, mob.Character.Conditions.AddCondition(fadeConditionId, false))
	expire(t, mob.Character.Conditions.List, fadeConditionId)
	events.DrainQueuedMessagesForTest(2)

	PruneConditions(events.NewTurn{TurnNumber: 1})
	line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "Skeleton fades.")
	require.NotEmpty(t, line)
	assert.Contains(t, line, `fg="mobname`)

	// And the same line is gated by sight.
	require.True(t, mob.Character.Conditions.AddCondition(fadeConditionId, false))
	expire(t, mob.Character.Conditions.List, fadeConditionId)
	room1.Lamp = nil // clear the phase-1 pin so darken() below actually darkens
	darken(t, 1)
	drainPlain(2)
	PruneConditions(events.NewTurn{TurnNumber: 2})
	assert.Equal(t, 0, countContaining(drainPlain(2), "fades."))
}

// TestMobConditionTriggerRoomText is the D4 guard. The player round tick has always
// sent a triggered condition's trigger_observer; tickMobConditions never did, so a mob
// holding a trigger-text condition showed nothing. No mob holder of the shipped
// trigger-text conditions could be staged in a playtest, so this is its only check.
func TestMobConditionTriggerRoomText(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	// Pins room 1 fully lit regardless of the ambient test round (see
	// combat_blind_warning_test.go).
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	mob := mobs.GetInstance(100)
	require.True(t, mob.Character.Conditions.AddCondition(shiverConditionId, false))
	events.DrainQueuedMessagesForTest(2)

	tickMobConditions(mob, 100)
	raw := events.DrainQueuedMessagesForTest(2)
	line := rawLineContaining(raw, "Skeleton shivers.")
	require.NotEmpty(t, line, "a sighted observer must see the mob's trigger text")
	assert.Contains(t, line, `fg="mobname`)
	delivered := 0
	for _, l := range raw {
		if strings.Contains(plainText(l), "Skeleton shivers.") {
			delivered++
		}
	}
	assert.Equal(t, 1, delivered, "the trigger line must arrive exactly once")

	// Sight-gated like every other condition room line. Clear the pin above
	// first: Room.Lamp overrides the biome darken() sets, so leaving it
	// would keep the room lit through the biome switch.
	rooms.LoadRoom(1).Lamp = nil
	darken(t, 1)
	drainPlain(2)
	tickMobConditions(mob, 100)
	assert.Equal(t, 0, countContaining(drainPlain(2), "shivers."))
}

// A light condition's end line describes the light going out, and the moment a
// light goes out is seen by everyone in the room with working eyes. But the
// light stops counting the instant the condition EXPIRES (Conditions.HasFlag skips
// expired conditions, and expiry happens on the round tick), while its end text is
// sent later, at the turn's prune. So a plain visual send judged sight in a
// room that was already dark, and silenced the line for exactly the people
// who had been seeing by that light. Found by the Task 2 review against
// shipped condition 1, Illumination. Since #220 the round tick snapshots the
// room just before the light runs out and the prune sends against that, so
// these tests run the tick rather than expiring the record by hand.

func TestConditionEndRoomText_LightConditionEndIsSeenByItsOwnLight_Player(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(lanternConditionId, false))
	require.Greater(t, room.LightLevel(), 0, "the lantern must light the cave, or this test proves nothing")
	expireOnNextTick(t, holder.Character.Conditions.List, lanternConditionId)
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	require.Equal(t, 0, room.LightLevel(), "the light is already out once the condition expires, before any prune")
	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia's light gutters out."))
}

// TestConditionEndRoomText_LightConditionEnd_SleeperStillGetsNothing proves the light
// line is still a SIGHT line, not audio: an observer who cannot see for a
// reason other than darkness is not told.
func TestConditionEndRoomText_LightConditionEnd_SleeperStillGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(lanternConditionId, false))
	require.True(t, users.GetByUserId(2).Character.Conditions.AddCondition(dozeConditionId, true))
	expireOnNextTick(t, holder.Character.Conditions.List, lanternConditionId)
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 0, countContaining(drainPlain(2), "light gutters out"))
}

func TestConditionEndRoomText_LightConditionEndIsSeenByItsOwnLight_Mob(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	mob := mobs.GetInstance(100)
	require.True(t, mob.Character.Conditions.AddCondition(lanternConditionId, false))
	require.Greater(t, room.LightLevel(), 0, "the lantern must light the cave, or this test proves nothing")
	expireOnNextTick(t, mob.Character.Conditions.List, lanternConditionId)
	drainPlain(2)

	tickMobConditions(mob, 100)
	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Skeleton's light gutters out."))
}

// TestConditionEndRoomText_InfraredObserverDoesNotReadABareHolderName is the
// End-phase twin of the start and trigger fixes in 39b75fe07. A bare
// {actee_plain} is invisible to tag-based Anonymize, so unless the sender
// passes the holder's plain name into HideNames, a shapes-only observer reads
// the real name. Shipped condition 9 authors exactly this line.
func TestConditionEndRoomText_InfraredObserverDoesNotReadABareHolderName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(shadeConditionId, false))
	require.True(t, users.GetByUserId(2).Character.Conditions.AddCondition(heatEyesConditionId, true))
	expire(t, holder.Character.Conditions.List, shadeConditionId)
	drainPlain(2)

	PruneConditions(events.NewTurn{TurnNumber: 1})

	lines := drainPlain(2)
	require.Equal(t, 1, countContaining(lines, "emerges from the shadows"),
		"the shapes observer must still receive the line, or this test proves nothing: %v", lines)
	assert.Zero(t, countContaining(lines, "Aliceia"),
		"an infrared-only observer read the holder's bare name: %v", lines)
}

// TestConditionEndRoomText_LightEndNamesTheHolderToWhoSawThemByIt is the
// faces side of shipped condition 1 ("The glow surrounding {actee_plain}
// fades away."): a heat-eyed observer who saw the holder clearly by the ember
// itself reads the name, because the line is judged against the room just
// before the ember went out. It replaces LightPathHasNoShapesTier, which
// pinned the old as-lit judgement; the shapes side, where the bare name is
// hidden, is TestLightEndLine_ShapesWatcherDoesNotReadABareHolderName.
func TestConditionEndRoomText_LightEndNamesTheHolderToWhoSawThemByIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(emberConditionId, false))
	require.True(t, users.GetByUserId(2).Character.Conditions.AddCondition(heatEyesConditionId, true))
	expireOnNextTick(t, holder.Character.Conditions.List, emberConditionId)
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	PruneConditions(events.NewTurn{TurnNumber: 1})

	lines := drainPlain(2)
	require.Equal(t, 1, countContaining(lines, "fades away"),
		"the light line must reach the observer, or this test proves nothing: %v", lines)
	assert.Equal(t, 1, countContaining(lines, "Aliceia"),
		"the observer saw the holder clearly by the ember, so the name is expected: %v", lines)
}

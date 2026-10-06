package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// File: condition_end_snapshot_test.go
//
// #220. A light or darkness record runs out inside Conditions.Trigger on the
// round tick, and its end line goes out at the next turn's prune. The owner
// rule of 2026-10-05: a line announcing a change is judged against the state
// BEFORE it resolves. So the round tick snapshots the room just before the
// record expires and the prune sends against that snapshot. Before this, a
// light's end line was judged as if the room were lit (which told a watcher
// who never could see by it) and a darkness's by the room after it lifted
// (which told a watcher who had been blind in it).

// expireOnNextTick sets a held record one trigger from the end, at the last
// round of its interval, so the next round tick expires it through Trigger
// itself, the path that takes the snapshot.
func expireOnNextTick(t *testing.T, list []*conditions.Condition, conditionId int) {
	t.Helper()
	spec := conditions.GetConditionSpec(conditionId)
	require.NotNil(t, spec)
	for _, b := range list {
		if b.ConditionId == conditionId {
			b.TriggersLeft = 1
			b.RoundCounter = spec.RoundInterval - 1
			require.True(t, b.ExpiresOnNextTrigger(spec), "fixture: the record must run out on the next tick")
			return
		}
	}
	t.Fatalf("condition %d not found to expire", conditionId)
}

// A faint light no normal eye can see by: the watcher was blind before it
// went out, so its end line tells them nothing. Judged as lit, it did.
func TestLightEndLine_WatcherWhoCouldNotSeeByItIsNotTold(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(wickConditionId, false))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(users.GetByUserId(2).Character, room),
		"fixture: the wick must be too faint for the watcher to see by")
	expireOnNextTick(t, holder.Character.Conditions.List, wickConditionId)
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Zero(t, countContaining(drainPlain(2), "gutters out"),
		"a watcher blind before the wick went out was told it went out")
}

// The light end line has a shapes tier now: a heat-eyed watcher who made out
// the holder only as a shape by a faint light reads the line with the bare
// name hidden. Judged as lit, every reader was at faces and read the name.
// This replaces TestConditionEndRoomText_LightPathHasNoShapesTier, which
// pinned the old as-lit judgement.
func TestLightEndLine_ShapesWatcherDoesNotReadABareHolderName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	holder := users.GetByUserId(1)
	watcher := users.GetByUserId(2)
	require.True(t, holder.Character.Conditions.AddCondition(wickConditionId, false))
	require.True(t, watcher.Character.Conditions.AddCondition(heatEyesConditionId, true))
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(watcher.Character, room),
		"fixture: the watcher must make out shapes only by the wick")
	expireOnNextTick(t, holder.Character.Conditions.List, wickConditionId)
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	PruneConditions(events.NewTurn{TurnNumber: 1})

	lines := drainPlain(2)
	require.Equal(t, 1, countContaining(lines, "gutters out"),
		"the shapes watcher must still receive the line, or this test proves nothing: %v", lines)
	assert.Zero(t, countContaining(lines, "Aliceia"),
		"a shapes-only watcher read the holder's bare name: %v", lines)
}

// A darkness's end line is judged by the darker room it ended in: a watcher
// blind in that darkness is not told it lifted, though the room is lit once
// it has. Judged by the room after, they were.
func TestDarknessEndLine_WatcherBlindInTheDarknessIsNotTold(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(50)
	holder := users.GetByUserId(1)
	watcher := users.GetByUserId(2)
	require.True(t, holder.Character.Conditions.AddCondition(gloomConditionId, false))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(watcher.Character, room),
		"fixture: the gloom must blind the watcher")
	expireOnNextTick(t, holder.Character.Conditions.List, gloomConditionId)
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	require.NotEqual(t, messaging.SightNone, messaging.ParticipantSight(watcher.Character, room),
		"fixture: the room must be lit again once the gloom has run out")
	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Zero(t, countContaining(drainPlain(2), "gloom around"),
		"a watcher blind in the gloom was told it lifted")
}

// The other side of the same judgement: a watcher whose heat sight read
// shapes inside the gloom is told it lifted, with the holder unnamed.
func TestDarknessEndLine_WatcherWhoSawShapesInTheDarknessIsTold(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(80)
	holder := users.GetByUserId(1)
	watcher := users.GetByUserId(2)
	require.True(t, holder.Character.Conditions.AddCondition(gloomConditionId, false))
	require.True(t, watcher.Character.Conditions.AddCondition(heatEyesConditionId, true))
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(watcher.Character, room),
		"fixture: heat sight must read shapes inside the gloom")
	expireOnNextTick(t, holder.Character.Conditions.List, gloomConditionId)
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	PruneConditions(events.NewTurn{TurnNumber: 1})
	lines := drainPlain(2)
	assert.Equal(t, 1, countContaining(lines, "gloom around"), "%v", lines)
	assert.Zero(t, countContaining(lines, "Aliceia"), "judged at shapes, the holder is not named: %v", lines)
}

// A mob's darkness is snapshotted on the mob round tick the same way.
func TestDarknessEndLine_MobHolderIsJudgedBeforeItLifts(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(50)
	mob := mobs.GetInstance(100)
	require.Equal(t, 1, mob.Character.RoomId, "fixture: the mob must share the watcher's room")
	require.True(t, mob.Character.Conditions.AddCondition(gloomConditionId, false))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(users.GetByUserId(2).Character, room))
	expireOnNextTick(t, mob.Character.Conditions.List, gloomConditionId)
	drainPlain(2)

	tickMobConditions(mob, 100)
	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Zero(t, countContaining(drainPlain(2), "gloom around"),
		"a watcher blind in the mob's gloom was told it lifted")
}

// The snapshot belongs to the room the light went out in. A holder who walks
// into another room before the prune has their end line judged by that room
// as it is: nobody there was in the snapshot, and sending against it would
// silence the line for everyone.
func TestLightEndLine_HolderWhoMovedIsJudgedByTheNewRoom(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room1, room2 := rooms.LoadRoom(1), rooms.LoadRoom(2)
	room2.Lamp = rooms.LampPtr(60)
	holder, watcher := users.GetByUserId(1), users.GetByUserId(2)
	room1.RemovePlayer(2)
	watcher.Character.RoomId = 2
	room2.AddPlayer(2)
	require.True(t, holder.Character.Conditions.AddCondition(lanternConditionId, false))
	expireOnNextTick(t, holder.Character.Conditions.List, lanternConditionId)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	room1.RemovePlayer(1)
	holder.Character.RoomId = 2
	rooms.MarkRoomOccupancy(2, room2.AddPlayer(1), 0)
	drainPlain(2)

	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia's light gutters out."),
		"a watcher in the holder's new, lit room must read the line")
}

// No snapshot outlives the prune that follows its round: one the prune does
// not consume (here, a record revived before it could be pruned) is dropped,
// so the map cannot grow.
func TestEndLineSnapshots_DoNotOutliveThePrune(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(lanternConditionId, false))
	expireOnNextTick(t, holder.Character.Conditions.List, lanternConditionId)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	require.Len(t, endLineSnapshots, 1, "the tick must keep a snapshot for the expiring lantern")
	require.True(t, holder.Character.Conditions.AddCondition(lanternConditionId, false), "revive it before the prune")

	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Empty(t, endLineSnapshots)
}

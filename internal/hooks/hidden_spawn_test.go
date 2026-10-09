package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Sight gates playtest fixes, F5: a mob spawned with condition 9 in its
// conditionids (an ambusher) is hidden. IsHidden reads only the Awareness
// machine, and the spawn path added record 9 without ever entering Hidden,
// so the playtest's Pale Lurker was listed in the room and its emotes
// showed. Adding record 9 to a Visible character now drives Awareness into
// Hidden; once anything reveals it, the spawn-time hide is spent.

// seedHiddenSpawnCondition seeds condition 9 with its shipped flags and end
// line (_datafiles/world/dogmud/conditions/9-hidden.yaml).
func seedHiddenSpawnCondition() func() {
	return conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {ConditionId: 9, Name: "Hidden",
			Flags:       []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat},
			EndRoomText: "{actee_plain} emerges from the shadows."},
	})
}

// spawnHiddenSkeleton gives the Skeleton (instance 100, room 1) condition 9
// the way a spawn does: permanent ids, then Validate(true). User 2 reads
// faces in room 1.
func spawnHiddenSkeleton(t *testing.T) *mobs.Mob {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedHiddenSpawnCondition())
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.Validate()
	m.Character.SetPermanentConditions([]int{9})
	m.Character.Validate(true)
	events.DrainQueuedMessagesForTest(2)
	return m
}

func TestHiddenSpawn_PermanentHiddenHidesTheMob(t *testing.T) {
	m := spawnHiddenSkeleton(t)

	require.True(t, m.Character.HasCondition(9))
	require.True(t, m.Character.IsHidden(), "condition 9 from conditionids must hide the mob")
	u := users.GetByUserId(2)
	require.NotNil(t, u)
	require.False(t, u.Character.Perceives(&m.Character), "a faces reader without see-hidden does not perceive it")
}

// Entering Hidden once: the sneak path (Concealing, then Hidden) re-adds
// record 9 through the mirror cascade, and that re-add must not try to hide
// an already Hidden character again.
func TestHiddenSpawn_EntersHiddenOnce(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedHiddenSpawnCondition())
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.Validate()
	entries := 0
	m.Character.Awareness.Inner().AfterTransition("hidden_spawn_test_count",
		func(from, to awareness.State, r state.TransitionReason) {
			if to == awareness.Hidden {
				entries++
			}
		})

	hideMob(t, m)
	require.True(t, m.Character.HasCondition(9))
	require.Equal(t, 1, entries)

	m.Character.Validate(true)
	require.Equal(t, 1, entries, "a Validate on a hidden mob does not re-enter Hidden")
}

// Combat reveals a spawned-hidden mob for good: CancelCombatConditions
// strips 9 from the permanent ids, so a later Validate(true) does not hide it
// again. The end line then reads at the reader's sight of the now visible mob.
func TestHiddenSpawn_CombatRevealSticks(t *testing.T) {
	m := spawnHiddenSkeleton(t)
	require.True(t, m.Character.IsHidden())

	m.Character.RevealForCombat()
	require.False(t, m.Character.IsHidden())
	m.Character.Validate(true)
	require.False(t, m.Character.IsHidden(), "the spawn-time hide is spent once combat reveals it")

	PruneConditions(events.NewTurn{TurnNumber: 1})
	got := drainPlain(2)
	require.Equal(t, []string{"Skeleton emerges from the shadows."}, got,
		"once, naming the mob combat has already shown the room")
}

// Any other reveal (a search, a newcomer spotting it) spends it too: the
// permanent id would otherwise re-hide the mob at its next Validate(true).
func TestHiddenSpawn_SpottedStaysSpotted(t *testing.T) {
	m := spawnHiddenSkeleton(t)
	require.True(t, m.Character.IsHidden())

	require.NoError(t, m.Character.Awareness.TransitionToRevealing(
		state.TransitionReason{Trigger: awareness.TriggerObserverSearch}))
	require.False(t, m.Character.IsHidden())
	m.Character.Validate(true)
	require.False(t, m.Character.IsHidden(), "a spotted ambusher does not slip back into hiding")
}

package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobcommands"
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
	// No Validate(true) here: the reveal itself must leave no live record 9.
	// The cascade's cancel used to run its Validate(true) while 9 was still a
	// permanent id, which revived the record on a visible mob: a stray
	// "emerges from the shadows" line later, and a true mob_has_condition 9.
	// HasCondition counts an expired, unpruned record, so read the live ones.
	require.Empty(t, m.Character.GetConditions(9), "the reveal leaves no live stealth record")
	require.False(t, m.Character.HasConditionFlag(conditions.Hidden))

	// The permanent id is gone too: a later refresh does not bring 9 back.
	m.Character.Validate(true)
	require.Empty(t, m.Character.GetConditions(9), "the permanent 9 is spent, not re-added")
	require.False(t, m.Character.IsHidden(), "a spotted ambusher does not slip back into hiding")
}

// A hidden-spawned mob that acts (get, give, eat, show, aid) cancels its own
// stealth through CancelConditionsWithFlag(Hidden) directly, not through an
// Awareness transition. That cancel's Validate(true) used to revive record 9
// from the permanent id while the machine was still Hidden, so the rescue
// that drives the machine out saw a live 9 and left the mob hidden.
func TestHiddenSpawn_ActingRevealsForGood(t *testing.T) {
	m := spawnHiddenSkeleton(t)
	require.True(t, m.Character.IsHidden())
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)
	room.Gold = 10
	t.Cleanup(func() { room.Gold = 0 })

	mobcommands.Get("gold", m, room)
	require.Zero(t, room.Gold, "the mob picked up the gold")
	require.False(t, m.Character.IsHidden(), "a mob that picks something up is seen doing it")
	require.Empty(t, m.Character.GetConditions(9), "no live stealth record on a visible mob")

	m.Character.Validate(true)
	require.False(t, m.Character.IsHidden())
	require.Empty(t, m.Character.GetConditions(9))
}

// Two mobs spawned from one template that carries conditionids [9, 85] (the
// Pale Lurker). Spotting the first must spend only that instance's hide: the
// permanent list used to alias the template's ConditionIds, and the in-place
// removal rewrote the template to [85, 85], so every later spawn was visible.
func TestHiddenSpawn_RevealLeavesTheTemplateAlone(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {ConditionId: 9, Name: "Hidden",
			Flags:       []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat},
			EndRoomText: "{actee_plain} emerges from the shadows."},
		85: {ConditionId: 85, Name: "Test Permanent Companion Record"},
	}))
	// The registry holds this pointer, so spec is the template itself (a
	// GetMobSpec read would hand back a copy).
	spec := &mobs.Mob{MobId: 3, Zone: "TestZone", ConditionIds: []int{9, 85},
		Character: characters.Character{Name: "Pale Lurker"}}
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{3: spec}, map[int]*mobs.Mob{}))

	first := mobs.NewMobById(3, 1)
	require.NotNil(t, first)
	require.True(t, first.Character.IsHidden(), "a [9, 85] spawn is hidden")

	require.NoError(t, first.Character.Awareness.TransitionToRevealing(
		state.TransitionReason{Trigger: awareness.TriggerObserverSearch}))
	require.False(t, first.Character.IsHidden())
	require.Equal(t, []int{9, 85}, spec.ConditionIds, "a reveal never edits the template")

	second := mobs.NewMobById(3, 1)
	require.NotNil(t, second)
	require.True(t, second.Character.IsHidden(), "the next spawn of the same mob is still hidden")
	require.NotEmpty(t, second.Character.GetConditions(85))
}

// A hide the Awareness machine refuses (a busy character's Visible to
// Concealing is vetoed) must not leave a record 9 that claims hidden on a
// visible character.
func TestHiddenSpawn_VetoedHideLeavesNoRecord(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedHiddenSpawnCondition())
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.Validate()
	m.Character.Awareness.RegisterActivityCheck(func() bool { return false })

	m.Character.SetPermanentConditions([]int{9})
	m.Character.Validate(true)
	require.False(t, m.Character.IsHidden())
	require.Empty(t, m.Character.GetConditions(9), "no stealth record on a character the machine kept visible")
	require.False(t, m.Character.HasConditionFlag(conditions.Hidden))
}

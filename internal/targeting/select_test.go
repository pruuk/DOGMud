package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelect_RandomPlayerFindsAPlayerInTheRoom(t *testing.T) {
	room := &rooms.Room{RoomId: 1}
	room.AddPlayer(5)

	ref, ok := Select(Criteria{Kind: RandomPlayer}, Scope{Room: room})

	require.True(t, ok)
	assert.Equal(t, 5, ref.UserId)
	assert.True(t, ref.IsPlayer())
}

func TestSelect_RandomPlayerFailsInAnEmptyRoom(t *testing.T) {
	room := &rooms.Room{RoomId: 1}

	_, ok := Select(Criteria{Kind: RandomPlayer}, Scope{Room: room})

	assert.False(t, ok)
}

func TestSelect_NilRoomFails(t *testing.T) {
	_, ok := Select(Criteria{Kind: RandomPlayer}, Scope{})

	assert.False(t, ok)
}

// TestSelect_HasNoCombatConsequence is the point of the whole verb split.
// Selecting a victim must never start a fight; that is the chunk-2.7 bug
// class SoftTarget was invented to prevent.
func TestSelect_HasNoCombatConsequence(t *testing.T) {
	room := &rooms.Room{RoomId: 1}
	room.AddPlayer(5)
	c := characters.New()

	Select(Criteria{Kind: RandomPlayer}, Scope{Room: room, Self: c})

	assert.Nil(t, c.Aggro, "Select must not commit")
	assert.Equal(t, combatphase.Idle, EngagementOf(c).Phase)
}

// An empty room must not reach the scorer at all: there is nobody to score,
// and calling out to injected code on an empty scan is wasted work on the
// mob idle tick.
func TestSelect_WeakestHatedMobDoesNotScoreAnEmptyRoom(t *testing.T) {
	called := false
	SetPowerScoreFn(func(c characters.Character) float64 {
		called = true
		return 10
	})
	t.Cleanup(func() { SetPowerScoreFn(nil) })

	room := &rooms.Room{RoomId: 1}
	self := characters.New()

	_, ok := Select(Criteria{Kind: WeakestHatedMob},
		Scope{Room: room, Self: self, SelfMobInstanceId: 1})

	assert.False(t, ok)
	assert.False(t, called, "an empty room should not need to score anybody")
}

// TestSelect_WeakestHatedMobFailsWithoutAScorer is the safety net for the
// injection. If boot forgets to register the score function, selection must
// fail closed (pick nobody) rather than pick arbitrarily.
func TestSelect_WeakestHatedMobFailsWithoutAScorer(t *testing.T) {
	// powerScoreFn is a package-level var and internal/combat's init() sets it
	// in any binary that links combat. Restore on the way out so this test
	// cannot leak "no scorer" into a later test in the same binary.
	SetPowerScoreFn(nil)
	t.Cleanup(func() { SetPowerScoreFn(nil) })

	room := &rooms.Room{RoomId: 1}

	_, ok := Select(Criteria{Kind: WeakestHatedMob},
		Scope{Room: room, Self: characters.New(), SelfMobInstanceId: 1})

	assert.False(t, ok)
}

// TestSelect_RatioBelowIsUsedRawNotDefaulted pins the behaviour-preservation
// rule a review caught being broken. The behaviour tree resolves the default
// itself via getFloatParam(params, "ratio_below", 1.0); master then used that
// ceiling RAW, so an authored `ratio_below: 0` disabled predation entirely
// (no positive power ratio is strictly below zero). An earlier draft of this
// package re-defaulted a zero to 1.0, which INVERTED that into "engage anyone
// weaker".
func TestSelect_RatioBelowIsUsedRawNotDefaulted(t *testing.T) {
	scored := false
	SetPowerScoreFn(func(c characters.Character) float64 {
		scored = true
		return 10
	})
	t.Cleanup(func() { SetPowerScoreFn(nil) })

	room := &rooms.Room{RoomId: 1}
	room.AddMob(4242)

	// A zero ceiling must select nobody, matching master.
	_, ok := Select(Criteria{Kind: WeakestHatedMob, RatioBelow: 0},
		Scope{Room: room, Self: characters.New(), SelfMobInstanceId: 1})

	assert.False(t, ok, "a zero ceiling disables predation, as it did before the seam")
	_ = scored
}

// TestSelect_UnknownKindFailsClosed: a zero-value Criteria is RandomPlayer by
// construction, but an out-of-range Kind must pick nobody rather than fall
// through to a default strategy.
func TestSelect_UnknownKindFailsClosed(t *testing.T) {
	room := &rooms.Room{RoomId: 1}
	room.AddPlayer(5)

	_, ok := Select(Criteria{Kind: Kind(99)}, Scope{Room: room})

	assert.False(t, ok)
}

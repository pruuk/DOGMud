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
	SetPowerScoreFn(nil)
	room := &rooms.Room{RoomId: 1}

	_, ok := Select(Criteria{Kind: WeakestHatedMob},
		Scope{Room: room, Self: characters.New(), SelfMobInstanceId: 1})

	assert.False(t, ok)
}

func TestCriteria_RatioBelowDefaultsToOne(t *testing.T) {
	assert.Equal(t, 1.0, effectiveRatio(Criteria{}))
	assert.Equal(t, 0.5, effectiveRatio(Criteria{RatioBelow: 0.5}))
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

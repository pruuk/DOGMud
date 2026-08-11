package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Chunk 5.14 — attacking a mob must pin it in place.
//
// `attack` sets only the PLAYER's aggro (usercommands/attack.go:212). The mob
// does not learn it is engaged until combat resolves on the NEXT round, in
// Phase 7 handleAggroAndAssist (NewRound_DoCombat_unified.go:185). IdleMobs
// gated solely on the mob's own IsInCombat() (NewRound_IdleMobs.go:63), so in
// that one-round window the mob was still "idle", a goal planner could move it,
// and the queued attack then found no target. No aggro was ever established and
// the loop repeated — reported from play as ten minutes of chasing a shaman
// with zero hits landed.
// ---------------------------------------------------------------------------

// stubTargets builds the two injected lookups from a simple room->users map and
// a user->target map.
func stubTargets(players map[int][]int, targets map[int]state.ActorRef) (
	func(int) []int, func(int) state.ActorRef,
) {
	return func(roomId int) []int { return players[roomId] },
		func(userId int) state.ActorRef { return targets[userId] }
}

func TestMobIsTargetedInRoom_PlayerTargetingMob(t *testing.T) {
	playersIn, targetOf := stubTargets(
		map[int][]int{10: {1}},
		map[int]state.ActorRef{1: {MobInstanceId: 55}},
	)
	assert.True(t, mobIsTargetedInRoom(55, 10, playersIn, targetOf),
		"a mob targeted by a player in its room must be pinned")
}

func TestMobIsTargetedInRoom_PlayerTargetingSomethingElse(t *testing.T) {
	playersIn, targetOf := stubTargets(
		map[int][]int{10: {1}},
		map[int]state.ActorRef{1: {MobInstanceId: 99}},
	)
	assert.False(t, mobIsTargetedInRoom(55, 10, playersIn, targetOf),
		"an untargeted mob must stay free to act")
}

func TestMobIsTargetedInRoom_NoPlayersInRoom(t *testing.T) {
	playersIn, targetOf := stubTargets(nil, nil)
	assert.False(t, mobIsTargetedInRoom(55, 10, playersIn, targetOf))
}

// One of several players in the room is enough.
func TestMobIsTargetedInRoom_OneOfSeveralPlayers(t *testing.T) {
	playersIn, targetOf := stubTargets(
		map[int][]int{10: {1, 2, 3}},
		map[int]state.ActorRef{
			1: {MobInstanceId: 99},
			2: {}, // idle player, zero-value ref
			3: {MobInstanceId: 55},
		},
	)
	assert.True(t, mobIsTargetedInRoom(55, 10, playersIn, targetOf))
}

// THE trap. A player with no target yields a zero-value ActorRef whose
// MobInstanceId is 0. Without the mobInstanceId < 1 guard, a mob with a zero
// instance id would match every idle player and pin itself forever.
func TestMobIsTargetedInRoom_ZeroMobIdDoesNotMatchIdlePlayers(t *testing.T) {
	playersIn, targetOf := stubTargets(
		map[int][]int{10: {1, 2}},
		map[int]state.ActorRef{
			1: {}, // no target
			2: {}, // no target
		},
	)
	assert.False(t, mobIsTargetedInRoom(0, 10, playersIn, targetOf),
		"a zero mob instance id must not match idle players' zero-value targets")
}

// A player targeting the mob but standing in a different room must not pin it —
// the room lookup is what scopes this.
func TestMobIsTargetedInRoom_TargetingPlayerElsewhereDoesNotPin(t *testing.T) {
	playersIn, targetOf := stubTargets(
		map[int][]int{10: {}, 11: {1}},
		map[int]state.ActorRef{1: {MobInstanceId: 55}},
	)
	assert.False(t, mobIsTargetedInRoom(55, 10, playersIn, targetOf),
		"a targeting player in another room must not pin this mob")
}

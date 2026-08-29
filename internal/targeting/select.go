package targeting

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Kind names a selection strategy.
type Kind int

const (
	RandomPlayer    Kind = iota // any player in the room
	WeakestHatedMob             // the weakest mob this actor hates
)

// Criteria says WHAT to look for.
type Criteria struct {
	Kind Kind

	// RatioBelow caps WeakestHatedMob: only candidates whose power ratio
	// against Self is strictly below this are eligible.
	//
	// REQUIRED for WeakestHatedMob, and used RAW. It is NOT defaulted here.
	// The behaviour tree resolves the default itself with
	// getFloatParam(params, "ratio_below", 1.0), and defaulting a second time
	// in this package would silently flip an authored `ratio_below: 0` from
	// "predation disabled" (nothing can be strictly below zero) to "engage
	// anyone weaker". That inversion was caught in review before it shipped;
	// no live content sets zero today, but the seam must not invent semantics
	// the code it replaced did not have.
	RatioBelow float64
}

// Scope says WHERE to look, and on whose behalf.
type Scope struct {
	Room *rooms.Room
	Self *characters.Character

	// SelfMobInstanceId is set when Self is a mob, so strategies that must
	// skip the actor itself can do so. Zero for players.
	SelfMobInstanceId int
}

// Select answers "who should I fight?" and has NO combat consequence.
//
// It never writes state. Committing to what it returns is Commit's job, and
// keeping the two apart is what lets a thief archetype pick a victim without
// starting a fight.
//
// Returns ok=false when nothing matches. Callers must treat that as a normal
// outcome, not an error.
func Select(c Criteria, s Scope) (state.ActorRef, bool) {
	if s.Room == nil {
		return state.ActorRef{}, false
	}
	switch c.Kind {
	case RandomPlayer:
		return selectRandomPlayer(s)
	case WeakestHatedMob:
		return selectWeakestHatedMob(c, s)
	}
	return state.ActorRef{}, false
}

func selectRandomPlayer(s Scope) (state.ActorRef, bool) {
	playerIds := s.Room.GetPlayers()
	if len(playerIds) == 0 {
		return state.ActorRef{}, false
	}
	return state.ActorRef{UserId: playerIds[util.Rand(len(playerIds))]}, true
}

// selectWeakestHatedMob mirrors actTargetWeakestMobInRoom's rules exactly:
// skip self, dead mobs, non-combatants, mobs HatesMob rejects, and (when the
// caller is itself charmed) fellow companions of the same owner. Players are
// never scanned; predation is a mob-vs-mob action.
//
// Fails closed when no score function is registered. Picking arbitrarily
// would silently change which mob gets eaten.
func selectWeakestHatedMob(c Criteria, s Scope) (state.ActorRef, bool) {
	if powerScoreFn == nil || s.Self == nil {
		return state.ActorRef{}, false
	}
	self := mobs.GetInstance(s.SelfMobInstanceId)
	if self == nil || self.IsNonCombatant() {
		return state.ActorRef{}, false
	}
	selfPower := powerScoreFn(*s.Self)
	if selfPower <= 0 {
		return state.ActorRef{}, false
	}

	callerCharmedBy := s.Self.GetCharmedUserId()
	bestId := 0
	bestRatio := c.RatioBelow

	for _, otherId := range s.Room.GetMobs() {
		if otherId == s.SelfMobInstanceId {
			continue
		}
		other := mobs.GetInstance(otherId)
		if other == nil || other.IsNonCombatant() || other.Character.Health <= 0 {
			continue
		}
		if callerCharmedBy > 0 && other.Character.IsCharmed(callerCharmedBy) {
			continue
		}
		if !self.HatesMob(other) {
			continue
		}
		targetPower := powerScoreFn(other.Character)
		if targetPower <= 0 {
			continue
		}
		if ratio := targetPower / selfPower; ratio < bestRatio {
			bestRatio = ratio
			bestId = otherId
		}
	}
	if bestId == 0 {
		return state.ActorRef{}, false
	}
	return state.ActorRef{MobInstanceId: bestId}, true
}

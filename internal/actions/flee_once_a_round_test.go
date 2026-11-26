package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// #951: a flee could be attempted more than once in a combat round. Once an
// attempt resolved (a failed flee returns the fleer to the fight), nothing
// stopped a new one before the next round. A flee may now be started once
// every round, by players and mobs alike, whatever the first attempt's
// outcome. Only the round a flee starts in counts (owner ruling on #956): the
// round it resolves in stays free for a new flee.

const fleeOnceRound uint64 = 2_000_000

// fleeOnceActors builds the fleer as a player and as a mob, alike.
var fleeOnceActors = []struct {
	name  string
	actor func(c *characters.Character, room *rooms.Room) Actor
}{
	{"player", func(c *characters.Character, room *rooms.Room) Actor {
		return NewUserActorInRoom(&users.UserRecord{UserId: fleeParityUserId, Character: c}, room)
	}},
	{"mob", func(c *characters.Character, room *rooms.Room) Actor {
		return NewMobActorInRoom(&mobs.Mob{InstanceId: fleeParityMobId, Character: *c}, room)
	}},
}

// fleeOnceFixture is an engaged fleer in a room with no exits, so every
// resolved flee fails deterministically (NoExit) and returns it to the fight.
func fleeOnceFixture(t *testing.T, build func(c *characters.Character, room *rooms.Room) Actor) (Actor, *characters.Character, *rooms.Room) {
	t.Helper()
	room := &rooms.Room{RoomId: fleeParityRoomId}
	actor := build(newFleeParityChar(), room)
	c := actor.GetCharacter() // the mob copies the struct; use its own
	engageForFlee(t, c)
	return actor, c, room
}

// failFlee resolves the attempt in flight and checks it failed back into the
// fight, which is what leaves the fleer free to try again.
func failFlee(t *testing.T, actor Actor, room *rooms.Room) {
	t.Helper()
	out := ResolveFlee(actor, room)
	if !out.Resolved || !out.NoExit {
		t.Fatalf("fixture: flee outcome %+v, want a resolved NoExit failure", out)
	}
	if s := actor.GetCharacter().CombatPhase.State(); s != combatphase.Engaged {
		t.Fatalf("fixture: failed flee left state %v, want Engaged", s)
	}
}

// assertRefusedThisRound checks a second flee is refused for the round and
// changes nothing: no stamina, no transition, no admission.
func assertRefusedThisRound(t *testing.T, actor Actor) {
	t.Helper()
	c := actor.GetCharacter()
	before := c.Stamina
	b := BeginFlee(actor, ``)
	if b.Accepted || b.Refusal != FleeRefuseThisRound {
		t.Fatalf("second flee in one round = %+v, want refused with FleeRefuseThisRound", b)
	}
	if c.Stamina != before {
		t.Errorf("refused flee spent stamina: %d -> %d", before, c.Stamina)
	}
	if c.IsDisengaging() {
		t.Error("refused flee entered Disengaging")
	}
	if _, admitted := c.TakeFleeAdmission(); admitted {
		t.Error("refused flee left an admission behind")
	}
}

func setFleeRound(t *testing.T, r uint64) {
	t.Helper()
	util.SetRoundCountForTest(r)
	t.Cleanup(util.ResetRoundCountForTest)
}

// The owner's repro: flee, the attempt fails, flee again in that round.
func TestBeginFlee_OnceARound_SecondAttemptAfterAFailureIsRefused(t *testing.T) {
	seedFleeConditions(t)
	for _, side := range fleeOnceActors {
		t.Run(side.name, func(t *testing.T) {
			setFleeRound(t, fleeOnceRound)
			actor, _, room := fleeOnceFixture(t, side.actor)

			if b := BeginFlee(actor, ``); !b.Accepted {
				t.Fatalf("first flee refused: %+v", b)
			}
			failFlee(t, actor, room)
			assertRefusedThisRound(t, actor)

			setFleeRound(t, fleeOnceRound+1)
			if b := BeginFlee(actor, ``); !b.Accepted {
				t.Fatalf("flee in the next round refused: %+v", b)
			}
		})
	}
}

// The usual cadence: a flee started in round N resolves in round N+1. Only
// the starting round counts, so N+1 stays free: a new flee started right
// after the failure is accepted, and a third in N+1 is refused.
func TestBeginFlee_OnceARound_TheResolvingRoundStaysFree(t *testing.T) {
	seedFleeConditions(t)
	for _, side := range fleeOnceActors {
		t.Run(side.name, func(t *testing.T) {
			setFleeRound(t, fleeOnceRound)
			actor, _, room := fleeOnceFixture(t, side.actor)

			if b := BeginFlee(actor, ``); !b.Accepted {
				t.Fatalf("first flee refused: %+v", b)
			}
			setFleeRound(t, fleeOnceRound+1)
			failFlee(t, actor, room)
			if b := BeginFlee(actor, ``); !b.Accepted {
				t.Fatalf("flee started in the round the last one resolved in = %+v, want accepted", b)
			}
			failFlee(t, actor, room)
			assertRefusedThisRound(t, actor)
		})
	}
}

// An attempt that never reached resolution still spent the round: a target
// death ends Disengaging, a new attacker re-engages the same round, and the
// fleer may not try again until the next one.
func TestBeginFlee_OnceARound_AnUnresolvedAttemptStillSpendsTheRound(t *testing.T) {
	seedFleeConditions(t)
	for _, side := range fleeOnceActors {
		t.Run(side.name, func(t *testing.T) {
			setFleeRound(t, fleeOnceRound)
			actor, c, _ := fleeOnceFixture(t, side.actor)
			if b := BeginFlee(actor, ``); !b.Accepted {
				t.Fatalf("first flee refused: %+v", b)
			}
			c.CombatPhase.ForceIdle(state.TransitionReason{Trigger: combatphase.TriggerTargetDied})
			c.CancelFleeAdmission()
			engageForFlee(t, c)
			assertRefusedThisRound(t, actor)
		})
	}
}

// A flee that was still resolving keeps its own, older refusal: the
// once-a-round line is for an attempt that has already ended.
func TestBeginFlee_OnceARound_InFlightAttemptKeepsAlreadyRefusal(t *testing.T) {
	seedFleeConditions(t)
	setFleeRound(t, fleeOnceRound)
	actor, _, _ := fleeOnceFixture(t, fleeOnceActors[0].actor)
	if b := BeginFlee(actor, ``); !b.Accepted {
		t.Fatalf("first flee refused: %+v", b)
	}
	if b := BeginFlee(actor, ``); b.Refusal != FleeRefuseAlready {
		t.Fatalf("flee while one resolves = %+v, want FleeRefuseAlready", b)
	}
}

// FleeGate is what the mob AI (companion flee, archer kiting, pack scatter)
// asks before it tries; it must see the round's attempt too, or a mob that
// has already tried would keep queueing refused flees.
func TestFleeGate_OnceARound_SeesTheRoundsAttempt(t *testing.T) {
	seedFleeConditions(t)
	setFleeRound(t, fleeOnceRound)
	actor, c, room := fleeOnceFixture(t, fleeOnceActors[1].actor)
	if b := BeginFlee(actor, ``); !b.Accepted {
		t.Fatalf("first flee refused: %+v", b)
	}
	failFlee(t, actor, room)
	if got := FleeGate(c); got != FleeRefuseThisRound {
		t.Fatalf("FleeGate after this round's attempt = %v, want FleeRefuseThisRound", got)
	}
	setFleeRound(t, fleeOnceRound+1)
	if got := FleeGate(c); got != FleeOK {
		t.Fatalf("FleeGate next round = %v, want FleeOK", got)
	}
}

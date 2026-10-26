package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #588: flee walked through quest gates. Fleeing down from Thornwall 485
// entered the quest 14 cellar without the quest, because fleeExit filtered
// only locked exits. Quest gates now apply to every move that walks through
// an exit (owner ruling 2026-10-26): flee never picks a gated exit, and with
// every exit gated it fails as it does with every exit locked.

const fleeGateQuestId = 999588

// registerFleeGate guards fleeParityNorthId for a player standing in the
// fleer's room who lacks the gate quest's start token.
func registerFleeGate(t *testing.T) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: fleeGateQuestId,
		Name:    "Flee Gate Quest",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:      "room_gate",
			Room:       fleeParityNorthId,
			Conditions: questengine.Conditions{Missing: []string{"999588-start"}, InRoom: fleeParityRoomId},
			Actions:    []questengine.ActionDef{{SendText: "You have no reason to go down there."}},
		}},
	})
}

// resolvePlayerFlee begins and resolves one player flee in a room with exits.
func resolvePlayerFlee(t *testing.T, exits map[string]exit.RoomExit, preferred string) FleeOutcome {
	t.Helper()
	c := newFleeParityChar()
	room := &rooms.Room{RoomId: fleeParityRoomId, Exits: exits}
	actor := NewUserActorInRoom(&users.UserRecord{UserId: fleeParityUserId, Character: c}, room)
	engageForFlee(t, c)
	if b := BeginFlee(actor, preferred); !b.Accepted {
		t.Fatalf("fixture: flee was not accepted: %+v", b)
	}
	return ResolveFlee(actor, room)
}

func TestResolveFlee_PlayerNeverPicksAGatedExit(t *testing.T) {
	seedFleeConditions(t)
	registerFleeGate(t)
	two := map[string]exit.RoomExit{"north": {RoomId: fleeParityNorthId}, "east": {RoomId: fleeParityEastId}}
	// With two exits a random pick lands on the gated one about half the
	// time, so 40 runs leave a 1 in 2^40 chance of a false pass.
	for i := 0; i < 40; i++ {
		o := resolvePlayerFlee(t, two, ``)
		if !o.Escaped() || o.ExitName != "east" {
			t.Fatalf("run %d: outcome %+v, want an escape through east (north is gated)", i, o)
		}
	}
}

func TestResolveFlee_PlayerPreferredGatedExitIsSkipped(t *testing.T) {
	seedFleeConditions(t)
	registerFleeGate(t)
	two := map[string]exit.RoomExit{"north": {RoomId: fleeParityNorthId}, "east": {RoomId: fleeParityEastId}}
	o := resolvePlayerFlee(t, two, "north")
	if !o.Escaped() || o.ExitName != "east" {
		t.Fatalf("outcome %+v, want the gated preferred exit skipped for east", o)
	}
}

// Every exit gated fails exactly as every exit locked does: NoExit, back in
// the fight. The locked row is the baseline the gated row must match.
func TestResolveFlee_AllGatedFailsLikeAllLocked(t *testing.T) {
	seedFleeConditions(t)
	registerFleeGate(t)
	locked := map[string]exit.RoomExit{"east": {RoomId: fleeParityEastId, Lock: gamelock.Lock{Difficulty: 10}}}
	gated := map[string]exit.RoomExit{"north": {RoomId: fleeParityNorthId}}

	lockedOut := resolvePlayerFlee(t, locked, ``)
	if !lockedOut.NoExit {
		t.Fatalf("fixture: an all-locked room should leave no exit, got %+v", lockedOut)
	}
	gatedOut := resolvePlayerFlee(t, gated, ``)
	if gatedOut != lockedOut {
		t.Fatalf("all gated = %+v, want the all-locked outcome %+v", gatedOut, lockedOut)
	}
}

// A player who holds the token is not gated, and mobs never are: the gate is
// a player's quest state.
func TestResolveFlee_GateOnlyStopsThoseItGates(t *testing.T) {
	seedFleeConditions(t)
	registerFleeGate(t)
	north := map[string]exit.RoomExit{"north": {RoomId: fleeParityNorthId}}

	t.Run("player with the token", func(t *testing.T) {
		c := newFleeParityChar()
		c.GiveQuestToken("999588-start")
		room := &rooms.Room{RoomId: fleeParityRoomId, Exits: north}
		actor := NewUserActorInRoom(&users.UserRecord{UserId: fleeParityUserId, Character: c}, room)
		engageForFlee(t, c)
		BeginFlee(actor, ``)
		if o := ResolveFlee(actor, room); !o.Escaped() || o.ExitName != "north" {
			t.Fatalf("outcome %+v, want an escape north", o)
		}
	})

	t.Run("mob", func(t *testing.T) {
		_, m := fleeBoth(t, fleeRow{name: "mob", engage: true, resolve: true, exits: north})
		if !m.outcome.Escaped() || m.outcome.ExitName != "north" {
			t.Fatalf("mob outcome %+v, want an escape north", m.outcome)
		}
	})
}

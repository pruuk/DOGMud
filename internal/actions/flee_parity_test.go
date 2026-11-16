package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The flee parity table (slice 4a). A player and a mob, built alike, begin
// and resolve a flee through actions.BeginFlee and actions.ResolveFlee; every
// row asserts the two come out the same. Before 4a a mob's flee was free,
// instant, and ignored roots, standing and whether it was even fighting.

const (
	fleeParityUserId   = 7401
	fleeParityMobId    = 98401
	fleeParityRoomId   = 99401
	fleeParityNorthId  = 99402
	fleeParityEastId   = 99403
	fleeRootCondId     = 9401
	fleeNoFleeCondId   = 9402
	fleeParityFlightId = "flee-parity-flight"
)

// fleeSide is one fleer after a row ran.
type fleeSide struct {
	begin      FleeBegin
	second     FleeBegin
	outcome    FleeOutcome
	state      combatphase.State
	staminaUse int
	admission  characters.FleeAdmission
	admitted   bool
}

func newFleeParityChar() *characters.Character {
	c := characters.New()
	c.Name = "Fleer"
	c.RoomId = fleeParityRoomId
	c.Mutations = map[string]int{}
	c.Validate()
	c.StaminaMax.Base = 500
	c.StaminaMax.Recalculate()
	c.Stamina = 500
	return c
}

func engageForFlee(t *testing.T, c *characters.Character) {
	t.Helper()
	if err := c.CombatPhase.TransitionToEngaging(
		combatphase.EngagingData{Target: state.ActorRef{MobInstanceId: 4242}},
		state.TransitionReason{Trigger: combatphase.TriggerAttackCommand},
	); err != nil {
		t.Fatalf("could not enter combat: %v", err)
	}
	c.CombatPhase.OnRoundTick()
}

type fleeRow struct {
	name       string
	engage     bool
	setup      func(t *testing.T, c *characters.Character)
	preferred  string
	beginTwice bool
	// resolve, when set, runs ResolveFlee after an accepted begin in a room
	// with the given exits; between runs mid-flee setup (e.g. a grapple).
	resolve bool
	exits   map[string]exit.RoomExit
	between func(t *testing.T, c *characters.Character)
}

// fleeBoth runs one row for a player and for a mob and returns both sides.
func fleeBoth(t *testing.T, row fleeRow) (fleeSide, fleeSide) {
	t.Helper()
	run := func(actorFor func(c *characters.Character, room *rooms.Room) Actor) fleeSide {
		c := newFleeParityChar()
		room := &rooms.Room{RoomId: fleeParityRoomId, Exits: row.exits}
		actor := actorFor(c, room)
		c = actor.GetCharacter() // the mob copies the struct; use its own
		if row.setup != nil {
			row.setup(t, c)
		}
		if row.engage {
			engageForFlee(t, c)
		}
		before := c.Stamina
		side := fleeSide{begin: BeginFlee(actor, row.preferred)}
		side.staminaUse = before - c.Stamina
		if row.beginTwice {
			side.second = BeginFlee(actor, row.preferred)
		}
		if row.resolve && side.begin.Accepted {
			if row.between != nil {
				row.between(t, c)
			}
			side.outcome = ResolveFlee(actor, room)
		} else {
			side.admission, side.admitted = c.TakeFleeAdmission()
		}
		side.state = c.CombatPhase.State()
		return side
	}
	player := run(func(c *characters.Character, room *rooms.Room) Actor {
		return NewUserActorInRoom(&users.UserRecord{UserId: fleeParityUserId, Character: c}, room)
	})
	mob := run(func(c *characters.Character, room *rooms.Room) Actor {
		m := &mobs.Mob{InstanceId: fleeParityMobId, Character: *c}
		return NewMobActorInRoom(m, room)
	})
	return player, mob
}

func seedFleeConditions(t *testing.T) {
	t.Helper()
	cleanup := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		fleeRootCondId:   {ConditionId: fleeRootCondId, Name: "Rooted", TriggerCount: 1, Flags: []conditions.Flag{conditions.NoMovement}},
		fleeNoFleeCondId: {ConditionId: fleeNoFleeCondId, Name: "Frenzied", TriggerCount: 1, Flags: []conditions.Flag{conditions.NoFlee}},
	})
	t.Cleanup(cleanup)
	cleanupMut := mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		fleeParityFlightId: {
			MutationId: fleeParityFlightId, Name: "Flight", Rarity: 8,
			Pros: []mutations.MutationEffect{{Type: "flag", Target: "flying"}},
		},
	})
	t.Cleanup(cleanupMut)
}

func addFleeCond(id int) func(t *testing.T, c *characters.Character) {
	return func(t *testing.T, c *characters.Character) {
		t.Helper()
		if !c.Conditions.AddCondition(id, false) {
			t.Fatalf("could not apply condition %d", id)
		}
	}
}

func knockDown(t *testing.T, c *characters.Character) {
	t.Helper()
	if c.Position == nil {
		c.Position = position.NewMachine()
	}
	r := state.TransitionReason{Trigger: "test_setup"}
	c.Position.ForceStanding(r)
	if err := c.Position.TransitionToProne(position.ProneData{}, r); err != nil {
		t.Fatalf("knock down: %v", err)
	}
}

func clinch(t *testing.T, c *characters.Character) {
	t.Helper()
	if c.Position == nil {
		c.Position = position.NewMachine()
	}
	c.Position.ForceStanding(state.TransitionReason{Trigger: "test_setup"})
	if err := c.Position.TransitionToClinch(
		position.GrappleData{Partner: state.ActorRef{UserId: 1}},
		state.TransitionReason{Trigger: position.TriggerGrappleEntry},
	); err != nil {
		t.Fatalf("clinch: %v", err)
	}
}

func TestFleeParity_Begin(t *testing.T) {
	seedFleeConditions(t)
	rows := []struct {
		row      fleeRow
		accepted bool
		refusal  FleeRefusal
		short    bool
	}{
		{fleeRow{name: "rooted refuses", engage: true, setup: addFleeCond(fleeRootCondId)}, false, FleeRefuseRooted, false},
		{fleeRow{name: "no-flee refuses", engage: true, setup: addFleeCond(fleeNoFleeCondId)}, false, FleeRefuseNoFlee, false},
		{fleeRow{name: "out of combat refuses", engage: false}, false, FleeRefuseNotInCombat, false},
		{fleeRow{name: "prone refuses", engage: true, setup: knockDown}, false, FleeRefuseProne, false},
		{fleeRow{name: "grappled refuses", engage: true, setup: clinch}, false, FleeRefuseGrappled, false},
		{fleeRow{name: "paid in full is accepted with skill", engage: true}, true, FleeOK, false},
		// Owner ruling 2026-11-16 (#401): flee is never refused for lack of
		// stamina. TestFleeNeverChargesAStep (internal/guards) keeps the flee
		// paths off the step charge that would refuse it.
		{fleeRow{name: "spent is accepted without skill", engage: true, setup: func(t *testing.T, c *characters.Character) { c.Stamina = 0 }}, true, FleeOK, true},
	}
	for _, tc := range rows {
		t.Run(tc.row.name, func(t *testing.T) {
			p, m := fleeBoth(t, tc.row)
			for who, s := range map[string]fleeSide{"player": p, "mob": m} {
				if s.begin.Accepted != tc.accepted || s.begin.Refusal != tc.refusal || s.begin.Short != tc.short {
					t.Errorf("%s begin = %+v, want accepted=%v refusal=%v short=%v", who, s.begin, tc.accepted, tc.refusal, tc.short)
				}
				if tc.accepted {
					if s.state != combatphase.Disengaging {
						t.Errorf("%s state = %v, want Disengaging", who, s.state)
					}
					if !s.admitted || !s.admission.Ready || s.admission.IncludeSkill == tc.short {
						t.Errorf("%s admission = %+v (%v), want ready with IncludeSkill=%v", who, s.admission, s.admitted, !tc.short)
					}
				} else if s.admitted {
					t.Errorf("%s refused flee left an admission", who)
				}
			}
			if p.staminaUse != m.staminaUse {
				t.Errorf("stamina paid: player %d, mob %d; want equal", p.staminaUse, m.staminaUse)
			}
			if tc.accepted && !tc.short && p.staminaUse <= 0 {
				t.Errorf("an accepted flee paid %d stamina, want > 0", p.staminaUse)
			}
		})
	}
}

func TestFleeParity_AlreadyDisengagingRefuses(t *testing.T) {
	seedFleeConditions(t)
	p, m := fleeBoth(t, fleeRow{name: "twice", engage: true, beginTwice: true})
	for who, s := range map[string]fleeSide{"player": p, "mob": m} {
		if !s.begin.Accepted || s.second.Refusal != FleeRefuseAlready {
			t.Errorf("%s: first %+v second %+v, want accepted then FleeRefuseAlready", who, s.begin, s.second)
		}
	}
}

func TestFleeParity_FlightHalvesTheCost(t *testing.T) {
	seedFleeConditions(t)
	_, ground := fleeBoth(t, fleeRow{name: "ground", engage: true})
	fp, fm := fleeBoth(t, fleeRow{name: "flying", engage: true, setup: func(t *testing.T, c *characters.Character) {
		c.Mutations = map[string]int{fleeParityFlightId: 1}
	}})
	if fp.staminaUse != fm.staminaUse {
		t.Fatalf("flying stamina paid: player %d, mob %d; want equal", fp.staminaUse, fm.staminaUse)
	}
	if fm.staminaUse >= ground.staminaUse {
		t.Fatalf("flying paid %d, grounded %d; flight must pay less", fm.staminaUse, ground.staminaUse)
	}
}

func TestFleeParity_Resolve(t *testing.T) {
	seedFleeConditions(t)
	north := map[string]exit.RoomExit{"north": {RoomId: fleeParityNorthId}}
	two := map[string]exit.RoomExit{"north": {RoomId: fleeParityNorthId}, "east": {RoomId: fleeParityEastId}}
	rows := []struct {
		row      fleeRow
		escaped  bool
		exitName string
		grappled bool
		noExit   bool
		state    combatphase.State
	}{
		{fleeRow{name: "escapes through the only exit", engage: true, resolve: true, exits: north}, true, "north", false, false, combatphase.Idle},
		{fleeRow{name: "takes the preferred exit", engage: true, resolve: true, exits: two, preferred: "east"}, true, "east", false, false, combatphase.Idle},
		{fleeRow{name: "no exit returns to the fight", engage: true, resolve: true}, false, "", false, true, combatphase.Engaged},
		{fleeRow{name: "grappled mid-flee returns to the fight", engage: true, resolve: true, exits: north, between: clinch}, false, "", true, false, combatphase.Engaged},
	}
	for _, tc := range rows {
		t.Run(tc.row.name, func(t *testing.T) {
			p, m := fleeBoth(t, tc.row)
			for who, s := range map[string]fleeSide{"player": p, "mob": m} {
				o := s.outcome
				if !o.Fleeing || !o.Resolved {
					t.Fatalf("%s outcome %+v, want a resolved flee", who, o)
				}
				if o.Escaped() != tc.escaped || o.ExitName != tc.exitName || o.Grappled != tc.grappled || o.NoExit != tc.noExit {
					t.Errorf("%s outcome = %+v, want escaped=%v exit=%q grappled=%v noExit=%v", who, o, tc.escaped, tc.exitName, tc.grappled, tc.noExit)
				}
				if s.state != tc.state {
					t.Errorf("%s state = %v, want %v", who, s.state, tc.state)
				}
			}
		})
	}
}

func TestResolveFlee_NotDisengagingIsNotAFlee(t *testing.T) {
	c := newFleeParityChar()
	engageForFlee(t, c)
	room := &rooms.Room{RoomId: fleeParityRoomId}
	u := &users.UserRecord{UserId: fleeParityUserId, Character: c}
	if o := ResolveFlee(NewUserActorInRoom(u, room), room); o.Fleeing || o.Resolved {
		t.Fatalf("an engaged fighter resolved a flee: %+v", o)
	}
}

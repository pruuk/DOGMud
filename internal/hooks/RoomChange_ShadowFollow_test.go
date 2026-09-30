package hooks

import (
	"os"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Room layout: the quarry leaves shadowRoomA for shadowRoomB by "north", or
// for shadowRoomTemp by the temporary exit "crack" (titled "a narrow crack",
// so the key and the title differ), or for shadowRoomMut by "veil", an exit
// that exists only while shadowMutatorId is active on shadowRoomA. shadowRoomFar
// has no exit from A: a move there is a teleport. Every character id is
// unique to this file.
const (
	shadowRoomA    = 71010
	shadowRoomB    = 71011
	shadowRoomFar  = 71012
	shadowRoomTemp = 71013
	shadowRoomMut  = 71014

	shadowQuarryUser = 7101
	shadowSlyUser    = 7102
	shadowQuarryMob  = 71101
	shadowSlyMob     = 71102
	shadowNobody     = 71199 // a target id no character in the scene has

	shadowMutatorId = "shadow-test-mutator"
)

// shadowSensedText is the sense line actions.ShadowSenseRoll sends a player
// quarry (unexported there).
const shadowSensedText = "You sense someone following close behind you."

// shadowScene is one fresh world per case: four rooms, a quarry and a
// shadower of each kind, condition 87 seeded.
type shadowScene struct {
	t                   *testing.T
	quarryUser, slyUser *users.UserRecord
	quarryMob, slyMob   *mobs.Mob
	roomB, roomFar      *rooms.Room
}

func newShadowScene(t *testing.T) *shadowScene {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		actions.ShadowingConditionId: {ConditionId: actions.ShadowingConditionId, Name: "Shadowing", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25},
	}))
	// shadowMutatorId's exit is registered globally and activated on
	// shadowRoomA (a zero-value Mutator is live: DespawnedRound == 0), giving
	// A a route to shadowRoomMut that exists only through ActiveMutators. A
	// zone config entry for "test" is required: Room.ActiveMutators only
	// consults a room's Mutators field once GetZoneConfig(r.Zone) is non-nil.
	t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
		MutatorId: shadowMutatorId,
		Exits:     map[string]exit.RoomExit{"veil": {RoomId: shadowRoomMut}},
	}))
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		shadowRoomA: {RoomId: shadowRoomA, Zone: "test",
			Exits:     map[string]exit.RoomExit{"north": {RoomId: shadowRoomB}},
			ExitsTemp: map[string]exit.TemporaryRoomExit{"crack": {RoomId: shadowRoomTemp, Title: "a narrow crack"}},
			Mutators:  mutators.MutatorList{{MutatorId: shadowMutatorId}}},
		shadowRoomB:    {RoomId: shadowRoomB, Zone: "test", Exits: map[string]exit.RoomExit{"south": {RoomId: shadowRoomA}}},
		shadowRoomFar:  {RoomId: shadowRoomFar, Zone: "test", Exits: map[string]exit.RoomExit{}},
		shadowRoomTemp: {RoomId: shadowRoomTemp, Zone: "test", Exits: map[string]exit.RoomExit{}},
		shadowRoomMut:  {RoomId: shadowRoomMut, Zone: "test", Exits: map[string]exit.RoomExit{}},
	}, map[string]*rooms.ZoneConfig{"test": {Name: "test"}}))

	s := &shadowScene{
		t:          t,
		quarryUser: users.NewTestUser(shadowQuarryUser, "quarry", "Quarry", 0),
		slyUser:    users.NewTestUser(shadowSlyUser, "sly", "Sly", 0),
		quarryMob:  &mobs.Mob{InstanceId: shadowQuarryMob, Character: *characters.New()},
		slyMob:     &mobs.Mob{InstanceId: shadowSlyMob, Character: *characters.New()},
		roomB:      rooms.LoadRoom(shadowRoomB),
		roomFar:    rooms.LoadRoom(shadowRoomFar),
	}
	s.quarryMob.Character.Name = "Stag"
	s.slyMob.Character.Name = "Lurker"
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		shadowQuarryUser: s.quarryUser, shadowSlyUser: s.slyUser,
	}))
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{}, map[int]*mobs.Mob{
		shadowQuarryMob: s.quarryMob, shadowSlyMob: s.slyMob,
	}))

	// s.place, below, calls Room.AddMob for a mob quarry or shadower, which
	// queues a RoomChange as a side effect. Nothing in this file ever calls
	// events.ProcessEvents to drain it, so left alone it would sit in the
	// shared global queue and leak into whatever test runs next.
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	// Leftovers from an earlier test must not count.
	events.DrainQueuedUserInputsForTest(shadowSlyUser)
	events.DrainQueuedUserInputsForTest(shadowQuarryUser)
	events.DrainQueuedInputsForTest(shadowSlyMob)
	events.DrainQueuedInputsForTest(shadowQuarryMob)
	events.DrainQueuedMessagesForTest(shadowSlyUser)
	events.DrainQueuedMessagesForTest(shadowQuarryUser)
	return s
}

// shadowKind names a side of the table: "player" or "mob".
type shadowKind string

const (
	shadowPlayer shadowKind = "player"
	shadowMob    shadowKind = "mob"
)

var shadowKinds = []shadowKind{shadowPlayer, shadowMob}

// sly is the shadower of kind k.
func (s *shadowScene) sly(k shadowKind) *characters.Character {
	if k == shadowPlayer {
		return s.slyUser.Character
	}
	return &s.slyMob.Character
}

// place puts the quarry (quarry=true) or the shadower of kind k in room.
func (s *shadowScene) place(k shadowKind, quarry bool, room *rooms.Room) {
	switch {
	case k == shadowPlayer && quarry:
		room.AddPlayer(shadowQuarryUser)
		s.quarryUser.Character.RoomId = room.RoomId
	case k == shadowPlayer:
		room.AddPlayer(shadowSlyUser)
		s.slyUser.Character.RoomId = room.RoomId
	case quarry:
		room.AddMob(shadowQuarryMob)
	default:
		room.AddMob(shadowSlyMob)
	}
}

// shadowTarget sets c's shadow on the quarry of kind k (or on nobody), with
// condition 87 when live, and hides c when hidden.
func (s *shadowScene) shadowTarget(c *characters.Character, k shadowKind, nobody, live, hidden bool) {
	s.t.Helper()
	switch {
	case nobody:
		c.SetMiscData("shadow-target-mob", shadowNobody)
	case k == shadowPlayer:
		c.SetMiscData("shadow-target-user", shadowQuarryUser)
	default:
		c.SetMiscData("shadow-target-mob", shadowQuarryMob)
	}
	if live {
		if err := c.AddCondition(actions.ShadowingConditionId, false); err != nil {
			s.t.Fatalf("add condition 87: %v", err)
		}
	}
	if hidden {
		r := state.TransitionReason{Trigger: "test"}
		c.Awareness.ForceVisible(r)
		_ = c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r)
		c.Awareness.ResolveConcealment(true, r)
		if !c.IsHidden() {
			s.t.Fatal("fixture: the shadower did not become hidden")
		}
	}
}

// moveEvent is the RoomChange of the character of kind k (quarry or shadower).
func moveEvent(k shadowKind, quarry bool, from, to int) events.RoomChange {
	evt := events.RoomChange{FromRoomId: from, ToRoomId: to}
	switch {
	case k == shadowPlayer && quarry:
		evt.UserId = shadowQuarryUser
	case k == shadowPlayer:
		evt.UserId = shadowSlyUser
	case quarry:
		evt.MobInstanceId = shadowQuarryMob
	default:
		evt.MobInstanceId = shadowSlyMob
	}
	return evt
}

// queued drains what the shadower of kind k has queued.
func (s *shadowScene) queued(k shadowKind) []string {
	if k == shadowPlayer {
		return events.DrainQueuedUserInputsForTest(shadowSlyUser)
	}
	return events.DrainQueuedInputsForTest(shadowSlyMob)
}

// shadowEnded reports whether c's shadow is gone: no target, and condition 87
// gone once the turn's prune runs (RemoveCondition only expires it).
func shadowEnded(c *characters.Character) bool {
	c.Conditions.Prune()
	userId, mobInstanceId := actions.ShadowTargetOf(c)
	return userId == 0 && mobInstanceId == 0 && !c.HasCondition(actions.ShadowingConditionId)
}

func shadowCooldown(c *characters.Character) int {
	return c.GetCooldown(skills.Skullduggery.String("shadow"))
}

// The follow pass, for every mover and shadower kind.
func TestRoomChangeShadowFollow_FollowPass(t *testing.T) {
	cases := []struct {
		name                   string
		live, hidden, nobody   bool
		shadowerRoom, toRoomId int
		want                   []string
		cleared                bool
	}{
		{name: "hidden, live, on the mover, in the old room: follows", live: true, hidden: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomB, want: []string{"north"}},
		{name: "not hidden: stays", live: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomB},
		{name: "in another room: stays", live: true, hidden: true, shadowerRoom: shadowRoomFar, toRoomId: shadowRoomB},
		{name: "shadowing someone else: stays", live: true, hidden: true, nobody: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomB},
		{name: "stale (no condition 87): stays and is cleared", hidden: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomB, cleared: true},
		{name: "teleport: no exit, stays", live: true, hidden: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomFar},
		{name: "temp exit: the key is queued, not the title", live: true, hidden: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomTemp, want: []string{"crack"}},
		{name: "mutator exit: the only route is an active mutator's exit", live: true, hidden: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomMut, want: []string{"veil"}},
	}
	for _, mover := range shadowKinds {
		for _, shadower := range shadowKinds {
			for _, tc := range cases {
				t.Run(string(mover)+" mover, "+string(shadower)+" shadower: "+tc.name, func(t *testing.T) {
					s := newShadowScene(t)
					s.place(mover, true, rooms.LoadRoom(tc.toRoomId))
					s.place(shadower, false, rooms.LoadRoom(tc.shadowerRoom))
					c := s.sly(shadower)
					s.shadowTarget(c, mover, tc.nobody, tc.live, tc.hidden)

					RoomChangeShadowFollow(moveEvent(mover, true, shadowRoomA, tc.toRoomId))

					got := s.queued(shadower)
					if strings.Join(got, ";") != strings.Join(tc.want, ";") {
						t.Fatalf("queued %q, want %q", got, tc.want)
					}
					userId, mobInstanceId := actions.ShadowTargetOf(c)
					stillSet := userId != 0 || mobInstanceId != 0
					if tc.cleared && stillSet {
						t.Errorf("stale state kept: ShadowTargetOf = (%d, %d)", userId, mobInstanceId)
					}
					if !tc.cleared && !stillSet {
						t.Error("the shadow target was cleared, but only stale state may be")
					}
					if got := shadowCooldown(c); got != 0 {
						t.Errorf("the follow pass started a %d round cooldown", got)
					}
				})
			}
		}
	}
}

// Arrival not hidden: the shadow ends with the cooldown, and a player
// shadower reads the spotted line.
func TestRoomChangeShadowFollow_ArrivalSpottedEndsTheShadow(t *testing.T) {
	for _, shadower := range shadowKinds {
		for _, quarry := range shadowKinds {
			t.Run(string(shadower)+" shadower, "+string(quarry)+" quarry", func(t *testing.T) {
				s := newShadowScene(t)
				s.place(quarry, true, s.roomB)
				s.place(shadower, false, s.roomB)
				c := s.sly(shadower)
				s.shadowTarget(c, quarry, false, true, false) // live, NOT hidden

				RoomChangeShadowFollow(moveEvent(shadower, false, shadowRoomA, shadowRoomB))

				if !shadowEnded(c) {
					t.Error("the spotted shadow did not end")
				}
				if shadowCooldown(c) <= 0 {
					t.Error("the spotted end started no cooldown")
				}
				spotted := 0
				for _, msg := range events.DrainQueuedMessagesForTest(shadowSlyUser) {
					if strings.Contains(msg, shadowSpottedLine) {
						spotted++
					}
				}
				want := 0
				if shadower == shadowPlayer {
					want = 1
				}
				if spotted != want {
					t.Errorf("spotted line sent %d times, want %d", spotted, want)
				}
			})
		}
	}
}

// Arrival must read IsHidden live, not evt.Unseen: entry detection can reveal
// a shadower in the gap between the RoomChange being stamped and it being
// dispatched. An event stamped Unseen (hidden at queue time) whose shadower
// is no longer hidden by dispatch time still ends the shadow. A listener
// that read evt.Unseen instead of live IsHidden would see Unseen == true and
// wrongly treat the shadower as still hidden, rolling the sense check
// instead of ending the shadow.
func TestRoomChangeShadowFollow_ArrivalReadsHiddenLiveNotEventUnseen(t *testing.T) {
	for _, shadower := range shadowKinds {
		for _, quarry := range shadowKinds {
			t.Run(string(shadower)+" shadower, "+string(quarry)+" quarry", func(t *testing.T) {
				s := newShadowScene(t)
				s.place(quarry, true, s.roomB)
				s.place(shadower, false, s.roomB)
				c := s.sly(shadower)
				s.shadowTarget(c, quarry, false, true, false) // live, NOT hidden (revealed since the event was stamped)

				evt := moveEvent(shadower, false, shadowRoomA, shadowRoomB)
				evt.Unseen = true // stale: was hidden when queued, no longer hidden now

				RoomChangeShadowFollow(evt)

				if !shadowEnded(c) {
					t.Error("a stale Unseen=true event kept the shadow alive; the arrival pass must read IsHidden live, not evt.Unseen")
				}
				if shadowCooldown(c) <= 0 {
					t.Error("the spotted end started no cooldown")
				}
			})
		}
	}
}

// Arrival still hidden: the quarry makes the sense roll on every arrival, the
// shadower trains Skullduggery on every roll, and only a player quarry reads
// a line. 200 arrivals, so the contest floor guarantees both outcomes (see
// the shadow sense tests in internal/actions).
func TestRoomChangeShadowFollow_ArrivalHiddenRollsTheSense(t *testing.T) {
	const arrivals = 200
	for _, shadower := range shadowKinds {
		for _, quarry := range shadowKinds {
			t.Run(string(shadower)+" shadower, "+string(quarry)+" quarry", func(t *testing.T) {
				s := newShadowScene(t)
				s.place(quarry, true, s.roomB)
				s.place(shadower, false, s.roomB)
				c := s.sly(shadower)
				s.shadowTarget(c, quarry, false, true, true)
				before := c.GetSkillUseCount(string(skills.Skullduggery))

				for i := 0; i < arrivals; i++ {
					RoomChangeShadowFollow(moveEvent(shadower, false, shadowRoomA, shadowRoomB))
				}

				if got := c.GetSkillUseCount(string(skills.Skullduggery)) - before; got != arrivals {
					t.Errorf("Skullduggery used %d times over %d arrivals, want one per roll on both outcomes", got, arrivals)
				}
				if shadowEnded(c) {
					t.Fatal("a hidden arrival ended the shadow")
				}
				sensed := 0
				for _, msg := range events.DrainQueuedMessagesForTest(shadowQuarryUser) {
					if strings.Contains(msg, shadowSensedText) {
						sensed++
					}
				}
				if quarry == shadowPlayer && (sensed == 0 || sensed == arrivals) {
					t.Errorf("player quarry sensed %d of %d arrivals: both outcomes must occur", sensed, arrivals)
				}
				if quarry == shadowMob && sensed != 0 {
					t.Errorf("a mob quarry's roll sent %d lines to a player", sensed)
				}
				if got := len(events.DrainQueuedMessagesForTest(shadowSlyUser)); got != 0 {
					t.Errorf("the shadower read %d lines on a hidden arrival", got)
				}
			})
		}
	}
}

// D4: a shadower that arrives in a room its quarry has already left gets no
// check, and a stale shadower (no condition 87) arriving gets none either.
func TestRoomChangeShadowFollow_ArrivalNeedsTheQuarryAndALiveShadow(t *testing.T) {
	for _, shadower := range shadowKinds {
		t.Run(string(shadower)+" shadower, quarry gone", func(t *testing.T) {
			s := newShadowScene(t)
			s.place(shadowPlayer, true, s.roomFar)
			s.place(shadower, false, s.roomB)
			c := s.sly(shadower)
			s.shadowTarget(c, shadowPlayer, false, true, false) // not hidden: would be spotted

			RoomChangeShadowFollow(moveEvent(shadower, false, shadowRoomA, shadowRoomB))

			if shadowEnded(c) || shadowCooldown(c) != 0 {
				t.Error("an arrival without the quarry ended the shadow")
			}
		})
		t.Run(string(shadower)+" shadower, stale", func(t *testing.T) {
			s := newShadowScene(t)
			s.place(shadowPlayer, true, s.roomB)
			s.place(shadower, false, s.roomB)
			c := s.sly(shadower)
			s.shadowTarget(c, shadowPlayer, false, false, false)

			RoomChangeShadowFollow(moveEvent(shadower, false, shadowRoomA, shadowRoomB))

			if shadowCooldown(c) != 0 {
				t.Error("a stale arrival started the spotted cooldown")
			}
			if userId, _ := actions.ShadowTargetOf(c); userId != shadowQuarryUser {
				t.Error("the arrival pass touched a stale shadow; only the follow pass clears one")
			}
		})
	}
}

// The listener is registered in place of the mob-only one.
func TestRoomChangeShadowFollowIsRegistered(t *testing.T) {
	src, err := os.ReadFile("hooks.go")
	if err != nil {
		t.Fatalf("reading hooks.go: %v", err)
	}
	if !strings.Contains(string(src), "events.RegisterListener(events.RoomChange{}, RoomChangeShadowFollow)") {
		t.Error("hooks.go no longer registers RoomChangeShadowFollow on RoomChange")
	}
}

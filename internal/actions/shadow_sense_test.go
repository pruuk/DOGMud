package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// shadowSenseTrials is how many rolls a sense test makes. The contest floor
// (Balance.ContestFloor, 0.125 in the Go defaults) gives each outcome at least
// a one in eight chance on every roll, so no single roll can be forced; over
// 200 rolls the chance of never seeing one outcome is below 0.875^200, about
// 3e-12.
const shadowSenseTrials = 200

// countSensedLines counts the sense line among userId's queued messages and
// drains them.
func countSensedLines(userId int) int {
	n := 0
	for _, msg := range events.DrainQueuedMessagesForTest(userId) {
		if strings.Contains(msg, shadowSensedLine) {
			n++
		}
	}
	return n
}

// A player target reads the sense line exactly when it senses the shadower,
// and the shadower trains Skullduggery on every roll, a win exactly when it
// went unsensed.
func TestShadowSenseRoll_PlayerTargetReadsTheLineAndBothOutcomesAward(t *testing.T) {
	const targetId = 7311
	target := users.NewTestUser(targetId, "senser", "Senser", 0)
	shadower := newShadowMobActor(100, 0, true)
	room := newStealTestRoom()
	targetActor := NewUserActorInRoom(target, room)
	events.DrainQueuedMessagesForTest(targetId)

	detected := 0
	for i := 0; i < shadowSenseTrials; i++ {
		if ShadowSenseRoll(shadower, targetActor, room) {
			detected++
		}
	}

	if detected == 0 || detected == shadowSenseTrials {
		t.Fatalf("detected %d of %d: both outcomes must occur for this test to prove anything", detected, shadowSenseTrials)
	}
	if got := countSensedLines(targetId); got != detected {
		t.Errorf("target read the sense line %d times, want %d (once per detection)", got, detected)
	}
	if len(shadower.awards) != shadowSenseTrials {
		t.Fatalf("shadower got %d awards over %d rolls, want one per roll on both outcomes", len(shadower.awards), shadowSenseTrials)
	}
	lost := 0
	for _, a := range shadower.awards {
		if !a.won {
			lost++
		}
		if len(a.cands) != 1 || a.cands[0].Skill != string(skills.Skullduggery) {
			t.Fatalf("award candidates %+v, want Skullduggery alone", a.cands)
		}
	}
	if lost != detected {
		t.Errorf("%d lost awards, want %d: a detected roll is the shadower's loss", lost, detected)
	}
}

// A mob target gets nothing visible, but the roll is real: it senses the
// shadower some of the time and the shadower trains either way.
func TestShadowSenseRoll_MobTargetShowsNothingAndBothOutcomesAward(t *testing.T) {
	const observerId = 7312
	observer := users.NewTestUser(observerId, "bystander", "Bystander", 0)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{observerId: observer})
	defer cleanupUsers()

	m := &mobs.Mob{InstanceId: 88312, Character: *characters.New()}
	m.Character.Name = "Watcher"
	m.Character.Stats.Perception.ValueAdj = 100
	room := newStealTestRoom()
	room.AddPlayer(observerId)
	defer room.RemovePlayer(observerId)

	shadower := newShadowPlayerActor(100, 0, true)
	events.DrainQueuedMessagesForTest(observerId)

	detected := 0
	for i := 0; i < shadowSenseTrials; i++ {
		if ShadowSenseRoll(shadower, NewMobActorInRoom(m, room), room) {
			detected++
		}
	}

	if detected == 0 || detected == shadowSenseTrials {
		t.Fatalf("detected %d of %d: both outcomes must occur", detected, shadowSenseTrials)
	}
	if got := len(events.DrainQueuedMessagesForTest(observerId)); got != 0 {
		t.Errorf("a player in the room received %d messages; sensing is awareness only", got)
	}
	if len(shadower.awards) != shadowSenseTrials {
		t.Errorf("shadower got %d awards over %d rolls, want one per roll", len(shadower.awards), shadowSenseTrials)
	}
}

// Ruling 5: starting a shadow on a mob runs the same sense roll as on a
// player. Before, it ran no contest and awarded a win every time.
func TestShadow_MobTargetRollsTheSenseContest(t *testing.T) {
	const instId = 88313
	m := &mobs.Mob{InstanceId: instId, Character: *characters.New()}
	m.Character.Name = "Sentry"
	m.Character.Stats.Perception.ValueAdj = 100
	mobs.SetInstanceForTest(instId, m)
	defer mobs.SetInstanceForTest(instId, nil)

	actor := newShadowMobActor(100, 0, true)

	detected := 0
	for i := 0; i < shadowSenseTrials; i++ {
		delete(actor.char.Cooldowns, skills.Skullduggery.String("shadow"))
		resetHiddenCondition(actor)
		result := Shadow(actor, ShadowOptions{TargetMobInstanceId: instId})
		if !result.Succeeded {
			t.Fatalf("trial %d: shadow did not start: %+v", i, result)
		}
		if result.Detected {
			detected++
		}
		if last := actor.awards[len(actor.awards)-1]; last.won == result.Detected {
			t.Fatalf("trial %d: award won=%v with Detected=%v; the award must be the roll's outcome", i, last.won, result.Detected)
		}
	}

	if detected == 0 || detected == shadowSenseTrials {
		t.Fatalf("detected %d of %d: a mob target must really roll", detected, shadowSenseTrials)
	}
	if len(actor.awards) != shadowSenseTrials {
		t.Errorf("%d awards over %d starts, want one per start", len(actor.awards), shadowSenseTrials)
	}
	if userId, mobInstanceId := ShadowTargetOf(actor.char); userId != 0 || mobInstanceId != instId {
		t.Errorf("ShadowTargetOf = (%d, %d), want (0, %d)", userId, mobInstanceId, instId)
	}
}

package actions

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
)

// Lighting plan 5b, score-only sites: the party who needs to see pays
// SightMult on their score. Where the score is a pure helper the check is an
// exact ratio (0.88 at lamp 90 against lamp 60). Where the score is built
// inline and handed straight to combat.RunContest there is no recording seam,
// so the test compares success RATES at a comfortable lamp (60, SightMult
// 1.0) and a dazzling one (90, SightMult 0.88).
//
// The bar is computed, not guessed: the dazzled rate must sit at least
// sightRampSEs standard errors of the difference below the measured
// comfortable rate (sightRampRateGap, shared with the search and track tests).
//
// The thief and defuser tests in this file sit at parity (about 50%
// comfortable), the steepest part of the contest curve: one standard error of
// the difference is about 1.3 points at 3000 trials, so the bar is near 4
// points; measured gaps on 2026-09-26 were 16 (thief) and 18 (defuser) points.
// The search and track tests (search_contest_test.go, track_contest_test.go)
// reuse the 175-vs-125 rate fixtures and sit near 91% comfortable: one
// standard error is about 0.86 points, so their bar is about 2.6 points
// against measured gaps of about 10 (search 92.2 to 81.3, track 91.9 to 82.3). Sabotage-checked the
// same day: with the thief's SightMult replaced by 1 the thief test fails.

const (
	sightRampTrials = 3000
	sightRampSEs    = 3.0
)

// sightRampRateGap fails the test unless dazzled sits at least sightRampSEs
// standard errors of the difference below comfortable. It first refuses a
// degenerate fixture: a comfortable rate near 0 or 1 (for example a fixture
// that breaks every attempt) or a zero standard error means the comparison
// cannot see the ramp, and must not leave the test green.
func sightRampRateGap(t *testing.T, what string, comfortable, dazzled float64) {
	t.Helper()
	if comfortable <= 0.05 || comfortable >= 0.95 {
		t.Fatalf("%s: fixture premise: comfortable rate %.1f%% is outside (5%%, 95%%); "+
			"the fixture is broken or saturated and cannot show the ramp", what, comfortable*100)
	}
	n := float64(sightRampTrials)
	se := math.Sqrt(comfortable*(1-comfortable)/n + dazzled*(1-dazzled)/n)
	if se == 0 {
		t.Fatalf("%s: fixture premise: zero standard error; the rates cannot be compared", what)
	}
	t.Logf("%s: comfortable %.1f%%, dazzled %.1f%%, se %.2f points",
		what, comfortable*100, dazzled*100, se*100)
	if bar := comfortable - sightRampSEs*se; dazzled > bar {
		t.Fatalf("%s: dazzled %.1f%% vs comfortable %.1f%%, want at most %.1f%% "+
			"(%.0f standard errors below); the sight ramp is not reaching the roll",
			what, dazzled*100, comfortable*100, bar*100, sightRampSEs)
	}
}

// The victim's (or bystander's) noticing score pays the victim's eyes.
func TestStealVictimScoreTakesTheVictimsEyes(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.Validate()
	configs.SetConfigForTest(t, cfg)
	v := characters.New()
	v.Stats.Perception.ValueAdj = 100
	comfortable := stealVictimScore(v, fixedLight(60))
	dazzled := stealVictimScore(v, fixedLight(90))
	if math.Abs(dazzled/comfortable-0.88) > 1e-9 {
		t.Errorf("dazzled/comfortable = %v, want 0.88", dazzled/comfortable)
	}
}

// The thief's attack score pays the thief's eyes. The victim is Blinded, so
// its own sight ramp is fully dark (0.80) at both lamps and only the thief's
// side moves between the two runs.
func TestStealPaysTheThiefsEyes(t *testing.T) {
	sw := float64(configs.GetBalanceConfig().SkillWeight)
	rate := func(lamp int) float64 {
		target := &mobs.Mob{InstanceId: testMobInstId}
		target.Character = *characters.New()
		target.Character.Name = "Bandit"
		target.Character.Stats.Perception.ValueAdj = 188 // x0.80 blind = 150.4
		if err := target.Character.Perception.TransitionTo(perception.Blinded,
			state.TransitionReason{Trigger: "test"}); err != nil {
			t.Fatalf("blind the victim: %v", err)
		}
		mobs.SetInstanceForTest(testMobInstId, target)
		defer mobs.SetInstanceForTest(testMobInstId, nil)

		won := 0
		for i := 0; i < sightRampTrials; i++ {
			// Rank 2 keeps this thief's score fixed; Dex tops it up to 150.
			actor := newStealPlayerActor(150-int(2*sw), 2)
			actor.room.Lamp = rooms.LampPtr(lamp)
			// A player's pickpocket is revealed after a pause, and only if
			// the mark is still in the thief's room (steal_pocket.go).
			target.Character.RoomId = actor.room.RoomId
			target.Character.Gold = 50
			if Steal(actor, StealOptions{TargetMobInstanceId: testMobInstId}).Succeeded {
				won++
			}
		}
		return float64(won) / sightRampTrials
	}
	sightRampRateGap(t, "thief", rate(60), rate(90))
}

// The defuser's score pays the defuser's eyes; the trap is a fixed number.
func TestDefusePaysTheDefusersEyes(t *testing.T) {
	rate := func(lamp int) float64 {
		won := 0
		for i := 0; i < sightRampTrials; i++ {
			// Perception 45 + rank 3 x 25 = 120 against lock difficulty 12 x 10:
			// parity when comfortable.
			actor := newDefuseActor(45, 3)
			actor.room.Lamp = rooms.LampPtr(lamp)
			actor.room.Containers = map[string]rooms.Container{
				defuseTestContainerName: {Lock: gamelock.Lock{
					Difficulty:       12,
					TrapConditionIds: []int{defuseTestTrapConditionId},
				}},
			}
			seedDisarmKit(actor, 9950, 0)
			if Defuse(actor, DefuseOptions{TargetNoun: defuseTestContainerName}).Succeeded {
				won++
			}
		}
		return float64(won) / sightRampTrials
	}
	sightRampRateGap(t, "defuser", rate(60), rate(90))
}

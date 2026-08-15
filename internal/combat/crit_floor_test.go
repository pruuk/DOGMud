package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// Chunk 5.11e — crit floors, 1% of CONTEST WINS, both directions.
//
// Deliberately 1% of wins and NOT 1% of swings. A badly outclassed attacker
// wins about 15% of contests, so 1% of swings would demand roughly 6.7% of
// their wins be crits — a higher per-win rate than an even match gets at 2.3%,
// which is incoherent. 1% of wins composes with the contest floor as two
// independent last resorts, each sized to the failure it protects.
//
// U6 Task 9 moved that denominator off res.hit and onto the contest winner. The
// two agreed until Task 10 made a defensive win deal partial damage.

func TestApplyCritFloor_AlreadyCritIsUntouched(t *testing.T) {
	// The floor only ever promotes. It must never demote a real crit, even
	// with the floor disabled.
	if !ApplyCritFloor(true, 0.0) {
		t.Error("a real crit must survive a zero floor")
	}
	if !ApplyCritFloor(true, 1.0) {
		t.Error("a real crit must survive a certain floor")
	}
}

func TestApplyCritFloor_ZeroFloorNeverPromotes(t *testing.T) {
	// Setting the knob to 0 must switch the mechanic off completely, which is
	// the escape hatch if it plays badly.
	for i := 0; i < 5000; i++ {
		if ApplyCritFloor(false, 0.0) {
			t.Fatal("a zero floor must never promote a non-crit")
		}
	}
}

func TestApplyCritFloor_CertainFloorAlwaysPromotes(t *testing.T) {
	for i := 0; i < 5000; i++ {
		if !ApplyCritFloor(false, 1.0) {
			t.Fatal("a floor of 1.0 must always promote")
		}
	}
}

func TestApplyCritFloor_RateMatchesTheFloor(t *testing.T) {
	const samples = 200000
	promoted := 0
	for i := 0; i < samples; i++ {
		if ApplyCritFloor(false, 0.01) {
			promoted++
		}
	}
	rate := float64(promoted) / samples
	if math.Abs(rate-0.01) > 0.002 {
		t.Errorf("promotion rate = %.4f, want ~0.01", rate)
	}
}

// ─── the load-bearing ordering invariant ────────────────────────────────────
//
// applyCritFloors takes its floors as PARAMETERS rather than reading the
// balance config, for two reasons. It makes these tests deterministic (a floor
// of 1.0 turns "does it respect the guard" from a statistical question into a
// yes/no one), and a Go test binary never loads _datafiles/config.yaml anyway,
// so config-driven floors would read 0 here and every assertion below would be
// vacuously true.

// dodgeBest is a minimal bestDefenseResult naming a real defence type, which is
// what the defensive floor keys on.
//
// U6 Task 9: the floors now key on WHO WON THE CONTEST, so the margin is no
// longer incidental to these fixtures. bestDefenseResult.margin is
// DEFENCE-POSITIVE, so a NEGATIVE margin is an attack win and a POSITIVE one is
// a defence win. dodgeBest() defaults to an attack win; use dodgeBestWon() to
// name the winner explicitly.
func dodgeBest() bestDefenseResult {
	return dodgeBestWon(attackWinMargin)
}

// Sign-named margin constants, because a bare -5.0 in a fixture reads as "some
// number" and the sign is the entire point.
const (
	attackWinMargin  = -5.0 // defence-positive convention: negative = attack won
	defenceWinMargin = 5.0  // positive = the defence won
)

func dodgeBestWon(margin float64) bestDefenseResult {
	return bestDefenseResult{defenseType: characters.DefenseDodge, margin: margin}
}

// TestApplyCritFloors_NeverPromotesAContestLoss is the whole reason the floor
// runs last.
//
// An attack crit FORCES a hit in resolveDefenseOutcome (the crit step returns
// res.hit = true). A crit floor evaluated before the outcome was final would
// therefore become a second, undeclared hit floor stacked on ContestFloor,
// letting hopeless attackers connect more often than the floor intends. Its
// observable form is an attack crit on a swing the attacker did not win.
//
// U6 Task 9 restated this from "a swing that MISSED" to "a swing the attacker
// did not WIN", and it is not a rename: Task 10 makes a defensive win deal
// partial damage, so the miss framing stops describing the same set of swings.
// The fixture below is the strict form — the defence won AND the swing still
// landed damage — which the old res.hit split would have handed to the
// attacker's floor.
func TestApplyCritFloors_NeverPromotesAContestLoss(t *testing.T) {
	for i := 0; i < 200; i++ {
		res := hitResolution{hit: true}
		applyCritFloors(&res, &AttackResult{}, dodgeBestWon(defenceWinMargin), 1.0, 0.0)

		if res.crit {
			t.Fatal("a swing the attacker did not win must never be promoted to an attack crit")
		}
	}

	// And the zero-damage form, which is what this test asserted before Task 9.
	res := hitResolution{hit: false}
	applyCritFloors(&res, &AttackResult{}, dodgeBestWon(defenceWinMargin), 1.0, 0.0)
	if res.crit {
		t.Error("a defended swing must never be promoted to an attack crit")
	}
	if res.hit {
		t.Error("the crit floor must never flip a miss into a hit")
	}
}

func TestApplyCritFloors_PromotesALandedHit(t *testing.T) {
	// dodgeBest() carries an attack-win margin, which since Task 9 is what the
	// attack floor keys on rather than res.hit.
	res := hitResolution{hit: true}
	applyCritFloors(&res, &AttackResult{}, dodgeBest(), 1.0, 0.0)

	if !res.crit {
		t.Error("a landed hit must be promotable by the attack floor")
	}
	if !res.hit {
		t.Error("promotion must not disturb the hit outcome")
	}
}

func TestApplyCritFloors_NeverPromotesAFumble(t *testing.T) {
	// An attack fumble is a miss by definition, and it is the ATTACKER's
	// blunder rather than a defensive success, so it must not hand the defender
	// a free riposte either. Both floors are set to certain to prove the guard
	// rather than the odds.
	res := hitResolution{hit: false, fumble: true}
	applyCritFloors(&res, &AttackResult{}, dodgeBest(), 1.0, 1.0)

	if res.crit {
		t.Error("a fumbled attack must never become a crit")
	}
	if res.defenseCrit {
		t.Error("an attacker's fumble is not a defensive crit")
	}
}

func TestApplyCritFloors_PromotesASuccessfulDefense(t *testing.T) {
	// "Successful defence" means the defence WON THE CONTEST since Task 9, not
	// merely that res.hit came out false, so the margin has to say so.
	result := &AttackResult{}
	res := hitResolution{hit: false}
	applyCritFloors(&res, result, dodgeBestWon(defenceWinMargin), 0.0, 1.0)

	if !res.defenseCrit {
		t.Error("a successful defence must be promotable by the defence floor")
	}
	if res.hit {
		t.Error("defensive promotion must not flip the hit outcome")
	}
	// Downstream riposte/auto-trip wiring reads these per-defence flags, so a
	// promotion that skipped them would be a crit nothing acts on.
	if !result.DodgeCritDetected {
		t.Error("promotion must set the per-defence crit flag")
	}
}

func TestApplyCritFloors_NoDefenseAttemptedIsNotADefensiveCrit(t *testing.T) {
	// defenseType "" means the defender never mounted a defence (an empty
	// defence sequence), so the contest never happened. Granting a riposte to
	// someone who did not defend would be nonsense, and setDefenseCritFlags has
	// no flag to set anyway.
	res := hitResolution{hit: false}
	applyCritFloors(&res, &AttackResult{}, bestDefenseResult{defenseType: ""}, 0.0, 1.0)

	if res.defenseCrit {
		t.Error("no defence was attempted; there is nothing to promote")
	}
}

// TestApplyCritFloors_UncontestedSwingTakesTheAttackFloor pins the handling of
// the uncontested swing, which is the one case where the two guards interact.
//
// runBestOfAllDefense leaves margin at math.Inf(-1) on exactly the path where it
// also leaves defenseType empty, and -Inf satisfies `margin <= 0`. That reads as
// a decisive attack win, which is precisely what an uncontested swing is: no
// defence was mounted, so the attacker cannot have lost.
//
// It therefore takes the ATTACK floor, and must never take the defence floor or
// set a per-defence crit flag.
//
// This preserves pre-U6 behaviour. Before Task 9 the split was res.hit, which
// was true on this path, so the attack floor already applied here. An earlier
// draft hoisted the defenceType guard above the margin test, which silently
// dropped that 1% promotion — a behaviour change U6 does not intend and did not
// declare. The guard belongs on the defence branch only.
func TestApplyCritFloors_UncontestedSwingTakesTheAttackFloor(t *testing.T) {
	promoted := 0
	for i := 0; i < 200; i++ {
		result := &AttackResult{}
		res := hitResolution{hit: true}
		applyCritFloors(&res, result, bestDefenseResult{margin: math.Inf(-1)}, 1.0, 1.0)

		if res.crit {
			promoted++
		}
		if res.defenseCrit {
			t.Fatal("an uncontested swing has no defender to promote")
		}
		if result.DodgeCritDetected || result.ParryCritDetected || result.BlockCritDetected {
			t.Fatal("an uncontested swing must not set a per-defence crit flag")
		}
	}

	if promoted != 200 {
		t.Fatalf("an attack floor of 1.0 must promote every uncontested swing, got %d of 200", promoted)
	}
}

// ─── U6 Task 9: the denominators are WHO WON, not whether it hit ────────────
//
// The floors used to split on res.hit: a landed swing went to the attack floor,
// anything else to the defence floor. That question stops being answerable in
// Task 10, where a defensive win deals PARTIAL damage rather than zero — every
// deflected swing then carries res.hit == true while the defence is the side
// that actually won the contest, and the old split would have handed all of them
// to the attacker's floor.
//
// best.margin is DEFENCE-POSITIVE, so `margin <= 0` is an attack win.

func TestApplyCritFloors_AttackFloorAppliesOnlyToContestWins(t *testing.T) {
	// Certain attack floor, dead defence floor. Every swing the ATTACK won must
	// be promoted, and nothing may leak to the defensive side.
	for i := 0; i < 200; i++ {
		result := &AttackResult{}
		res := hitResolution{hit: true}
		applyCritFloors(&res, result, dodgeBestWon(attackWinMargin), 1.0, 0.0)

		if !res.crit {
			t.Fatal("a swing that won the contest must be promotable by the attack floor")
		}
		if res.defenseCrit {
			t.Fatal("an attack win must never be promoted on the defensive side")
		}
		if result.DodgeCritDetected {
			t.Fatal("an attack win must not set a per-defence crit flag")
		}
	}
}

func TestApplyCritFloors_DefenceFloorAppliesOnlyToDefenceWins(t *testing.T) {
	// res.hit is deliberately TRUE here. That is the Task 10 shape: a defended
	// swing that still deals partial damage. Under the old res.hit split this
	// case went down the ATTACKER's branch and the defender's floor never ran.
	for i := 0; i < 200; i++ {
		result := &AttackResult{}
		res := hitResolution{hit: true}
		applyCritFloors(&res, result, dodgeBestWon(defenceWinMargin), 0.0, 1.0)

		if !res.defenseCrit {
			t.Fatal("a swing the defence won must be promotable by the defence floor, even when it dealt damage")
		}
		if res.crit {
			t.Fatal("a defence win must never be promoted on the attacking side")
		}
		// Downstream riposte / auto-trip / auto-bash wiring reads the
		// per-defence flags, so a promotion that skipped them is a crit nothing
		// acts on.
		if !result.DodgeCritDetected {
			t.Fatal("defensive promotion must set the per-defence crit flag")
		}
		if !res.hit {
			t.Fatal("the crit floor must not disturb the hit outcome")
		}
	}
}

func TestApplyCritFloors_FlooredOutcomeIsNeverPromoted(t *testing.T) {
	// A floored outcome carries the +-1 sentinel margin: it represents an
	// outcome the contest did not actually produce. Promoting it would hand a
	// decisive result to the side that lost the roll. Both floors are certain,
	// so this proves the guard rather than the odds.
	for _, tc := range []struct {
		name   string
		margin float64
		hit    bool
	}{
		{"floored attack win", attackWinMargin, true},
		{"floored defence win", defenceWinMargin, true},
		{"floored defence win, zero damage", defenceWinMargin, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < 200; i++ {
				result := &AttackResult{}
				best := dodgeBestWon(tc.margin)
				best.floored = true
				res := hitResolution{hit: tc.hit}
				applyCritFloors(&res, result, best, 1.0, 1.0)

				if res.crit {
					t.Fatal("a floored outcome must never be promoted to an attack crit")
				}
				if res.defenseCrit {
					t.Fatal("a floored outcome must never be promoted to a defensive crit")
				}
				if result.DodgeCritDetected {
					t.Fatal("a floored outcome must not set a per-defence crit flag")
				}
			}
		})
	}
}

func TestApplyCritFloors_NeverDemotesARealCrit(t *testing.T) {
	res := hitResolution{hit: true, crit: true}
	applyCritFloors(&res, &AttackResult{}, dodgeBest(), 0.0, 0.0)
	if !res.crit {
		t.Error("a real crit must survive a zero floor")
	}

	// The defensive half must be a DEFENCE win, or it returns on the attack
	// branch and never reaches the code this asserts about.
	defRes := hitResolution{hit: false, defenseCrit: true}
	applyCritFloors(&defRes, &AttackResult{}, dodgeBestWon(defenceWinMargin), 0.0, 0.0)
	if !defRes.defenseCrit {
		t.Error("a real defensive crit must survive a zero floor")
	}
}

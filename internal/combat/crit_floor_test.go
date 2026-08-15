package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// Chunk 5.11e — crit floors, 1% of HITS, both directions.
//
// Deliberately 1% of hits and NOT 1% of swings. A badly outclassed attacker
// hits about 15% of the time, so 1% of swings would demand roughly 6.7% of
// their hits be crits — a higher per-hit rate than an even match gets at 2.3%,
// which is incoherent. 1% of hits composes with the 5.9 hit floor as two
// independent last resorts, each sized to the failure it protects.

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
func dodgeBest() bestDefenseResult {
	return bestDefenseResult{defenseType: characters.DefenseDodge}
}

// TestApplyCritFloors_NeverPromotesAMiss is the whole reason the floor runs
// last.
//
// An attack crit FORCES a hit in resolveDefenseOutcome (the crit step returns
// res.hit = true). A crit floor evaluated before the hit outcome was final
// would therefore become a second, undeclared hit floor stacked on
// ContestFloor, letting hopeless attackers connect more often than the floor
// intends. Its observable form is a crit on a swing that missed.
func TestApplyCritFloors_NeverPromotesAMiss(t *testing.T) {
	res := hitResolution{hit: false}
	applyCritFloors(&res, &AttackResult{}, dodgeBest(), 1.0, 0.0)

	if res.crit {
		t.Error("a missed swing must never be promoted to a crit")
	}
	if res.hit {
		t.Error("the crit floor must never flip a miss into a hit")
	}
}

func TestApplyCritFloors_PromotesALandedHit(t *testing.T) {
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
	result := &AttackResult{}
	res := hitResolution{hit: false}
	applyCritFloors(&res, result, dodgeBest(), 0.0, 1.0)

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
	// defenseType "" means the defender never mounted a defence (no stamina,
	// or an empty defence sequence) and the miss came from the 5.9 defence
	// floor instead. Granting a riposte to someone who did not defend would be
	// nonsense, and setDefenseCritFlags has no flag to set anyway.
	res := hitResolution{hit: false}
	applyCritFloors(&res, &AttackResult{}, bestDefenseResult{defenseType: ""}, 0.0, 1.0)

	if res.defenseCrit {
		t.Error("no defence was attempted; there is nothing to promote")
	}
}

func TestApplyCritFloors_NeverDemotesARealCrit(t *testing.T) {
	res := hitResolution{hit: true, crit: true}
	applyCritFloors(&res, &AttackResult{}, dodgeBest(), 0.0, 0.0)
	if !res.crit {
		t.Error("a real crit must survive a zero floor")
	}

	defRes := hitResolution{hit: false, defenseCrit: true}
	applyCritFloors(&defRes, &AttackResult{}, dodgeBest(), 0.0, 0.0)
	if !defRes.defenseCrit {
		t.Error("a real defensive crit must survive a zero floor")
	}
}

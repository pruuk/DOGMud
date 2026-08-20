package hooks

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// TestProcessGrapplePair_StashesDriftSnapshot verifies that after a
// processGrapplePair call, both the controller and the controlled
// character have a DriftRollSnapshot populated with the current round
// number and non-zero z-scores.
func TestProcessGrapplePair_StashesDriftSnapshot(t *testing.T) {
	a := characters.New()
	a.SetUserId(1)
	b := characters.New()
	b.SetUserId(2)

	// Give both characters meaningful stats so the roll produces
	// a non-trivial result.
	a.Stats.Strength.Base = 100
	b.Stats.Strength.Base = 100
	a.Stats.Dexterity.Base = 100
	b.Stats.Dexterity.Base = 100
	a.Stamina = 100
	b.Stamina = 100
	a.StaminaMax.Base = 100
	b.StaminaMax.Base = 100
	a.StaminaMax.Value = 100
	b.StaminaMax.Value = 100

	// Walk Standing → Clinch → Mount (same path as ConsistencyCheck tests).
	if err := position.TransitionPair(a, b, position.Clinch,
		state.TransitionReason{Trigger: position.TriggerGrappleEntry}); err != nil {
		t.Fatalf("TransitionPair → Clinch failed: %v", err)
	}
	if err := position.TransitionPair(a, b, position.Mount,
		state.TransitionReason{Trigger: position.TriggerTakedownMount}); err != nil {
		t.Fatalf("TransitionPair → Mount failed: %v", err)
	}

	roundBefore := util.GetRoundCount()
	processGrapplePair(a, b)
	roundAfter := util.GetRoundCount()

	// Both snapshots should be stamped with a round in [roundBefore, roundAfter].
	if a.LastDriftRoll.Round < roundBefore || a.LastDriftRoll.Round > roundAfter {
		t.Errorf("a.LastDriftRoll.Round = %d, expected in [%d, %d]",
			a.LastDriftRoll.Round, roundBefore, roundAfter)
	}
	if b.LastDriftRoll.Round < roundBefore || b.LastDriftRoll.Round > roundAfter {
		t.Errorf("b.LastDriftRoll.Round = %d, expected in [%d, %d]",
			b.LastDriftRoll.Round, roundBefore, roundAfter)
	}

	// Both sides should see the same round.
	if a.LastDriftRoll.Round != b.LastDriftRoll.Round {
		t.Errorf("round mismatch: a=%d b=%d", a.LastDriftRoll.Round, b.LastDriftRoll.Round)
	}

	// Z-scores should be non-zero: the contest core rolls both sides at
	// stat=100, far enough from zero that both z-scores will be set.
	if a.LastDriftRoll.AttackerZScore == 0 && a.LastDriftRoll.DefenderZScore == 0 {
		t.Error("both z-scores are zero — snapshot was not populated")
	}

	// Both snapshots should store the same margin (same roll, stored on both sides).
	if a.LastDriftRoll.MarginAttacker != b.LastDriftRoll.MarginAttacker {
		t.Errorf("margin mismatch: a=%v b=%v",
			a.LastDriftRoll.MarginAttacker, b.LastDriftRoll.MarginAttacker)
	}
}

// makeGrappleCharacter constructs a minimal Character with the given
// stats + skill level for grappleScore unit tests. Stamina and
// encumbrance multipliers will resolve to 1.0 because no penalties
// apply (Stamina at max, no items carried).
//
// Note: characters.New() calls ensureAllSkills which floors every
// skill at 1. makeGrappleCharacter always writes the explicit ucSkill
// value (including 0) to override that floor, so tests can verify
// the formula's skill term in isolation.
func makeGrappleCharacter(t *testing.T, str, dex, ucSkill int) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Stats.Strength.Base = str
	c.Stats.Dexterity.Base = dex
	c.Stats.Strength.Recalculate()
	c.Stats.Dexterity.Recalculate()
	c.Stamina = 1000
	c.StaminaMax.Value = 1000
	if c.Skills == nil {
		c.Skills = map[string]int{}
	}
	// Always write ucSkill (even 0) to override the rank-1 floor set by
	// ensureAllSkills during New().
	c.Skills[string(skills.UnarmedCombat)] = ucSkill
	return c
}

// grappleScoreApproxEqual returns true when |a-b| < eps. Used for
// float comparisons in grappleScore tests (2.2*50 produces a tiny
// rounding error with IEEE-754 doubles).
func grappleScoreApproxEqual(a, b float64) bool {
	const eps = 1e-9
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

func TestGrappleScore_StrCoefficient(t *testing.T) {
	cfg := configs.GetBalanceConfig()
	c := makeGrappleCharacter(t, 100, 0, 0)
	got := grappleScore(c, false, cfg, true)
	want := 70.0 // 0.7 * 100 + 0 + 0
	if !grappleScoreApproxEqual(got, want) {
		t.Errorf("Str=100 only → score = %v, want %v", got, want)
	}
}

func TestGrappleScore_DexCoefficient(t *testing.T) {
	cfg := configs.GetBalanceConfig()
	c := makeGrappleCharacter(t, 0, 100, 0)
	got := grappleScore(c, false, cfg, true)
	want := 30.0 // 0 + 0.3 * 100 + 0
	if !grappleScoreApproxEqual(got, want) {
		t.Errorf("Dex=100 only → score = %v, want %v", got, want)
	}
}

// U6b Task 14: this test previously pinned the hardcoded 2.0 defender
// skill coefficient — a defect (every other additive contest score uses
// the global SkillWeight). Now pins skill × SkillWeight.
func TestGrappleScore_SkillCoefficientDefender(t *testing.T) {
	cfg := configs.GetBalanceConfig()
	cfg.SkillWeight = 5.0 // shipped value; grappleScore must read it from cfg
	c := makeGrappleCharacter(t, 0, 0, 50)
	got := grappleScore(c, false, cfg, true)
	want := 250.0 // 0 + 0 + SkillWeight(5) * 50
	if !grappleScoreApproxEqual(got, want) {
		t.Errorf("UC=50 defender → score = %v, want %v", got, want)
	}
}

// U6b Task 14: previously pinned the hardcoded 2.2 aggressor coefficient
// (an accidental 2.2-vs-2.0 edge) — a defect. The aggressor's edge is now
// the explicit GrappleAggressorDriftBonus multiplier on the whole score.
func TestGrappleScore_SkillCoefficientAggressor(t *testing.T) {
	cfg := configs.GetBalanceConfig()
	cfg.SkillWeight = 5.0
	cfg.GrappleAggressorDriftBonus = 1.038 // shipped value (modelling solve)
	c := makeGrappleCharacter(t, 0, 0, 50)
	got := grappleScore(c, true, cfg, true)
	want := 250.0 * 1.038 // (0 + 0 + 5*50) × aggressor bonus
	if !grappleScoreApproxEqual(got, want) {
		t.Errorf("UC=50 aggressor → score = %v, want %v", got, want)
	}
}

// U6b Task 14: previously pinned the 2.0 defender coefficient (defect);
// now the full formula at SkillWeight.
func TestGrappleScore_CombinedFormulaDefender(t *testing.T) {
	cfg := configs.GetBalanceConfig()
	cfg.SkillWeight = 5.0
	c := makeGrappleCharacter(t, 100, 100, 30)
	got := grappleScore(c, false, cfg, true)
	want := 250.0 // 0.7*100 + 0.3*100 + 5*30 = 70 + 30 + 150
	if !grappleScoreApproxEqual(got, want) {
		t.Errorf("S100/D100/UC30 defender → score = %v, want %v", got, want)
	}
}

// U6b Task 14: previously pinned the 2.2 aggressor coefficient (defect);
// now the full formula at SkillWeight × GrappleAggressorDriftBonus.
func TestGrappleScore_CombinedFormulaAggressor(t *testing.T) {
	cfg := configs.GetBalanceConfig()
	cfg.SkillWeight = 5.0
	cfg.GrappleAggressorDriftBonus = 1.038
	c := makeGrappleCharacter(t, 100, 100, 30)
	got := grappleScore(c, true, cfg, true)
	want := 250.0 * 1.038 // (0.7*100 + 0.3*100 + 5*30) × aggressor bonus
	if !grappleScoreApproxEqual(got, want) {
		t.Errorf("S100/D100/UC30 aggressor → score = %v, want %v", got, want)
	}
}

// TestGrappleScore_AggressorBonusIsWholeScoreMultiplier pins the shape of
// the restored aggressor edge (U6b Task 14 gate decision §5.4): a single
// multiplier on the aggressor's whole drift score, config-driven — not a
// per-term coefficient tweak.
func TestGrappleScore_AggressorBonusIsWholeScoreMultiplier(t *testing.T) {
	cfg := configs.GetBalanceConfig()
	cfg.SkillWeight = 5.0
	cfg.GrappleAggressorDriftBonus = 1.25 // arbitrary non-default: must be read from cfg
	c := makeGrappleCharacter(t, 90, 110, 20)
	def := grappleScore(c, false, cfg, true)
	agg := grappleScore(c, true, cfg, true)
	if !grappleScoreApproxEqual(agg, def*1.25) {
		t.Errorf("aggressor score = %v, want defender score %v × 1.25 = %v", agg, def, def*1.25)
	}
}

func TestGrappleScore_EscapeModifierIgnored(t *testing.T) {
	// EscapeModifier on body armor should NOT change the score.
	// Verification: grappleScore never references EscapeModifier
	// (no field read in the function body).
	cfg := configs.GetBalanceConfig()
	c := makeGrappleCharacter(t, 100, 100, 0)
	baseline := grappleScore(c, false, cfg, true)
	got := grappleScore(c, false, cfg, true)
	if !grappleScoreApproxEqual(got, baseline) {
		t.Errorf("score should not depend on EscapeModifier; got %v, baseline %v", got, baseline)
	}
}

func TestGrappleScore_NilCharacter(t *testing.T) {
	cfg := configs.GetBalanceConfig()
	got := grappleScore(nil, false, cfg, true)
	if got != 0 {
		t.Errorf("nil character → score = %v, want 0", got)
	}
}

// TestGrappleScore_QuesterVsBoarSurvives is the regression test for
// the 2026-05-19 bug where quester0 immediately escaped from every
// grapple against a steppe boar. With the new formula, quester0's
// 30 unarmed-combat training should bring them into competitive
// range (not 80+ points behind).
//
// Stats from production YAML:
//
//	quester0: Str.Base=103 + Training=13 = 116; Dex.Base=113 + Training=13 = 126
//	quester0 skill: unarmed-combat=30
//	steppe boar (typical spawn): Str≈149, Dex≈91, no skill
//
// Quester0 grappling boar: quester0 is aggressor. Expected (U6b Task 14
// maths — SkillWeight 5, aggressor bonus 1.038, √2-normalised z):
//
//	atk = ((0.7*116 + 0.3*126) + 5*30) × 1.038 = 269 × 1.038 = 279.2
//	def = (0.7*149 + 0.3*91)  + 5*0  = 131.6
//	margin = 147.6, z = 147.6/(279.2*0.15*√2) ≈ +2.49 → 3-step advance.
//
// U6b Task 14: this test previously pinned TWO defects — the 2.2/2.0
// coefficients and the √2-less z (inflated ~41%). Under the corrected
// maths quester0's expected band moves from 2-step to 3-step because
// SkillWeight (5) rewards the 30-rank training gap far more than the
// old 2.2 coefficient did.
//
// We don't roll the dice here (too flaky) — we just verify the
// deterministic score components and the resulting margin.
func TestGrappleScore_QuesterVsBoarSurvives(t *testing.T) {
	cfg := configs.GetBalanceConfig()
	cfg.SkillWeight = 5.0
	cfg.GrappleAggressorDriftBonus = 1.038

	quester := makeGrappleCharacter(t, 116, 126, 30)
	boar := makeGrappleCharacter(t, 149, 91, 0)

	atkScore := grappleScore(quester, true, cfg, true) // quester is aggressor
	defScore := grappleScore(boar, false, cfg, true)

	// Quester0's score should exceed boar's despite raw Str gap.
	if atkScore <= defScore {
		t.Errorf("quester0 (trained) score %.1f should exceed boar score %.1f", atkScore, defScore)
	}

	// Expected mean z lands in the 3-step band (z >= 2.0). The √2 factor
	// mirrors processGrapplePairWithContest: both sides roll with the
	// attacker's stdDev, so the margin's stdDev is stdDev*√2.
	margin := atkScore - defScore
	sigma := atkScore * 0.15 * math.Sqrt2 // RollSpread default × √2
	z := margin / sigma
	if z < 2.0 {
		t.Errorf("expected z >= 2.0 (3-step advance band), got z=%.2f (atk=%.1f def=%.1f margin=%.1f sigma=%.1f)",
			z, atkScore, defScore, margin, sigma)
	}
}

// TestGrappleScore_UntrainedVsTrainedDefenderEscapes is the inverse
// regression: an untrained aggressor against a trained defender
// should produce z ≤ -2.0 (escape). Confirms the formula doesn't
// silently make grapples always-favor-aggressor.
// U6b Task 14: this test previously computed z without the √2 the live
// path was missing (a pinned defect). Updated to the √2-normalised form;
// the escape-band conclusion survives the correction.
func TestGrappleScore_UntrainedVsTrainedDefenderEscapes(t *testing.T) {
	cfg := configs.GetBalanceConfig()
	cfg.SkillWeight = 5.0
	cfg.GrappleAggressorDriftBonus = 1.038

	untrained := makeGrappleCharacter(t, 100, 100, 0)
	trained := makeGrappleCharacter(t, 100, 100, 30)

	atkScore := grappleScore(untrained, true, cfg, true) // 100 × 1.038 = 103.8
	defScore := grappleScore(trained, false, cfg, true)  // 100 + 150 = 250

	margin := atkScore - defScore
	sigma := atkScore * 0.15 * math.Sqrt2
	z := margin / sigma

	if z > -2.0 {
		t.Errorf("expected z ≤ -2.0 (defender escape band), got z=%.2f (atk=%.1f def=%.1f)",
			z, atkScore, defScore)
	}
}

// TestGrappleScore_EqualPairHolds confirms balanced grapplers land
// in the Hold band (|z| < 0.5) when their stats and skill match.
//
// U6b Task 14: previously computed z without the √2 (pinned defect) and
// relied on the 2.2-vs-2.0 accidental edge; now uses √2-normalised z and
// the deliberate GrappleAggressorDriftBonus. At parity the mean z is
// (1 − 1/1.038)/(0.15·√2) ≈ 0.17: still comfortably in the Hold band.
func TestGrappleScore_EqualPairHolds(t *testing.T) {
	cfg := configs.GetBalanceConfig()
	cfg.SkillWeight = 5.0
	cfg.GrappleAggressorDriftBonus = 1.038

	a := makeGrappleCharacter(t, 100, 100, 15)
	b := makeGrappleCharacter(t, 100, 100, 15)

	atkScore := grappleScore(a, true, cfg, true)  // (100 + 75) × 1.038 = 181.65
	defScore := grappleScore(b, false, cfg, true) // 100 + 75 = 175

	margin := atkScore - defScore
	sigma := atkScore * 0.15 * math.Sqrt2
	z := margin / sigma

	if z < 0 || z >= 0.5 {
		t.Errorf("expected z in [0, 0.5) (Hold band, slight aggressor edge), got z=%.2f (atk=%.1f def=%.1f)",
			z, atkScore, defScore)
	}
}

// Catches treating "short" as a zero score or bypassing the established
// depletion/load effectiveness penalties along with the skill term.
func TestGrappleScoreWithoutSkillKeepsStatAndEffectivenessTerms(t *testing.T) {
	cfg := configs.GetBalanceConfig()
	cfg.GrappleStaminaPenaltyMax = 0.6
	cfg.GrappleStaminaPenaltyCurve = 1
	cfg.GrappleEncumbrancePenaltyMax = 0.8
	cfg.GrappleEncumbrancePenaltyCurve = 1.5
	cfg.GrappleAggressorDriftBonus = 1.038 // U6b Task 14: aggressor edge is a config multiplier
	c := makeGrappleCharacter(t, 100, 100, 50)
	c.Stamina = 0
	c.StaminaMax.Value = 100
	loadGrapplerToFraction(c, 99200, 1)

	got := grappleScore(c, true, cfg, false)
	want := 100.0 * 0.4 * (1.0 - 0.8*math.Pow(1.0/3.0, 1.5)) * 1.038
	if !grappleScoreApproxEqual(got, want) {
		t.Fatalf("short score = %.6f, want %.6f from stats x stamina x encumbrance",
			got, want)
	}
}

// TestProcessGrapplePair_DriftZUsesSqrt2Normalisation pins the U6b Task 14
// √2 fix. Both sides of the drift contest roll with the attacker's stdDev,
// so the margin's standard deviation is stdDev·√2; dividing the margin by
// stdDev alone (the pre-fix code, flagged NOTE(U6)) inflated every drift z
// by ~41%. With an injected margin of 0.60 at stdDev 1:
//
//	pre-fix z = 0.60          → 1-step advance band (Clinch → Mount)
//	fixed z   = 0.60/√2 ≈ 0.42 → Hold band (no transition)
//
// A margin of 0.75 (fixed z ≈ 0.53) still advances, proving the band
// boundary moved by exactly the √2 factor rather than outcomes being
// disabled wholesale.
func TestProcessGrapplePair_DriftZUsesSqrt2Normalisation(t *testing.T) {
	makePair := func() (*characters.Character, *characters.Character) {
		a := characters.New()
		a.SetUserId(11)
		b := characters.New()
		b.SetUserId(12)
		for _, c := range []*characters.Character{a, b} {
			c.Stats.Strength.Base = 100
			c.Stats.Dexterity.Base = 100
			c.Stats.Strength.Recalculate()
			c.Stats.Dexterity.Recalculate()
			c.Stamina = 100
			c.StaminaMax.Value = 100
		}
		if err := position.TransitionPair(a, b, position.Clinch,
			state.TransitionReason{Trigger: position.TriggerGrappleEntry}); err != nil {
			t.Fatalf("TransitionPair → Clinch failed: %v", err)
		}
		return a, b
	}

	// Margin 0.60, stdDev 1: pre-fix this advanced; fixed z ≈ 0.42 → Hold.
	a, b := makePair()
	processGrapplePairWithContest(a, b, fixedGrappleContest(0.60))
	if a.Position.State() != position.Clinch || b.Position.State() != position.Clinch {
		t.Errorf("margin 0.60 (fixed z ≈ 0.42) must Hold in Clinch; got controller=%v controlled=%v",
			a.Position.State(), b.Position.State())
	}

	// Margin 0.75, stdDev 1: fixed z ≈ 0.53 → 1-step advance still fires.
	a, b = makePair()
	processGrapplePairWithContest(a, b, fixedGrappleContest(0.75))
	if a.Position.State() == position.Clinch {
		t.Errorf("margin 0.75 (fixed z ≈ 0.53) must advance out of Clinch; controller still %v",
			a.Position.State())
	}
}

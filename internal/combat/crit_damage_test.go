package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Chunk 5.11g — skill-scaled crit damage.
//
// These tests run with the Go defaults (CritDamageBase 2.0,
// CritDamagePerSkill 0.05) because a Go test binary's CWD is its own package
// directory, so _datafiles/config.yaml is never loaded. Asserting the shipped
// values here would make the test a config mirror rather than a formula check.

func TestCritDamageMultiplier_MatchesSpecTable(t *testing.T) {
	// The table published in the 5.11 design spec, section E. If these move,
	// the player-facing promise moved with them.
	cases := []struct {
		skill int
		want  float64
	}{
		{1, 2.05},
		{25, 3.25},
		{50, 4.50},
		{69, 5.45}, // Meirok
		{100, 7.00},
	}

	for _, c := range cases {
		got := CritDamageMultiplier(c.skill)
		if math.Abs(got-c.want) > 0.001 {
			t.Errorf("CritDamageMultiplier(%d) = %f, want %f", c.skill, got, c.want)
		}
	}
}

func TestCritDamageMultiplier_FloorsAtBase(t *testing.T) {
	// A crit must never be worth LESS than a normal hit's raw damage. Skill 0
	// and a negative rank (mobs and unranked skills both report low) must both
	// land on the base, not below it.
	if got := CritDamageMultiplier(0); math.Abs(got-2.0) > 0.001 {
		t.Errorf("CritDamageMultiplier(0) = %f, want 2.0", got)
	}
	if got := CritDamageMultiplier(-10); math.Abs(got-2.0) > 0.001 {
		t.Errorf("CritDamageMultiplier(-10) = %f, want 2.0 (no negative scaling)", got)
	}
}

func TestCritDamageMultiplier_IsLinearNotSqrt(t *testing.T) {
	// The spec chose linear over sqrt precisely so investment past the middle
	// keeps paying. Equal skill steps must yield equal multiplier steps — a
	// sqrt curve would fail this, which is the regression this guards.
	step1 := CritDamageMultiplier(20) - CritDamageMultiplier(10)
	step2 := CritDamageMultiplier(80) - CritDamageMultiplier(70)
	if math.Abs(step1-step2) > 0.001 {
		t.Errorf("step 10->20 = %f but step 70->80 = %f; curve is not linear", step1, step2)
	}
}

// ─── shared spell/taunt resolver ────────────────────────────────────────────

func TestCritOrMitigatedDamage_CritBypassesMitigationAndScales(t *testing.T) {
	// The two halves of a crit in the spell and conviction channels: it ignores
	// the defender's mitigation entirely AND multiplies by skill. Holding
	// mitigation at a punishing 0.75 proves the bypass; the ratio proves the
	// scaling.
	const (
		samples = 20000
		rawDmg  = 60.0
		rank    = 40
	)

	critTotal, normalTotal := 0.0, 0.0
	for i := 0; i < samples; i++ {
		critTotal += float64(CritOrMitigatedDamage(rawDmg, rank, true, 0.75, 0.75))
		normalTotal += float64(CritOrMitigatedDamage(rawDmg, rank, false, 0.0, 0.75))
	}

	// Normal + zero mitigation lands on rawDmg, so the ratio isolates the
	// multiplier even though the crit sample faced 75% mitigation.
	got := (critTotal / samples) / (normalTotal / samples)
	want := CritDamageMultiplier(rank)
	if math.Abs(got-want) > 0.05*want {
		t.Errorf("crit/normal = %f, want ~%f", got, want)
	}
}

func TestCritOrMitigatedDamage_NormalHitRespectsMitigation(t *testing.T) {
	const (
		samples = 20000
		rawDmg  = 100.0
	)

	mitigated, plain := 0.0, 0.0
	for i := 0; i < samples; i++ {
		mitigated += float64(CritOrMitigatedDamage(rawDmg, 10, false, 0.50, 0.75))
		plain += float64(CritOrMitigatedDamage(rawDmg, 10, false, 0.0, 0.75))
	}

	ratio := (mitigated / samples) / (plain / samples)
	if math.Abs(ratio-0.5) > 0.05 {
		t.Errorf("50%% mitigation gave ratio %f, want ~0.5", ratio)
	}
}

func TestCritOrMitigatedDamage_FloorsAtOne(t *testing.T) {
	// Spell and taunt both floor at 1 rather than 0 — a landed hit that deals
	// nothing reads to the player as a bug. Note melee deliberately differs
	// (it floors at 0), which is why this resolver is not shared with it.
	for i := 0; i < 200; i++ {
		if got := CritOrMitigatedDamage(0.0, 1, false, 0.75, 0.75); got < 1 {
			t.Fatalf("CritOrMitigatedDamage floored at %d, want >= 1", got)
		}
	}
}

// ─── melee call site ────────────────────────────────────────────────────────

// meanOf returns the arithmetic mean of xs.
func meanOf(xs []float64) float64 {
	total := 0.0
	for _, x := range xs {
		total += x
	}
	return total / float64(len(xs))
}

func TestCalcHitDamage_CritScalesByCritDamageMultiplier(t *testing.T) {
	// A crit rolls around rawDmgForCrit. With the multiplier applied it must
	// roll around rawDmgForCrit * critDmgMult — and because dice.RollStat
	// derives its spread from the mean it is handed, the multiplication has to
	// happen BEFORE the roll, not after it. Sampling the mean catches an
	// after-the-roll implementation only indirectly, so the spread invariant is
	// pinned separately by TestCalcHitDamage_CritSpreadTracksCritMean.
	const (
		samples   = 20000
		critMean  = 40.0
		mult      = 3.0
		tolerance = 0.05
	)

	plain := swingDamageParams{dmgMean: 10.0, rawDmgForCrit: critMean, critDmgMult: 1.0}
	scaled := swingDamageParams{dmgMean: 10.0, rawDmgForCrit: critMean, critDmgMult: mult}

	plainRolls := make([]float64, samples)
	scaledRolls := make([]float64, samples)
	for i := 0; i < samples; i++ {
		p, _ := calcHitDamage(&AttackResult{}, true, false, plain)
		plainRolls[i] = float64(p)
		s, _ := calcHitDamage(&AttackResult{}, true, false, scaled)
		scaledRolls[i] = float64(s)
	}

	ratio := meanOf(scaledRolls) / meanOf(plainRolls)
	if math.Abs(ratio-mult) > tolerance*mult {
		t.Errorf("crit damage ratio = %f, want ~%f (critDmgMult not applied)", ratio, mult)
	}
}

func TestCalcHitDamage_NormalHitIgnoresCritDamageMultiplier(t *testing.T) {
	// The multiplier is crit-only. If it leaked into the normal-hit branch it
	// would multiply every swing in the game, which would read as a damage
	// explosion rather than as a crit change.
	const samples = 20000

	plain := swingDamageParams{dmgMean: 20.0, rawDmgForCrit: 40.0, critDmgMult: 1.0}
	scaled := swingDamageParams{dmgMean: 20.0, rawDmgForCrit: 40.0, critDmgMult: 8.0}

	plainRolls := make([]float64, samples)
	scaledRolls := make([]float64, samples)
	for i := 0; i < samples; i++ {
		p, _ := calcHitDamage(&AttackResult{}, false, false, plain)
		plainRolls[i] = float64(p)
		s, _ := calcHitDamage(&AttackResult{}, false, false, scaled)
		scaledRolls[i] = float64(s)
	}

	ratio := meanOf(scaledRolls) / meanOf(plainRolls)
	if math.Abs(ratio-1.0) > 0.05 {
		t.Errorf("normal-hit damage ratio = %f, want ~1.0; critDmgMult leaked into normal hits", ratio)
	}
}

func TestBuildDamageParams_SetsCritMultFromCombatSkill(t *testing.T) {
	// The wiring seam: buildDamageParams must source the multiplier from the
	// ATTACKER's combat skill. Reading the target's skill, or the wrong skill
	// tag, compiles cleanly and is invisible without this.
	//
	// The attacker is bare-handed, so GetCombatSkillTag resolves to
	// unarmed-combat — setting weapon-combat here would silently fall through
	// to the rank-1 default and pass against a broken implementation.
	attacker := &characters.Character{}
	attacker.Skills = map[string]int{string(skills.UnarmedCombat): 40}
	attacker.Stats.Strength.Base = 100
	attacker.Stats.Strength.Recalculate()
	attacker.Health = 100
	attacker.HealthMax.Value = 100

	target := &characters.Character{}
	target.Health = 100
	target.HealthMax.Value = 100

	sdp := buildDamageParams(attacker, target, weaponSetup{weaponDmgMult: 1.0}, 0, User)

	want := CritDamageMultiplier(40)
	if math.Abs(sdp.critDmgMult-want) > 0.001 {
		t.Errorf("buildDamageParams critDmgMult = %f, want %f (rank-40 weapon-combat)", sdp.critDmgMult, want)
	}
}

func TestCritDamageMultiplier_KeepsClimbingPastSoftCap(t *testing.T) {
	// SkillMultiplier clamps at the soft cap (50). Crit damage deliberately
	// does not — "skill past the soft cap must do something" is the whole
	// premise of chunk 5.7 that 5.11 answers.
	if CritDamageMultiplier(100) <= CritDamageMultiplier(50) {
		t.Errorf("CritDamageMultiplier must keep climbing past the skill soft cap")
	}
}

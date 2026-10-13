package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// expectedSwings must be the live plan, swing for swing: the same weapons,
// swing counts, damage means and attack scores calculateCombat uses.
func TestExpectedSwings_IsTheLivePlan(t *testing.T) {
	pinParityBalance(t)
	a := parityCombatant(t, "attacker")
	d := parityCombatant(t, "defender")

	plan := buildAttackPlan(a, d)
	got := expectedSwings(a, d, User)
	if len(got) != len(plan.weapons) {
		t.Fatalf("%d expectations for %d planned weapons", len(got), len(plan.weapons))
	}
	for i, ws := range plan.weapons {
		sdp := buildDamageParams(a, d, ws, a.StatMod(`damage`), User)
		if got[i].swings != ws.swingCount || got[i].dmgMean != sdp.dmgMean {
			t.Errorf("weapon %d: swings %d dmgMean %v, live %d %v", i, got[i].swings, got[i].dmgMean, ws.swingCount, sdp.dmgMean)
		}
		if math.Abs(got[i].attackScore-160) > 1e-9 {
			t.Errorf("weapon %d attack score %v, want the parity 160", i, got[i].attackScore)
		}
	}
}

func TestCritShare_ParityIsTheTwoSigmaTail(t *testing.T) {
	// Equal scores, bar 2.0: P(Z >= 2) for a standard normal.
	if got := critShare(160, 160, 2.0); math.Abs(got-0.0227501) > 1e-6 {
		t.Fatalf("critShare(parity, bar 2) = %v, want 0.02275", got)
	}
}

func TestSelfCritShare_IsTheOneSidedTail(t *testing.T) {
	if got := selfCritShare(2.0); math.Abs(got-0.0227501) > 1e-6 {
		t.Fatalf("selfCritShare(2) = %v, want 0.02275", got)
	}
}

// At parity (equal scores) a swing lands half the time, and crits at the
// two-sigma-style tail of a zero margin; the whole per-round figure is pinned
// per weapon, inside a loose bracket of every swing landing. Gap compression
// is a no-op at parity (the attacker is not ahead), and the contest floor F
// leaves the hit chance at 0.5(1-F) + 0.5F = 0.5 but scales the crit share by
// (1-F): a floor-granted win cannot crit and a flipped natural crit is lost
// (owner ruling 2026-10-13).
func TestMeleeDamagePerRound_HitChanceBrackets(t *testing.T) {
	pinParityBalance(t)
	// The shipped saturation (_datafiles/config.yaml ContestGapSaturation
	// 2.80; the Go default 0 turns compression off), so the claim that
	// compression is a no-op at parity is actually exercised.
	setContestGapSaturationForTest(t, 2.8)
	a := parityCombatant(t, "attacker")
	d := parityCombatant(t, "defender")
	floor := contestFloor()
	if floor <= 0 {
		t.Fatalf("contest floor %v; the parity expectation needs a live floor to mean anything", floor)
	}

	full, want := 0.0, 0.0
	for _, sw := range expectedSwings(a, d, User) {
		full += float64(sw.swings) * sw.dmgMean
		hit := 0.5
		c := 0.5 * math.Erfc((sw.critBar-0)/math.Sqrt2) * (1 - floor)
		want += float64(sw.swings) * ((hit-c)*sw.dmgMean + c*sw.critMean)
	}
	got := meleeDamagePerRound(a, d, User)
	t.Logf("meleeDamagePerRound = %v, full = %v, ratio = %v", got, full, got/full)
	if got <= 0.4*full || got >= 0.7*full {
		t.Fatalf("parity damage per round %v, want about half of every swing landing (%v)", got, full)
	}
	// dice.normalCDF is an Abramowitz-Stegun approximation (error up to 7.5e-8),
	// so the live hit chance at parity is 0.5 only to about that precision.
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("parity damage per round %v, want the exact parity expectation %v", got, want)
	}
	entries := meleeDefenceEntries(d)
	if s := bestCompressedDefence(160, entries); len(entries) == 0 || math.Abs(s-160) > 1e-9 {
		t.Fatalf("parity defender's best compressed defence %v (%d entries), want the uncompressed 160", s, len(entries))
	}
}

// PowerScore's terms are the same expectations against the neutral target.
func TestPowerScoreTerms_ReadTheSharedPlan(t *testing.T) {
	pinParityBalance(t)
	a := parityCombatant(t, "fighter")
	want := 0.0
	for _, sw := range expectedSwings(a, neutralTarget(), User) {
		want += float64(sw.swings) * sw.dmgMean
	}
	if got := meleeOffenseRaw(a, User); got != want {
		t.Fatalf("meleeOffenseRaw = %v, want %v", got, want)
	}
	dodge, _, _ := defenceEntryScore(a, combatvocab.DefenceDodge, false, neutralCtx())
	parry, _, _ := defenceEntryScore(a, combatvocab.DefenceParry, false, neutralCtx())
	block, _, _ := defenceEntryScore(a, combatvocab.DefenceBlock, false, neutralCtx())
	if got := meleeAvoidance(a); math.Abs(got-(dodge+parry+block)/3) > 1e-9 {
		t.Fatalf("meleeAvoidance = %v, want the mean of the three live scores", got)
	}
}

// A caster's best damaging spell, mitigated, spread over the shared cooldown.
func TestSpellDamagePerRound(t *testing.T) {
	pinParityBalance(t)
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"test-bolt": {SpellId: "test-bolt", PrimaryStat: "willpower", DamageMultiplier: 1.5, DamageType: combatvocab.DamageMental},
	}))
	caster := spellFixtureCaster(t)
	caster.SpellBook = map[string]int{"test-bolt": 1}
	target := parityCombatant(t, "target")

	raw, _, _ := SpellRawDamage(spells.GetSpell("test-bolt"), caster)
	pct, curve, _, _ := SpellMitigation(spells.GetSpell("test-bolt"), target)
	// Casting shares the special-move cooldown, so one cast per cooldown.
	cooldown := float64(configs.GetBalanceConfig().SpecialMoveCooldown)
	if cooldown <= 0 {
		t.Fatalf("SpecialMoveCooldown = %v; the test needs a positive cooldown to mean anything", cooldown)
	}
	want := ApplyMitigation(raw, pct, curve) / cooldown
	if got := spellDamagePerRound(caster, target); math.Abs(got-want) > 1e-9 {
		t.Fatalf("spellDamagePerRound = %v, want %v", got, want)
	}
	if got := spellDamagePerRound(parityCombatant(t, "no spells"), target); got != 0 {
		t.Fatalf("a character with no spells: %v, want 0", got)
	}
}

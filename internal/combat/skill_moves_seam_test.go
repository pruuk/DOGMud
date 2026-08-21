package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// U6b Task 6: ExecuteSkillMove routes bash/kick/trip through the channel
// seam (ResolveChannelAttack) instead of the old scalar RunContest against
// a single dex+skill number. These tests pin the NEW surface:
//
//   - the defence is a SET from DefenceEntriesFor (equipment-gated),
//   - the winning defence is CHARGED and PROGRESSED (defender economy),
//   - Crit/Fumble exist, derived once inside the seam,
//   - Hit stays the contest WIN (!out.Defended) — defended-partial damage
//     lands with Hit == false and StatusApplied/KnockedDown stay gated on
//     the win (the review-trap formula `!Defended || DamageMultiplier > 0`
//     must never come back).

// bashSeamParams builds bash-shaped params on the new seam surface.
func bashSeamParams(atk, def *characters.Character) SkillMoveParams {
	return SkillMoveParams{
		Attacker: atk, Defender: def,
		Channel: ChannelMelee,
		Attack: AttackSide{
			Stat: 100, StatName: "strength",
			Skill: skills.WeaponCombat, SkillRank: 40,
			Mult: 1.0,
		},
		DamagePercent: 1.0,
		// Not certainty-critical here: none of this file's assertions read
		// result.KnockedDown (the fixed defended/fumble outcomes gate the
		// knockdown branch off entirely, and the two Hit==true cases don't
		// assert on it either). U10 made knockdown an opposed contest, so a
		// factor of 2.0 no longer guarantees anything on its own -- kept at
		// the pre-U10 value only because nothing here depends on the result.
		KnockdownFactor:      2.0,
		DamageStat:           100,
		MitigationMultiplier: 1.0,
		KnockdownToSupine:    true,
	}
}

func newSeamTestDefender() *characters.Character {
	def := characters.New()
	def.HealthMax.Value = 100000
	def.Health = 100000
	def.StaminaMax.Value = 200
	def.Stamina = 200
	return def
}

// A defended bash: the contest runs against the equipment-gated defence set,
// the winning defence is charged and progressed, and the defended outcome is
// NOT a hit even when partial damage lands.
func TestExecuteSkillMove_RoutesThroughSeam(t *testing.T) {
	pinDefenceAdmissionConfig(t)

	atk := characters.New()
	def := newSeamTestDefender()

	var seen []contest.Entry
	runner := func(atkScore float64, entries []contest.Entry) contest.Result {
		seen = entries
		stdDev := dice.StdDevFor(atkScore)
		// A bare (non-crit) defensive win: normalized defence margin 0.5, so
		// the damage multiplier is in (0, 0.5) — the partial band.
		return contest.Result{
			Contested: true,
			Winner:    entries[0].Name,
			Success:   false,
			Margin:    -0.5 * stdDev * math.Sqrt2,
			AttackRoll: dice.RollResult{
				Mean: atkScore, StdDev: stdDev, ZScore: 0,
			},
			DefenseRoll: dice.RollResult{
				Mean: entries[0].Score, StdDev: stdDev, ZScore: 0.5,
			},
		}
	}

	usesBefore := def.GetSkillUseCount("unarmed-combat")
	staminaBefore := def.Stamina
	healthBefore := def.Health

	res := executeSkillMoveWithRunner(bashSeamParams(atk, def), runner)

	// (c) Equipment-gated set: a shieldless, bare-handed defender rolls dodge
	// ONLY — block (and parry) must never enter the contest.
	if len(seen) != 1 || seen[0].Name != characters.DefenseDodge {
		t.Fatalf("defence set = %v, want exactly [dodge] for a bare defender", seen)
	}
	if res.Defence.DefenceType != characters.DefenseDodge {
		t.Errorf("Defence.DefenceType = %q, want dodge", res.Defence.DefenceType)
	}

	// (a) The winning defence was CHARGED (defender economy: defending a bash
	// now costs stamina, exactly like defending an autoattack).
	if res.Defence.Cost.Status == characters.CostNoCharge {
		t.Error("winning defence was not charged (CostNoCharge)")
	}
	if res.Defence.Cost.Charged <= 0 || def.Stamina >= staminaBefore {
		t.Errorf("charge did not drain the pool: Charged=%d stamina %d -> %d",
			res.Defence.Cost.Charged, staminaBefore, def.Stamina)
	}

	// (b) Defence progression fired exactly once (dodge trains unarmed-combat).
	if got := def.GetSkillUseCount("unarmed-combat") - usesBefore; got != 1 {
		t.Errorf("defence progression fired %d ordinary events, want 1", got)
	}

	// Trap 1: Hit is the contest WIN. A defended attempt with partial damage
	// must NOT read as a hit, and the binary status stays gated on the win.
	if res.Hit {
		t.Error("a defended attempt reported Hit == true")
	}
	if res.StatusApplied || res.KnockedDown {
		t.Errorf("defended attempt leaked status: StatusApplied=%v KnockedDown=%v",
			res.StatusApplied, res.KnockedDown)
	}

	// The partial mechanism survives the seam: a bare defensive win still
	// deals reduced damage, applied to the pool.
	if res.Damage < 1 {
		t.Errorf("bare defensive win dealt %d damage, want >= 1 (partial band)", res.Damage)
	}
	if def.Health >= healthBefore {
		t.Error("reported partial damage was not applied to the defender's health pool")
	}
}

// The crit tier exists now: a decisive margin against CritBarFor's pair bar
// sets Crit, and damage routes through CritOrMitigatedDamage with the RAW
// rank — mitigation bypassed, mean scaled by CritDamageMultiplier(rank).
func TestExecuteSkillMove_CritTier(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	cfg := configs.GetConfig()
	cfg.Balance.CritBarSkillSlope = 0.05
	cfg.Balance.CritBarFloor = 1.5
	cfg.Balance.CritBarCeiling = 3.0
	cfg.Balance.MeleeDamageScale = 0.5
	cfg.Balance.GlobalDamageMultiplier = 1.0
	cfg.Balance.SkillMultiplierBase = 1.0
	cfg.Balance.SkillMultiplierMax = 3.0
	cfg.Balance.SkillSoftCap = 50
	cfg.Balance.CritDamageBase = 2.0
	cfg.Balance.CritDamagePerSkill = 0.2
	configs.SetConfigForTest(t, cfg)

	atk := characters.New()
	def := newSeamTestDefender()

	runner := func(atkScore float64, entries []contest.Entry) contest.Result {
		stdDev := dice.StdDevFor(atkScore)
		// Decisive attack win: normalized margin 1.8. The attacker's rank-40
		// advantage over the rank-0 dodge clamps the pair bar to the 1.5
		// floor, so 1.8 crits here and would NOT have at the const 2.0 bar.
		return contest.Result{
			Contested: true,
			Winner:    entries[0].Name,
			Success:   true,
			Margin:    1.8 * stdDev * math.Sqrt2,
			AttackRoll: dice.RollResult{
				Value: atkScore + 1.8*stdDev, Mean: atkScore,
				StdDev: stdDev, ZScore: 1.8,
			},
			DefenseRoll: dice.RollResult{
				Mean: entries[0].Score, StdDev: stdDev,
			},
		}
	}

	p := bashSeamParams(atk, def)
	res := executeSkillMoveWithRunner(p, runner)

	if !res.Crit {
		t.Fatal("decisive margin did not surface as result.Crit")
	}
	if !res.Hit {
		t.Error("a critting attack must also be a Hit")
	}

	// Damage is CritOrMitigatedDamage(rawDmg, RAW rank, ...): mean scaled by
	// CritDamageMultiplier(40) = 2.0 + 0.2*40 = 10. A non-crit against this
	// unarmoured defender would average rawDmg; 4 sigma of the crit roll
	// still sits far above 3x rawDmg, so this cleanly pins that the raw rank
	// reached the crit multiplier and the DamageMultiplier was not applied.
	rawDmg := CalcRawDamage(p.DamageStat, p.Attack.SkillRank, p.DamagePercent, ChannelPhysical)
	expected := rawDmg * CritDamageMultiplier(p.Attack.SkillRank)
	tolerance := 4*dice.StdDevFor(expected) + 1
	if math.Abs(float64(res.Damage)-expected) > tolerance {
		t.Errorf("crit damage = %d, want %.1f +/- %.1f", res.Damage, expected, tolerance)
	}
	if float64(res.Damage) <= 3*rawDmg {
		t.Errorf("crit damage %d does not exceed 3x non-crit raw %.1f — crit multiplier missing", res.Damage, rawDmg)
	}
}

// The fumble abort is NEW for these moves (Assumption 11): a fumbled swing
// aborts BEFORE success — no damage, no status, no knockdown — even when the
// roll technically won the contest.
func TestExecuteSkillMove_FumbleAborts(t *testing.T) {
	pinDefenceAdmissionConfig(t)

	atk := characters.New()
	def := newSeamTestDefender()
	healthBefore := def.Health

	runner := func(atkScore float64, entries []contest.Entry) contest.Result {
		stdDev := dice.StdDevFor(atkScore)
		return contest.Result{
			Contested: true,
			Winner:    entries[0].Name,
			Success:   true, // the roll "won" ...
			Margin:    30,
			AttackRoll: dice.RollResult{ // ... on a fumbled swing
				Value: atkScore - 2.5*stdDev, Mean: atkScore,
				StdDev: stdDev, ZScore: -2.5,
			},
			DefenseRoll: dice.RollResult{
				Mean: entries[0].Score, StdDev: stdDev,
			},
		}
	}

	res := executeSkillMoveWithRunner(bashSeamParams(atk, def), runner)

	if !res.Fumble {
		t.Fatal("ZScore -2.5 did not surface as result.Fumble")
	}
	if res.Hit || res.Damage != 0 || res.StatusApplied || res.KnockedDown {
		t.Errorf("fumble did not abort: Hit=%v Damage=%d StatusApplied=%v KnockedDown=%v",
			res.Hit, res.Damage, res.StatusApplied, res.KnockedDown)
	}
	if def.Health != healthBefore {
		t.Error("a fumbled swing dealt damage")
	}
}

// IsCounter is PLUMBED in this task and CONSUMED by Task 10: a move executed
// as a counter must still resolve identically (same defence set, same
// DefensiveCrit reporting) — the flag's only job, once Task 10 lands, is to
// keep the counter tier from firing off a move that IS the counter, so no
// counter chain can recurse. Until then it must be behaviour-neutral.
func TestExecuteSkillMove_IsCounterSuppressesCounterTier(t *testing.T) {
	pinDefenceAdmissionConfig(t)

	runner := func(atkScore float64, entries []contest.Entry) contest.Result {
		stdDev := dice.StdDevFor(atkScore)
		// Decisive DEFENSIVE win: normalized defence margin 2.5 → defensive
		// crit, damage multiplier 0.
		return contest.Result{
			Contested: true,
			Winner:    entries[0].Name,
			Success:   false,
			Margin:    -2.5 * stdDev * math.Sqrt2,
			AttackRoll: dice.RollResult{
				Mean: atkScore, StdDev: stdDev, ZScore: -1.0,
			},
			DefenseRoll: dice.RollResult{
				Mean: entries[0].Score, StdDev: stdDev, ZScore: 2.5,
			},
		}
	}

	atk := characters.New()
	def := newSeamTestDefender()
	p := bashSeamParams(atk, def)
	p.IsCounter = true
	res := executeSkillMoveWithRunner(p, runner)

	if !res.Defence.DefensiveCrit {
		t.Error("decisive defensive win did not report Defence.DefensiveCrit")
	}
	if res.Hit || res.Damage != 0 || res.StatusApplied || res.KnockedDown {
		t.Errorf("defensive crit outcome leaked: Hit=%v Damage=%d StatusApplied=%v KnockedDown=%v",
			res.Hit, res.Damage, res.StatusApplied, res.KnockedDown)
	}

	// Behaviour-neutral today: the same contest without the flag resolves the
	// same way.
	def2 := newSeamTestDefender()
	p2 := bashSeamParams(atk, def2)
	p2.IsCounter = false
	res2 := executeSkillMoveWithRunner(p2, runner)
	if res2.Hit != res.Hit || res2.Defence.DefensiveCrit != res.Defence.DefensiveCrit {
		t.Error("IsCounter changed resolution before Task 10 gave it a consumer")
	}

	// U6b Task 7 (the auto-trip/auto-bash counters now carry the flag): an
	// IsCounter move that WINS its contest must still resolve normally —
	// hit, damage applied, status landed. The flag only marks the move as a
	// counter for Task 10's tier; it must never suppress the counter-move's
	// OWN effects.
	winRunner := func(atkScore float64, entries []contest.Entry) contest.Result {
		stdDev := dice.StdDevFor(atkScore)
		return contest.Result{
			Contested: true,
			Winner:    entries[0].Name,
			Success:   true,
			Margin:    0.5 * stdDev * math.Sqrt2, // clean win, under the crit bar
			AttackRoll: dice.RollResult{
				Value: atkScore, Mean: atkScore, StdDev: stdDev, ZScore: 0,
			},
			DefenseRoll: dice.RollResult{
				Mean: entries[0].Score, StdDev: stdDev, ZScore: 0,
			},
		}
	}
	def3 := newSeamTestDefender()
	healthBefore := def3.Health
	p3 := bashSeamParams(atk, def3)
	p3.IsCounter = true
	res3 := executeSkillMoveWithRunner(p3, winRunner)
	if !res3.Hit || res3.Damage < 1 || !res3.StatusApplied {
		t.Errorf("IsCounter suppressed the counter-move's own resolution: Hit=%v Damage=%d StatusApplied=%v",
			res3.Hit, res3.Damage, res3.StatusApplied)
	}
	if def3.Health >= healthBefore {
		t.Error("IsCounter winning move's damage was not applied to the defender's pool")
	}
}

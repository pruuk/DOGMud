package combat

import (
	"math"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// Layer 1 of the fight read (#301): one side's expected output and defence,
// built ONLY from the functions live combat calls. Nothing here may carry its
// own copy of a swing, hit, damage or defence formula; the drift guards in
// fight_read_drift_guard_test.go (repo root) and fight_expect_stat_test.go
// hold that line. Everything is pure: no dice, no state written.

// neutralCtx is a prediction's combat context: full sight both ways and no
// light penalty, so the sight ramp scores 1.0.
func neutralCtx() combatContext {
	return combatContext{sourceSight: messaging.SightFull, targetSight: messaging.SightFull}
}

var (
	neutralOnce sync.Once
	neutralChar *characters.Character
)

// neutralTarget is an opponent with no armour and no species: what PowerScore
// measures offense against, so that term has no opponent in it. Built once;
// nothing may mutate it.
func neutralTarget() *characters.Character {
	neutralOnce.Do(func() {
		c := characters.New()
		c.Name = "neutral target"
		_ = c.Validate()
		neutralChar = c
	})
	return neutralChar
}

// sourceTypeOf is the SourceTarget combat uses for c.
func sourceTypeOf(c *characters.Character) SourceTarget {
	if c.IsMob {
		return Mob
	}
	return User
}

// swingExpectation is one weapon's planned round: how many swings, the attack
// score each swing rolls, a normal hit's and a crit's mean damage, and the
// crit bar.
type swingExpectation struct {
	swings      int
	attackScore float64
	dmgMean     float64
	critMean    float64
	critBar     float64
}

// expectedSwings is src's planned round against tgt, weapon by weapon, from
// the calls calculateCombat makes: buildAttackPlan, buildDamageParams,
// calcAttackScore times applyPositionHitModifiers, and calcCritThreshold.
func expectedSwings(src, tgt *characters.Character, srcType SourceTarget) []swingExpectation {
	plan := buildAttackPlan(src, tgt)
	statModBonus := src.StatMod(`damage`)
	ctx := neutralCtx()
	bar := calcCritThreshold(src, tgt)
	out := make([]swingExpectation, 0, len(plan.weapons))
	for _, ws := range plan.weapons {
		sdp := buildDamageParams(src, tgt, ws, statModBonus, srcType)
		out = append(out, swingExpectation{
			swings:      ws.swingCount,
			attackScore: calcAttackScore(src, tgt, ws.weapon, ws.penalty, ctx) * applyPositionHitModifiers(src, tgt),
			dmgMean:     sdp.dmgMean,
			critMean:    sdp.rawDmgForCrit * sdp.critDmgMult * (1 - sdp.critReduction),
			critBar:     bar,
		})
	}
	return out
}

// bestMeleeDefenceScore is defender's strongest melee defence, scored by
// defenceEntryScore as the live contest scores it; 0 with none.
func bestMeleeDefenceScore(defender *characters.Character) float64 {
	best := 0.0
	ctx := neutralCtx()
	for _, d := range DefenceEntriesFor(combatvocab.Melee(combatvocab.TargetSingle), defender, DefenceEntryOpts{}) {
		if s, _, _ := defenceEntryScore(defender, d, false, ctx); s > best {
			best = s
		}
	}
	return best
}

// critShare is the chance a swing crits: the attack-positive margin,
// normalised as ContestCritAt normalises it (roll spread times root two),
// reaching bar.
func critShare(atk, def, bar float64) float64 {
	sigma := dice.StdDevFor(atk)
	if sigma <= 0 {
		return 0
	}
	z := bar - (atk-def)/(sigma*math.Sqrt2)
	return 0.5 * math.Erfc(z/math.Sqrt2)
}

// meleeDamagePerRound is src's expected melee damage to tgt in one round.
// It approximates the live contest on purpose: one defence (the best), no
// contest floor, gap compression, fumbles or partial deflection. The read is
// rough by design (spec), and fight_expect_stat_test.go measures how rough.
func meleeDamagePerRound(src, tgt *characters.Character, srcType SourceTarget) float64 {
	def := bestMeleeDefenceScore(tgt)
	total := 0.0
	for _, sw := range expectedSwings(src, tgt, srcType) {
		hit := 1.0
		if def > 0 {
			hit = dice.OpposedSuccessChance(sw.attackScore, def, dice.StdDevFor(sw.attackScore))
		}
		crit := math.Min(critShare(sw.attackScore, def, sw.critBar), hit)
		total += float64(sw.swings) * ((hit-crit)*sw.dmgMean + crit*sw.critMean)
	}
	return total
}

// spellDamagePerRound is src's best damaging spell against tgt, mitigated,
// spread over the shared special-move cooldown that casting claims.
func spellDamagePerRound(src, tgt *characters.Character) float64 {
	best := 0.0
	for spellId := range src.SpellBook {
		if !src.HasSpell(spellId) {
			continue
		}
		sd := spells.GetSpell(spellId)
		raw, _, ok := SpellRawDamage(sd, src)
		if !ok {
			continue
		}
		pct, curve, _, chOk := SpellMitigation(sd, tgt)
		if !chOk {
			continue
		}
		if d := ApplyMitigation(raw, pct, curve); d > best {
			best = d
		}
	}
	cd := float64(configs.GetBalanceConfig().SpecialMoveCooldown)
	if cd < 1 {
		cd = 1
	}
	return best / cd
}

// meleeOffenseRaw is PowerScore's melee offense: every planned swing's mean
// damage against the neutral target, before hit chance.
func meleeOffenseRaw(char *characters.Character, srcType SourceTarget) float64 {
	total := 0.0
	for _, sw := range expectedSwings(char, neutralTarget(), srcType) {
		total += float64(sw.swings) * sw.dmgMean
	}
	return total
}

// meleeAvoidance is PowerScore's avoidance: the mean of char's dodge, parry
// and block as the live contest scores them.
func meleeAvoidance(char *characters.Character) float64 {
	ctx := neutralCtx()
	sum := 0.0
	for _, d := range []combatvocab.Defence{combatvocab.DefenceDodge, combatvocab.DefenceParry, combatvocab.DefenceBlock} {
		s, _, _ := defenceEntryScore(char, d, false, ctx)
		sum += s
	}
	return sum / 3.0
}

package combat

// U6b Task 19 — the statistical melee parity gate (arc completion criterion 5),
// in GO, against the REAL seam. A Python model extension can only reproduce the
// model; it can never detect a Go-side leak from Tasks 1-2 (a crit-bar leak, a
// defence-set gate divergence, a floor moved, a margin mis-signed). This test
// drives resolveCombatRound — the full production melee path: plan, admission,
// contest, floors, crits, fumbles, deflection multiplier, damage roll — for
// 200,000 live-dice swings per mitigation cell and asserts the empirical mean
// damage per swing lands within ±10% of an analytic anchor computed inside the
// test from the same shipped formula.
//
// The anchor is built in two layers:
//
//  1. DETERMINISTIC PINS. The attack score, dodge score, crit bar, defence
//     set, and damage means are asserted EXACTLY (1e-9) against hand-derived
//     values from the shipped formulas. Score/pipeline drift fails here, with
//     an explanation, before any dice are thrown.
//  2. PROBABILITY MODEL. Expected damage per swing is computed by numerically
//     integrating the joint distribution of the two roll z-scores over an
//     exact transcription of RunWithFloors + resolveDefenseOutcomeInner +
//     applyCritFloors + calcHitDamage. A Task 1 bar leak or a Task 2 gate
//     divergence moves the empirical mean; the anchor does not.
//
// Matchup: the PARITY cell. Both sides stat 100, unarmed-combat 30, bare
// hands. Attack score = dodge score = 100 + 30×SkillWeight(2.0) = 160, crit
// bar = CritBarFor(30,30) = 2.0 both ways, ContestFloor 0.125. Three
// mitigation cells on the defender: light (0%), mid (40%), BIS (75% — the
// PhysicalMitigationCap). Mitigation is injected as a buff statmod
// (physical_mitigation), which GetPhysicalMitigation folds in alongside gear.
//
// Knob values are pinned to the Go defaults the test binary actually runs
// under (config.yaml never loads in tests): SkillWeight 2.0, ContestFloor
// 0.125, RollSpread 0.15 (read live via dice.StdDevFor, which is the same
// source production uses), CritBar slope/floor/ceiling 0.05/1.5/3.0, crit
// floors 0.01/0.01, CritDamage 2.0 + 0.05×rank, damage pipeline
// 0.30 × 0.30 × 1.0.

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/statmods"
)

const (
	// paritySwings is the per-cell sample size. At 200k swings the standard
	// error of the mean is well under 1% of the mean in every cell, against a
	// ±10% gate.
	paritySwings = 200000

	// parityMitBuffId seeds a test-only buff spec carrying the cell's
	// physical_mitigation statmod.
	parityMitBuffId = 9001

	parityStatValue = 100
	paritySkillRank = 30
)

// pinParityBalance pins every knob the anchor reads to the Go-default values,
// so the anchor and the seam cannot disagree about a knob without one of the
// deterministic pins below catching it. SetConfigForTest restores on cleanup.
func pinParityBalance(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	b := &cfg.Balance
	b.SkillWeight = 2.0
	b.ContestFloor = 0.125
	b.DodgeEffectiveness = 1.0
	b.CritBarSkillSlope = 0.05
	b.CritBarFloor = 1.5
	b.CritBarCeiling = 3.0
	b.MinAttackCritChance = 0.01
	b.MinDefenseCritChance = 0.01
	b.CritDamageBase = 2.0
	b.CritDamagePerSkill = 0.05
	b.UnarmedDamageMultiplier = 0.30
	b.MeleeDamageScale = 0.30
	b.GlobalDamageMultiplier = 1.0
	b.MobDamageMultiplier = 1.0
	b.SkillMultiplierBase = 1.0
	b.SkillMultiplierMax = 3.0
	b.SkillSoftCap = 50
	b.PhysicalMitigationCap = 0.75
	b.StaminaPenaltyMax = 0.28
	b.HealthPenaltyMax = 0.28
	b.ResourcePenaltyCurve = 2.0
	// Progression must stay off: 200k swings of live progression would move
	// the skill ranks mid-run and slide both scores under the anchor.
	cfg.GamePlay.UseSkillProgression = false
	configs.SetConfigForTest(t, cfg)
}

// parityCombatant builds one bare-handed stat-100 rank-30 combatant with
// pools deep enough that the per-swing resource multipliers stay at 1.0.
func parityCombatant(t *testing.T, name string) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Name = name
	c.RoomId = 1
	c.Stats.Strength.Base = parityStatValue
	c.Stats.Dexterity.Base = parityStatValue
	c.Stats.Vitality.Base = parityStatValue
	c.Stats.Strength.Recalculate()
	c.Stats.Dexterity.Recalculate()
	c.Stats.Vitality.Recalculate()
	if err := c.Validate(); err != nil {
		t.Fatalf("validate %s: %v", name, err)
	}
	c.Skills[string(skills.UnarmedCombat)] = paritySkillRank
	c.HealthMax.Value = 1000000
	c.Health = 1000000
	c.StaminaMax.Value = 1000000
	c.Stamina = 1000000
	return c
}

// parityAnchor carries the deterministic inputs of the probability model.
type parityAnchor struct {
	sigma        float64 // StdDevFor(attack score); both rolls use it
	floor        float64 // ContestFloor, as RunWithFloors clamps it
	critBar      float64 // CritBarFor(30,30)
	defBar       float64 // DefenseCritBar()
	atkCritFloor float64 // MinAttackCritChance
	defCritFloor float64 // MinDefenseCritChance
	scoreGap     float64 // attack score - defence score (0 in the parity cell)

	eClean float64 // E[damage] of a non-crit full hit: round(max(0, N(dmgMean, σd)))
	eCrit  float64 // E[damage] of a crit: round(max(0, N(critMean, σc)))

	// gDefend[i] tabulates E[max(1, round(round(roll)·mult))] for
	// mult = i·gDefendStep — the deflected-hit path with its double rounding
	// and its min-1 clamp, which matter at BIS-cell damage levels.
	gDefend     []float64
	gDefendStep float64
}

func stdNormCDF(z float64) float64 { return 0.5 * math.Erfc(-z/math.Sqrt2) }
func stdNormPDF(z float64) float64 {
	return math.Exp(-0.5*z*z) / math.Sqrt(2*math.Pi)
}

// integerDamagePMF returns P(round(max(0, N(mean, StdDevFor(mean)))) = k) for
// k in [0, hi], matching calcHitDamage's roll exactly.
func integerDamagePMF(mean float64) []float64 {
	sd := dice.StdDevFor(mean)
	hi := int(math.Ceil(mean + 10*sd))
	pmf := make([]float64, hi+1)
	// k = 0 absorbs everything below 0.5 (negative rolls clamp to 0).
	pmf[0] = stdNormCDF((0.5 - mean) / sd)
	for k := 1; k <= hi; k++ {
		lo := (float64(k) - 0.5 - mean) / sd
		up := (float64(k) + 0.5 - mean) / sd
		pmf[k] = stdNormCDF(up) - stdNormCDF(lo)
	}
	return pmf
}

func buildParityAnchor(atkScore, defScore, dmgMean, critMean float64) parityAnchor {
	bal := configs.GetBalanceConfig()
	a := parityAnchor{
		sigma:        dice.StdDevFor(atkScore),
		floor:        float64(bal.ContestFloor),
		critBar:      CritBarFor(paritySkillRank, paritySkillRank),
		defBar:       DefenseCritBar(),
		atkCritFloor: float64(bal.MinAttackCritChance),
		defCritFloor: float64(bal.MinDefenseCritChance),
		scoreGap:     atkScore - defScore,
		gDefendStep:  0.0005,
	}

	cleanPMF := integerDamagePMF(dmgMean)
	for k, p := range cleanPMF {
		a.eClean += float64(k) * p
	}
	critPMF := integerDamagePMF(critMean)
	for k, p := range critPMF {
		a.eCrit += float64(k) * p
	}

	// Deflected-hit table: calcHitDamage rounds the roll to an int, the
	// deflection multiplies and rounds again, and a landing hit floors at 1.
	steps := int(0.5/a.gDefendStep) + 2
	a.gDefend = make([]float64, steps)
	for i := 0; i < steps; i++ {
		mult := float64(i) * a.gDefendStep
		e := 0.0
		for k := 1; k < len(cleanPMF); k++ {
			dmg := math.Round(float64(k) * mult)
			if mult > 0 && dmg < 1 {
				dmg = 1
			}
			e += cleanPMF[k] * dmg
		}
		a.gDefend[i] = e
	}
	return a
}

func (a parityAnchor) deflected(mult float64) float64 {
	i := int(math.Round(mult / a.gDefendStep))
	if i < 0 {
		i = 0
	}
	if i >= len(a.gDefend) {
		i = len(a.gDefend) - 1
	}
	return a.gDefend[i]
}

// expectedSwingDamageAt is an exact transcription of the per-swing outcome
// logic — resolveDefenseOutcomeInner's branch order, applyCritFloors'
// promotions, and calcHitDamage's two damage means — evaluated at one point
// (za, zd) of the two roll z-scores, for one floor branch.
//
// bm is bestDefenseResult.margin: DEFENCE-positive, the negation of the
// attack-positive contest margin; on a floored branch it carries the ∓1
// sentinel exactly as RunWithFloors stamps it.
func (a parityAnchor) expectedSwingDamageAt(za, zd, bm float64, floored bool) float64 {
	attackFumble := za <= -2.0
	defenseFumble := zd <= -2.0

	// Step 1: fumbles, in the resolver's order. An attack fumble (double or
	// not) is a miss and applyCritFloors skips fumbles entirely.
	if attackFumble {
		return 0
	}

	mAtk := -bm / (a.sigma * math.Sqrt2)
	mDef := bm / (a.sigma * math.Sqrt2)
	attackCrit := mAtk >= a.critBar
	defenseCrit := mDef >= a.defBar

	if defenseFumble {
		// Defence fumble guarantees a full hit (crit only if the margin says
		// so); applyCritFloors still runs on the result.
		base := a.eClean
		if attackCrit {
			base = a.eCrit
		}
		if floored {
			return base
		}
		if bm <= 0 {
			if attackCrit {
				return a.eCrit
			}
			return a.atkCritFloor*a.eCrit + (1-a.atkCritFloor)*a.eClean
		}
		// Defence-won margin: the defence-crit floor can still negate.
		return (1 - a.defCritFloor) * base
	}

	// Step 2: the winner, already floored inside RunContest.
	attackWon := bm <= 0

	// Step 3: crit, only on the winning side, never when floored.
	if !floored {
		if attackWon && attackCrit {
			return a.eCrit
		}
		if !attackWon && defenseCrit {
			return 0
		}
	}

	// Step 4: normal outcomes, plus applyCritFloors' promotions.
	if attackWon {
		if floored {
			return a.eClean
		}
		return a.atkCritFloor*a.eCrit + (1-a.atkCritFloor)*a.eClean
	}

	// Deflected hit. A floored save takes the bare 50% and skips the floors;
	// a rolled save mitigates along the curve and the defence-crit floor can
	// still promote it to a full negation.
	if floored {
		return a.deflected(1.0 - DefenceMitigation(0))
	}
	mult := 1.0 - DefenceMitigation(mDef)
	return (1 - a.defCritFloor) * a.deflected(mult)
}

// analyticMeanSwingDamage integrates expectedSwingDamageAt over the joint
// standard-normal density of the two roll z-scores, mixing the ContestFloor
// flip in as a probability.
func (a parityAnchor) analyticMeanSwingDamage() float64 {
	const h = 0.01
	const lim = 6.0
	total := 0.0
	for za := -lim; za <= lim; za += h {
		wa := stdNormPDF(za) * h
		for zd := -lim; zd <= lim; zd += h {
			w := wa * stdNormPDF(zd) * h
			margin := a.scoreGap + a.sigma*(za-zd) // attack-positive
			unfloored := a.expectedSwingDamageAt(za, zd, -margin, false)
			// RunWithFloors flips whichever outcome occurred, stamping the
			// sentinel: flipped-to-fail Margin=-1 (bm=+1), flipped-to-win
			// Margin=+1 (bm=-1).
			var bmSentinel float64
			if margin > 0 {
				bmSentinel = 1
			} else {
				bmSentinel = -1
			}
			flipped := a.expectedSwingDamageAt(za, zd, bmSentinel, true)
			total += w * ((1-a.floor)*unfloored + a.floor*flipped)
		}
	}
	return total
}

func TestMeleeParityDamagePerSwing(t *testing.T) {
	if testing.Short() {
		t.Skip("statistical parity gate; skipped in -short")
	}
	pinParityBalance(t)

	bal := configs.GetBalanceConfig()
	skillWeight := float64(bal.SkillWeight)
	wantScore := float64(parityStatValue) + float64(paritySkillRank)*skillWeight // 160

	// The shipped damage formula, by hand: stat × SkillMultiplier(rank) ×
	// unarmedMult × MeleeDamageScale × GlobalDamageMultiplier.
	skillMult := 1.0 + (3.0-1.0)*math.Sqrt(float64(paritySkillRank)/50.0)
	rawDamage := float64(parityStatValue) * skillMult * 0.30 * 0.30 * 1.0
	critDmgMult := 2.0 + 0.05*float64(paritySkillRank) // 3.5

	cells := []struct {
		name   string
		mitPct int
	}{
		{"light-0pct", 0},
		{"mid-40pct", 40},
		{"BIS-75pct-cap", 75},
	}

	for _, cell := range cells {
		cell := cell
		t.Run(cell.name, func(t *testing.T) {
			attacker := parityCombatant(t, "parity attacker")
			attacker.SetAggro(0, 1, characters.DefaultAttack)
			defender := parityCombatant(t, "parity defender")

			if cell.mitPct > 0 {
				t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
					parityMitBuffId: {
						BuffId:       parityMitBuffId,
						Name:         "parity mitigation",
						TriggerCount: 1,
						StatMods:     statmods.StatMods{"physical_mitigation": cell.mitPct},
					},
				}))
				defender.Buffs.List = append(defender.Buffs.List, &buffs.Buff{
					BuffId: parityMitBuffId, TriggersLeft: 1000000000,
				})
				defender.Buffs.Validate(true)
			}

			// ── Layer 1: deterministic pins ─────────────────────────────
			ctx := combatContext{sourceCanSee: true, targetCanSee: true}

			if got := defender.GetPhysicalMitigation(); math.Abs(got-float64(cell.mitPct)/100.0) > 1e-9 {
				t.Fatalf("defender mitigation = %.4f, want %.2f", got, float64(cell.mitPct)/100.0)
			}
			entries := DefenceEntriesFor(ChannelMelee, defender, DefenceEntryOpts{})
			if len(entries) != 1 || entries[0] != characters.DefenseDodge {
				t.Fatalf("bare-handed melee defence set = %v, want [dodge] — the Task 2 equipment gate moved", entries)
			}
			atkScore := calcAttackScore(attacker, defender, items.Item{}, 0, ctx)
			if math.Abs(atkScore-wantScore) > 1e-9 {
				t.Fatalf("attack score = %.4f, want %.4f (stat %d + rank %d × SkillWeight %.1f)",
					atkScore, wantScore, parityStatValue, paritySkillRank, skillWeight)
			}
			defScore := defender.GetDefenseScoreFor(characters.DefenseDodge, true) *
				defenceEffectiveness(characters.DefenseDodge)
			if math.Abs(defScore-wantScore) > 1e-9 {
				t.Fatalf("dodge score = %.4f, want %.4f", defScore, wantScore)
			}
			if got := calcCritThreshold(attacker, defender); got != 2.0 {
				t.Fatalf("parity crit bar = %v, want 2.0 — CritBarFor leaked", got)
			}

			plan := buildAttackPlan(attacker, defender)
			if len(plan.weapons) != 2 {
				t.Fatalf("bare-handed plan has %d weapons, want 2 fists", len(plan.weapons))
			}
			for i, ws := range plan.weapons {
				if ws.penalty != 0 {
					t.Fatalf("fist %d carries dual-wield penalty %d, want 0 (natural weapons)", i, ws.penalty)
				}
			}
			sdp := buildDamageParams(attacker, defender, plan.weapons[0], 0, User)
			wantDmgMean := ApplyMitigation(rawDamage, float64(cell.mitPct)/100.0, 0.75)
			if math.Abs(sdp.dmgMean-wantDmgMean) > 1e-9 {
				t.Fatalf("dmgMean = %.6f, want %.6f from the shipped formula", sdp.dmgMean, wantDmgMean)
			}
			if math.Abs(sdp.rawDmgForCrit-rawDamage) > 1e-9 {
				t.Fatalf("rawDmgForCrit = %.6f, want unmitigated %.6f", sdp.rawDmgForCrit, rawDamage)
			}
			if math.Abs(sdp.critDmgMult-critDmgMult) > 1e-9 {
				t.Fatalf("critDmgMult = %.6f, want %.6f", sdp.critDmgMult, critDmgMult)
			}
			critMean := sdp.rawDmgForCrit * sdp.critDmgMult

			// ── Layer 2: the probability anchor ─────────────────────────
			anchor := buildParityAnchor(atkScore, defScore, sdp.dmgMean, critMean)
			want := anchor.analyticMeanSwingDamage()

			// ── The 200k-swing live-dice run through the real seam ──────
			swings := 0
			totalDamage := 0.0
			hits, crits := 0, 0
			for swings < paritySwings {
				attacker.Stamina = attacker.StaminaMax.Value
				attacker.Health = attacker.HealthMax.Value
				defender.Stamina = defender.StaminaMax.Value
				defender.Health = defender.HealthMax.Value
				// A double fumble knocks both prone mid-run; reset so every
				// round samples the anchored standing matchup.
				if !attacker.Position.IsStanding() {
					attacker.Position = position.NewMachine()
				}
				if !defender.Position.IsStanding() {
					defender.Position = position.NewMachine()
				}
				result, cost := resolveCombatRound(attacker, defender, User, User, ctx)
				if cost.Short() {
					t.Fatalf("admission ran short at stamina %d — fixture pools are wrong", attacker.Stamina)
				}
				for _, ev := range result.SwingEvents {
					swings++
					totalDamage += float64(ev.Damage)
					if ev.Hit {
						hits++
					}
					if ev.Crit {
						crits++
					}
				}
			}
			got := totalDamage / float64(swings)
			delta := (got - want) / want

			t.Logf("cell %s: swings=%d analytic=%.4f observed=%.4f delta=%+.2f%% (hit %.1f%%, crit %.2f%%)",
				cell.name, swings, want, got, delta*100,
				100*float64(hits)/float64(swings), 100*float64(crits)/float64(swings))

			if math.Abs(delta) > 0.10 {
				t.Errorf("mean damage per swing %.4f deviates %+.1f%% from the analytic anchor %.4f (±10%% gate) — a bar, floor, gate or margin leak has moved the melee distribution",
					got, delta*100, want)
			}
		})
	}
}

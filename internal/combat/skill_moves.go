package combat

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// SkillMoveResult holds the outcome of a bash/kick/trip execution.
// Callers use these fields to dispatch messaging and analytics.
type SkillMoveResult struct {
	CritSource string // "rolled"|"sleeping"|"crit_on_win"; diagnostic only
	// Hit is the contest WIN, exactly as it has always been. Defended-partial
	// damage lands with Hit == false; do NOT widen this to "dealt damage"
	// (`!Defended || DamageMultiplier > 0`) — that formula would make nearly
	// every defended outcome a "hit" and fire knockdown rolls on defended
	// bashes (caught in adversarial review before it shipped).
	Hit bool

	// Crit is the attacker's margin-derived critical, decided ONCE inside
	// ResolveChannelAttack against CritBarFor's pair bar (U6b). A crit
	// bypasses mitigation via CritOrMitigatedDamage. Only ResolveChannelAttack
	// sets it.
	Crit bool

	// Fumble is a NEW abort for these moves (U6b, Assumption 11): a fumbled
	// swing (self-relative AttackRoll.ZScore at or under -DefenseCritBar())
	// aborts BEFORE success — no damage, no status — even when the roll won.
	Fumble bool

	Damage int
	// StatusApplied reports whether the maneuver's status effect landed.
	// This stays binary -- unlike Damage, there is no "partially tripped":
	// a defended attempt can deal partial damage via the shared
	// defenceDamageMultiplier curve, but StatusApplied (and KnockedDown)
	// can only be true when Hit is also true.
	StatusApplied bool
	KnockedDown   bool
	TargetMaxHP   int

	// Defence exposes the seam's full outcome — DefenceType, DefensiveCrit,
	// normalized margin, and the committed cost — for narration and the
	// Task 10 counter tier.
	Defence ChannelDefenceResult

	// IsCounter echoes SkillMoveParams.IsCounter so counter-tier wiring that
	// only sees the RESULT can refuse to fire off a move that IS a counter
	// (Task 10: a counter never earns a counter).
	IsCounter bool
}

// SkillMoveParams configures a skill move (bash/kick/trip) execution.
type SkillMoveParams struct {
	Attacker *characters.Character
	Defender *characters.Character

	// Channel selects the defence set (ChannelMelee for the physical moves,
	// ChannelRanged for fire). Required — the legacy scalar-defence path was
	// deleted in U6b Task 7; every caller sets Channel + Attack.
	Channel AttackChannel

	// Attack is the attacker's half of the contest. Callers pass the RAW
	// skill rank; the seam applies SkillWeight (the x1 -> x5 flip lives in
	// AttackSide.score(), not here). Attack.SkillRank is the single rank
	// input for the score, the crit bar, AND the damage multiplier curve.
	Attack AttackSide

	// IsCounter marks a move executed AS a counter. Plumbed in U6b Task 6,
	// consumed by Task 10: when set, the counter tier must not fire from
	// this move (a counter must never trigger another counter).
	IsCounter bool

	DamagePercent        float64 // config knob (e.g. BashDamagePercent)
	KnockdownFactor      float64 // config knob (e.g. BashKnockdownFactor)
	DamageStat           int     // stat for CalcRawDamage (always Strength)
	MitigationMultiplier float64 // 1.0 = full, 0.5 = half mitigation (stomp)

	// BonusCritMultiplier scales the crit mean only. 0 means 1.0.
	BonusCritMultiplier float64

	// KnockdownToSupine: false (default) → defender falls face-forward
	// to Prone (TriggerKnockdownFaceForward). true → defender knocked
	// backward to Supine (TriggerKnockdownFaceBackward). Bash + charge
	// opt into the Supine path; trip / kick / hamstring / bite stay
	// face-forward.
	KnockdownToSupine bool
}

// knockdownSurvivesGlobalDamper applies KnockdownFrequencyScale, the single dial
// for how often ANY contested knockdown actually lands. Returns true to keep a
// knockdown the contest already won.
//
// ⚠️ It deliberately scales the OUTCOME, not the attacker's score. Measured at
// score parity, 40k trials per cell, a score multiplier behaves like this:
//
//	factor   bash    trip    kick
//	 0.25    12.5%   12.6%   12.6%   <- all collapsed onto ContestFloor
//	 0.50    12.5%   12.4%   12.1%   <- still on the floor: no resolution
//	 0.75    16.8%   20.6%   14.0%
//	 1.00    50.1%   57.4%   38.4%   <- the intended per-move rates
//	 1.50    83.1%   84.4%   80.2%
//	 2.00    86.8%   86.9%   86.3%   <- all saturated at the ceiling
//
// So a score multiplier is unusable below ~0.75, saturated above ~1.5, and at
// both ends it FLATTENS the bash/trip/kick ordering that U10 tuned. The
// steepness is intrinsic: RollSpread is 0.15, so a 25% score edge is already
// about 1.6 standard deviations.
//
// Scaling the outcome instead is exact and linear: 0.5 means precisely half as
// many knockdowns, and every move is scaled by the same factor, so their
// relative ordering is untouched at every setting.
//
// The trade is that it can only REDUCE. To make knockdowns MORE common, raise
// the per-move *KnockdownFactor knobs; there is no way to promote a contest
// the attacker lost without knowing the probability it lost by.
//
// The validator forces the value into (0, 1], so an absent or zero key reads as
// 1.0 (unchanged) rather than as "disable all knockdowns". The guards below are
// defensive only.
func knockdownSurvivesGlobalDamper() bool {
	return rollKnockdownDamper(float64(configs.GetBalanceConfig().KnockdownFrequencyScale))
}

// rollKnockdownDamper is the pure half, split out so the linearity that
// justified this design over a score multiplier is directly testable without
// standing up a whole contest.
func rollKnockdownDamper(chance float64) bool {
	if chance >= 1.0 {
		return true
	}
	if chance <= 0 {
		return false
	}
	return util.Rand(10000) < int(chance*10000)
}

// ExecuteSkillMove performs the core combat resolution for bash/kick/trip.
// It handles the opposed contest (via ResolveChannelAttack: equipment-gated
// defence set, defence costing + skill-strip, defence progression, and the
// once-per-contest attacker crit/fumble verdicts),
// the damage pipeline, knockdown determination, and applies HP reduction +
// prone status. Callers handle messaging and analytics.
//
// Reach adjustment (chunk 4c): kick/stomp/knee variants are body-driven
// (foot/knee impacts), not weapon-driven, so they do NOT apply the reach
// utility curve. The knee variant's DamagePercent (KneeDamagePercent=1.00)
// is already calibrated for the grapple context. Grapple-entry, trip,
// and bash are force-driven and are similarly reach-agnostic.
func ExecuteSkillMove(p SkillMoveParams) SkillMoveResult {
	// Read the swappable contest core (production-initialised to RunContest,
	// never repointed outside tests) rather than RunContest directly, so
	// SetChannelAttackContestRunnerForTest reaches out-of-package callers of
	// THIS entry point too — U6b Task 8's fire seam tests drive the real
	// ExecuteFire path against a deterministic contest exactly the way the
	// taunt (Task 5) and spell (Task 4) collapse tests already do for
	// ResolveChannelAttack.
	return executeSkillMoveWithRunner(p, channelAttackContestRunner)
}

// executeSkillMoveWithRunner is ExecuteSkillMove with an injectable contest
// runner, so tests can force crit/fumble/defended outcomes deterministically.
func executeSkillMoveWithRunner(p SkillMoveParams, runner defenceContestRunner) SkillMoveResult {
	result := SkillMoveResult{IsCounter: p.IsCounter}

	// Get target's max HP for damage descriptions
	result.TargetMaxHP = p.Defender.HealthMax.Value

	mitigMult := p.MitigationMultiplier
	if mitigMult <= 0 {
		mitigMult = 1.0
	}
	mitig := p.Defender.GetPhysicalMitigation() * mitigMult
	cap := MitigationCap(ChannelPhysical)

	// U6b Task 6: ONE contest through the channel seam. The defence is a
	// SET (DefenceEntriesFor: a shieldless defender never rolls block),
	// the winning defence is quoted, charged, skill-stripped when
	// unaffordable, and progressed — the defender economy the melee path
	// already has. The crit/fumble bonus tier fires once INSIDE the seam;
	// nothing here derives a second verdict. (Task 7 deleted the legacy
	// scalar-defence branch; the seam is now the only path.)
	out := resolveChannelAttackWithRunner(p.Channel, p.Attack, p.Attacker, p.Defender, runner)
	result.Defence = out
	result.Crit = out.AttackerCrit
	result.CritSource = out.CritSource
	result.Fumble = out.AttackerFumble

	// Fumble aborts BEFORE success (Assumption 11): no damage, no status,
	// no knockdown. The defence was still charged and progressed inside
	// the seam — the defender mounted it either way.
	if result.Fumble {
		return result
	}

	// The contest WIN.
	result.Hit = !out.Defended

	// Damage pipeline: crit bypasses mitigation and scales by
	// CritDamageMultiplier(RAW rank); non-crits mitigate then scale by
	// the defence multiplier (1.0 on an attack win by construction, 0.5
	// on a floored save, 0.0-0.5 on a rolled defensive win).
	rawDmg := CalcRawDamage(p.DamageStat, p.Attack.SkillRank, p.DamagePercent, ChannelPhysical)

	// NPC-only physical multiplier.
	//
	// ⚠️ This used to be applied at ONE site, the mob melee auto-attack in
	// combat_helpers.go, despite being documented as the NPC physical dial. So
	// every mob special move (all 16), the dodge-crit counter-sweep and every
	// mob ranged shot -- all of which route through here -- silently ignored
	// it, and a mob's bash was scaled differently from its own punch.
	//
	// Applied here too as of 2026-08-30, so the knob means what its name says:
	// ALL NPC physical damage. That is what makes MeleeDamageScale usable as a
	// player-side dial, since the two channels share it.
	if p.Attacker != nil && p.Attacker.IsMob {
		rawDmg *= float64(configs.GetBalanceConfig().MobDamageMultiplier)
	}
	dmg := CritOrMitigatedDamageScaled(rawDmg, p.Attack.SkillRank, result.Crit, mitig, cap, p.BonusCritMultiplier)
	if !result.Crit {
		dmg = int(float64(dmg) * out.DamageMultiplier)
		if out.DamageMultiplier > 0 && dmg < 1 {
			dmg = 1
		}
	}
	result.Damage = dmg

	if result.Damage > 0 {
		p.Defender.ApplyHarm(characters.PoolHealth, result.Damage,
			state.ActorRef{UserId: p.Attacker.GetUserId(), MobInstanceId: p.Attacker.MobInstanceId})
	}

	if result.Hit {
		result.StatusApplied = true

		// Knockdown is an opposed contest (U10): the move's attack score
		// times its intended-rate factor, against the defender's
		// Dex + unarmed-combat x SkillWeight. This is a NAMED REBALANCE:
		// the old thresholds delivered ~91% trip / ~2.3% kick despite
		// claiming 60/35 (normal-curve thresholds, not percentages); the
		// contest delivers the intended rates at parity. Control-immune
		// defenders are immovable and never contest. Factor 0 = no
		// knockdown component (no contest, no progression).
		if !mutations.IsControlImmune(p.Defender.Mutations) && p.KnockdownFactor > 0 {
			defScore := float64(p.Defender.Stats.Dexterity.ValueAdj) +
				float64(p.Defender.GetSkillLevel(skills.UnarmedCombat))*float64(configs.GetBalanceConfig().SkillWeight)
			kd := RunContest(p.Attack.score()*p.KnockdownFactor, []contest.Entry{{Score: defScore}})
			if kd.Success && knockdownSurvivesGlobalDamper() {
				result.KnockedDown = true
			} else {
				// Success-only progression: the defender fires one
				// unarmed-combat event only on a RESIST — on top of the
				// seam's ordinary per-contest defence award, which is
				// unchanged. Routed through the U9 applier: the progression
				// seam guard forbids direct OnSkillUse calls in this package.
				p.Defender.ApplyProgression(
					progression.OrdinaryEvents(progression.Outcome{DefenderSkill: string(skills.UnarmedCombat)}),
					progression.SideDefender, p.Defender.GetUserId(), 0)
			}
		}

		// Apply knockdown if rolled. Chunk 4b W4 cutover: fire the
		// FSM transition (Prone or Supine per KnockdownToSupine)
		// alongside the legacy CombatPosition / PositionRoundsMin
		// writes. If the FSM transition fails (e.g. target was
		// already grappling and not in Standing), the legacy fields
		// are NOT updated either so the two views stay consistent.
		if result.KnockedDown && p.Defender.Position == nil {
			// A pre-Validate() Character has no Position FSM and can't be
			// knocked down. Prod combatants are always Validated (this
			// mirrors the guard in HandleGrappleCritFailure); treat it like
			// a failed transition and don't narrate a knockdown that the
			// FSM never recorded.
			result.KnockedDown = false
		} else if result.KnockedDown {
			var fsmErr error
			if p.KnockdownToSupine {
				fsmErr = p.Defender.Position.TransitionToSupine(
					position.SupineData{MinRecoveryRounds: 2},
					state.TransitionReason{Trigger: position.TriggerKnockdownFaceBackward},
				)
			} else {
				fsmErr = p.Defender.Position.TransitionToProne(
					position.ProneData{MinRecoveryRounds: 2},
					state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward},
				)
			}
			if fsmErr != nil {
				mudlog.Warn("ExecuteSkillMove: knockdown transition failed",
					"to_supine", p.KnockdownToSupine, "err", fsmErr)
				// The position did not actually change (e.g. the target was
				// already grappled/prone, not Standing), so DON'T report a
				// knockdown — otherwise every caller narrates a takedown that
				// never happened (the grapple move-collision bug). The move
				// still connected and dealt damage; it just didn't knock down.
				result.KnockedDown = false
			}
		}
	}

	return result
}

package combat

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// SkillMoveResult holds the outcome of a bash/kick/trip execution.
// Callers use these fields to dispatch messaging and analytics.
type SkillMoveResult struct {
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
	KnockdownChance      int     // config knob (e.g. BashKnockdownChance)
	DamageStat           int     // stat for CalcRawDamage (always Strength)
	MitigationMultiplier float64 // 1.0 = full, 0.5 = half mitigation (stomp)

	// KnockdownToSupine: false (default) → defender falls face-forward
	// to Prone (TriggerKnockdownFaceForward). true → defender knocked
	// backward to Supine (TriggerKnockdownFaceBackward). Bash + charge
	// opt into the Supine path; trip / kick / hamstring / bite stay
	// face-forward.
	KnockdownToSupine bool
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
	return executeSkillMoveWithRunner(p, RunContest)
}

// executeSkillMoveWithRunner is ExecuteSkillMove with an injectable contest
// runner, so tests can force crit/fumble/defended outcomes deterministically.
func executeSkillMoveWithRunner(p SkillMoveParams, runner defenceContestRunner) SkillMoveResult {
	result := SkillMoveResult{}

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
	dmg := CritOrMitigatedDamage(rawDmg, p.Attack.SkillRank, result.Crit, mitig, cap)
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

		// Knockdown roll — standardized to dice.RollStat(50). Control-immune
		// defenders (Ironhide's Living Carapace, Colossus's Ossified Frame) are
		// immovable and cannot be knocked down — the blow still lands and deals
		// damage, it just doesn't take them off their feet.
		if !mutations.IsControlImmune(p.Defender.Mutations) {
			knockdownRoll := dice.RollStat(50)
			if knockdownRoll.Value < float64(p.KnockdownChance) {
				result.KnockedDown = true
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

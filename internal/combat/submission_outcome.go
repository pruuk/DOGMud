package combat

import (
	"maps"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// Condition ids the submission outcome applies. Both are flagged silent-start in
// their YAML: they must be applied synchronously (their statmods must be live
// for the rest of this round's dispatch, and the broken limb lands mid-cascade)
// and internal/combat sends no player text anywhere, so the applier's caller
// owes the victim the authored start line. See SubmissionOutcomeEffects.
const (
	// BrokenLimbConditionId is condition 83, applied by a cripple-policy success on a
	// joint submission.
	BrokenLimbConditionId = 83
	// StunnedConditionId is condition 84, applied by a crit-tier mercy release.
	StunnedConditionId = 84
)

// SubmissionOutcomeEffects reports the silent-start conditions
// ResolveSubmissionOutcome actually applied, so the caller can narrate them.
// internal/combat has no player-text path, so the submission hook reads this
// and sends each condition's authored start line to a player victim right after the
// outcome. A nil victim means that effect did not land this round; a condition whose
// apply failed is not reported, so nobody is told about a stun they never took.
type SubmissionOutcomeEffects struct {
	// StunnedVictim holds condition 84, or nil.
	StunnedVictim *characters.Character
	// BrokenLimbVictim holds condition 83, or nil.
	BrokenLimbVictim *characters.Character
	// BrokenBodyPart is the limb the break names ("arm", "shoulder"), empty
	// when no limb was broken.
	BrokenBodyPart string
}

// Callbacks for submission narration. Registered by the hooks package
// in its init() to avoid a combat → hooks import cycle. nil-safe;
// ResolveSubmissionOutcome no-ops if not registered.
var (
	onSubmissionOpening func(
		attempter, recipient *characters.Character,
		subType position.SubmissionType,
	)
	onSubmissionResolution func(
		attempter, recipient *characters.Character,
		subType position.SubmissionType,
		tier SubmissionTier,
		policy characters.SubmissionPolicy,
		bodyPart string,
	)
)

// RegisterSubmissionMessaging registers callbacks for narration of
// submission attempts and resolutions. Called from the hooks package
// in init() to avoid a combat → hooks import cycle.
func RegisterSubmissionMessaging(
	opening func(attempter, recipient *characters.Character, subType position.SubmissionType),
	resolution func(attempter, recipient *characters.Character, subType position.SubmissionType, tier SubmissionTier, policy characters.SubmissionPolicy, bodyPart string),
) {
	onSubmissionOpening = opening
	onSubmissionResolution = resolution
}

// ResolveSubmissionOutcome applies the attempt result to the
// attempter + recipient based on the attempter's SubmissionPolicy.
// Side effects per tier:
//
//   - Bad tier:   attempter knocked Prone, pair breaks to Standing.
//   - Neutral:    no-op (caller handles messaging for the miss).
//   - Success:    outcome resolved per attempter's SubmissionPolicy.
//   - Crit:       same as Success; also applies a 1-round Stunned
//     condition on the recipient when policy is mercy (T10 stub).
//
// Policy routing on Success/Crit:
//
//   - Mercy:    clean release; both return to Standing.
//   - Subdue:   death cascade, NoDeprogression (T8), gold transfer
//     fraction (T8).
//   - Cripple:  same as Subdue + broken-limb condition (T9) when the sub
//     type has a body-part target. Choke subs (RNC, Triangle,
//     Anaconda) degrade to Subdue because they don't break limbs.
//   - Lethal:   full death cascade (deprogression intact; T8
//     distinguishes this from Subdue/Cripple via flag).
//
// The defender's SurrenderPolicy is intentionally not consulted here
// — per the design spec, only a mercy-policy attempter honours the
// tap signal, and that check happens inside applyMercyRelease.
// Other policies ignore the tap as a realism call.
//
// Returns the silent-start conditions it applied (see
// SubmissionOutcomeEffects). internal/combat sends no player text, so the
// caller narrates the stun and the broken limb to a player victim.
func ResolveSubmissionOutcome(
	attempter *characters.Character,
	recipient *characters.Character,
	result SubmissionAttemptResult,
	role Role,
) SubmissionOutcomeEffects {
	// Resolve the effective body part now so messaging and outcome
	// both use the same degraded value (choke cripple → subdue).
	bodyPart := position.CrippleBodyPart(result.SubType)
	effectivePolicy := attempter.SubmissionPolicy
	if effectivePolicy == characters.PolicyCripple && bodyPart == "" {
		effectivePolicy = characters.PolicySubdue
	}

	// Opening narration fires before the outcome is applied so the
	// player reads the attempt beat before any unconscious state lands.
	if onSubmissionOpening != nil {
		onSubmissionOpening(attempter, recipient, result.SubType)
	}

	var effects SubmissionOutcomeEffects
	switch result.Tier {
	case SubTierBad:
		applyBadTier(attempter, recipient)
	case SubTierNeutral:
		// no mechanical effect; fall through to resolution messaging
	case SubTierSuccess, SubTierCrit:
		effects = applySuccessByPolicy(attempter, recipient, result)
	}

	// Resolution narration fires after the outcome is applied.
	if onSubmissionResolution != nil {
		onSubmissionResolution(
			attempter, recipient,
			result.SubType,
			result.Tier,
			effectivePolicy,
			bodyPart,
		)
	}

	return effects
}

// applyBadTier handles the Sub-TierBad outcome: break the grapple to
// Standing, then knock the attempter Prone for at least 2 rounds.
func applyBadTier(attempter, recipient *characters.Character) {
	if err := position.TransitionPair(
		attempter, recipient, position.Standing,
		state.TransitionReason{Trigger: position.TriggerGrappleBreak},
	); err != nil {
		mudlog.Warn("submission bad-tier: TransitionPair → Standing failed",
			"attempter", attempter.Name, "err", err)
		return
	}
	// No nil-guard needed here: TransitionPair above returns an error (and
	// we return) when either Position is nil, so this line is only reached
	// with a non-nil attempter.Position.
	_ = attempter.Position.TransitionToProne(
		position.ProneData{MinRecoveryRounds: 2},
		state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward},
	)
}

// applySuccessByPolicy dispatches to the correct outcome path based
// on the attempter's SubmissionPolicy. Also applies the Crit-tier
// Stunned condition when policy is mercy. Returns the silent-start conditions it
// applied so the caller can narrate them; the victim of either is always
// the recipient, since both the death cascade and the crit stun land on
// the side the attempt was made against.
func applySuccessByPolicy(
	attempter *characters.Character,
	recipient *characters.Character,
	result SubmissionAttemptResult,
) SubmissionOutcomeEffects {
	var effects SubmissionOutcomeEffects
	policy := attempter.SubmissionPolicy

	// Choke degradation: cripple on a choke-class sub becomes subdue
	// because chokes don't break body parts.
	bodyPart := position.CrippleBodyPart(result.SubType)
	if policy == characters.PolicyCripple && bodyPart == "" {
		policy = characters.PolicySubdue
	}

	// Crit: apply Stunned condition on recipient only when mercy keeps
	// the recipient in play. For subdue/cripple/lethal the recipient
	// enters the death cascade and the condition would be a no-op.
	if result.Tier == SubTierCrit && policy == characters.PolicyMercy {
		if applyStunnedCondition(recipient) {
			effects.StunnedVictim = recipient
		}
	}

	switch policy {
	case characters.PolicyMercy:
		applyMercyRelease(attempter, recipient)
	case characters.PolicySubdue:
		applyDeathCascade(attempter, recipient,
			true /* noDeprogression */, false /* brokenLimb */, "")
	case characters.PolicyCripple:
		if applyDeathCascade(attempter, recipient,
			true /* noDeprogression */, true /* brokenLimb */, bodyPart) {
			effects.BrokenLimbVictim = recipient
			effects.BrokenBodyPart = bodyPart
		}
	case characters.PolicyLethal:
		applyDeathCascade(attempter, recipient,
			false /* deprogression normally */, false, "")
	}

	return effects
}

// applyMercyRelease executes the mercy-policy path: clean grapple
// break to Standing. Neither combatant takes damage. A brief
// post-mercy recovery harmful condition (T9/4f) is stubbed below.
func applyMercyRelease(attempter, recipient *characters.Character) {
	if err := position.TransitionPair(
		attempter, recipient, position.Standing,
		state.TransitionReason{Trigger: position.TriggerGrappleBreak},
	); err != nil {
		mudlog.Warn("submission mercy release: TransitionPair → Standing failed",
			"attempter", attempter.Name, "err", err)
	}
	// T9/4f stub: optional post-mercy stamina drag on the recipient.
	// Placeholder — apply via recipient.AddCondition(<id>) once a
	// recovery harmful-condition YAML is registered.
}

// applyDeathCascade routes the victim through the Life cascade with
// the T8 DeadData fields populated.
//
// noDeprogression — when true (subdue/cripple), Death_PlayerCleanup
// skips stat/skill rollback so the defender wakes at the temple without
// losing training.
//
// brokenLimb + brokenBodyPart — when true (cripple with a joint sub),
// a broken-limb condition is applied after the cascade.
//
// Returns true only when the broken-limb condition actually landed, so the
// caller narrates a break that happened rather than one it asked for: an
// already-dead victim short-circuits the whole cascade, and a choke sub
// carries no body part.
func applyDeathCascade(
	killer *characters.Character,
	victim *characters.Character,
	noDeprogression bool,
	brokenLimb bool,
	brokenBodyPart string,
) bool {
	cfg := configs.GetBalanceConfig()
	goldFrac := 0.0
	if noDeprogression {
		goldFrac = float64(cfg.SubGoldLossFraction)
	}

	killerRef := state.ActorRef{
		UserId:        killer.GetUserId(),
		MobInstanceId: killer.MobInstanceId,
	}

	if !victim.IsAlive() {
		return false
	}

	damageSnapshot := snapshotVictimDamage(victim)

	// Call TransitionToDead directly (instead of victim.Die) so we can
	// populate the new T8 fields on DeadData. The Respawning + Alive
	// transitions that follow mirror Die()'s player cascade.
	_ = victim.Life.TransitionToDead(
		life.DeadData{
			Killer:           killerRef,
			DamageMap:        damageSnapshot,
			NoDeprogression:  noDeprogression,
			GoldLossFraction: goldFrac,
		},
		state.TransitionReason{
			Trigger: life.TriggerSubmission,
			Actor:   killerRef,
		},
	)

	// Players continue through the respawn cycle; mobs stop at Dead.
	if victim.GetUserId() != 0 {
		_ = victim.Life.TransitionToRespawning(
			life.RespawningData{DestRoomId: victim.ResolveRespawnRoom()},
			state.TransitionReason{Trigger: life.TriggerRespawnReady},
		)
		_ = victim.Life.TransitionToAlive(
			state.TransitionReason{Trigger: life.TriggerRespawnComplete},
		)
	}

	if brokenLimb {
		return applyBrokenLimbCondition(victim, brokenBodyPart)
	}
	return false
}

// snapshotVictimDamage returns a shallow copy of the victim's
// PlayerDamage map so the DeadData carries a stable snapshot.
func snapshotVictimDamage(victim *characters.Character) map[int]int {
	if len(victim.PlayerDamage) == 0 {
		return nil
	}
	dst := make(map[int]int, len(victim.PlayerDamage))
	maps.Copy(dst, victim.PlayerDamage)
	return dst
}

// applyBrokenLimbCondition applies the chunk-4d broken-limb condition (id 83).
// Reports whether it landed: the caller narrates the authored start line to
// a player victim, and a break nobody took must not be announced.
//
// The condition is applied synchronously on the character, not through the condition
// event, because the outcome path above reads the victim's state back in the
// same round dispatch. Condition 83 is therefore flagged silent-start and its
// start line belongs to whoever called us.
func applyBrokenLimbCondition(victim *characters.Character, bodyPart string) bool {
	if victim == nil || bodyPart == "" {
		return false
	}
	// The body part ("arm" / "shoulder") is flavor only, spent by the
	// resolution narration in Position_Messaging. Future per-arm tracking
	// could drive weapon-specific accuracy penalties.
	return victim.AddCondition(BrokenLimbConditionId, false) == nil
}

// applyStunnedCondition applies the 1-round Stunned condition (id 84). Reports whether
// it landed, for the same reason applyBrokenLimbCondition does. Condition 84 is
// silent-start on the same grounds: it is applied synchronously and
// internal/combat cannot send the holder a line.
func applyStunnedCondition(c *characters.Character) bool {
	if c == nil {
		return false
	}
	return c.AddCondition(StunnedConditionId, false) == nil
}

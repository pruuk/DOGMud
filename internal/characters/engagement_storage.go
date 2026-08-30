// engagement_storage.go holds the engagement storage primitives: SetAggro,
// EndAggro and IsAggro.
//
// It was combat_state_compat.go, named for the Character.Aggro struct it
// existed to keep compatible. U12c-2 deleted that struct, so the file is named
// for what it now does. Renamed rather than deleted because SetAggro and
// EndAggro SURVIVE the collapse -- they are the storage primitives, writing
// CombatPhase alone. (The U12 spec's section 5 said to delete them; that was
// wrong, and section 6.3.7 records the correction.)
//
// The rule these enforce is a CALLER restriction, not a deletion: everything
// outside internal/characters and internal/targeting goes through the seam.
// aggro_writer_guard_test.go is what holds that.

package characters

import (
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// AggroType names the KIND of engagement a commit is starting. U12c-2 reduced
// it to that one job: it is a parameter now, never stored state.
//
// The Character.Aggro struct it used to live on is gone, and with it the four
// jobs it was quietly doing. Each moved to the machine that models it:
//
//	Flee           -> the Disengaging combat phase
//	SpellCast      -> the Casting activity, whose CastingData carries the aim
//	Shooting       -> derived from the equipped weapon (Engagement.Ranged)
//	SurpriseAttack -> CombatPhase.OpeningUnspent
//	RoundsWaiting  -> CombatPhase.RoundsWaiting (see its two-counter note)
//
// ⚠️ Flee and SpellCast are no longer REACHABLE as arguments: nothing calls
// SetAggro with either, because becoming Disengaging goes through
// TransitionToDisengaging and casting goes through SetCast. They survive as
// enum values only so the switch in TauntHoldBlocks keeps a correct default.
// Do not add a new caller for them; add a machine state instead.
type AggroType int

const (
	// Enumerated Aggro Types
	DefaultAttack AggroType = iota // Regular H2H combat, everything can decay to this. Starts at zero
	Shooting
	SurpriseAttack
	SpellCast
	Flee
)

// SpellAggroInfo carries a cast's target metadata into SetCast, which records
// it as activity.CastingData. U12c-2: it is a call parameter now, not stored
// state -- the Aggro struct that used to hold it is gone.
type SpellAggroInfo struct {
	SpellId              string
	SpellRest            string
	TargetUserIds        []int
	TargetMobInstanceIds []int
}

// userUntargetableFn is registered from hooks at boot. Returns true if
// the user with the given id is protected from incoming aggro (e.g.,
// post-respawn grace period). Called from SetAggro before setting
// aggro on a player target. nil = no check (safe default for tests).
var userUntargetableFn func(userId int) bool

// SetUserUntargetableCheck registers the untargetable-user check used
// by SetAggro's player-target gate. Follows the callback pattern used
// by rooms.SetCompanionTransport / rooms.SetBTreeStateEvictor to
// avoid the users→characters import cycle (characters cannot import
// users directly).
//
// Repeated registrations overwrite; pass nil to disable.
func SetUserUntargetableCheck(fn func(userId int) bool) {
	userUntargetableFn = fn
}

// TrackPlayerDamage records damage dealt by a player to this character.
func (c *Character) TrackPlayerDamage(userId int, damageAmt int) {

	roundNow := util.GetRoundCount()
	if len(c.PlayerDamage) == 0 {
		c.PlayerDamage = map[int]int{}
	} else {
		if roundNow-c.LastPlayerDamage > 30 {
			clear(c.PlayerDamage)
		}
	}

	c.PlayerDamage[userId] = c.PlayerDamage[userId] + damageAmt
	c.LastPlayerDamage = roundNow

}

// SetAggro sets the character's combat target and transitions Combat Phase to
// Engaging. All writes to Aggro go through this method (dual-write to
// CombatPhase for parity). Direct field reads of .Aggro remain valid.
func (c *Character) SetAggro(userId int, mobInstanceId int, aggroType AggroType, roundsWaitTime ...int) {
	// Grace-period guard: don't acquire aggro on a grace-protected
	// player. Other target shapes (mob, spellcast) are unaffected.
	if userId > 0 && userUntargetableFn != nil && userUntargetableFn(userId) {
		return
	}

	// Taunt-hold guard: while a taunt has this character pinned onto the
	// taunter, ignore basic-attack re-aggro that would switch the target
	// away (reactive `attack` re-aggro, per-round reciprocal, target-switch).
	// ForceTauntAggro sets the lock to the taunter first, so its own set
	// passes through. SpellCast/Flee and same-target sets are never blocked.
	if c.TauntHoldBlocks(userId, mobInstanceId, aggroType) {
		return
	}

	var combatAddlWaitRounds int = 0

	if len(roundsWaitTime) > 0 {
		for _, waitAmt := range roundsWaitTime {
			combatAddlWaitRounds += waitAmt
		}
	} else {
		combatAddlWaitRounds = c.Equipment.Weapon.GetSpec().WaitRounds + c.Equipment.Offhand.GetSpec().WaitRounds
	}

	// U12c-2: the DefaultAttack -> Shooting derivation that stood here is gone.
	// It was stored state computed from the equipped weapon at commit time,
	// while targeting.Engagement.Ranged derives the same fact live. Stored and
	// derived state cannot disagree if only one of them exists.

	// U12c-2: the combat phase machine IS the storage now, so this primitive
	// has to be total. Handed a Character whose machine was never built -- a
	// bare struct literal, which no production load path produces but many
	// fixtures do -- its only two options are to drop the write silently or to
	// build the storage. Dropping is the failure mode U12c-2 Task 2 named: the
	// write goes nowhere and the test measures nothing.
	//
	// The lazily built machine carries no registered vetoes and no self ref,
	// so it is NOT equivalent to a Validated one. That is acceptable precisely
	// because production never reaches here: every load path Validates, and
	// Validate builds the machine.
	if c.CombatPhase == nil {
		c.CombatPhase = combatphase.NewMachine()
	}

	// U12c-0b: the transition DECIDES. It used to run after the Aggro write
	// with its error discarded, so a vetoed commit left Aggro holding a target
	// the machine had rejected and the two stores disagreed by construction.
	//
	// Refusing to write here is consistent with this function's own shape: it
	// already returns without writing for the grace-period guard and the
	// taunt-hold guard above.
	//
	// A nil CombatPhase is the legacy/fixture path — there is nothing to
	// refuse, so the write proceeds as it always did.
	if c.CombatPhase != nil {
		trigger := combatphase.TriggerAttackCommand
		if aggroType == SurpriseAttack {
			trigger = combatphase.TriggerSurpriseAttack
		}
		if err := c.CombatPhase.TransitionToEngaging(combatphase.EngagingData{
			Target: state.ActorRef{
				UserId:        userId,
				MobInstanceId: mobInstanceId,
			},
			RoundsUntil: combatAddlWaitRounds,
		}, state.TransitionReason{
			Trigger: trigger,
			Actor:   c.ActorRef(),
			Target:  state.ActorRef{UserId: userId, MobInstanceId: mobInstanceId},
		}); err != nil {
			return
		}
		// U12c-2: the actor's round budget lives on the machine now. Seeded
		// here, beside the wind-up it happens to match, so the two counters
		// stay visibly distinct. See the two-counter note in combatphase.
		c.SetRoundsWaiting(combatAddlWaitRounds)
	}

	// Clear grapple state if switching targets. AFTER the transition, so a
	// commit that was refused does not clear grapple for a switch that never
	// happened. ClearGrappleState is currently a no-op, so this ordering is
	// defensive rather than observable — which is also why it has no test.
	if prev := c.CurrentCombatTarget(); !prev.IsZero() {
		if prev.UserId != userId || prev.MobInstanceId != mobInstanceId {
			c.ClearGrappleState()
		}
	}
}

// EndAggro clears the character's combat target and forces Combat Phase to Idle.
func (c *Character) EndAggro() {
	c.ClearTauntHold()
	c.ClearGrappleState()
	// U10d: the engaged-aim cue is once per ENGAGEMENT, and this is where an
	// engagement ends. Without this a shooter who explains it once would never
	// hear it again for the rest of the session.
	c.RangedEngagedCueSpoken = false
	if c.CombatPhase != nil && c.CombatPhase.IsInCombat() {
		c.CombatPhase.ForceIdle(state.TransitionReason{
			Trigger: combatphase.TriggerForceIdle,
		})
	}
}

// ClearGrappleState clears all grapple-related state.
// Called when combat ends, targets change, or participant dies.
func (c *Character) ClearGrappleState() {
	// FSM owns position state; no legacy fields to clear.
	// Position_Cascades.go handles the FSM reset on Alive→Dead via observer.
	// For mid-combat target switches, the FSM is left as-is; the next
	// round's state will reflect reality.
}

// IsAggro returns true if the character is currently engaged with the
// given target (by userId or mobInstanceId).
func (c *Character) IsAggro(targetUserId int, targetMobInstanceId int) bool {

	if target := c.CurrentCombatTarget(); !target.IsZero() {
		if target.MobInstanceId > 0 && target.MobInstanceId == targetMobInstanceId {
			return true
		}
		if target.UserId > 0 && target.UserId == targetUserId {
			return true
		}
	}

	// U12c-2: a pending cast's aim lives on the Activity machine now, not in
	// Aggro.SpellInfo. Checked outside the Aggro block on purpose: a caster's
	// aim counts as aggro whether or not it also holds a plain target.
	if cd, ok := c.CastingData(); ok {
		for _, uId := range cd.TargetUserIds {
			if uId == targetUserId && targetUserId > 0 {
				return true
			}
		}
		for _, mId := range cd.TargetMobInstanceIds {
			if mId == targetMobInstanceId && targetMobInstanceId > 0 {
				return true
			}
		}
	}
	return false
}

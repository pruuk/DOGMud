package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// GrappleResult holds the outcome of a grapple attempt for the caller to use
// when formatting messages, firing events, and updating UI.
type GrappleResult struct {
	Cost characters.CostCommitResult

	// Target is the resolved aggro target. Valid only when Executed is true.
	Target AggroTarget

	// MoveResult is the outcome from ExecuteGrappleMove. Valid only when
	// Executed is true.
	MoveResult combat.GrappleMoveResult

	// Executed reports whether the grapple was actually performed. False when
	// any early-exit condition fired (OnCooldown, NoTarget).
	Executed bool

	// OnCooldown is true when the special-move cooldown blocked the grapple.
	OnCooldown bool

	// Crafting is true when the actor is mid-craft and cannot grapple.
	Crafting bool

	// NoTarget is true when there is no aggro target or the target is gone.
	NoTarget bool

	// GrappleImmune is true when the grapple was refused for a body reason --
	// either the actor cannot grapple at all or the target cannot be grappled.
	// Read ImmuneReason to tell those apart; they need different messages.
	GrappleImmune bool

	// ImmuneReason explains WHY GrappleImmune is set.
	//
	// ⚠️ Two of the four reasons are about the ACTOR, not the target, and
	// those results deliberately carry NO Target. A caller that prints the
	// target's name unconditionally renders an empty name for them -- which is
	// exactly the defect this field exists to make impossible to write.
	ImmuneReason GrappleImmunityReason

	// TargetGrappling is true when the target is already in a grapple.
	TargetGrappling bool
}

// GrappleImmunityReason explains a refused grapple precisely enough for the
// caller to say something true about it.
//
// It exists because one message served all four cases: "You reach for X but
// your hands pass right through!" That is right for something INCORPOREAL and
// actively misleading for everything else. The Arena Champion is refused
// because colossus-form makes it immovable -- the opposite of intangible -- and
// the player was told their hands went through a creature that is in fact too
// solid to shift.
type GrappleImmunityReason int

const (
	// GrappleImmuneNone means no immunity applied.
	GrappleImmuneNone GrappleImmunityReason = iota
	// GrappleImmuneSelfSpecies: the ACTOR's species cannot grapple. No Target.
	GrappleImmuneSelfSpecies
	// GrappleImmuneSelfNoArms: the ACTOR has no arms to seize with. No Target.
	GrappleImmuneSelfNoArms
	// GrappleImmuneTargetIncorporeal: the target has no solid body to hold.
	GrappleImmuneTargetIncorporeal
	// GrappleImmuneTargetImmovable: the target is control-immune -- colossus-form
	// or Ironhide's Living Carapace. Solid, and simply cannot be moved.
	GrappleImmuneTargetImmovable
)

// ExecuteGrapple performs the core grapple resolution shared between player
// and mob callers. It handles:
//   - Special-move cooldown (using SpecialMoveCooldown from balance config)
//   - Target resolution via ResolveAggroTarget
//   - ExecuteGrappleMove via combat package
//   - RecordAndWait (analytics + round consumption)
//
// Callers are responsible for all messaging, skill progression events, and
// any combat-initiation logic (e.g. SetAggro for player out-of-combat
// grapple).
func ExecuteGrapple(actor Actor) GrappleResult {
	char := actor.GetCharacter()

	// Don't interrupt any active activity (cast/craft/salvage) to grapple.
	if char.IsActing() {
		return GrappleResult{Crafting: true}
	}

	// Resolve the aggro target.
	target := resolveActionTarget(actor, char)
	if !target.Found {
		return GrappleResult{NoTarget: true}
	}

	// Grapple-immune species can't initiate grapple either (ethereal, fire, etc.)
	if sp := species.GetSpecies(char.SpeciesId); sp != nil && sp.GrappleImmune {
		return GrappleResult{GrappleImmune: true, ImmuneReason: GrappleImmuneSelfSpecies}
	}

	// Grappling is a humanoid technique — requires arms to seize/hold.
	if !char.HasBodyPart("arms") {
		return GrappleResult{GrappleImmune: true, ImmuneReason: GrappleImmuneSelfNoArms}
	}
	if target.Char.IsGrappling() {
		return GrappleResult{Target: target, TargetGrappling: true}
	}

	// Grapple immunity (ethereal creatures, fire elementals, etc.)
	if sp := species.GetSpecies(target.Char.SpeciesId); sp != nil && sp.GrappleImmune {
		return GrappleResult{Target: target, GrappleImmune: true, ImmuneReason: GrappleImmuneTargetIncorporeal}
	}
	// Control-immune (immovable) targets cannot be grappled — Ironhide's Living
	// Carapace, Colossus's Ossified Frame.
	if mutations.IsControlImmune(target.Char.Mutations) {
		return GrappleResult{Target: target, GrappleImmune: true, ImmuneReason: GrappleImmuneTargetImmovable}
	}

	cfg := configs.GetBalanceConfig()
	if !char.CooldownReady("special-move") {
		return GrappleResult{OnCooldown: true}
	}
	cost := admitFullCost(actor, costs.ActionGrapple, characters.PoolStamina, float64(cfg.SpecialMoveBaseStaminaCost))
	if cost.Status == characters.CostRefused {
		return GrappleResult{Cost: cost}
	}
	if !char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		return GrappleResult{Cost: cost, OnCooldown: true}
	}
	commitMeleeEngagement(actor)

	// Execute the grapple move. Player actors pass their UserId; mobs pass 0.
	attackerId := actor.GetUserId()
	result := combat.ExecuteGrappleMove(char, target.Char, attackerId, actor.GetRoom())

	// Determine source/target types for analytics.
	sourceType := combat.User
	if !actor.IsPlayer() {
		sourceType = combat.Mob
	}
	targetType := combat.User
	if target.MobInstanceId > 0 {
		targetType = combat.Mob
	}

	// Record analytics and consume the combat round.
	RecordAndWait(char, "grapple", sourceType, target.Char, targetType, result.Success, 0, util.GetRoundCount())

	// Progression: unarmed-combat on executed grapple (moved from user/mob wrappers)
	// U10b-1 Task 18b: win OR lose. This was gated on the hit, so a special
	// move that missed trained nothing -- the same defect a failed craft had
	// before Task 16. The gate is now the AWARD WEIGHT rather than a
	// precondition; a thrown move is a resolved contest either way.
	// GRAPPLING TRAINS STRENGTH, not unarmed-combat's usual dexterity.
	// Owner ruling 2026-08-26: a grapple is a contest of leverage and hold,
	// which is what strength is for, and it is one of the two replacements for
	// the attack-side strength faucet this task deleted (the other is the
	// stamina regen tick).
	//
	// Expressed by naming Candidate.Stat explicitly rather than by changing
	// the skill: the move is still unarmed-combat, it just trains a different
	// stat. This is the same shape combat.DefenceSkillAndStat already uses for
	// BLOCK (weapon-combat / strength) and DEFY (rhetoric / willpower), so it
	// is an established pattern rather than a new one.
	//
	// ⚠️ Built by hand rather than through CandidateFor, which always uses the
	// skill's PRIMARY stat. The Roll is unused here -- a single candidate wins
	// its Best-of whatever it rolled -- so nothing is lost by not rolling one.
	c := actor.GetCharacter()
	actor.AwardResolved(result.Success, progression.Candidate{
		Skill: string(skills.UnarmedCombat),
		Stat:  "strength",
		Level: c.GetSkillLevel(skills.UnarmedCombat),
	})

	return GrappleResult{
		Cost:       cost,
		Target:     target,
		MoveResult: result,
		Executed:   true,
	}
}

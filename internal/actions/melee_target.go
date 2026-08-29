package actions

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// MeleeTargetOpts carries the per-verb phrasing for StageMeleeTarget.
//
// Only Verb is required; the message fields default from it and exist so verbs
// whose wording differs (trip, pounce) keep their exact player-facing text.
type MeleeTargetOpts struct {
	// Verb is the lowercase command name, e.g. "bash". It drives the defaults
	// for every message below.
	Verb string

	// CraftingVerb overrides the verb inside the crafting-guard message.
	// Defaults to Verb. `trip` uses "trip someone".
	CraftingVerb string

	// PromptMsg overrides the empty-target prompt.
	// Defaults to "<Verb> whom?" (title-cased). `pounce` uses "Pounce on whom?".
	PromptMsg string

	// SelfTargetMsg overrides the self-targeting rejection.
	// Defaults to "You can't <Verb> yourself." `pounce` adds "on".
	SelfTargetMsg string

	// CharmedMsg overrides the charmed-target (player companion) rejection.
	// Defaults to "You can't <Verb> a companion." `pounce` adds "on".
	CharmedMsg string
}

func (o MeleeTargetOpts) craftingMsg() string {
	v := o.CraftingVerb
	if v == "" {
		v = o.Verb
	}
	return fmt.Sprintf(
		`<ansi fg="red">You can't %s while focused on your work. Finish or be interrupted first.</ansi>`, v)
}

func (o MeleeTargetOpts) promptMsg() string {
	if o.PromptMsg != "" {
		return o.PromptMsg
	}
	if o.Verb == "" {
		return "Attack whom?"
	}
	return strings.ToUpper(o.Verb[:1]) + o.Verb[1:] + " whom?"
}

func (o MeleeTargetOpts) charmedMsg() string {
	if o.CharmedMsg != "" {
		return o.CharmedMsg
	}
	return fmt.Sprintf("You can't %s a companion.", o.Verb)
}

func (o MeleeTargetOpts) selfTargetMsg() string {
	if o.SelfTargetMsg != "" {
		return o.SelfTargetMsg
	}
	return fmt.Sprintf("You can't %s yourself.", o.Verb)
}

// stagedMeleeActor is a UserActor that has RESOLVED a target but not yet
// ENGAGED it. It exists because U8 admission has to be able to refuse: the
// engagement side effects (SetAggro, SeedAggression) are captured in commit and
// replayed only once the action has actually been paid for and has taken its
// cooldown. Committing is idempotent -- committed guards a second call, so a
// command that both stages and later re-enters cannot double-seed aggression.
type stagedMeleeActor struct {
	Actor
	target    AggroTarget
	commit    func()
	committed bool
}

// actionTarget resolves the staged target the same way an engaged actor's aggro
// would, WITHOUT requiring aggro to have been set. The Execute* actions call it
// through resolveActionTarget so their target lookup works identically before
// and after engagement.
func (a *stagedMeleeActor) actionTarget() AggroTarget {
	aggro := &characters.Aggro{UserId: a.target.UserId, MobInstanceId: a.target.MobInstanceId}
	return ResolveAggroTarget(aggro)
}

func (a *stagedMeleeActor) commitMeleeEngagement() {
	if a.committed {
		return
	}
	a.committed = true
	if a.commit != nil {
		a.commit()
	}
}

// resolveActionTarget reads the target an action should act on. A staged actor
// answers from its captured target; anything else (mobs, and players already in
// combat) falls back to live aggro, which is what they engaged through.
func resolveActionTarget(actor Actor, char *characters.Character) AggroTarget {
	if staged, ok := actor.(interface{ actionTarget() AggroTarget }); ok {
		return staged.actionTarget()
	}
	return ResolveAggroTarget(char.Aggro)
}

// commitMeleeEngagement fires the deferred engagement, if this actor has one.
// Actions call it AFTER cost admission and the consuming cooldown succeed, so a
// refused action leaves no aggro, no aggression seed, and no fight.
func commitMeleeEngagement(actor Actor) {
	if staged, ok := actor.(interface{ commitMeleeEngagement() }); ok {
		staged.commitMeleeEngagement()
	}
}

// StageMeleeTarget runs the shared gate-and-resolve preamble that every melee
// special move needs before delegating to its Execute* action. It validates and
// resolves a target WITHOUT setting aggro or seeding aggression; admission-gated
// actions commit the returned actor only after their cost and consuming
// cooldown succeed.
//
// It returns handled=true when it has already sent the player a message and the
// calling command should return immediately. handled=false means the actor is
// staged (or was already in combat) and the caller should proceed.
//
// Steps, in order:
//  1. Refuse if the actor is mid-activity (craft/salvage/cast).
//  2. If already in combat, do nothing — the existing target stands.
//  3. Otherwise require a target, resolve it, and reject self-targeting,
//     non-combatants, attack-immune mobs, player companions (charmed),
//     PvP-disallowed players, and the actor's own party members.
//  4. CAPTURE, rather than perform, the engagement that gives the subsequent
//     Execute* call something to act on. U8 replays it via
//     commitMeleeEngagement once the action has been paid for.
//
// This was copy-pasted across 11 command files (bash, drain, gore, grapple,
// kick, maul, pounce, rake, taunt, throttle, trip), differing only in wording.
// Beyond the duplication, each copy performed a redundant second room scan
// (room.FindByName) purely to distinguish "can't target yourself" from "not
// here" — that scan happens once here, and only on the error path.
func StageMeleeTarget(user *users.UserRecord, room *rooms.Room, rest string, opts MeleeTargetOpts) (Actor, bool) {
	base := &UserActor{User: user, Room: room}
	if user.Character.IsActing() {
		user.SendText(messaging.CategorySystem, opts.craftingMsg())
		return nil, true
	}
	if user.Character.IsInCombat() {
		return base, false
	}
	if rest == "" {
		user.SendText(messaging.CategorySystem, opts.promptMsg())
		return nil, true
	}

	target, err := ResolveTargetActor(room, rest, ResolveTargetOptions{ExcludeUserId: user.UserId})
	if err != nil {
		// Self-exclusion collapses to NotFound, so distinguish the two here.
		if pId, _ := room.FindByName(rest); pId == user.UserId {
			user.SendText(messaging.CategorySystem, opts.selfTargetMsg())
			return nil, true
		}
		user.SendText(messaging.CategorySystem, "You don't see them here.")
		return nil, true
	}

	if !target.IsPlayer() {
		mob := target.(*MobActor).Mob
		// Player companions are off-limits to every melee special move, the
		// same way `attack` already refuses them. Before this was centralised,
		// only taunt enforced it, so a companion could be bashed/kicked/
		// grappled/tripped with no message at all.
		switch mobs.CheckPlayerHarm(mob) {
		case mobs.HarmBlockedCompanion:
			user.SendText(messaging.CategorySystem, opts.charmedMsg())
			return nil, true
		case mobs.HarmBlockedNonCombatant, mobs.HarmBlockedAttackImmune:
			user.SendText(messaging.CategorySystem,
				fmt.Sprintf(`You can't attack <ansi fg="mobname">%s</ansi>.`, mob.Character.Name))
			return nil, true
		}
		// Judge fresh aggression HERE, at staging time, and capture the verdict
		// in the closure. The same rule as before U8 -- either no prior aggro at
		// all, or aggro pointed at a different target, and it must be read on
		// this side of SetAggro, which overwrites it. Same test as attack.go.
		// Staging makes the ordering easier to get wrong, not harder: the
		// closure runs after admission, by which time Aggro may have been set by
		// something else, so the answer has to be frozen at this line.
		freshAggro := user.Character.Aggro == nil || user.Character.Aggro.MobInstanceId != mob.InstanceId
		stagedTarget := AggroTarget{Char: &mob.Character, Name: mob.Character.Name, MobInstanceId: mob.InstanceId, Found: true}
		// Every melee special move engages through here: kick, bash, trip,
		// grapple, taunt, and the mutation attacks (gore, maul, pounce, rake,
		// throttle, drain). Until 2026-08-14 none of them seeded anything, so
		// all eleven were invisible to the revenge, opinion and justice
		// systems while `attack` and `shoot` were not. Seeding at this shared
		// seam rather than in each command is what stops them diverging again.
		//
		// taunt is included on purpose. It deals conviction damage, so it is an
		// attack in this world, and treating it as one is simpler than carving
		// out an exception nobody would remember.
		//
		// U8: deferred into commit so a REFUSED action does not seed aggression
		// or start a fight the actor could not afford to start.
		return &stagedMeleeActor{Actor: base, target: stagedTarget, commit: func() {
			targeting.Commit(user.Character,
				state.ActorRef{MobInstanceId: mob.InstanceId}, targeting.ReasonAttack)
			SeedAggression(user, mob, room, freshAggro)
		}}, false
	}

	p := target.(*UserActor).User
	if pvpErr := room.CanPvp(user, p); pvpErr != nil {
		user.SendText(messaging.CategorySystem, pvpErr.Error())
		return nil, true
	}
	// Party members are not valid targets. room.CanPvp does NOT cover this — it
	// only checks fighting-allowed, PVP-enabled, the experience threshold and
	// PVP-area — so without this an enabled-PvP area let a player bash, kick,
	// grapple or trip their own party member, while `attack` and `shoot` both
	// refused. Same bypass shape as the charmed-companion gate above.
	if partyInfo := parties.Get(user.UserId); partyInfo != nil && partyInfo.IsMember(p.UserId) {
		user.SendText(messaging.CategorySystem,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> is in your party!`, p.Character.Name))
		return nil, true
	}

	stagedTarget := AggroTarget{Char: p.Character, Name: p.Character.Name, UserId: p.UserId, Found: true}
	return &stagedMeleeActor{Actor: base, target: stagedTarget, commit: func() {
		targeting.Commit(user.Character,
			state.ActorRef{UserId: p.UserId}, targeting.ReasonAttack)
	}}, false
}

// AcquireMeleeTarget is DELETED, deliberately. It was the eager
// gate-and-engage helper all eleven melee specials used before U8, and it was
// briefly kept as a "legacy helper for callers that need it" wrapper with zero
// production callers. Keeping it would have kept a second, eager-engagement
// path alive that a future command could pick up by accident and thereby
// bypass admission ordering entirely -- engaging first and paying afterwards is
// exactly the bug U8 exists to make unrepresentable. Use StageMeleeTarget and
// commit through commitMeleeEngagement once the action is paid for.
//
// command_readiness_drift_test.go asserts no shared action calls it; with the
// function gone that guarantee is structural rather than tested.

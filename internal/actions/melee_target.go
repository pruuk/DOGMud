package actions

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// MeleeTargetOpts carries the per-verb phrasing for AcquireMeleeTarget.
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

type stagedMeleeActor struct {
	Actor
	target    AggroTarget
	commit    func()
	committed bool
}

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

func resolveActionTarget(actor Actor, char *characters.Character) AggroTarget {
	if staged, ok := actor.(interface{ actionTarget() AggroTarget }); ok {
		return staged.actionTarget()
	}
	return ResolveAggroTarget(char.Aggro)
}

func commitMeleeEngagement(actor Actor) {
	if staged, ok := actor.(interface{ commitMeleeEngagement() }); ok {
		staged.commitMeleeEngagement()
	}
}

// StageMeleeTarget validates and resolves a target without setting aggro or
// seeding aggression. Admission-gated actions commit the returned actor only
// after their cost and consuming cooldown succeed.
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
		if pId, _ := room.FindByName(rest); pId == user.UserId {
			user.SendText(messaging.CategorySystem, opts.selfTargetMsg())
			return nil, true
		}
		user.SendText(messaging.CategorySystem, "You don't see them here.")
		return nil, true
	}

	if !target.IsPlayer() {
		mob := target.(*MobActor).Mob
		switch mobs.CheckPlayerHarm(mob) {
		case mobs.HarmBlockedCompanion:
			user.SendText(messaging.CategorySystem, opts.charmedMsg())
			return nil, true
		case mobs.HarmBlockedNonCombatant, mobs.HarmBlockedAttackImmune:
			user.SendText(messaging.CategorySystem,
				fmt.Sprintf(`You can't attack <ansi fg="mobname">%s</ansi>.`, mob.Character.Name))
			return nil, true
		}
		freshAggro := user.Character.Aggro == nil || user.Character.Aggro.MobInstanceId != mob.InstanceId
		stagedTarget := AggroTarget{Char: &mob.Character, Name: mob.Character.Name, MobInstanceId: mob.InstanceId, Found: true}
		return &stagedMeleeActor{Actor: base, target: stagedTarget, commit: func() {
			user.Character.SetAggro(0, mob.InstanceId, characters.DefaultAttack)
			SeedAggression(user, mob, room, freshAggro)
		}}, false
	}

	p := target.(*UserActor).User
	if pvpErr := room.CanPvp(user, p); pvpErr != nil {
		user.SendText(messaging.CategorySystem, pvpErr.Error())
		return nil, true
	}
	if partyInfo := parties.Get(user.UserId); partyInfo != nil && partyInfo.IsMember(p.UserId) {
		user.SendText(messaging.CategorySystem,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> is in your party!`, p.Character.Name))
		return nil, true
	}

	stagedTarget := AggroTarget{Char: p.Character, Name: p.Character.Name, UserId: p.UserId, Found: true}
	return &stagedMeleeActor{Actor: base, target: stagedTarget, commit: func() {
		user.Character.SetAggro(p.UserId, 0, characters.DefaultAttack)
	}}, false
}

// AcquireMeleeTarget preserves eager engagement for existing callers such as
// taunt. Physical specials stage instead and commit inside Execute*.
func AcquireMeleeTarget(user *users.UserRecord, room *rooms.Room, rest string, opts MeleeTargetOpts) bool {
	actor, handled := StageMeleeTarget(user, room, rest, opts)
	if handled {
		return true
	}
	commitMeleeEngagement(actor)
	return false
}

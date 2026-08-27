package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ShadowOptions parameterizes a shadow attempt.
// Exactly one of TargetMobInstanceId / TargetUserId must be set.
type ShadowOptions struct {
	TargetMobInstanceId int
	TargetUserId        int
}

// ShadowResult is the structured outcome of a shadow attempt.
type ShadowResult struct {
	Succeeded  bool   // target id was stored and shadow tracking began
	Detected   bool   // target won the initial detection roll
	TargetName string // display name of the target
	OnCooldown bool   // attempt was blocked by shadow cooldown
	Reason     string // when Succeeded==false and !OnCooldown, why
}

// Shadow attempts to track a target while hidden. Requires the actor to
// already carry buff 9 (Hidden). On success, stores the target's id in
// the actor's misc-data so the engine's existing shadow-follow event
// listener can auto-follow on the target's room-change events. An initial
// detection roll is performed for player targets: if the target wins, they
// sense pursuit (Detected=true) but shadow still begins.
func Shadow(actor Actor, opts ShadowOptions) ShadowResult {
	char := actor.GetCharacter()

	// Must be hidden to shadow.
	if !char.IsHidden() {
		actor.SendText(messaging.CategorySystem,
			"You must be hidden to shadow someone. "+
				`Try <ansi fg="command">sneak</ansi> first.`)
		return ShadowResult{Reason: "not hidden"}
	}

	// Combat gate.
	if char.Aggro != nil {
		actor.SendText(messaging.CategorySystem, "You can't do that while in combat!")
		return ShadowResult{Reason: "in combat"}
	}

	// Require a target.
	if opts.TargetMobInstanceId == 0 && opts.TargetUserId == 0 {
		actor.SendText(messaging.CategorySystem, "Shadow whom?")
		return ShadowResult{Reason: "no target"}
	}

	cfg := configs.GetBalanceConfig()
	cooldownKey := skills.Skullduggery.String(`shadow`)

	// Check cooldown before doing target resolution.
	if !char.TryCooldown(cooldownKey,
		fmt.Sprintf(`%d rounds`, cfg.ShadowCooldown)) {
		return ShadowResult{
			OnCooldown: true,
			Reason: fmt.Sprintf("%d rounds remaining",
				char.GetCooldown(cooldownKey)),
		}
	}

	if opts.TargetMobInstanceId > 0 {
		return shadowMob(actor, opts.TargetMobInstanceId, cfg)
	}

	return shadowPlayer(actor, opts.TargetUserId, cfg)
}

// shadowMob handles the mob-target shadow path.
func shadowMob(actor Actor, mobInstanceId int, cfg configs.Balance) ShadowResult {
	m := mobs.GetInstance(mobInstanceId)
	if m == nil {
		actor.SendText(messaging.CategorySystem, "They seem to have vanished.")
		return ShadowResult{Reason: "target not found"}
	}

	char := actor.GetCharacter()
	char.SetMiscData("shadow-target-user", nil)
	char.SetMiscData("shadow-target-mob", m.InstanceId)
	actor.AddBuff(87, "skill")

	actor.SendText(messaging.CategorySystem, fmt.Sprintf(
		`You begin shadowing <ansi fg="mobname">%s</ansi>, `+
			`moving silently in their wake.`,
		m.Character.Name))

	// U10b-1 Task 18: the MOB-target shadow runs no contest at all -- the buff
	// is applied and the shadow simply begins -- so there is nothing to lose
	// and won is unconditionally true. Contrast shadowPlayer below, which does
	// roll against the target and passes !detected.
	actor.AwardResolved(true, actor.GetCharacter().CandidateFor(string(skills.Skullduggery)))

	// Quest engine notification — player actors only.
	if actor.IsPlayer() {
		if u := users.GetByUserId(actor.GetUserId()); u != nil {
			room := actor.GetRoom()
			bridge := questengine.NewGameBridge(u, room.RoomId)
			questengine.GetEngine().Notify("command", questengine.EventDetails{
				UserId:  actor.GetUserId(),
				RoomId:  room.RoomId,
				Command: "shadow",
			}, bridge, bridge)

		}
	}

	return ShadowResult{
		Succeeded:  true,
		TargetName: m.Character.Name,
	}
}

// shadowPlayer handles the player-target shadow path. An initial detection
// roll is performed: if the target wins (their Per+Search beats the actor's
// Dex+Skullduggery), the target senses pursuit. Shadow still begins
// regardless — the Detected flag is informational.
func shadowPlayer(actor Actor, targetUserId int, cfg configs.Balance) ShadowResult {
	targetUser := users.GetByUserId(targetUserId)
	if targetUser == nil {
		actor.SendText(messaging.CategorySystem, "They seem to have vanished.")
		return ShadowResult{Reason: "target not found"}
	}

	char := actor.GetCharacter()
	char.SetMiscData("shadow-target-user", targetUser.UserId)
	char.SetMiscData("shadow-target-mob", nil)
	actor.AddBuff(87, "skill")

	actor.SendText(messaging.CategorySystem, fmt.Sprintf(
		`You begin shadowing <ansi fg="username">%s</ansi>, `+
			`watching their every move.`,
		targetUser.Character.Name))

	// Quest engine notification — player actors only.
	if actor.IsPlayer() {
		if u := users.GetByUserId(actor.GetUserId()); u != nil {
			room := actor.GetRoom()
			bridge := questengine.NewGameBridge(u, room.RoomId)
			questengine.GetEngine().Notify("command", questengine.EventDetails{
				UserId:  actor.GetUserId(),
				RoomId:  room.RoomId,
				Command: "shadow",
			}, bridge, bridge)

		}
	}

	// Initial detection roll: the TARGET is the attacker here -- they are the
	// one trying to notice -- so the shadowing actor's sneak score is the
	// defending entry. Same formula as shadowDetectionRoll in
	// usercommands/skill.skullduggery.shadow.go (Per+Search vs Dex+Skullduggery).
	sneakScore := CalcSneakScoreVsObserver(char, targetUser.Character, actor.GetRoom())
	searchScore := CalcDetectionScore(targetUser.Character)
	detected := combat.RunContest(searchScore, []contest.Entry{{Score: sneakScore}}).Success
	// U10b-1 Task 18: moved DOWN below the detection contest and given its
	// outcome. detected means the TARGET spotted the shadower -- RunContest is
	// called with the target's search score as the ATTACKER -- so the
	// shadower's win is !detected.
	//
	// The plan called this roll "informational only" because the shadow begins
	// either way. That is true of the ShadowResult flag and false of the
	// CONTEST: skullduggery really was rolled against a real opposing score,
	// which is exactly what a resolved action is.
	actor.AwardResolved(!detected, actor.GetCharacter().CandidateFor(string(skills.Skullduggery)))
	if detected {
		targetUser.SendText(messaging.CategorySystem, "You sense someone following close behind you.")
	}

	return ShadowResult{
		Succeeded:  true,
		Detected:   detected,
		TargetName: targetUser.Character.Name,
	}
}

package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ShadowingConditionId is condition 87, "Shadowing": held while a shadow is
// live. It is named here and nowhere else (shadow_follow_guard_test.go).
const ShadowingConditionId = 87

// The two misc-data keys that hold a shadow's quarry. At most one is set.
// Only this file reads or writes them (shadow_follow_guard_test.go); every
// other package goes through ShadowTargetOf, ClearShadow and EndShadow.
const (
	shadowTargetUserKey = "shadow-target-user"
	shadowTargetMobKey  = "shadow-target-mob"
)

// shadowSensedLine is what a player target reads when it senses a shadower.
const shadowSensedLine = "You sense someone following close behind you."

// ShadowOptions parameterizes a shadow attempt.
// Exactly one of TargetMobInstanceId / TargetUserId must be set.
type ShadowOptions struct {
	TargetMobInstanceId int
	TargetUserId        int
}

// ShadowResult is the structured outcome of a shadow attempt.
type ShadowResult struct {
	Succeeded  bool   // target id was stored and shadow tracking began
	Detected   bool   // target won the initial sense roll
	TargetName string // display name of the target
	OnCooldown bool   // attempt was blocked by shadow cooldown
	Reason     string // when Succeeded==false and !OnCooldown, why
}

// Shadow attempts to track a target while hidden. The actor must be hidden
// (Character.IsHidden). On success it stores the target's id (ShadowTargetOf
// reads it back) and applies ShadowingConditionId, so
// hooks.RoomChangeShadowFollow moves the actor after its quarry. The target
// then makes the initial sense roll (ShadowSenseRoll), player or mob alike:
// if it wins it senses pursuit (Detected=true), but the shadow begins either
// way.
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
	if char.IsInCombat() {
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
		return shadowMob(actor, opts.TargetMobInstanceId)
	}

	return shadowPlayer(actor, opts.TargetUserId)
}

// shadowMob handles the mob-target shadow path.
func shadowMob(actor Actor, mobInstanceId int) ShadowResult {
	m := mobs.GetInstance(mobInstanceId)
	if m == nil {
		actor.SendText(messaging.CategorySystem, "They seem to have vanished.")
		return ShadowResult{Reason: "target not found"}
	}

	char := actor.GetCharacter()
	char.SetMiscData(shadowTargetUserKey, nil)
	char.SetMiscData(shadowTargetMobKey, m.InstanceId)
	actor.AddCondition(ShadowingConditionId, "skill")

	actor.SendText(messaging.CategorySystem, fmt.Sprintf(
		`You begin shadowing <ansi fg="mobname">%s</ansi>, `+
			`moving silently in their wake.`,
		m.Character.Name))

	// Parity slice 6, ruling 5: a mob target makes the same sense roll a
	// player target does. Until then this path ran no contest and awarded a
	// win outright. The roll awards Skullduggery on both outcomes.
	room := actor.GetRoom()
	detected := ShadowSenseRoll(actor, NewMobActorInRoom(m, room), room)

	// Quest engine notification — player actors only.
	if actor.IsPlayer() {
		if u := users.GetByUserId(actor.GetUserId()); u != nil {
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
		Detected:   detected,
		TargetName: m.Character.Name,
	}
}

// shadowPlayer handles the player-target shadow path. The target makes the
// initial sense roll (ShadowSenseRoll); the shadow begins either way, and
// Detected reports the roll.
func shadowPlayer(actor Actor, targetUserId int) ShadowResult {
	targetUser := users.GetByUserId(targetUserId)
	if targetUser == nil {
		actor.SendText(messaging.CategorySystem, "They seem to have vanished.")
		return ShadowResult{Reason: "target not found"}
	}

	char := actor.GetCharacter()
	char.SetMiscData(shadowTargetUserKey, targetUser.UserId)
	char.SetMiscData(shadowTargetMobKey, nil)
	actor.AddCondition(ShadowingConditionId, "skill")

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

	room := actor.GetRoom()
	detected := ShadowSenseRoll(actor, NewUserActorInRoom(targetUser, room), room)

	return ShadowResult{
		Succeeded:  true,
		Detected:   detected,
		TargetName: targetUser.Character.Name,
	}
}

// ShadowSenseRoll is the one contest a shadow runs: does the target sense
// the shadower? The TARGET is the attacker (it is the one trying to notice),
// its CalcDetectionScore against the shadower's CalcSneakScoreVsObserver,
// with the shadower paying its sight ramp in room's light. It is rolled when
// a shadow starts and each time the shadower arrives in its quarry's room
// (hooks.RoomChangeShadowFollow).
//
// It awards the shadower's Skullduggery on both outcomes, the resolved-contest
// convention (U10b-2): a win when the target did not sense it. A player
// target that senses it reads shadowSensedLine; a mob target learns nothing
// visible (Actor.SendText is a no-op for a mob). It reveals nothing: a reveal
// is entry detection's job. It returns whether the target sensed the shadower.
func ShadowSenseRoll(shadower, target Actor, room *rooms.Room) bool {
	sight := combat.SightRoom(room)
	shadowChar := shadower.GetCharacter()
	targetChar := target.GetCharacter()

	sneakScore := CalcSneakScoreVsObserver(shadowChar, targetChar, sight)
	// sight ramp (plan 5b): the shadower needs to see the quarry to keep on it.
	sneakScore *= messaging.SightMult(shadowChar, sight)
	senseScore := CalcDetectionScore(targetChar, sight)

	detected := combat.RunContest(senseScore, []contest.Entry{{Score: sneakScore}}).Success
	shadower.AwardResolved(!detected, shadowChar.CandidateFor(string(skills.Skullduggery)))
	if detected {
		target.SendText(messaging.CategorySystem, shadowSensedLine)
	}
	return detected
}

// ShadowTargetOf returns the quarry c is shadowing: a player's user id or a
// mob's instance id, the other zero, or both zero when c shadows no one. It
// reads the target only; whether the shadow is live is
// c.HasCondition(ShadowingConditionId).
func ShadowTargetOf(c *characters.Character) (userId, mobInstanceId int) {
	if c == nil {
		return 0, 0
	}
	userId, _ = c.GetMiscData(shadowTargetUserKey).(int)
	mobInstanceId, _ = c.GetMiscData(shadowTargetMobKey).(int)
	return userId, mobInstanceId
}

// ClearShadow drops c's shadow outright: both target keys and
// ShadowingConditionId. No cooldown and no message, which is what the
// stale-state guard and the target death and logoff cleanups want. A shadow
// that ENDS (spotted, or `shadow stop`) goes through EndShadow instead.
func ClearShadow(c *characters.Character) {
	if c == nil {
		return
	}
	c.SetMiscData(shadowTargetUserKey, nil)
	c.SetMiscData(shadowTargetMobKey, nil)
	c.RemoveCondition(ShadowingConditionId)
}

// EndShadow ends actor's shadow: ClearShadow, then the shadow cooldown
// (Balance.ShadowCooldown rounds), then reason to the actor when it is not
// empty (a mob reads nothing).
func EndShadow(actor Actor, reason string) {
	char := actor.GetCharacter()
	ClearShadow(char)

	cfg := configs.GetBalanceConfig()
	char.TryCooldown(skills.Skullduggery.String(`shadow`),
		fmt.Sprintf(`%d rounds`, cfg.ShadowCooldown))

	if reason != "" {
		actor.SendText(messaging.CategorySystem, reason)
	}
}

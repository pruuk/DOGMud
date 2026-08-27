package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

/*
Skullduggery Skill
Level 1 - Sneak: attempt to enter a hidden state outside of combat.
Uses an opposed roll against each observer in the room.
*/
func Sneak(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	skillLevel := user.Character.GetSkillLevel(skills.Skullduggery)

	// If they don't have the skill, act like it's not a valid command
	if skillLevel < 1 {
		return false, nil
	}

	// Can't sneak while crafting or otherwise occupied
	if !user.Character.IsFree() {
		user.SendText(messaging.CategorySystem, `You are busy with something else.`)
		return true, nil
	}

	cfg := configs.GetBalanceConfig()
	sneakCooldownKey := skills.Skullduggery.String(`sneak`)

	// Check cooldown — only block if on cooldown from a prior failure
	if !user.Character.CooldownReady(sneakCooldownKey) {
		remaining := user.Character.Cooldowns[sneakCooldownKey]
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			"You need to wait %d more rounds before you can try that again.",
			remaining))
		return true, nil
	}

	result := actions.Sneak(&actions.UserActor{User: user, Room: room})
	if result.Cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(result.Cost))
		return true, nil
	}

	switch {
	case result.AlreadyHidden:
		user.SendText(messaging.CategorySystem, "You're already hidden!")
		return true, nil

	case result.InCombat:
		user.SendText(messaging.CategorySystem, "You can't do that while in combat!")
		return true, nil

	case result.SpottedByName != "":
		// Apply failure cooldown so the player can't spam sneak
		if cfg.SneakFailCooldown > 0 {
			user.Character.TryCooldown(sneakCooldownKey,
				fmt.Sprintf(`%d rounds`, cfg.SneakFailCooldown))
		}
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`You try to blend into the shadows but <ansi fg="mobname">%s</ansi> notices you.`,
			result.SpottedByName))

		// U10b-1 Task 18: the SPOTTED branch, so won is false -- this is the
		// loss half of the sneak contest and now pays
		// ProgressionFailureFraction rather than a full event.
		//
		// Still gated on RollHappened: a sneak with nothing in the room to
		// notice you resolved no contest, so there is no loss to pay a
		// fraction on. Converted off the direct CheckSkillProgression call,
		// which bypassed every entry point.
		if result.RollHappened {
			user.Character.AwardResolved(user.UserId, false,
				user.Character.CandidateFor(string(skills.Skullduggery)))
		}
		return true, nil
	}

	// Success
	user.SendText(messaging.CategorySystem, `You slip into the shadows.`)

	// U10b-1 Task 18: the SUCCESS branch, won: true.
	//
	// The explicit events.SkillUsed emission that stood here is GONE rather
	// than kept: AwardResolved reaches OnSkillUseScaled, which emits it, so
	// keeping both would fire the quest event twice for one sneak. Its Details
	// field ("sneak") is not lost in any meaningful sense -- SkillUseQuestNotify
	// reads only UserId and Skill, and nothing in the repo reads Details.
	if result.RollHappened {
		user.Character.AwardResolved(user.UserId, true,
			user.Character.CandidateFor(string(skills.Skullduggery)))
	}

	return true, nil
}

package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/refusal"
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
		refusal.WhileBusy(user, "sneak")
		return true, nil
	}

	cfg := configs.GetBalanceConfig()
	sneakCooldownKey := skills.Skullduggery.String(`sneak`)

	// Check cooldown — only block if on cooldown from a prior failure
	if !user.Character.CooldownReady(sneakCooldownKey) {
		refusal.Cooldown(user, "sneak")
		return true, nil
	}

	result := actions.Sneak(&actions.UserActor{User: user, Room: room})
	if result.Cost.Status == characters.CostRefused {
		refusal.Cost(user, result.Cost)
		return true, nil
	}

	switch {
	case result.AlreadyHidden:
		user.SendText(messaging.CategoryRefusal, "You're already hidden!")
		// The goal holds: the player is hidden (as warcry and trip do).
		notifySneakQuest(user, room)
		return true, nil

	case result.InCombat:
		refusal.WhileFighting(user, "sneak")
		return true, nil

	case result.ReplacedShroud:
		// Already hidden by a weaker Empathic Shroud: the sneak took the hide
		// over with no roll (#444), so there is nothing to practise.
		user.SendText(messaging.CategorySystem, `You let the shroud fall away and slip into the shadows on your own.`)
		notifySneakQuest(user, room)
		return true, nil

	case result.SpottedBy != nil:
		// Apply failure cooldown so the player can't spam sneak
		if cfg.SneakFailCooldown > 0 {
			user.Character.TryCooldown(sneakCooldownKey,
				fmt.Sprintf(`%d rounds`, cfg.SneakFailCooldown))
		}
		// The spotter is named only as far as the sneaker can see (#215).
		// The spotter acts on the sneaker, so the line rides Emote or
		// MobEmote by the spotter's kind, as a sneaking arrival's spotted
		// line does (actions.EntryDetection; M7 slice 4, rule N2).
		user.SendText(messaging.EmoteCategoryFor(result.SpottedBy.GetUserId() > 0),
			actions.SpottedLine(user.Character, room, result.SpottedBy))

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

	notifySneakQuest(user, room)
	return true, nil
}

// notifySneakQuest tells the quest engine the player is now hidden, on the
// `command` success event. First Shot's (quest 50) sneak step listens for it;
// before 2026-10-17 sneak sent no command event at all, so that step could not
// be finished. Sent only when the player ends up hidden: a refusal or a
// spotted sneak sends nothing. The skill_use event AwardResolved emits is a
// different event and does not reach a `command` trigger.
func notifySneakQuest(user *users.UserRecord, room *rooms.Room) {
	bridge := questengine.NewGameBridge(user, room.RoomId)
	questengine.GetEngine().Notify("command", questengine.EventDetails{
		UserId:  user.UserId,
		RoomId:  room.RoomId,
		Command: "sneak",
	}, bridge, bridge)
}

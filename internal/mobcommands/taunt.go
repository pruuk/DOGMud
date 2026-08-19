package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

var executeTauntAction = actions.ExecuteTaunt

// Taunt is a generic conviction attack with non-wolf flavor text. Used by
// tank archetypes (golems, elementals) where wolf-themed "howl" messaging
// would be incongruous. Mechanically identical to Howl; purely a flavor wrapper.
func Taunt(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	if !mob.Character.IsInCombat() {
		return true, nil
	}

	actor := &actions.MobActor{Mob: mob, Room: room}
	result := executeTauntAction(actor)
	if result.Cost.Status == characters.CostRefused {
		return true, nil
	}

	if !result.Executed {
		return true, nil
	}

	targetName := result.Target.Name

	var targetPlayer *users.UserRecord
	if result.Target.UserId > 0 {
		targetPlayer = users.GetByUserId(result.Target.UserId)
	}
	targetIdentity := targetName
	if targetPlayer != nil {
		targetIdentity = targetPlayer.Character.GetPlayerName(targetPlayer.UserId).String()
	} else if targetMob := mobs.GetInstance(result.Target.MobInstanceId); targetMob != nil {
		targetIdentity = targetMob.Character.GetMobNameIndexed(0,
			room.GetMobDuplicateIndex(targetMob.InstanceId)).String()
	}

	switch {
	case result.Fumble:
		sendAudioRoomText(room, mob, messaging.CategoryTauntFailure,
			`Something bellows a challenge that breaks into a strangled gasp.`,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> bellows a challenge that breaks into a strangled gasp.`, mob.Character.Name))

	case result.Hit:
		if !result.Defence.Defended {
			if targetPlayer != nil {
				if canSeeInDark(targetPlayer, room) {
					targetPlayer.SendText(messaging.CategoryTauntSuccess, fmt.Sprintf(`<ansi fg="mobname">%s</ansi>'s thunderous challenge rattles your nerve! (<ansi fg="damage">%s</ansi>)`, mob.Character.Name, result.DmgDesc))
				} else {
					targetPlayer.SendText(messaging.CategoryTauntSuccess, fmt.Sprintf(`A thunderous challenge rattles your nerve! (<ansi fg="damage">%s</ansi>)`, result.DmgDesc))
				}
			}
			sendAudioRoomText(room, mob, messaging.CategoryTauntSuccess,
				fmt.Sprintf(`Something bellows a thunderous challenge at <ansi fg="username">%s</ansi>!`, targetName),
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> bellows a thunderous challenge at <ansi fg="username">%s</ansi>!`, mob.Character.Name, targetName))
		}
		sendChannelDefenceMessages(result.Defence, mob, targetPlayer, room, targetIdentity, "taunt")

		// Aggro-pull confirmation: the taunt yanked the target off its prior
		// foe and pinned it (taunt-hold). AggroPulled is only ever set when the
		// target is a mob, so the name colors as a mobname.
		if result.AggroPulled {
			sendAudioRoomText(room, mob, messaging.CategoryTauntSuccess,
				`Something wheels around, drawn to a new challenger.`,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> wheels around and locks onto <ansi fg="mobname">%s</ansi>!`, targetName, mob.Character.Name))
		}

	default: // miss
		if targetPlayer != nil {
			if canSeeInDark(targetPlayer, room) {
				targetPlayer.SendText(messaging.CategoryTauntResist, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> bellows a challenge, but you brush it off.`, mob.Character.Name))
			} else {
				targetPlayer.SendText(messaging.CategoryTauntResist, `Something bellows a challenge, but you brush it off.`)
			}
		}
		sendAudioRoomText(room, mob, messaging.CategoryTauntResist,
			fmt.Sprintf(`Something bellows a challenge at <ansi fg="username">%s</ansi>, but they shrug it off.`, targetName),
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> bellows a challenge at <ansi fg="username">%s</ansi>, but they shrug it off.`, mob.Character.Name, targetName))
	}

	return true, nil
}

func sendChannelDefenceMessages(out combat.ChannelDefenceResult, mob *mobs.Mob,
	defender *users.UserRecord, room *rooms.Room, defenderName, attack string) {
	if defender != nil {
		if text := combat.ChannelDefenceShortageText(out, defender.Character); text != "" {
			defender.SendText(messaging.CategorySystem, text)
		}
	}
	attackerName := mob.Character.GetMobNameIndexed(0, room.GetMobDuplicateIndex(mob.InstanceId)).String()
	defenderIdentity := defenderName
	if defender != nil {
		defenderIdentity = defender.Character.GetPlayerName(defender.UserId).String()
	}
	triad := combat.RenderChannelDefenceMessages(out, combat.ChannelDefenceIdentities{
		Attacker: attackerName,
		Defender: defenderIdentity,
	}, attack)
	if triad.ToRoom == "" {
		return
	}
	excluded := make([]int, 0, 1)
	if defender != nil {
		personal := string(triad.ToDefender)
		if !canSeeInDark(defender, room) {
			personal = messaging.Anonymize(personal)
		}
		defender.SendText(messaging.CategoryTauntResist, personal)
		excluded = append(excluded, defender.UserId)
	}
	visible := string(triad.ToRoom)
	unseen := messaging.Anonymize(visible)
	sendAudioRoomText(room, mob, messaging.CategoryTauntResist, unseen, visible, excluded...)
}

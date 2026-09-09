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
		if !sendMobTauntTriad(combat.TauntFumble, "", messaging.CategoryTauntFailure,
			mob, targetName, targetPlayer, room) {
			sendAudioRoomText(room, mob, messaging.CategoryTauntFailure,
				`Something bellows a challenge that breaks into a strangled gasp.`,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> bellows a challenge that breaks into a strangled gasp.`, mob.Character.Name))
		}

	case result.Hit:
		if !result.Defence.Defended {
			// The critical band exists in the authored data and the mob side
			// never reached it: this switch used to treat every landed taunt as
			// an ordinary hit, so a mob's crit read exactly like its worst jab.
			intensity := combat.TauntHit
			if result.Crit {
				intensity = combat.TauntCritical
			}
			if !sendMobTauntTriad(intensity, result.DmgDesc, messaging.CategoryTauntSuccess,
				mob, targetName, targetPlayer, room) {
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
		if !sendMobTauntTriad(combat.TauntMiss, "", messaging.CategoryTauntResist,
			mob, targetName, targetPlayer, room) {
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
	}

	return true, nil
}

// sendMobTauntTriad narrates one mob taunt from the AUTHORED store to every
// audience that can receive it, and reports whether it said anything.
//
// Until 2026-09-09 the mob side never touched taunt-messages at all: it
// hand-rolled its own "bellows a thunderous challenge" text while the player
// side read rhetoric.yaml. One event, two narration sources, drifting apart
// with nobody to notice. Routing it here lights up the todefender pool, which
// no player could previously ever see: a taunted MOB has no client, and PVP
// ships disabled so a player could never be the target of a player taunt.
//
// Returns false when the store holds nothing, so the caller can fall back to
// the legacy literals. That path is live in any context that never loaded world
// data, which includes unit tests.
//
// ⚠️ DARKNESS IS HAND-ROLLED HERE, and has to be. sendAudioRoomText delivers on
// the AUDIO channel, which messaging's pipeline never sight-gates and never
// anonymizes, so the unseen variant is built explicitly with messaging.Anonymize
// rather than inherited. That is also why the {sourcetype} and {targettype}
// tokens must resolve to real name aliases: Anonymize matches on
// username|mobname|petname, and a tag outside that set leaks the name.
func sendMobTauntTriad(intensity combat.TauntIntensity, dmgDesc string, cat messaging.Category,
	mob *mobs.Mob, targetName string, targetPlayer *users.UserRecord, room *rooms.Room) bool {

	targetType := "mobname"
	if targetPlayer != nil {
		targetType = "username"
	}

	triad := combat.GetTauntTriad(intensity, mob.Character.Name, targetName,
		"mobname", targetType, dmgDesc)

	if triad.ToRoom == "" {
		return false
	}

	// ToAttacker is deliberately dropped: the actor is a mob and has no client.
	excluded := make([]int, 0, 1)
	if targetPlayer != nil && triad.ToDefender != "" {
		personal := triad.ToDefender
		if !canSeeInDark(targetPlayer, room) {
			personal = messaging.Anonymize(personal)
		}
		targetPlayer.SendText(cat, personal)
		excluded = append(excluded, targetPlayer.UserId)
	}

	sendAudioRoomText(room, mob, cat, messaging.Anonymize(triad.ToRoom), triad.ToRoom, excluded...)
	return true
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

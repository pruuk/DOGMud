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

	return narrateMobTaunt(mob, room, result, mob.Character.TauntPool(), "taunt", tauntLegacy, tauntAggroLine)
}

// mobTauntLegacy holds the fallback literals a mob taunt or howl sends when
// the taunt store is empty (unit tests that never load world data). Every
// field is a Sprintf format; the argument order is fixed per field.
type mobTauntLegacy struct {
	// Fumble: room line. Args: mob name.
	Fumble string
	// HitPersonal: line to a player target. Args: mob name, damage text.
	HitPersonal string
	// HitRoom: room line. Args: mob name, target name.
	HitRoom string
	// MissPersonal: line to a player target. Args: mob name.
	MissPersonal string
	// MissRoom: room line. Args: mob name, target name.
	MissRoom string
}

var tauntLegacy = mobTauntLegacy{
	Fumble:       `<ansi fg="mobname">%s</ansi> bellows a challenge that breaks into a strangled gasp.`,
	HitPersonal:  `<ansi fg="mobname">%s</ansi>'s thunderous challenge rattles your nerve! (<ansi fg="damage">%s</ansi>)`,
	HitRoom:      `<ansi fg="mobname">%s</ansi> bellows a thunderous challenge at <ansi fg="username">%s</ansi>!`,
	MissPersonal: `<ansi fg="mobname">%s</ansi> bellows a challenge, but you brush it off.`,
	MissRoom:     `<ansi fg="mobname">%s</ansi> bellows a challenge at <ansi fg="username">%s</ansi>, but they shrug it off.`,
}

// Aggro-pull room lines. Args: the pulled target's name, the mob's name.
const (
	tauntAggroLine = `<ansi fg="mobname">%s</ansi> wheels around and locks onto <ansi fg="mobname">%s</ansi>!`
	howlAggroLine  = `<ansi fg="mobname">%s</ansi> turns from its prey and snarls at <ansi fg="mobname">%s</ansi>!`
)

// narrateMobTaunt is the shared narration flow of a mob taunt and a mob howl: the
// same conviction attack, narrated from pool (combat.TauntPoolRhetoric,
// combat.TauntPoolWordless or combat.TauntPoolHowl). legacy holds the fallback
// literals used only when the store is empty. aggroLine is the room line, as a Sprintf format, for an
// aggro pull. command ("taunt" or "howl") is what the channel-defence
// messages call the attack.
//
// The callers (Taunt, Howl) each run executeTauntAction and its refusal checks
// themselves, so the admission guards in command_readiness_drift_test.go and
// predator_test.go keep seeing the action seam in the wrapper; this function
// only narrates an Executed result.
func narrateMobTaunt(mob *mobs.Mob, room *rooms.Room, result actions.TauntResult, pool, command string, legacy mobTauntLegacy, aggroLine string) (bool, error) {
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
		if !sendMobTauntTriad(pool, combat.TauntFumble, "", "", messaging.CategoryTauntFailure,
			mob, targetName, targetPlayer, room) {
			room.SendTextHidingNames(messaging.CategoryTauntFailure,
				fmt.Sprintf(legacy.Fumble, mob.Character.Name),
				[]string{mob.Character.Name}, messaging.HideNames)
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
			if !sendMobTauntTriad(pool, intensity, result.DmgDesc, result.DmgDescToTarget, messaging.CategoryTauntSuccess,
				mob, targetName, targetPlayer, room) {
				if targetPlayer != nil {
					personalText := fmt.Sprintf(legacy.HitPersonal, mob.Character.Name, result.DmgDescToTarget)
					sight := messaging.ParticipantSight(targetPlayer.Character, room)
					targetPlayer.SendText(messaging.CategoryTauntSuccess, messaging.HideNames(personalText, []string{mob.Character.Name}, sight))
				}
				room.SendTextHidingNames(messaging.CategoryTauntSuccess,
					fmt.Sprintf(legacy.HitRoom, mob.Character.Name, targetName),
					[]string{mob.Character.Name, targetName}, messaging.HideNames)
			}
		}
		sendChannelDefenceMessages(result.Defence, mob, targetPlayer, room, targetIdentity, targetName, command)

		// Aggro-pull confirmation: the taunt yanked the target off its prior
		// foe and pinned it (taunt-hold). AggroPulled is only ever set when the
		// target is a mob, so the name colors as a mobname.
		if result.AggroPulled {
			room.SendTextHidingNames(messaging.CategoryTauntSuccess,
				fmt.Sprintf(aggroLine, targetName, mob.Character.Name),
				[]string{mob.Character.Name, targetName}, messaging.HideNames)
		}

	default: // miss
		if !sendMobTauntTriad(pool, combat.TauntMiss, "", "", messaging.CategoryTauntResist,
			mob, targetName, targetPlayer, room) {
			if targetPlayer != nil {
				personalText := fmt.Sprintf(legacy.MissPersonal, mob.Character.Name)
				sight := messaging.ParticipantSight(targetPlayer.Character, room)
				targetPlayer.SendText(messaging.CategoryTauntResist, messaging.HideNames(personalText, []string{mob.Character.Name}, sight))
			}
			room.SendTextHidingNames(messaging.CategoryTauntResist,
				fmt.Sprintf(legacy.MissRoom, mob.Character.Name, targetName),
				[]string{mob.Character.Name, targetName}, messaging.HideNames)
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
// ⚠️ DARKNESS IS HAND-ROLLED HERE, and has to be. The AUDIO channel never runs
// through messaging's pipeline sight gate, so both the personal line (via
// messaging.ParticipantSight + messaging.HideNames) and the room line (via
// rooms.Room.SendTextHidingNames) hide names explicitly rather than inheriting
// it. That is also why the {actortype} and {acteetype} tokens must resolve to
// real name aliases: HideNames' tag-consuming match looks for
// username|mobname|petname, and a tag outside that set leaks the name.
func sendMobTauntTriad(pool string, intensity combat.TauntIntensity, dmgDesc, targetDmgDesc string, cat messaging.Category,
	mob *mobs.Mob, targetName string, targetPlayer *users.UserRecord, room *rooms.Room) bool {

	targetType := "mobname"
	if targetPlayer != nil {
		targetType = "username"
	}

	triad := combat.GetTauntTriadFrom(pool, intensity, mob.Character.Name, targetName,
		"mobname", targetType, dmgDesc).WithDefenderDamage(dmgDesc, targetDmgDesc)

	if triad.ToRoom == "" {
		return false
	}

	// ToAttacker is deliberately dropped: the actor is a mob and has no client.
	excluded := make([]int, 0, 1)
	if targetPlayer != nil && triad.ToDefender != "" {
		sight := messaging.ParticipantSight(targetPlayer.Character, room)
		personal := messaging.HideNames(triad.ToDefender, []string{mob.Character.Name}, sight)
		targetPlayer.SendText(cat, personal)
		excluded = append(excluded, targetPlayer.UserId)
	}

	room.SendTextHidingNames(cat, triad.ToRoom, []string{mob.Character.Name, targetName}, messaging.HideNames, excluded...)
	return true
}

// defenderPlainName is the defender's BARE name (no ansi tag, no duplicate
// suffix) -- callers already compute one alongside the rendered identity
// they pass as defenderName (howl.go/taunt.go's targetName), needed here
// because messaging.HideNames matches on the bare name, not on the fully
// rendered identity string this function builds the triad's text from.
func sendChannelDefenceMessages(out combat.ChannelDefenceResult, mob *mobs.Mob,
	defender *users.UserRecord, room *rooms.Room, defenderName, defenderPlainName, attack string) {
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
		sight := messaging.ParticipantSight(defender.Character, room)
		personal := messaging.HideNames(string(triad.ToDefender), []string{mob.Character.Name}, sight)
		defender.SendText(messaging.CategoryTauntResist, personal)
		excluded = append(excluded, defender.UserId)
	}
	visible := string(triad.ToRoom)
	room.SendTextHidingNames(messaging.CategoryTauntResist, visible,
		[]string{mob.Character.Name, defenderPlainName}, messaging.HideNames, excluded...)
}

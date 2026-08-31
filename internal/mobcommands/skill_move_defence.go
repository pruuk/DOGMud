package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// sendMoveDefenceTriad speaks the channel defence triad for a DEFENDED special
// move made by a mob (bash/kick/trip and the beast moves), naming the defence
// that actually stopped it — U6b Task 9. Mirrors sendChannelDefenceMessages
// (the taunt sender in this package), including darkness-aware anonymization.
//
// roomOnly follows melee's partial convention: when the defended move still
// dealt partial damage, the caller's composite personal lines carry the damage
// description, so only the room line comes from the triad. When the defence
// fully stopped the move (a defensive crit), all three lines come from the
// triad (the mob attacker itself receives no text).
//
// Returns false when there is nothing to speak: the outcome was not a defence.
// The caller then falls back to its plain miss text.
//
// It no longer returns false for a MISSING POOL. As of 2026-08-31,
// combat.RenderChannelDefenceMessages substitutes generic narration when a pool
// cannot be resolved, so a defended outcome always speaks. That second reason
// used to be live and is now dead; do not reintroduce a pool check here.
func sendMoveDefenceTriad(mob *mobs.Mob, room *rooms.Room, target actions.AggroTarget,
	out combat.ChannelDefenceResult, attack string, category messaging.Category, roomOnly bool) bool {

	var targetUser *users.UserRecord
	if target.UserId > 0 {
		targetUser = users.GetByUserId(target.UserId)
	}

	identities := combat.ChannelDefenceIdentities{
		Attacker: mob.Character.GetMobNameIndexed(0, room.GetMobDuplicateIndex(mob.InstanceId)).String(),
		Defender: target.Name,
	}
	if targetUser != nil {
		identities.Defender = targetUser.Character.GetPlayerName(targetUser.UserId).String()
	}

	triad := combat.RenderChannelDefenceMessages(out, identities, attack)
	if triad.ToRoom == "" {
		return false
	}

	if targetUser != nil {
		if text := combat.ChannelDefenceShortageText(out, targetUser.Character); text != "" {
			targetUser.SendText(messaging.CategorySystem, text)
		}
	}

	excluded := make([]int, 0, 1)
	if targetUser != nil {
		if !roomOnly {
			personal := string(triad.ToDefender)
			if !canSeeInDark(targetUser, room) {
				personal = messaging.Anonymize(personal)
			}
			targetUser.SendText(category, personal)
		}
		excluded = append(excluded, targetUser.UserId)
	}
	visible := string(triad.ToRoom)
	sendAudioRoomText(room, mob, category, messaging.Anonymize(visible), visible, excluded...)
	return true
}

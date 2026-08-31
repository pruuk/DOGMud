package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// sendMoveDefenceTriad speaks the channel defence triad for a DEFENDED special
// move (bash/kick/trip and the beast moves), naming the defence that actually
// stopped it — U6b Task 9. Mirrors the taunt/spell senders in this package and
// in internal/hooks.
//
// roomOnly follows melee's partial convention (sendDefenseMessages): when the
// defended move still dealt partial damage, the caller's composite personal
// lines carry the damage description, so only the room line comes from the
// triad — room lines never carry damage, so they stay coherent. When the
// defence fully stopped the move (a defensive crit), roomOnly is false and all
// three lines come from the triad.
//
// Returns false when there is nothing to speak: the outcome was not a defence
// (e.g. a fumbled swing that had actually won its roll). The caller then falls
// back to its plain miss text.
//
// It no longer returns false for a MISSING POOL. As of 2026-08-31,
// combat.RenderChannelDefenceMessages substitutes generic narration when a pool
// cannot be resolved, so a defended outcome always speaks. That second reason
// used to be live and is now dead; do not reintroduce a pool check here.
func sendMoveDefenceTriad(user *users.UserRecord, room *rooms.Room, target actions.AggroTarget,
	out combat.ChannelDefenceResult, attack string, category messaging.Category, roomOnly bool) bool {

	var targetUser *users.UserRecord
	if target.UserId > 0 {
		targetUser = users.GetByUserId(target.UserId)
	}

	identities := combat.ChannelDefenceIdentities{
		Attacker: user.Character.GetPlayerName(user.UserId).String(),
		Defender: target.Name,
	}
	if targetUser != nil {
		identities.Defender = targetUser.Character.GetPlayerName(user.UserId).String()
	} else if targetMob := mobs.GetInstance(target.MobInstanceId); targetMob != nil {
		identities.Defender = targetMob.Character.GetMobNameIndexed(user.UserId,
			room.GetMobDuplicateIndex(targetMob.InstanceId)).String()
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

	if !roomOnly {
		user.SendText(category, string(triad.ToAttacker))
		if targetUser != nil {
			targetUser.SendText(category, string(triad.ToDefender))
		}
	}
	room.SendTextVisual(category, string(triad.ToRoom), user.UserId, target.UserId)
	return true
}

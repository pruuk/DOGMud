package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

/*
Skullduggery Skill
Level 3 - Shadow: follow a target between rooms while remaining hidden.
When the target moves through an exit, the shadower moves with them
(hooks.RoomChangeShadowFollow). On each arrival the target may sense the
shadower (actions.ShadowSenseRoll). The shadow ends if the shadower arrives
no longer hidden, or on "shadow stop" (actions.EndShadow).
*/
func Shadow(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	skillLevel := user.Character.GetSkillLevel(skills.Skullduggery)

	// Requires skullduggery rank 3
	if skillLevel < 1 {
		return false, nil
	}
	if skillLevel < 3 {
		user.SendText(messaging.CategorySystem, "You aren't advanced enough at skullduggery for that.")
		return true, nil
	}

	rest = strings.TrimSpace(rest)

	// "shadow stop" cancels an active shadow
	if strings.ToLower(rest) == "stop" {
		if targetUserId, targetMobId := actions.ShadowTargetOf(user.Character); targetUserId != 0 || targetMobId != 0 {
			actions.EndShadow(actions.NewUserActorInRoom(user, room), "You stop shadowing your target.")
		} else {
			user.SendText(messaging.CategorySystem, "You aren't shadowing anyone.")
		}
		return true, nil
	}

	if rest == "" {
		user.SendText(messaging.CategorySystem, "Shadow whom?")
		return true, nil
	}

	// #454: a typed name resolves only at full sight.
	name, refusal := actions.AimBySight(user.Character, user.UserId, room, strings.ToLower(rest), `shadow`)
	if refusal != `` {
		user.SendText(messaging.CategorySystem, refusal)
		return true, nil
	}
	rest = name

	// Resolve target in the current room, excluding the player themselves.
	target, err := actions.ResolveTargetActor(room, strings.ToLower(rest), actions.ResolveTargetOptions{
		ExcludeUserId: user.UserId,
		Viewer:        user.Character,
	})
	if err != nil {
		// Check whether the name matched the player themselves.
		if pId, _ := room.FindByName(strings.ToLower(rest)); pId == user.UserId {
			user.SendText(messaging.CategorySystem, "You can't shadow yourself.")
			return true, nil
		}
		user.SendText(messaging.CategorySystem, "Shadow whom?")
		return true, nil
	}

	opts := actions.ShadowOptions{}
	if target.IsPlayer() {
		opts.TargetUserId = target.GetUserId()
	} else {
		opts.TargetMobInstanceId = target.GetMobInstanceId()
	}

	actor := actions.UserActorAtSight(user, room)
	result := actions.Shadow(actor, opts)

	if result.OnCooldown {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			"You need to wait %s before shadowing again.",
			result.Reason))
	}

	return true, nil
}

package usercommands

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Talk opens a conversation: the NPC's opener as speech, then the private
// topic menu (#375). The flow lives in internal/conversation.
func Talk(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	args := util.SplitButRespectQuotes(rest)

	if len(args) == 0 {
		refusal.Prompt(user, "talk to", refusal.Whom, "talk <name>")
		return true, nil
	}

	// Support "talk to <npc>" as well as "talk <npc>"
	if strings.ToLower(args[0]) == `to` {
		if len(args) < 2 {
			refusal.Prompt(user, "talk to", refusal.Whom, "talk <name>")
			return true, nil
		}
		args = args[1:]
	}

	// A conversation needs the NPC seen by name, so at shapes no aim can
	// succeed; say what is missing instead of hinting at `talk figure`.
	if conversation.RefusedAtShapes(user, room) {
		return true, nil
	}

	// #454, owner call 2: a typed name resolves only at full sight, or
	// talking would confirm who is there in the dark.
	searchName, aimRefusal := actions.AimBySight(user.Character, user.UserId, room, args[0], `talk`)
	if aimRefusal != `` {
		user.SendText(messaging.CategoryRefusal, aimRefusal)
		return true, nil
	}

	target, err := actions.ResolveTargetActor(room, searchName, actions.ResolveTargetOptions{Viewer: user.Character})
	if err != nil {
		refusal.NotHereCreature(user)
		return true, nil
	}
	if target.IsPlayer() {
		refusal.TalkToPlayer(user)
		return true, nil
	}
	mob := target.(*actions.MobActor).Mob

	// A sleeping NPC cannot hold a conversation, and must not grant a quest
	// (moved from ask, #375).
	if actions.RefuseMobIfAsleep(mob, user) {
		return true, nil
	}
	conversation.Talk(user, room, mob)
	return true, nil
}

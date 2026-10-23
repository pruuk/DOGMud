package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// askUsage names ask's one remaining use: chat with a bonded companion (R13).
const askUsage = "ask <companion> <words>"

// Ask is chat with the player's own bonded companion (R13). Conversations
// with anyone else happen through talk (#375 spec 4 item 7): ask on any
// other NPC prints one private redirect and does nothing else.
func Ask(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	args := util.SplitButRespectQuotes(rest)

	if len(args) == 0 {

		for _, mId := range room.GetMobs(rooms.FindCharmed) {
			mob := mobs.GetInstance(mId)
			if mob == nil {
				continue
			}
			if mob.Character.IsCharmed(user.UserId) {
				mob.Command(`emote regards you blankly. It doesn't seem to understand.`)
				return true, nil
			}
		}

		refusal.Prompt(user, "ask", refusal.What, askUsage)
		return true, nil
	}

	// #454, owner call 2: a typed name resolves only at full sight, or
	// asking would confirm who is there in the dark.
	searchName, aimRefusal := actions.AimBySight(user.Character, user.UserId, room, args[0], `ask`)
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
		user.SendText(messaging.CategoryRefusal, `You can't ask another player.`)
		return true, nil
	}

	mob := target.(*actions.MobActor).Mob
	typedName := args[0]

	// A companion is the player's own charmed follower or any bonded AI
	// companion (the module answers its owner and others in the room). Every
	// other NPC is a talk conversation.
	if !mob.Character.IsCharmed(user.UserId) && !companionai.IsBondedCompanion(mob.InstanceId) {
		redirectToTalk(user, typedName)
		return true, nil
	}

	// A sleeping companion cannot chat.
	if actions.RefuseMobIfAsleep(mob, user) {
		return true, nil
	}

	args = args[1:]

	// players may type "ask <companion> to <do something>"
	if len(args) > 1 && strings.ToLower(args[0]) == `to` {
		args = args[1:]
	}
	if len(args) > 1 && strings.ToLower(args[0]) == `about` {
		args = args[1:]
	}

	if len(args) == 0 {
		refusal.Prompt(user, "ask", refusal.What, askUsage)
		return true, nil
	}

	// A bonded AI companion converses. The aicompanion module claims the ask
	// when it drives this mob.
	if companionai.RouteAsk(user.UserId, mob.InstanceId, strings.Join(args, ` `)) {
		return true, nil
	}

	// Companions can't be ordered around: they act on their own.
	if mob.Character.IsCharmed(user.UserId) {
		mob.Command(`emote regards you blankly. It doesn't seem to understand.`)
		return true, nil
	}
	// Someone else's bonded companion the module did not claim.
	redirectToTalk(user, typedName)
	return true, nil
}

// redirectToTalk is ask's one answer for an NPC conversation (#375 spec 4
// item 7): private, and nothing reaches the room. typedName is the name as
// the player typed it (the resolved search name can be an internal "#id").
func redirectToTalk(user *users.UserRecord, typedName string) {
	user.SendText(messaging.CategoryRefusal, fmt.Sprintf(
		`Conversations happen through <ansi fg="command">talk</ansi> now. Try <ansi fg="command">talk %s</ansi>.`,
		util.EscapeAnsiTags(strings.ToLower(typedName))))
}

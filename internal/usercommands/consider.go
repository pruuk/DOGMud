package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Consider(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	args := util.SplitButRespectQuotes(rest)
	if len(args) == 0 {
		user.SendText(messaging.CategorySystem, "Consider whom? (try consider <target>)")
		return true, nil
	}

	// Resolve against the WHOLE argument, not args[0]. Truncating to the first
	// word is invisible when that word is distinctive ("bandit" still
	// prefix-matches "Bandit Scout") but silently considers the wrong creature
	// when two share it -- `consider bandit archer` reported on the Bandit
	// Scout. room.FindByName handles multi-word input, and look already passes
	// its full argument; this brings consider into line.
	// #454: a typed name resolves only at full sight.
	name, refusal := actions.AimBySight(user.Character, user.UserId, room, strings.Join(args, " "), `consider`)
	if refusal != `` {
		user.SendText(messaging.CategorySystem, refusal)
		return true, nil
	}
	target, err := actions.ResolveTargetActor(room, name,
		actions.ResolveTargetOptions{ExcludeUserId: user.UserId, Viewer: user.Character})
	if err != nil {
		// Always give feedback for an unresolved target — dead, absent, or no
		// match — instead of silently no-oping (which left the player unsure
		// whether the command even registered).
		user.SendText(messaging.CategorySystem, "You don't see them here.")
		return true, nil
	}

	// A mob the player may not harm is not a fight to weigh up: attack
	// refuses it through the same check, so consider says so instead of
	// rating the odds (#286). The quest notification below still fires.
	if m := mobs.GetInstance(target.GetMobInstanceId()); mobs.CheckPlayerHarm(m).Blocked() {
		user.SendText(messaging.CategorySystem, messaging.HideNames(
			fmt.Sprintf(`You can't fight <ansi fg="mobname">%s</ansi>.`, m.Character.Name),
			[]string{m.Character.Name}, messaging.ParticipantSight(user.Character, room)))
	} else {
		actor := actions.UserActorAtSight(user, room)
		actions.Consider(actor, target)
	}

	// Quest engine: command notification
	bridge := questengine.NewGameBridge(user, room.RoomId)
	questengine.GetEngine().Notify("command", questengine.EventDetails{
		UserId:  user.UserId,
		RoomId:  room.RoomId,
		Command: "consider",
	}, bridge, bridge)

	return true, nil
}

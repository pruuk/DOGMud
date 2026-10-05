package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gather"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Mining (wilderness trades): `prospect` and `mine`.

// Prospect handles `prospect`: read the ore in this room.
func Prospect(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !messaging.CanSeeClearly(user.Character, room) {
		user.SendText(messaging.CategorySystem, `It's too dark to read the rock.`)
		return true, nil
	}
	actions.Prospect(&actions.UserActor{User: user, Room: room})
	return true, nil
}

// Mine handles `mine` / `mine <ore>`: work one load out of the room's vein.
func Mine(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !user.Character.IsFree() {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You're already busy working on something.</ansi>`)
		return true, nil
	}
	if !messaging.CanSeeClearly(user.Character, room) {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">It's too dark to swing a pick safely here.</ansi>`)
		return true, nil
	}
	v, ore, ok := actions.RoomVein(room, util.GetRoundCount())
	if !ok || ore == nil {
		user.SendText(messaging.CategorySystem, `There is no ore worth digging here. Look in caves, on mountainsides and along cliffs.`)
		return true, nil
	}
	if want := strings.ToLower(strings.TrimSpace(rest)); want != `` && !mineWordMatches(want, ore.Name) {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`There's no %s here. Type <ansi fg="command">prospect</ansi> to see what the rock carries, or just <ansi fg="command">mine</ansi>.`, want))
		return true, nil
	}
	if v.Stock <= 0 {
		user.SendText(messaging.CategorySystem, `The seam here is worked out. Type <ansi fg="command">prospect</ansi> to see when it will be worth digging again, or try the rock next door.`)
		return true, nil
	}
	pick, hasPick := gather.BestTool(user.Character, items.ToolPick)
	if !hasPick {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You need a pick to mine.</ansi> Smiths and the Pothole Coulee mine foreman sell rough ones. (<ansi fg="command">help tools</ansi>)`)
		return true, nil
	}
	if pick.Tier < items.ToolTier(ore.MinPick) {
		if actions.OreKnown(&actions.UserActor{User: user, Room: room}, ore) {
			user.SendText(messaging.CategorySystem, `<ansi fg="red">`+actions.PickTooPoor(ore)+`</ansi>`)
		} else {
			user.SendText(messaging.CategorySystem, `<ansi fg="red">This rock is too hard for your pick: it only rings off it. You need a better pick.</ansi>`)
		}
		return true, nil
	}

	rounds := gather.Rounds(actions.MineRounds(ore.Tier), pick, true)
	if err := user.Character.Activity.TransitionToSalvaging(
		activity.SalvagingData{
			ItemUuid:    fmt.Sprintf(`%s%d`, actions.MineActivityPrefix, room.RoomId),
			RoundsTotal: rounds,
			RoomId:      room.RoomId,
		},
		state.TransitionReason{
			Trigger: activity.TriggerSalvageBegin,
			Actor:   state.ActorRef{UserId: user.UserId},
		},
	); err != nil {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You're already busy working on something.</ansi>`)
		return true, nil
	}
	user.SendText(messaging.CategorySystem, fmt.Sprintf(
		`<ansi fg="yellow">You pick your spot on the rock face and set to work with your <ansi fg="itemname">%s</ansi>...</ansi>`,
		pick.Item.DisplayName()))
	return true, nil
}

// mineWordMatches accepts the ore's name or any generic word for ore.
func mineWordMatches(word, ore string) bool {
	switch word {
	case `ore`, `rock`, `vein`, `seam`, `stone`:
		return true
	}
	return word == ore || strings.HasPrefix(ore, word) || strings.Contains(ore, word)
}

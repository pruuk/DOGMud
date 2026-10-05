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

// Lumberjacking (wilderness trades, phase 4): `survey trees` and `chop`.

// Survey handles `survey` / `survey trees`.
func Survey(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !messaging.CanSeeClearly(user.Character, room) {
		user.SendText(messaging.CategorySystem, `It's too dark to make out the trees.`)
		return true, nil
	}
	actions.SurveyTrees(&actions.UserActor{User: user, Room: room})
	return true, nil
}

// Chop handles `chop` / `chop <tree>`: fell one tree of the room's stand.
func Chop(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !user.Character.IsFree() {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You're already busy working on something.</ansi>`)
		return true, nil
	}
	if !messaging.CanSeeClearly(user.Character, room) {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">It's too dark to swing an axe safely here.</ansi>`)
		return true, nil
	}
	st, sp, ok := actions.RoomStand(room, util.GetRoundCount())
	if !ok || sp == nil {
		user.SendText(messaging.CategorySystem, `There is no timber worth cutting here. Look for a forest, deep woods or a marsh.`)
		return true, nil
	}
	if want := strings.ToLower(strings.TrimSpace(rest)); want != `` && !chopWordMatches(want, sp.Name) {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`There's no %s here. Type <ansi fg="command">survey trees</ansi> to see what grows here, or just <ansi fg="command">chop</ansi>.`, want))
		return true, nil
	}
	if st.Stock <= 0 {
		user.SendText(messaging.CategorySystem, `The stand here has been cut back to stumps. Type <ansi fg="command">survey trees</ansi> to see when it will be ready again, or try the woods next door.`)
		return true, nil
	}
	axe, hasAxe := gather.BestTool(user.Character, items.ToolAxe)
	if !hasAxe {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You need an axe to fell a tree.</ansi> A hatchet will do, but a woodcutter's axe is far better. (<ansi fg="command">help tools</ansi>)`)
		return true, nil
	}

	if axe.Tier < items.ToolTier(sp.MinAxe()) {
		if actions.SpeciesKnown(&actions.UserActor{User: user, Room: room}, sp) {
			user.SendText(messaging.CategorySystem, `<ansi fg="red">`+actions.AxeTooPoor(sp)+`</ansi>`)
		} else {
			user.SendText(messaging.CategorySystem, `<ansi fg="red">This wood is too hard for your axe: it would only chip the edge. You need a better axe.</ansi>`)
		}
		return true, nil
	}

	rounds := gather.Rounds(actions.ChopRounds(sp.Tier), axe, true)
	if err := user.Character.Activity.TransitionToSalvaging(
		activity.SalvagingData{
			ItemUuid:    fmt.Sprintf(`%s%d`, actions.ChopActivityPrefix, room.RoomId),
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
		`<ansi fg="yellow">You pick a tree, plant your feet and start in on it with your <ansi fg="itemname">%s</ansi>...</ansi>`,
		axe.Item.DisplayName()))
	return true, nil
}

// chopWordMatches accepts the species name or any generic word for a tree.
func chopWordMatches(word, species string) bool {
	switch word {
	case `tree`, `trees`, `wood`, `timber`, `log`, `logs`, `down`:
		return true
	}
	return word == species || strings.HasPrefix(species, word) || strings.Contains(species, word)
}

package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

/*
* Role Permissions:
* earnmutation 			(Admin)
 */

// EarnMutation is the playtest lever for #680. `earnmutation [player]` spends
// one earned mutation event through actions.EarnMutation, the code the combat
// tick runs, without waiting for progress to fill. With no player it acts on
// the admin. (Not `mutate earn`: `mutate` is a player help alias for the
// mutations page, and the admin page would be shadowed.)
func EarnMutation(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	args := util.SplitButRespectQuotes(rest)

	target := user
	if len(args) > 0 {
		if target = users.GetByCharacterName(args[0]); target == nil {
			user.SendText(messaging.CategoryRefusal, fmt.Sprintf(
				`No player named "%s" is online.`, refusal.Echo(args[0])))
			return true, nil
		}
	}

	id := actions.EarnMutation(target)
	if id == `` {
		user.SendText(messaging.CategoryAdmin, fmt.Sprintf(
			`%s earned nothing: no mutation fits. Their progress is kept.`, target.Character.Name))
		return true, nil
	}

	// EarnMutation leaves validation to the round tick's end-of-round
	// validate, which an admin command does not get. Validate derives
	// ExtraArms from the extra-arms rank.
	if err := target.Character.Validate(); err != nil {
		user.SendText(messaging.CategoryAdmin, fmt.Sprintf(
			`%s: validate after earning reported: %v`, target.Character.Name, err))
	}

	user.SendText(messaging.CategoryAdmin, fmt.Sprintf(
		`%s earned %s, now rank %d.`, target.Character.Name, id, target.Character.Mutations[id]))
	return true, nil
}

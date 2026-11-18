package usercommands

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"

	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Command queues input for a mob or player in the room, which runs it with
// its own rights. Only an admin may command a player whose role is admin.
//
// Role Permissions:
//   - command  (All)
func Command(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// args should look like one of the following:
	// target conditionId - put condition on target if in the room
	// conditionId - put condition on self
	// search searchTerm - search for condition by name, display results
	args := util.SplitButRespectQuotes(rest)

	if len(args) < 2 {
		// send some sort of help info?
		mobCommands := mobcommands.GetAllMobCommands()

		infoOutput, _ := templates.Process("admincommands/help/command.command", mobCommands, user.UserId)
		user.SendText(messaging.CategoryUsage, infoOutput)
		return true, nil
	}

	searchName := args[0]
	args = args[1:]
	cmd := strings.TrimPrefix(rest, searchName+` `)
	//cmd := strings.Join(args, ` `)

	target, err := actions.ResolveTargetActor(room, searchName)
	if err != nil {
		return true, nil
	}

	playerId := target.GetUserId()
	mobId := target.GetMobInstanceId()

	// A commanded player runs the input with their own role, so commanding
	// an admin is itself an admin's act.
	if playerId > 0 && user.Role != users.RoleAdmin {
		if targetUser := users.GetByUserId(playerId); targetUser != nil && targetUser.Role == users.RoleAdmin {
			user.SendText(messaging.CategoryAdmin, `<ansi fg="alert-4">Only an admin can command an admin.</ansi>`)
			return true, nil
		}
	}

	// Use the index for how many turns to defer the extra commands
	readyTurn := util.GetTurnCount()
	for oneCmd := range strings.SplitSeq(cmd, `;`) {
		if mobId > 0 {

			events.AddToQueue(events.Input{
				MobInstanceId: mobId,
				InputText:     oneCmd,
				ReadyTurn:     readyTurn,
			})

		} else if playerId > 0 {

			events.AddToQueue(events.Input{
				UserId:    playerId,
				InputText: oneCmd,
				ReadyTurn: readyTurn,
			})

		}
		readyTurn++
	}

	return true, nil
}

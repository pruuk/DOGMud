package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/util"

	"github.com/GoMudEngine/GoMud/internal/users"
)

// roleChangeRefusal returns why caller may not set target's role to newRole,
// or an empty string when the change may go ahead. An admin's role is never
// changed here, only an admin grants the admin role, and nobody changes
// their own role except back down to the built-in user role.
func roleChangeRefusal(caller, target *users.UserRecord, newRole string) string {
	if target.Role == users.RoleAdmin {
		return `Admin permissions cannot be removed this way.`
	}
	if target.Role == newRole {
		return `That permission is already set for this user.`
	}
	if newRole == users.RoleAdmin && caller.Role != users.RoleAdmin {
		return `Only an admin can grant the admin role.`
	}
	if target.UserId == caller.UserId && newRole != users.RoleUser {
		return `You cannot raise your own role.`
	}
	return ``
}

// Modify changes player records; today only `modify role <username> <role>`.
//
// Role Permissions:
//   - modify       (All)
//   - modify.role  (Change user roles, within the limits of roleChangeRefusal)
func Modify(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	args := util.SplitButRespectQuotes(rest)

	if len(args) < 3 {
		infoOutput, _ := templates.Process("admincommands/help/command.modify", nil, user.UserId)
		user.SendText(messaging.CategoryUsage, infoOutput)
		return true, nil
	}

	// modify permission <username> <newperms>
	if args[0] == `role` {

		if !user.HasRolePermission(`modify.role`) {
			user.SendText(messaging.CategoryAdmin, `you do not have <ansi fg="command">modify.role</ansi> permission`)
			return true, nil
		}

		searchUser := args[1]
		newRole := args[2]

		allRoles := configs.GetRolesConfig()
		_, roleExists := allRoles[newRole]

		if newRole != users.RoleAdmin && newRole != users.RoleUser && !roleExists {
			user.SendText(messaging.CategoryAdmin, `<ansi fg="alert-4">Invalid permission type.</ansi>`)
			return true, nil
		}

		foundUsername := ``
		foundCharacterName := ``

		for _, u := range users.GetAllActiveUsers() {
			if strings.EqualFold(searchUser, u.Username) {

				if why := roleChangeRefusal(user, u, newRole); why != `` {
					user.SendText(messaging.CategoryAdmin, `<ansi fg="alert-4">`+why+`</ansi>`)
					return true, nil
				}

				foundCharacterName = u.Character.Name
				foundUsername = u.Username

				u.Role = newRole

				users.SaveUser(u)

				u.SendText(messaging.CategoryWarning, `<ansi fg="alert-3">Your role has been set to: `+newRole+`</ansi>`)
				break
			}
		}

		if len(foundUsername) == 0 {
			users.SearchOfflineUsers(func(u *users.UserRecord) bool {

				if strings.EqualFold(searchUser, u.Username) {

					if why := roleChangeRefusal(user, u, newRole); why != `` {
						user.SendText(messaging.CategoryAdmin, `<ansi fg="alert-4">`+why+`</ansi>`)
						return false
					}

					foundCharacterName = u.Character.Name
					foundUsername = u.Username

					u.Role = newRole

					users.SaveUser(u)

					return false
				}

				return true
			})
		}

		if len(foundUsername) > 0 {
			user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`Role changed for user <ansi fg="username">%s</ansi> (Character name: <ansi fg="username">%s</ansi>).`, foundUsername, foundCharacterName))
			return true, nil
		}

	}

	return true, nil
}

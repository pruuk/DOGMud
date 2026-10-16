package usercommands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"

	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

/*
* Role Permissions:
* skillset 				(All)
 */
func Skillset(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// args should look like one of the following:
	// target conditionId - put condition on target if in the room
	// conditionId - put condition on self
	// search searchTerm - search for condition by name, display results
	args := util.SplitButRespectQuotes(rest)

	if len(args) < 2 {
		// send some sort of help info?
		infoOutput, _ := templates.Process("admincommands/help/command.skillset", nil, user.UserId)
		user.SendText(messaging.CategoryUsage, infoOutput)

		user.SendText(messaging.CategoryUsage, `Skill Names:`)
		for _, name := range skills.GetAllSkillNames() {
			user.SendText(messaging.CategoryUsage, `  <ansi fg="skill">`+string(name)+`</ansi>`)
		}

		return true, nil
	}

	var targetUser *users.UserRecord = user

	if target, err := actions.ResolveTargetActor(room, args[0]); err == nil && target.IsPlayer() {
		targetUser = target.(*actions.UserActor).User
		args = args[1:]
	}

	if len(args) < 2 {
		// send some sort of help info?
		infoOutput, _ := templates.Process("admincommands/help/command.skillset", nil, user.UserId)
		user.SendText(messaging.CategoryUsage, infoOutput)

		user.SendText(messaging.CategoryUsage, `Skill Names:`)
		for _, name := range skills.GetAllSkillNames() {
			user.SendText(messaging.CategoryUsage, `  <ansi fg="skill">`+string(name)+`</ansi>`)
		}

		return true, nil
	}

	// The skill notice is staff output when staff set their own skills and a
	// notice of a staff action when the target is someone else (#371 R3).
	skillNoticeCat := messaging.CategoryAdmin
	if targetUser.UserId != user.UserId {
		skillNoticeCat = messaging.CategoryWarning
	}

	if args[0] == `all` {
		skillValueInt, _ := strconv.Atoi(args[1])

		for _, skillName := range skills.GetAllSkillNames() {
			targetUser.Character.SetSkill(string(skillName), skillValueInt)
			targetUser.SendText(skillNoticeCat, fmt.Sprintf(`Your "<ansi fg="skill">%s</ansi>" skill level has been set to <ansi fg="red">%d</ansi>.`, skillName, skillValueInt))
		}

		if targetUser.UserId != user.UserId {
			user.SendText(messaging.CategoryAdmin, "done.")
		}

		return true, nil
	}

	skillName := strings.ToLower(args[0])
	skillValueInt, _ := strconv.Atoi(args[1])

	found := skills.SkillExists(skillName)

	if found {
		targetUser.Character.SetSkill(skillName, skillValueInt)
		targetUser.SendText(skillNoticeCat, fmt.Sprintf(`Your "<ansi fg="skill">%s</ansi>" skill level has been set to <ansi fg="red">%d</ansi>.`, skillName, skillValueInt))

		if targetUser.UserId != user.UserId {
			user.SendText(messaging.CategoryAdmin, "done.")
		}
	} else {
		targetUser.SendText(messaging.CategoryAdmin, fmt.Sprintf(`Skill "%s" not found.`, skillName))
	}

	return true, nil
}

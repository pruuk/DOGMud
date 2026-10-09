package usercommands

import (
	"fmt"
	"strconv"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/language"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// onlineColumns lists the online table's columns by translation key, in
// order. The admin view adds UserId, Zone and RoomId and drops Title, which
// had pushed it to 102 columns (#449); players keep Title.
func onlineColumns(isAdmin bool) []string {
	if isAdmin {
		return []string{`UserId`, `User.Name`, `Online`, `Role`, `Zone`, `RoomId`}
	}
	return []string{`User.Name`, `Title`, `Online`, `Role`}
}

func Online(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	isAdmin := user.Role != users.RoleUser

	columns := onlineColumns(isAdmin)
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = language.T(column)
	}

	allFormatting := [][]string{}

	rows := [][]string{}

	userCt := 0
	aiCt := 0
	humanCt := 0
	for _, uid := range users.GetOnlineUserIds() {

		u := users.GetByUserId(uid)

		if u != nil {

			onlineInfo := u.GetOnlineInfo()

			userCt++
			if onlineInfo.IsAI {
				aiCt++
			} else {
				humanCt++
			}

			onlineTime := onlineInfo.OnlineTimeStr
			if onlineInfo.IsAFK {
				onlineTime += ` <ansi fg="8">(afk)</ansi>`
			}

			permClass := `user`
			if onlineInfo.Role != users.RoleUser {
				if onlineInfo.Role == users.RoleAdmin {
					permClass = `admin`
				} else {
					permClass = `mod`
				}
			}

			// For admins, show [AI] tag next to AI-connected player names
			charName := onlineInfo.CharacterName
			if isAdmin && onlineInfo.IsAI {
				charName += ` <ansi fg="8">[AI]</ansi>`
			}

			// Each column's value and its formatting, keyed as onlineColumns
			// names them.
			cells := map[string][2]string{
				`UserId`:    {strconv.Itoa(u.UserId), `<ansi fg="userid">%s</ansi>`},
				`User.Name`: {charName, `<ansi fg="username">%s</ansi>`},
				`Title`:     {onlineInfo.Title, `<ansi fg="white-bold">%s</ansi>`},
				`Online`:    {onlineTime, `<ansi fg="magenta">%s</ansi>`},
				`Role`:      {onlineInfo.Role, `<ansi fg="role-` + permClass + `-bold">%s</ansi>`},
				`Zone`:      {u.Character.Zone, `<ansi fg="zone">%s</ansi>`},
				`RoomId`:    {strconv.Itoa(u.Character.RoomId), `<ansi fg="1">%s</ansi>`},
			}
			row := make([]string, len(columns))
			formatting := make([]string, len(columns))
			for i, column := range columns {
				row[i], formatting[i] = cells[column][0], cells[column][1]
			}

			allFormatting = append(allFormatting, formatting)

			rows = append(rows, row)
		}
	}

	tableTitle := fmt.Sprintf(language.T(`%d users online`), userCt)
	if userCt == 1 {
		tableTitle = fmt.Sprintf(language.T(`%d user online`), userCt)
	}
	// Add AI/human breakdown for admins
	if isAdmin && aiCt > 0 {
		tableTitle += fmt.Sprintf(` (%d human, %d AI)`, humanCt, aiCt)
	}

	onlineResultsTable := templates.GetTable(tableTitle, headers, rows, allFormatting...)
	tplTxt, _ := templates.Process("tables/generic", onlineResultsTable, user.UserId)
	user.SendText(messaging.CategorySystem, tplTxt)

	return true, nil
}

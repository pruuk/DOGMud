package usercommands

import (
	"fmt"
	"slices"
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

// onlineEntry is one online user's cell values before formatting.
type onlineEntry struct {
	userId, name, title, online, role, permClass, zone, roomId string
	isAI, isAFK                                                bool
}

// cells returns the entry's values and formatting in column order.
func (e onlineEntry) cells(columns []string) (row []string, formatting []string) {
	name := e.name
	if e.isAI {
		name += ` <ansi fg="8">[AI]</ansi>`
	}
	online := e.online
	if e.isAFK {
		online += ` <ansi fg="8">(afk)</ansi>`
	}
	all := map[string][2]string{
		`UserId`:    {e.userId, `<ansi fg="userid">%s</ansi>`},
		`User.Name`: {name, `<ansi fg="username">%s</ansi>`},
		`Title`:     {e.title, `<ansi fg="white-bold">%s</ansi>`},
		`Online`:    {online, `<ansi fg="magenta">%s</ansi>`},
		`Role`:      {e.role, `<ansi fg="role-` + e.permClass + `-bold">%s</ansi>`},
		`Zone`:      {e.zone, `<ansi fg="zone">%s</ansi>`},
		`RoomId`:    {e.roomId, `<ansi fg="1">%s</ansi>`},
	}
	row = make([]string, len(columns))
	formatting = make([]string, len(columns))
	for i, column := range columns {
		row[i], formatting[i] = all[column][0], all[column][1]
	}
	return row, formatting
}

// Online lists who is online in tables/generic. Every name and zone is shown
// whole: a list wider than the reader takes the table's plain-column or
// two-line form, at 80 columns as on a phone (plan 3b-3, ruling 7; the
// admin view once cut the zone, then the name, to "..." to stay within 80).
// Login runs it too (OnLoginCommands).
func Online(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	isAdmin := user.Role != users.RoleUser

	columns := onlineColumns(isAdmin)
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = language.T(column)
	}

	entries := []onlineEntry{}

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

			permClass := `user`
			if onlineInfo.Role != users.RoleUser {
				if onlineInfo.Role == users.RoleAdmin {
					permClass = `admin`
				} else {
					permClass = `mod`
				}
			}

			entries = append(entries, onlineEntry{
				userId:    strconv.Itoa(u.UserId),
				name:      onlineInfo.CharacterName,
				title:     onlineInfo.Title,
				online:    onlineInfo.OnlineTimeStr,
				role:      onlineInfo.Role,
				permClass: permClass,
				zone:      u.Character.Zone,
				roomId:    strconv.Itoa(u.Character.RoomId),
				// For admins, show [AI] tag next to AI-connected player names
				isAI:  isAdmin && onlineInfo.IsAI,
				isAFK: onlineInfo.IsAFK,
			})
		}
	}

	allFormatting := [][]string{}
	rows := [][]string{}
	for _, e := range entries {
		row, formatting := e.cells(columns)
		rows = append(rows, row)
		allFormatting = append(allFormatting, formatting)
	}

	tableTitle := fmt.Sprintf(language.T(`%d users online`), userCt)
	if userCt == 1 {
		tableTitle = fmt.Sprintf(language.T(`%d user online`), userCt)
	}
	// Add AI/human breakdown for admins
	if isAdmin && aiCt > 0 {
		tableTitle += fmt.Sprintf(` (%d human, %d AI)`, humanCt, aiCt)
	}

	// The Name leads each item of the two-line form, in the admin view too
	// where UserId comes first (plan 3b-3, ruling 6).
	onlineResultsTable := templates.GetTable(tableTitle, headers, rows, allFormatting...).WithLead(slices.Index(columns, `User.Name`))
	tplTxt, _ := templates.Process("tables/generic", onlineResultsTable, user.UserId)
	user.SendText(messaging.CategoryScreen, tplTxt)

	return true, nil
}

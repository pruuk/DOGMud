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
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/mattn/go-runewidth"
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

// onlineMaxWidth is the widest the admin online table may print.
const onlineMaxWidth = 80

// onlineTableWidth is the width tables/generic prints: each column's widest
// cell (or header) plus a space either side, and one border per column plus
// the closing one.
func onlineTableWidth(columns []string, entries []onlineEntry) int {
	width := len(columns) + 1
	for i := range columns {
		width += onlineColumnWidth(columns, entries, i) + 2
	}
	return width
}

func onlineColumnWidth(columns []string, entries []onlineEntry, col int) int {
	w := util.VisibleWidth(language.T(columns[col]))
	for _, e := range entries {
		row, _ := e.cells(columns)
		if cw := util.VisibleWidth(row[col]); cw > w {
			w = cw
		}
	}
	return w
}

// fitOnlineEntries keeps the admin table within onlineMaxWidth (#449). A
// long zone name, a name at NameSizeMax with its [AI] tag and an afk marker
// ran it past 80. It trims the Zone cell first, since RoomId still names the
// room, down to the header's width; only if that is not enough does it trim
// the character name, keeping the [AI] tag. A trimmed value ends in "...".
// The player view is returned as built.
func fitOnlineEntries(columns []string, entries []onlineEntry, isAdmin bool) []onlineEntry {
	if !isAdmin || len(entries) == 0 {
		return entries
	}
	out := append([]onlineEntry(nil), entries...)

	trims := []struct {
		column string
		field  func(e *onlineEntry) *string
	}{
		{`Zone`, func(e *onlineEntry) *string { return &e.zone }},
		{`User.Name`, func(e *onlineEntry) *string { return &e.name }},
	}
	for _, trim := range trims {
		over := onlineTableWidth(columns, out) - onlineMaxWidth
		if over <= 0 {
			break
		}
		col := slices.Index(columns, trim.column)
		if col < 0 {
			continue
		}
		target := max(onlineColumnWidth(columns, out, col)-over, util.VisibleWidth(language.T(trim.column)))
		for i := range out {
			row, _ := out[i].cells(columns)
			cellWidth := util.VisibleWidth(row[col])
			if cellWidth <= target {
				continue
			}
			field := trim.field(&out[i])
			budget := target - (cellWidth - util.VisibleWidth(*field))
			*field = runewidth.Truncate(*field, max(budget, 0), `...`)
		}
	}
	return out
}

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

	entries = fitOnlineEntries(columns, entries, isAdmin)

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

	onlineResultsTable := templates.GetTable(tableTitle, headers, rows, allFormatting...)
	tplTxt, _ := templates.Process("tables/generic", onlineResultsTable, user.UserId)
	user.SendText(messaging.CategorySystem, tplTxt)

	return true, nil
}

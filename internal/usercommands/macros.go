package usercommands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Macros(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	if len(user.Macros) == 0 {
		user.SendText(messaging.CategoryRefusal, "You have no macros set.")
		return true, nil
	}

	sortedKeys := make([]string, 0, len(user.Macros))

	for k := range user.Macros {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// Plan 3b-3, from the drawing macros (narrow-screens-3b3-mock): the 80
	// layout is redrawn (a long macro ran past 80), so each command wraps
	// between words to the form (users.RedrawnFormWidth), continuing 2 past
	// its letter (ruling 2). At 80 and 42 the key sits at 2 and its
	// commands at 6; at 30 at 1 and 3. A command line that fits is the line
	// master drew.
	formWidth := users.RedrawnFormWidth(user.WrapWidth())
	keyIndent, rowIndent := 2, 6
	if formWidth < 42 {
		keyIndent, rowIndent = 1, 3
	}

	user.SendText(messaging.CategoryScreen, `<ansi fg="226">Your macros:</ansi>`)
	for _, macro := range sortedKeys {
		macroCommand := user.Macros[macro]
		user.SendBlankLine(messaging.CategorySystem)

		user.SendText(messaging.CategoryScreen, fmt.Sprintf(`%s<ansi fg="228">%s</ansi>:`, strings.Repeat(" ", keyIndent), macro))

		commandParts := strings.Split(macroCommand, `;`)

		for i, cmd := range commandParts {

			cmdParts := strings.Split(cmd, ` `)
			cmdAlone := cmdParts[0]
			cmdRest := ``
			if len(cmdParts) > 1 {
				cmdRest = strings.Join(cmdParts[1:], ` `)
			}
			letter := string(rune(97+i)) + `)`
			command := `<ansi fg="command">` + cmdAlone + `</ansi>`

			line := fmt.Sprintf(`%s%s %s %s`, strings.Repeat(" ", rowIndent), letter, command, cmdRest)
			if util.VisibleWidth(line) > formWidth {
				line = strings.Join(messaging.PackUnits(append([]string{letter, command}, strings.Fields(cmdRest)...),
					rowIndent, rowIndent+2, formWidth), "\n")
			}
			user.SendText(messaging.CategoryScreen, line)
		}
	}
	user.SendBlankLine(messaging.CategorySystem)
	user.SendText(messaging.CategorySystem, `To use a macro, type <ansi fg="command">={num}</ansi>.`)
	user.SendText(messaging.CategorySystem, `Some terminals support pressing the associated F-Key (<ansi fg="228">F1</ansi>, <ansi fg="228">F2</ansi>, etc.)`)

	return true, nil
}

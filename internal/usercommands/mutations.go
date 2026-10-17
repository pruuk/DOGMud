package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Mutations lists all mutations the player has acquired from the Chrysalis.
func Mutations(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	if len(user.Character.Mutations) == 0 {
		user.SendText(messaging.CategorySystem, `<ansi fg="magenta">The Chrysalis has not yet reshaped you. No mutations have emerged.</ansi>`)
		return true, nil
	}

	user.SendText(messaging.CategorySystem, ``)
	user.SendText(messaging.CategorySystem, `<ansi fg="magenta"> .:. <ansi fg="yellow">Your Mutations</ansi> .:.</ansi>`)
	user.SendText(messaging.CategorySystem, ``)

	// CategorySystem is never wrapped by the pipeline, so each prose line
	// wraps itself to the reader's width, continuation lines keeping the
	// line's indent (util.SplitStringNL's prefix, as admin.server.go uses).
	width := user.GetLineWidth()
	indented := func(indent, text string) string {
		return indent + util.SplitStringNL(text, width-len(indent), indent)
	}

	for mutId, level := range user.Character.Mutations {
		depth := mutations.LevelDescription(level)
		spec := mutations.GetMutation(mutId)
		if spec == nil {
			mudlog.Error("mutations: missing spec", "mutationId", mutId, "userId", user.UserId)
			continue
		}
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`  <ansi fg="yellow">%s</ansi> <ansi fg="magenta">(%s)</ansi>`, spec.Name, depth))
		user.SendText(messaging.CategorySystem, indented(`    `, fmt.Sprintf(`<ansi fg="white">%s</ansi>`, spec.Description)))

		// What it actually does (descriptive, no raw numbers): benefits then
		// drawbacks. Effects with no player-facing summary are skipped.
		for _, e := range spec.Pros {
			if d := mutations.DescribeEffect(e); d != "" {
				user.SendText(messaging.CategorySystem, indented(`      `, fmt.Sprintf(`<ansi fg="green">+ %s</ansi>`, d)))
			}
		}
		for _, e := range spec.Cons {
			if d := mutations.DescribeEffect(e); d != "" {
				user.SendText(messaging.CategorySystem, indented(`      `, fmt.Sprintf(`<ansi fg="red">- %s</ansi>`, d)))
			}
		}
	}

	user.SendText(messaging.CategorySystem, ``)
	return true, nil
}

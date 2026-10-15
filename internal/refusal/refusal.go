// Package refusal holds one helper per refusal situation (messaging M7,
// #371): the line a player reads when a command cannot do what they asked.
// Each helper sends exactly one line on messaging.CategoryRefusal and has a
// ...Text sibling (or a ...Line constant) returning it. See context.md.
package refusal

import (
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Recipient is whoever a refusal is told to: *users.UserRecord and every
// actions.Actor satisfy it (a mob actor's SendText does nothing).
type Recipient interface {
	SendText(cat messaging.Category, msg string)
}

// echoMax is the most runes of a player's own input a line repeats, so the
// longest echo still fits 80 columns.
const echoMax = 24

var ansiTag = regexp.MustCompile(`</?ansi[^>]*>`)

func send(to Recipient, line string) {
	to.SendText(messaging.CategoryRefusal, line)
}

// echo is typed as a refusal may repeat it: tags stripped, trimmed, capped at
// echoMax runes, and any leftover markup escaped, so typed input cannot style
// the line. Only names the player holds or knows are echoed (spec section 2).
func echo(typed string) string {
	s := strings.TrimSpace(ansiTag.ReplaceAllString(typed, ``))
	if r := []rune(s); len(r) > echoMax {
		s = strings.TrimSpace(string(r[:echoMax])) + `...`
	}
	return util.EscapeAnsiTags(s)
}

// hint is the tail naming a command the player can type next.
func hint(command string) string {
	return ` Try <ansi fg="command">` + command + `</ansi>.`
}

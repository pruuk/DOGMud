// Package refusal holds one helper per refusal situation (messaging M7,
// #371): the line a player reads when a command cannot do what they asked.
// Each helper sends exactly one line on messaging.CategoryRefusal (Failed
// sends on CategoryError) and has a
// ...Text sibling (or a ...Line constant) returning it. See context.md.
package refusal

import (
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/mattn/go-runewidth"
)

// Recipient is whoever a refusal is told to: *users.UserRecord and every
// actions.Actor satisfy it (a mob actor's SendText does nothing).
type Recipient interface {
	SendText(cat messaging.Category, msg string)
}

// echoMax is the most display columns of a player's own input a line repeats
// before the "..." tail (Truncate keeps the whole echo within echoMax+3), so
// the longest echo still fits 80 columns, wide characters included.
const echoMax = 24

var ansiTag = regexp.MustCompile(`</?ansi[^>]*>`)

func send(to Recipient, line string) {
	to.SendText(messaging.CategoryRefusal, line)
}

// echo is typed as a refusal may repeat it: tags stripped, trimmed, capped at
// echoMax display columns, and any leftover markup escaped, so typed input cannot style
// the line. Only names the player holds or knows are echoed (spec section 2).
func echo(typed string) string {
	s := strings.TrimSpace(ansiTag.ReplaceAllString(typed, ``))
	s = strings.TrimSpace(runewidth.Truncate(s, echoMax+3, `...`))
	return util.EscapeAnsiTags(s)
}

// hint is the tail naming a command the player can type next.
func hint(command string) string {
	return ` Try <ansi fg="command">` + command + `</ansi>.`
}

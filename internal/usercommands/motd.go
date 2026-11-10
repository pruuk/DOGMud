package usercommands

import (
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// motdAnsiTagRE strips <ansi …> / </ansi> tags for visible-width math
// when padding MOTD lines. Mirrors the pattern in messaging/wrap.go.
var motdAnsiTagRE = regexp.MustCompile(`<ansi[^>]*>|</ansi>`)

// motdVisibleWidth is a line's display width in columns, not bytes, so a
// curly quote or an accent pads like any other character (#592).
func motdVisibleWidth(s string) int {
	return util.VisibleWidth(s)
}

func Motd(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	m := configs.GetServerConfig().Motd.String()
	text, err := templates.ProcessText(m, nil)
	if err != nil {
		text = m
	}

	user.SendText(messaging.CategoryBroadcast, motdBox(text, user.WrapWidth()))

	return true, nil
}

// motdBoxMinWidth is the narrowest box the spaced-out title fits in: the
// title and its leading space take 41 of the lw-4 content columns.
const motdBoxMinWidth = 45

// motdBox draws the MOTD text in a double-line box exactly lw columns wide.
// A screen narrower than the title, or text holding a word wider than the
// box (a long link), gets the narrow form instead: a plain title and the text
// wrapped to the screen, because a box cannot hold either (spec section 2).
func motdBox(text string, lw int) string {
	if lw < motdBoxMinWidth || motdLongestWord(text) > lw-5 {
		return motdPlain(text, lw)
	}
	// Box outer width scales with the player's configured line width.
	// Layout: " " + "║" + " " + <content padded to padWidth> + "║"
	//          1     1     1          ...                       1   = lw
	// So padWidth = lw - 4 and wrap width = padWidth - 1 (leave a
	// trailing space gutter inside the right border). A border row is
	// " " + "╔" + <innerWidth rules> + "╗", so innerWidth = lw - 3: the
	// rules fill the same columns as the content line's " " + padWidth.
	if lw < 20 {
		lw = 20 // sanity floor; below this the box can't render
	}
	innerWidth := lw - 3 // chars between the ╔ ╗ corners
	padWidth := lw - 4   // ` ║ ` (3) + `║` (1) = 4 frame chars per line
	wrapWidth := padWidth - 1

	wrapped := strings.Split(messaging.WrapAnsi(text, wrapWidth), "\n")

	horiz := strings.Repeat("═", innerWidth)

	// Title row: pad the visible title to padWidth (ANSI tags don't
	// count toward width — we add them after measuring).
	title := `.:  M E S S A G E   O F   T H E   D A Y`
	// Legacy layout was ` ║  TITLE...║` — content cell already opens with
	// one space (the ` ║ ` frame), so we add 1 more space inside to match
	// the legacy 2-space leading indent.
	titleInner := " " + title
	if len(titleInner) > padWidth {
		titleInner = titleInner[:padWidth]
	}
	titlePadded := titleInner + strings.Repeat(" ", padWidth-len(titleInner))
	// Re-insert the ansi styling around the visible title only.
	titleStyled := strings.Replace(titlePadded, title, `<ansi fg="cyan-bold">`+title+`</ansi>`, 1)

	var output string
	output += `<ansi fg="yellow"> ╔` + horiz + `╗` + "\n"
	output += ` ║ ` + titleStyled + `║` + "\n"
	output += ` ╠` + horiz + `╣` + "\n"
	for _, line := range wrapped {
		// Pad each line to padWidth visible chars inside the box.
		// ANSI tag bytes don't count toward width.
		padLen := padWidth - motdVisibleWidth(line)
		if padLen < 0 {
			padLen = 0
		}
		output += ` ║ ` + line + strings.Repeat(" ", padLen) + `║` + "\n"
	}
	output += ` ╚` + horiz + `╝</ansi>` + "\n"

	return output
}

// motdPlain is the MOTD's narrow form: the title on its own line and the
// text wrapped to lw. Only a single word wider than lw can run past it.
func motdPlain(text string, lw int) string {
	return `<ansi fg="cyan-bold">Message of the Day</ansi>` + "\n" +
		`<ansi fg="yellow">` + messaging.WrapAnsi(text, lw) + `</ansi>` + "\n"
}

// motdLongestWord is the display width of the longest word in text.
func motdLongestWord(text string) int {
	longest := 0
	for _, word := range strings.Fields(motdAnsiTagRE.ReplaceAllString(text, "")) {
		longest = max(longest, motdVisibleWidth(word))
	}
	return longest
}

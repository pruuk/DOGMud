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

// motdBox draws the MOTD text in a double-line box exactly lw columns wide,
// at 80 columns and wider, as a desktop has always seen it. A narrower
// screen, or text holding a word wider than the box (a long link), gets the
// ruled form instead (motdRuled), because the narrow forms have rules only,
// no corners or side bars (plan 3b-3, ruling 1): the 42 form from 42 to 79
// columns, as a template's 42 form is picked there, and the 30 form below.
func motdBox(text string, lw int) string {
	if lw < users.MaxAutoWrapWidth {
		return motdRuled(text, narrowFormWidth(lw))
	}
	if motdLongestWord(text) > lw-5 {
		return motdRuled(text, users.MaxAutoWrapWidth)
	}
	// Box outer width scales with the player's configured line width.
	// Layout: " " + "║" + " " + <content padded to padWidth> + "║"
	//          1     1     1          ...                       1   = lw
	// So padWidth = lw - 4 and wrap width = padWidth - 1 (leave a
	// trailing space gutter inside the right border). A border row is
	// " " + "╔" + <innerWidth rules> + "╗", so innerWidth = lw - 3: the
	// rules fill the same columns as the content line's " " + padWidth.
	innerWidth := lw - 3 // chars between the ╔ ╗ corners
	padWidth := lw - 4   // ` ║ ` (3) + `║` (1) = 4 frame chars per line
	wrapWidth := padWidth - 1

	wrapped := strings.Split(messaging.WrapAnsi(text, wrapWidth), "\n")

	horiz := strings.Repeat("═", innerWidth)

	// Title row: pad the visible title to padWidth (ANSI tags don't
	// count toward width — we add them after measuring).
	title := motdSpacedTitle
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

// motdSpacedTitle is the title the box and the wider ruled forms show;
// motdPlainTitle is the one a form narrower than it shows.
const (
	motdSpacedTitle = `.:  M E S S A G E   O F   T H E   D A Y`
	motdPlainTitle  = `.:  MESSAGE OF THE DAY`
)

// motdRuled is the MOTD's ruled form, formWidth columns wide, as drawn
// (login_motd_and_who in narrow-screens-3b-mock): a `═` rule, the title at
// indent 2, a rule, the text, a closing rule, every rule formWidth wide and
// no corners. The spaced title where it fits, else the plain one. The text
// sits at indent 2 (1 below 42 columns), wrapped to the room left; a line
// holding a word wider than that room (a long link) starts at column 0, and
// only a word wider than the whole form can run past it.
func motdRuled(text string, formWidth int) string {
	title := motdSpacedTitle
	if 2+motdVisibleWidth(title) > formWidth {
		title = motdPlainTitle
	}
	indent := 2
	if formWidth < 42 {
		indent = 1
	}
	rule := strings.Repeat("═", formWidth)
	out := `<ansi fg="yellow">` + rule + "\n" +
		`  <ansi fg="cyan-bold">` + title + `</ansi>` + "\n" +
		rule + "\n"
	for _, line := range strings.Split(messaging.WrapAnsi(text, formWidth-indent), "\n") {
		if motdVisibleWidth(line) <= formWidth-indent {
			line = strings.Repeat(" ", indent) + line
		}
		out += line + "\n"
	}
	return out + rule + `</ansi>` + "\n"
}

// motdLongestWord is the display width of the longest word in text.
func motdLongestWord(text string) int {
	longest := 0
	for _, word := range strings.Fields(motdAnsiTagRE.ReplaceAllString(text, "")) {
		longest = max(longest, motdVisibleWidth(word))
	}
	return longest
}

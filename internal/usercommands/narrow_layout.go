package usercommands

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Narrow forms of the screens built here in Go (web client base spec,
// section 2): the companion screens (plan 3b-1), and the craft, guild and
// MOTD screens (plan 3b-3). They pick a form as templates.Process picks a
// template's: the 80-column screen to any reader it fits, else the form
// drawn to 42 columns, or to 30 below that. Every form shows everything the
// 80-column screen does, and breaks lines only between whole units (a name,
// an ingredient with its count, a command), unless a unit is wider than the
// room left at its indent (ruling 3).

// narrowFormWidth is the width of the narrow form for a reader width
// columns wide: 42, or 30 below that (or the reader's width, if narrower
// still). A reader between 42 and 80 gets the 42 form, as a template's.
func narrowFormWidth(width int) int {
	if width >= 42 {
		return 42
	}
	return min(width, 30)
}

// screenFormWidth is the width to draw a screen to for a reader width
// columns wide: 0 (the 80-column screen) when lines all fit, else
// narrowFormWidth. It measures, so it is for a screen a player asks for,
// never a per-round or broadcast one.
func screenFormWidth(width int, lines ...string) int {
	for _, line := range lines {
		if util.VisibleWidth(line) > width {
			return narrowFormWidth(width)
		}
	}
	return 0
}

// redrawnFormWidth is the width to draw a screen to whose 80-column layout
// was itself redrawn to fit 80 (the craft screens, `guild list`): that
// layout at 80 and wider, whatever the reader's width past 80, else
// narrowFormWidth.
func redrawnFormWidth(width int) int {
	if width >= users.MaxAutoWrapWidth {
		return users.MaxAutoWrapWidth
	}
	return narrowFormWidth(width)
}

// packUnits lays units out in lines of at most width columns: the first
// line indented by first, the rest by rest, breaking only between units. A
// unit goes on the line in hand when it fits there (the first line measured
// from its own indent), else on a new line. Only a unit too long for a whole
// new line at rest is wrapped between its words (messaging.WrapAnsi), and
// its pieces keep that indent (ruling 3). A piece with no space left to
// split at that fits the line but not the indent is pulled left to end at
// the right edge; one wider than the line (a legacy 32-letter name at 30,
// ruling 8) runs past it on a line of its own.
func packUnits(units []string, first, rest, width int) []string {
	lines := []string{}
	line, used := strings.Repeat(" ", first), first
	empty := true
	newLine := func() {
		lines = append(lines, line)
		line, used, empty = strings.Repeat(" ", rest), rest, true
	}
	place := func(word string) {
		w := util.VisibleWidth(word)
		sep := 1
		if empty {
			sep = 0
		}
		if used+sep+w > width && !empty {
			newLine()
			sep = 0
		}
		line += strings.Repeat(" ", sep) + word
		used += sep + w
		empty = false
	}
	for _, unit := range units {
		w := util.VisibleWidth(unit)
		sep := 1
		if empty {
			sep = 0
		}
		if used+sep+w <= width || w <= width-rest {
			place(unit)
			continue
		}
		if !empty {
			newLine()
		}
		for _, part := range strings.Split(messaging.WrapAnsi(unit, width-rest), "\n") {
			// A piece with no space to split at (a long character
			// name) that fits the line but not the indent is pulled
			// left to end at the right edge, as the mock drew such a
			// unit, on a line of its own.
			if pw := util.VisibleWidth(part); pw > width-rest && pw <= width {
				if !empty {
					newLine()
				}
				lines = append(lines, strings.Repeat(" ", width-pw)+part)
				line, used, empty = strings.Repeat(" ", rest), rest, true
				continue
			}
			place(part)
		}
	}
	lines = append(lines, line)
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return lines
}

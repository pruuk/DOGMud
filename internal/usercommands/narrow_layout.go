package usercommands

import (
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
// room left at its indent (ruling 3). messaging.PackUnits lays them out.

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

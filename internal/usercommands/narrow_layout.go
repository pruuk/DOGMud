package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Narrow forms of the screens built here in Go (web client base spec,
// section 2): the companion screens (plan 3b-1), and the craft, guild and
// MOTD screens (plan 3b-3). Most pick a form by band, without measuring it:
// the 80-column screen (to any reader it fits, for those that measure it
// with screenFormWidth), else the form drawn to 42 columns from 42 up, or
// to 30 below 42. So a 40 or 41 column reader gets the 30 form where
// templates.Process, which measures each form, could take the 42 one. The
// guild roster measures as Process does (measuredFormLines). Every form
// shows everything the 80-column screen does, and breaks lines only between
// whole units (a name, an ingredient with its count, a command), unless a
// unit is wider than the room left at its indent (ruling 3).
// messaging.PackUnits lays them out.

// The form widths come from users.NarrowFormWidth (42, or 30) and
// users.RedrawnFormWidth (80 for a screen whose 80-column layout was itself
// redrawn), shared with the scan screen in internal/actions.

// screenFormWidth is the width to draw a screen to for a reader width
// columns wide: 0 (the 80-column screen) when lines all fit, else
// users.NarrowFormWidth. It measures, so it is for a screen a player asks
// for, never a per-round or broadcast one.
func screenFormWidth(width int, lines ...string) int {
	for _, line := range lines {
		if util.VisibleWidth(line) > width {
			return users.NarrowFormWidth(width)
		}
	}
	return 0
}

// measuredFormLines is a narrow form laid out by pack, picked as
// templates.Process picks a template's: the form drawn to 42 columns when
// all its lines fit width, else the one drawn to users.NarrowFormWidth
// (30, or the reader's width if narrower; 42 again from 42 up, where only a
// legacy name can overrun). It measures, so it is for a screen a player
// asks for, never a per-round or broadcast one.
func measuredFormLines(width int, pack func(formWidth int) []string) []string {
	lines := pack(42)
	for _, line := range lines {
		if util.VisibleWidth(line) > width {
			return pack(users.NarrowFormWidth(width))
		}
	}
	return lines
}

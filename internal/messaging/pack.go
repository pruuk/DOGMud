package messaging

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// PackUnits lays units out in lines of at most width columns: the first
// line indented by first, the rest by rest, breaking only between units. A
// unit goes on the line in hand when it fits there (the first line measured
// from its own indent), else on a new line. Only a unit too long for a whole
// new line at rest is wrapped between its words (WrapAnsi), and its pieces
// keep that indent (web client base spec, ruling 3). A piece with no space
// left to split at that fits the line but not the indent is pulled left to
// end at the right edge; one wider than the line (a legacy 32-letter name
// at 30, ruling 8) runs past it on a line of its own.
//
// The narrow forms built in Go (internal/usercommands) and the two-line
// table form (templates.TemplateTable.DetailLines) lay out with it.
func PackUnits(units []string, first, rest, width int) []string {
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
		for _, part := range strings.Split(WrapAnsi(unit, width-rest), "\n") {
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

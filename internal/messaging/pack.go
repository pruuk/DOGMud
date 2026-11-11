package messaging

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// PackUnits lays units out in lines of at most width columns: the first
// line indented by first, the rest by rest, breaking only between units. A
// unit goes on the line in hand when it fits there (the first line measured
// from its own indent), else on a new line. Only a unit too long for a whole
// new line at rest is wrapped between its words (WrapAnsi), and its pieces
// keep that indent (web client base spec, ruling 3). A piece with no space
// left to split at:
//
//   - that fits the line but not the indent is pulled left to end at the
//     right edge, on a line of its own (plan 3b-3, decision 1);
//   - that is wider than the whole line breaks at the right edge, as the
//     wrapbreak template function breaks signs and runes (a 40-letter guild
//     name, a long link in a macro);
//   - unless it is a character name, which is never broken: a legacy name
//     longer than today's limit runs past the edge on a line of its own at
//     its indent (ruling 8, the accepted exception of plan 3b-1).
//
// names lists the units that are character names, by value. The caller
// names them, rather than the units that may be broken, so a unit nobody
// thought about breaks and stays on the screen; only a name, the one
// accepted exception, can run past it.
//
// The narrow forms built in Go (internal/usercommands, scan) lay out with
// it; the two-line table form uses PackUnitsKeepIndent.
func PackUnits(units []string, first, rest, width int, names ...string) []string {
	return pack(units, first, rest, width, true, func(unit string) bool { return slices.Contains(names, unit) })
}

// PackUnitsKeepIndent is PackUnits for a layout where a column left of the
// indent means something: in the two-line table form
// (templates.TemplateTable.DetailLines) column 0 starts the next item. No
// piece is pulled left of its indent and no word is broken, so a piece
// wider than the room at the indent runs past the edge on a line of its
// own. A table cannot tell a name from other text, and the only spaceless
// detail pieces that long are character names (auction history's Seller
// and Buyer), the accepted exception of ruling 8.
func PackUnitsKeepIndent(units []string, first, rest, width int) []string {
	return pack(units, first, rest, width, false, func(string) bool { return true })
}

// pack is PackUnits and PackUnitsKeepIndent: pullLeft says whether a
// spaceless piece that fits the line but not its indent is pulled left, and
// isName whether a unit's pieces may never be broken inside a word.
func pack(units []string, first, rest, width int, pullLeft bool, isName func(string) bool) []string {
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
			pw := util.VisibleWidth(part)
			switch {
			case pw <= width-rest:
				place(part)
			case pw <= width && pullLeft:
				// It fits the line but not the indent: start further
				// left, as the mock drew such a unit.
				if !empty {
					newLine()
				}
				lines = append(lines, strings.Repeat(" ", width-pw)+part)
				line, used, empty = strings.Repeat(" ", rest), rest, true
			case isName(unit):
				// Never broken: past the edge, alone, at its indent.
				if !empty {
					newLine()
				}
				place(part)
				newLine()
			default:
				// Broken at the right edge, the first piece filling the
				// line it starts on.
				if !empty {
					newLine()
				}
				for i, piece := range breakWord(part, width-used, width-rest) {
					if i > 0 {
						newLine()
					}
					place(piece)
				}
			}
		}
	}
	if !empty || len(lines) == 0 {
		lines = append(lines, line)
	}
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return lines
}

// breakWord splits word, which has no space, into pieces: the first
// firstWidth columns wide, the rest restWidth, the last what is left. Each
// piece closes the colour spans open at its end and the next reopens them
// (WrapAnsi does that at the line breaks put between the pieces).
func breakWord(word string, firstWidth, restWidth int) []string {
	var b strings.Builder
	cols, limit := 0, max(1, firstWidth)
	for i := 0; i < len(word); {
		if n := ansiTagLen(word[i:]); n > 0 {
			b.WriteString(word[i : i+n])
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(word[i:])
		w := runeColumns(r)
		if cols > 0 && cols+w > limit {
			b.WriteByte('\n')
			cols, limit = 0, max(1, restWidth)
		}
		b.WriteString(word[i : i+size])
		cols += w
		i += size
	}
	// Every piece fits the wider of the two widths, so WrapAnsi only
	// closes and reopens colour at the breaks.
	return strings.Split(WrapAnsi(b.String(), max(firstWidth, restWidth)), "\n")
}

// Package menu is DOGMud's one menu primitive: a numbered list of choices,
// how it looks, and how a typed line resolves against it. It holds no session
// state and does no I/O; internal/users (game menus) and internal/prompt
// (builder and player flows) supply the input sessions.
package menu

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Markers carried in the 4-wide marker column. The symbol carries the
// meaning; colour only reinforces it (owner is green/blue colourblind).
const (
	MarkerQuest   = "[!]" // new quest
	MarkerHandIn  = "[?]" // quest ready to hand in
	MarkerInfo    = "[*]" // writer-flagged info not yet heard
	MarkerChecked = "[x]" // a builder toggle that is selected
)

const (
	lineWidth    = 80
	numberColor  = "command"
	markerColor  = "yellow-bold"
	dimColor     = "8"
	columnWidth  = 39
	leftMargin   = "  "
	markerColumn = 4
)

// Entry is one numbered choice.
type Entry struct {
	Id      string   // returned on choice; also matched when typed exactly
	Marker  string   // "", or one of the Marker constants
	Label   string   // natural language, one line
	Detail  string   // optional second line, indented under the label
	Dim     bool     // rendered grey: heard info, small talk
	Aliases []string // extra exact words that pick this entry ("y", "n")
}

// Menu is a heading, numbered entries, and an optional final Leave entry.
type Menu struct {
	Heading    string
	Entries    []Entry
	Leave      string // label of the final numbered leave entry; "" for none
	Default    string // Id chosen on Enter; "" for none
	Footer     string // overrides the computed footer
	Columns    int    // 2 lays name-only lists side by side (long admin lists); Detail is not shown in this mode
	Prefix     bool   // a unique label prefix resolves (builders only)
	AcceptText bool   // an unmatched line resolves as Text (free-text builders)

	// LeaveAliases are extra exact words that pick Leave, checked before
	// numbers so a builder's "0" can mean done. Ignored when Leave is "".
	LeaveAliases []string
}

// all returns the entries as numbered, Leave included.
func (m Menu) all() []Entry {
	if m.Leave == "" {
		return m.Entries
	}
	out := make([]Entry, 0, len(m.Entries)+1)
	out = append(out, m.Entries...)
	return append(out, Entry{Label: m.Leave})
}

// Render draws the menu at the 80-column wrap. Callers send it on
// messaging.CategoryScreen (player menus) or CategoryAdmin (builders).
func Render(m Menu) string {
	var b strings.Builder
	if m.Heading != "" {
		b.WriteString(m.Heading)
		b.WriteString("\n\n")
	}
	entries := m.all()
	if len(entries) > 0 {
		if m.Columns == 2 {
			renderColumns(&b, entries)
		} else {
			numW := len(strconv.Itoa(len(entries)))
			for i, e := range entries {
				renderEntry(&b, i+1, numW, e)
			}
		}
		b.WriteString("\n")
	}
	b.WriteString(footer(m))
	return b.String()
}

func numberText(n, numW int) string {
	return fmt.Sprintf(`<ansi fg="%s">%*d.</ansi>`, numberColor, numW, n)
}

func markerText(marker string) string {
	if marker == "" {
		return strings.Repeat(" ", markerColumn)
	}
	return fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, markerColor, marker) +
		strings.Repeat(" ", max(0, markerColumn-utf8.RuneCountInString(marker)))
}

func labelText(e Entry) string {
	if e.Dim {
		return fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, dimColor, e.Label)
	}
	return e.Label
}

func renderEntry(b *strings.Builder, n, numW int, e Entry) {
	b.WriteString(leftMargin)
	b.WriteString(numberText(n, numW))
	b.WriteString(" ")
	b.WriteString(markerText(e.Marker))
	b.WriteString(labelText(e))
	b.WriteString("\n")
	if e.Detail == "" {
		return
	}
	indent := strings.Repeat(" ", len(leftMargin)+numW+2+markerColumn)
	for _, line := range wrap(e.Detail, lineWidth-len(indent)) {
		b.WriteString(indent)
		b.WriteString(line)
		b.WriteString("\n")
	}
}

func renderColumns(b *strings.Builder, entries []Entry) {
	numW := len(strconv.Itoa(len(entries)))
	// One column is reserved as a separator so a full-width label never
	// touches the next cell's number.
	labelW := columnWidth - numW - 2 - markerColumn - 1
	for i, e := range entries {
		if i%2 == 0 {
			b.WriteString(leftMargin)
		}
		label := e.Label
		if rs := []rune(label); len(rs) > labelW {
			label = string(rs[:labelW])
		}
		cell := numberText(i+1, numW) + " " + markerText(e.Marker) + labelText(Entry{Label: label, Dim: e.Dim})
		b.WriteString(cell)
		if i%2 == 0 && i+1 < len(entries) {
			b.WriteString(strings.Repeat(" ", columnWidth-(numW+2+markerColumn+utf8.RuneCountInString(label))))
		} else {
			b.WriteString("\n")
		}
	}
}

func footer(m Menu) string {
	if m.Footer != "" {
		return m.Footer
	}
	if m.Default != "" {
		for i, e := range m.Entries {
			if e.Id == m.Default {
				return fmt.Sprintf("Type a number, or press Enter for %d.", i+1)
			}
		}
	}
	if m.Leave != "" {
		return "Type a number, or press Enter to leave."
	}
	return "Type a number."
}

// wrap splits text into lines of at most width visible characters on word
// boundaries. A single word longer than width gets a line of its own.
func wrap(text string, width int) []string {
	var lines []string
	var cur strings.Builder
	curLen := 0 // runes in cur
	for _, word := range strings.Fields(text) {
		wl := utf8.RuneCountInString(word)
		if curLen > 0 && curLen+1+wl > width {
			lines = append(lines, cur.String())
			cur.Reset()
			curLen = 0
		}
		if curLen > 0 {
			cur.WriteString(" ")
			curLen++
		}
		cur.WriteString(word)
		curLen += wl
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

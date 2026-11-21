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

	"github.com/GoMudEngine/GoMud/internal/messaging"
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
	numberColor  = "command"
	markerColor  = "yellow-bold"
	dimColor     = "8"
	labelColor   = "menu-label" // undimmed lines; brighter than the grey screen category
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
	Heading string
	// Body is optional paragraphs between the heading and the entries,
	// each wrapped to the width given to Render (an offer card's objective
	// and reward). The labels lint does not read it.
	Body       []string
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

// Render draws the menu for a reader width columns wide (their
// UserRecord.WrapWidth). Callers send it on messaging.CategoryScreen (player
// menus) or CategoryAdmin (builders).
func Render(m Menu, width int) string {
	var b strings.Builder
	if m.Heading != "" {
		b.WriteString(strings.Join(wrapLines(m.Heading, width), "\n"))
		b.WriteString("\n\n")
	}
	if len(m.Body) > 0 {
		for _, para := range m.Body {
			for _, line := range wrapLines(para, width) {
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	entries := m.all()
	if len(entries) > 0 {
		if m.Columns == 2 {
			renderColumns(&b, entries)
		} else {
			numW := len(strconv.Itoa(len(entries)))
			for i, e := range entries {
				renderEntry(&b, i+1, numW, e, width)
			}
		}
		b.WriteString("\n")
	}
	b.WriteString(strings.Join(wrapLines(footer(m), width), "\n"))
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

// lineColor is the colour of an entry's label and detail. Every line gets an
// explicit colour: menus ride grey categories (screen, admin), so a dimmed
// line is only dimmer when the others are coloured brighter.
func lineColor(e Entry) string {
	if e.Dim {
		return dimColor
	}
	return labelColor
}

func labelText(e Entry) string {
	return fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, lineColor(e), e.Label)
}

// labelLines splits a label that does not fit in avail columns on word
// boundaries; a label that fits, or a reader too narrow for any label column,
// keeps it on one line.
func labelLines(label string, avail int) []string {
	if avail <= 0 || utf8.RuneCountInString(label) <= avail {
		return []string{label}
	}
	if lines := wrapLines(label, avail); len(lines) > 0 {
		return lines
	}
	return []string{label}
}

func renderEntry(b *strings.Builder, n, numW int, e Entry, width int) {
	b.WriteString(leftMargin)
	b.WriteString(numberText(n, numW))
	b.WriteString(" ")
	b.WriteString(markerText(e.Marker))
	// A label wider than the reader wraps, each further line indented under
	// the label's first character (#842: a phone client cut labels mid-word).
	labelStart := len(leftMargin) + numW + 2 + max(markerColumn, utf8.RuneCountInString(e.Marker))
	for i, line := range labelLines(e.Label, width-labelStart) {
		if i > 0 {
			b.WriteString("\n")
			b.WriteString(strings.Repeat(" ", labelStart))
		}
		b.WriteString(labelText(Entry{Label: line, Dim: e.Dim}))
	}
	b.WriteString("\n")
	if e.Detail == "" {
		return
	}
	indent := strings.Repeat(" ", len(leftMargin)+numW+2+markerColumn)
	for _, line := range wrapLines(e.Detail, width-len(indent)) {
		b.WriteString(indent)
		fmt.Fprintf(b, `<ansi fg="%s">%s</ansi>`, lineColor(e), line)
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

// wrapLines splits text into lines with the game's one word wrapper,
// messaging.WrapAnsi. Empty text gives no lines.
func wrapLines(text string, width int) []string {
	if text == "" {
		return nil
	}
	return strings.Split(messaging.WrapAnsi(text, width), "\n")
}

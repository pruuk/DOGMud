package templates

import (
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Help markup (web client base plan 4, owner ruling 2026-11-12: help files
// state their layout and the game never guesses). Four marker lines at
// column 0: "::rows", "::list" and "::keep" open a block and "::" closes it.
// Everything outside a block is paragraphs. internal/guards'
// help_markup_guard_test.go holds every help file to these rules.
const (
	helpMarkRows = "::rows"
	helpMarkList = "::list"
	helpMarkKeep = "::keep"
	helpMarkEnd  = "::"
)

// helpJoin holds the words of one unit together while it is packed: one
// column to util.VisibleWidth and to WrapAnsi, and never a break. \x1f will
// not do here: VisibleWidth measures it as 0.
const helpJoin = " "

var (
	helpListMarker = regexp.MustCompile(`^(- |\* |\d+\. )`)
	helpGap        = regexp.MustCompile(`\S( {2,})\S`)
	helpHeading    = regexp.MustCompile(`^([━─]+) (.+) ([━─]+)$`)
	helpCloseTag   = regexp.MustCompile(`^</ansi>`)
)

// ProcessHelp renders a help template for a reader whose screen is width
// columns wide, without the screen-reader cleanup (owner ruling 2026-11-12:
// that cleanup blanks "[", "]" and "+", which help uses for optional
// arguments and formulas), and lays it out to that width.
func ProcessHelp(fname string, data any, width int) (string, error) {
	out, err := ProcessForWidth(fname, data, false, width)
	if err != nil {
		return out, err
	}
	return LayoutHelp(out, width), nil
}

type helpBlock struct {
	kind  string // "para", "rows", "list", "keep" or "blank"
	lines []string
}

// LayoutHelp lays a rendered help page out to width columns (at most
// FullWidth). A paragraph or list whose lines all fit keeps its authored
// breaks; one that does not is rejoined and packed. A row always takes the
// same-line form: its label, then its text wrapping two columns in. A kept
// line is never touched. Marker lines are dropped.
func LayoutHelp(text string, width int) string {
	width = min(width, FullWidth)
	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	out := []string{}
	for _, b := range helpBlocks(lines) {
		switch b.kind {
		case "blank", "keep":
			out = append(out, b.lines...)
		case "para":
			out = append(out, layoutHelpPara(b.lines, width)...)
		case "list":
			out = append(out, layoutHelpEntries(b.lines, width, false)...)
		case "rows":
			out = append(out, layoutHelpEntries(b.lines, width, true)...)
		}
	}
	return strings.Join(out, "\n")
}

// helpBlocks splits lines into blocks: at blank lines, at marker lines, and
// after a paragraph line that ends in ":".
func helpBlocks(lines []string) []helpBlock {
	blocks := []helpBlock{}
	open := ""       // the open marker's kind, "" outside a block
	boundary := true // the next line starts a new block
	for _, line := range lines {
		if open == "" && (line == helpMarkRows || line == helpMarkList || line == helpMarkKeep) {
			open, boundary = strings.TrimPrefix(line, "::"), true
			continue
		}
		if open != "" && line == helpMarkEnd {
			open, boundary = "", true
			continue
		}
		if strings.TrimSpace(ansiTag.ReplaceAllString(line, "")) == "" {
			blocks = append(blocks, helpBlock{"blank", []string{line}})
			boundary = true
			continue
		}
		kind := open
		if kind == "" {
			kind = "para"
		}
		if boundary || blocks[len(blocks)-1].kind != kind {
			blocks = append(blocks, helpBlock{kind, nil})
		}
		blocks[len(blocks)-1].lines = append(blocks[len(blocks)-1].lines, line)
		boundary = kind == "para" && strings.HasSuffix(strings.TrimRight(helpVisible(line), " "), ":")
	}
	return blocks
}

func helpVisible(s string) string { return ansiTag.ReplaceAllString(s, "") }

func helpIndent(s string) int {
	v := helpVisible(s)
	return len(v) - len(strings.TrimLeft(v, " "))
}

func helpFits(lines []string, width int) bool {
	for _, l := range lines {
		if util.VisibleWidth(strings.TrimRight(l, " ")) > width {
			return false
		}
	}
	return true
}

func layoutHelpPara(lines []string, width int) []string {
	if helpFits(lines, width) {
		return lines
	}
	if len(lines) == 1 {
		if h, ok := shrinkHelpHeading(lines[0], width); ok {
			return []string{h}
		}
	}
	indent := helpIndent(lines[0])
	return packHelp(strings.Join(lines, " "), nil, indent, indent, width)
}

// shrinkHelpHeading drops rule marks from a "━━━ Title ━━━" heading until it
// fits, keeping its colour tags.
func shrinkHelpHeading(line string, width int) (string, bool) {
	runes := visibleRunes(line)
	if len(runes) == 0 {
		return "", false
	}
	v := string(func() []rune {
		r := make([]rune, len(runes))
		for i, x := range runes {
			r[i] = x.r
		}
		return r
	}())
	m := helpHeading.FindStringSubmatch(v)
	if m == nil {
		return "", false
	}
	mark := []rune(m[1])[0]
	before, after := line[:runes[0].start], line[runes[len(runes)-1].end:]
	for n := len([]rune(m[1])); n >= 0; n-- {
		cand := m[2]
		if n > 0 {
			cand = strings.Repeat(string(mark), n) + " " + m[2] + " " + strings.Repeat(string(mark), n)
		}
		if util.VisibleWidth(cand) <= width {
			return before + cand + after, true
		}
	}
	return "", false
}

// layoutHelpEntries lays out a ::rows or ::list block. An entry starts at the
// block's least indent; deeper lines continue it.
func layoutHelpEntries(lines []string, width int, rows bool) []string {
	if !rows && helpFits(lines, width) {
		return lines
	}
	base := helpIndent(lines[0])
	for _, l := range lines {
		base = min(base, helpIndent(l))
	}
	entries := [][]string{}
	for _, l := range lines {
		if helpIndent(l) == base || len(entries) == 0 {
			entries = append(entries, []string{l})
			continue
		}
		entries[len(entries)-1] = append(entries[len(entries)-1], l)
	}
	out := []string{}
	for _, e := range entries {
		head, rest := e[0], strings.Join(e[1:], " ")
		v := []rune(helpVisible(head))
		body := string(v[base:])
		var cut int // visible runes of head that make the label or marker
		if rows {
			m := helpGap.FindStringSubmatchIndex(body)
			if m == nil {
				out = append(out, packHelp(strings.Join(e, " "), nil, base, base+2, width)...)
				continue
			}
			cut = base + len([]rune(body[:m[2]]))
		} else {
			m := helpListMarker.FindString(body)
			if m == "" {
				out = append(out, packHelp(strings.Join(e, " "), nil, base, base+2, width)...)
				continue
			}
			cut = base + len([]rune(m)) - 1 // the marker without its space
		}
		label, text := cutHelpVisible(head, cut)
		text = strings.TrimSpace(text + " " + rest)
		label = trimHelpVisible(label)
		if !rows {
			hang := base + len([]rune(helpListMarker.FindString(body)))
			out = append(out, packHelp(text, []string{label}, base, hang, width)...)
			continue
		}
		var lead []string
		if util.VisibleWidth(label) <= width-base {
			lead = []string{protectHelpSpaces(label)}
		} else {
			lead = helpUnits(label, width-base-2)
		}
		if !strings.HasSuffix(helpVisible(label), ":") {
			lead[len(lead)-1] += helpJoin
		}
		out = append(out, packHelp(text, lead, base, base+2, width)...)
	}
	return out
}

// cutHelpVisible splits s after its first n visible runes. Closing tags right
// after the cut (past any spaces) go with the left part, so a label's own
// colour closes with it.
func cutHelpVisible(s string, n int) (string, string) {
	runes := visibleRunes(s)
	if n <= 0 {
		return "", s
	}
	if n >= len(runes) {
		return s, ""
	}
	at := runes[n-1].end
	for j := at; j < len(s); {
		if s[j] == ' ' {
			j++
			continue
		}
		if m := helpCloseTag.FindString(s[j:]); m != "" {
			j += len(m)
			at = j
			continue
		}
		break
	}
	return s[:at], s[at:]
}

// trimHelpVisible drops visible spaces at both ends of s, keeping its tags.
func trimHelpVisible(s string) string {
	runes := visibleRunes(s)
	what := make([]string, len(runes))
	first, last := -1, -1
	for i, v := range runes {
		what[i] = s[v.start:v.end]
		if v.r != ' ' {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	for i := range runes {
		if i < first || i > last {
			what[i] = ""
		}
	}
	return rebuild(s, runes, what)
}

// protectHelpSpaces joins every visible space of s, so s packs as one unit.
func protectHelpSpaces(s string) string {
	runes := visibleRunes(s)
	what := make([]string, len(runes))
	for i, v := range runes {
		what[i] = s[v.start:v.end]
		if v.r == ' ' {
			what[i] = helpJoin
		}
	}
	return rebuild(s, runes, what)
}

// helpUnits splits s into the units PackUnits places: at spaces, except
// inside a colour span no wider than limit, which stays one unit (a command
// or a name never splits while it fits a line, owner 2026-11-10). Tags with
// no visible text go with the next unit.
func helpUnits(s string, limit int) []string {
	runes := visibleRunes(s)
	what := make([]string, len(runes))
	for i, v := range runes {
		what[i] = s[v.start:v.end]
	}
	opens := []int{}
	for _, t := range ansiTag.FindAllStringIndex(s, -1) {
		if !strings.HasPrefix(s[t[0]:t[1]], "</") {
			opens = append(opens, t[1])
			continue
		}
		if len(opens) == 0 {
			continue
		}
		open := opens[len(opens)-1]
		opens = opens[:len(opens)-1]
		w, inside := 0, []int{}
		for i, v := range runes {
			if v.start >= open && v.end <= t[0] {
				w += util.VisibleWidth(string(v.r))
				inside = append(inside, i)
			}
		}
		if w == 0 || w > limit {
			continue
		}
		for _, i := range inside {
			if runes[i].r == ' ' {
				what[i] = helpJoin
			}
		}
	}
	units, pending := []string{}, ""
	for _, u := range splitOutsideTags(rebuild(s, runes, what)) {
		if util.VisibleWidth(u) == 0 {
			pending += u
			continue
		}
		units = append(units, pending+u)
		pending = ""
	}
	if pending != "" {
		if len(units) == 0 {
			return []string{pending}
		}
		units[len(units)-1] += pending
	}
	return units
}

// splitOutsideTags splits s at each ' ' that is not inside a colour tag (the
// space in `<ansi fg="x">` is part of the tag, not a break).
func splitOutsideTags(s string) []string {
	parts := []string{}
	start := 0
	for i := 0; i < len(s); {
		if s[i] == '<' {
			if loc := ansiTag.FindStringIndex(s[i:]); loc != nil && loc[0] == 0 {
				i += loc[1]
				continue
			}
		}
		if s[i] == ' ' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
		i++
	}
	return append(parts, s[start:])
}

// packHelp packs lead and the units of text into lines: the first indented
// by first, the rest by rest. WrapAnsi then closes and reopens colour at each
// line break, so every line stands alone.
func packHelp(text string, lead []string, first, rest, width int) []string {
	units := append(append([]string{}, lead...), helpUnits(text, width-rest)...)
	lines := messaging.PackUnits(units, first, rest, width)
	joined := messaging.WrapAnsi(strings.Join(lines, "\n"), width)
	out := strings.Split(strings.ReplaceAll(joined, helpJoin, " "), "\n")
	for i := range out {
		out[i] = strings.TrimRight(out[i], " ")
	}
	return out
}

package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

// Help files state their layout and the game never guesses (web client base
// plan 4, owner ruling 2026-11-12). Every line whose layout would otherwise
// be a guess must sit in a marked block. See internal/templates/helplayout.go
// for what the markers mean.

var (
	helpTag        = regexp.MustCompile(`</?ansi[^>]*>`)
	helpAction     = regexp.MustCompile(`^\s*(\{\{[^}]*\}\}\s*)+$`)
	helpGapRun     = regexp.MustCompile(`\S {2,}\S`)
	helpGapSpaces  = regexp.MustCompile(`\S( {2,})\S`)
	helpListStart  = regexp.MustCompile(`^(- |\* |\d+\. )`)
	helpLoneSpan   = regexp.MustCompile(`^\s*<ansi fg="(command|skill)">(.*)</ansi>\s*$`)
	helpRuleLine   = regexp.MustCompile(`[━─]`)
	helpHeadingVis = regexp.MustCompile(`^[━─]+ .+ [━─]+$`)
)

// helpLoneUnit reports a line that is only one command or skill span. The
// span's text may hold argument placeholders such as "<item>" or "<a|b>",
// which are not colour tags.
func helpLoneUnit(line string) bool {
	m := helpLoneSpan.FindStringSubmatch(line)
	return m != nil && !helpTag.MatchString(m[2])
}

// helpTextColumn is the visible column where a row's text starts: after its
// indent, its label and the first run of two or more spaces. -1 when the row
// has no such gap.
func helpTextColumn(indent int, body string) int {
	loc := helpGapSpaces.FindStringSubmatchIndex(body)
	if loc == nil {
		return -1
	}
	// loc[3] ends the run of spaces: the text's first byte.
	return indent + len([]rune(body[:loc[3]]))
}

// helpFormName is a help file that is not a page of its own: a narrow form
// (colors.w42) or a screen-reader file, which belong to their base page.
var helpFormName = regexp.MustCompile(`^(.+)\.(w(\d+)|screenreader)$`)

// helpFolderFiles walks the help folders once: the pages (help topics) and
// the form and screen-reader files beside them, each "help/<file name less
// .template>" to its path.
func helpFolderFiles(t *testing.T) (pages, forms map[string]string) {
	t.Helper()
	pages, forms = map[string]string{}, map[string]string{}
	for _, folder := range templateFolders(t) {
		matches, err := filepath.Glob(filepath.Join(folder, "help", "*.template"))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range matches {
			name := "help/" + strings.TrimSuffix(filepath.Base(m), ".template")
			if helpFormName.MatchString(name) {
				forms[name] = m
				continue
			}
			pages[name] = m
		}
	}
	if len(pages) < 480 {
		t.Fatalf("found %d help templates; the folders are not being reached", len(pages))
	}
	return pages, forms
}

// helpTemplateFiles are the help pages: every help file that is a topic.
func helpTemplateFiles(t *testing.T) map[string]string {
	t.Helper()
	pages, _ := helpFolderFiles(t)
	return pages
}

// helpPageForms lists, per page, the widths it has a form drawn to.
func helpPageForms(forms map[string]string) map[string][]int {
	have := map[string][]int{}
	for name := range forms {
		if m := helpFormName.FindStringSubmatch(name); m != nil && m[3] != "" {
			w, _ := strconv.Atoi(m[3])
			have[m[1]] = append(have[m[1]], w)
		}
	}
	return have
}

// helpShownWidth is the narrowest width a help file is ever shown at: the
// narrowest width templates' formFor (forms.go) maps to it. A page with
// forms is shown only at FullWidth and wider. A form is shown from its own
// width up to the next form, and the page's narrowest form below its own
// width too, down to the narrowest screen. A page with no forms, and a
// screen-reader file, is shown at the narrowest screen.
func helpShownWidth(name string, pageForms map[string][]int) int {
	if widths := pageForms[name]; len(widths) > 0 {
		return templates.FullWidth
	}
	m := helpFormName.FindStringSubmatch(name)
	if m == nil || m[3] == "" {
		return users.MinWrapWidth
	}
	own, _ := strconv.Atoi(m[3])
	for _, w := range pageForms[m[1]] {
		if w < own {
			return own
		}
	}
	return users.MinWrapWidth
}

// The shown width follows formFor: a page with forms at 80, a form at its
// own width when a narrower form exists, the narrowest form and a page with
// no forms at 30.
func TestHelpShownWidth(t *testing.T) {
	pageForms := map[string][]int{"help/a": {42, 30}, "help/b": {42}, "help/c": {42, 40, 30}}
	for name, want := range map[string]int{
		"help/a": 80, "help/a.w42": 42, "help/a.w30": 30,
		"help/b": 80, "help/b.w42": 30,
		"help/c.w42": 42, "help/c.w40": 40, "help/c.w30": 30,
		"help/plain": 30, "help/a.screenreader": 30,
	} {
		assert.Equal(t, want, helpShownWidth(name, pageForms), name)
	}
}

func TestHelpFilesStateTheirLayout(t *testing.T) {
	pages, forms := helpFolderFiles(t)
	pageForms := helpPageForms(forms)
	all := map[string]string{}
	for name, path := range pages {
		all[name] = path
	}
	for name, path := range forms {
		all[name] = path
	}
	for name, path := range all {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ReplaceAll(string(raw), "\r\n", "\n")
		if strings.Contains(text, "\r") {
			t.Errorf("%s: a carriage return is left after CRLF is removed", name)
		}
		if strings.Contains(text, " ") {
			t.Errorf("%s: a no-break space (the layout pass's own joiner)", name)
		}
		for _, p := range helpMarkupProblems(strings.Split(text, "\n"), helpShownWidth(name, pageForms)) {
			t.Errorf("%s: %s", name, p)
		}
	}
}

// helpMarkupProblems lists the layout problems in a help file's lines. Kept
// lines are judged at keepWidth, the narrowest width the file is shown at.
func helpMarkupProblems(lines []string, keepWidth int) []string {
	problems := []string{}
	add := func(n int, msg string) { problems = append(problems, fmt.Sprintf("line %d: %s", n+1, msg)) }
	open, base := "", -1
	textCol := -1                 // the text column of the ::rows entry in hand, -1 if none
	prevVis, prevIndent := "", -1 // the previous unmarked line in this run, "" after a blank
	for i, line := range lines {
		vis := helpTag.ReplaceAllString(line, "")
		trimmed := strings.TrimRight(vis, " ")
		switch {
		case line == "::rows" || line == "::list" || line == "::keep":
			if open != "" {
				add(i, "a block opens inside "+open)
			}
			open, base, prevVis, textCol = line, -1, "", -1
			continue
		case line == "::":
			if open == "" {
				add(i, `"::" closes no block`)
			}
			open, prevVis = "", ""
			continue
		case strings.HasPrefix(strings.TrimSpace(vis), "::"):
			add(i, "an unknown marker or a marker not at column 0")
			continue
		}
		if helpAction.MatchString(line) {
			continue // a template action alone prints no line
		}
		if strings.TrimSpace(vis) == "" {
			prevVis, prevIndent = "", -1
			continue
		}
		indent := len(vis) - len(strings.TrimLeft(vis, " "))
		body := strings.TrimLeft(vis, " ")
		switch open {
		case "::rows", "::list":
			if base < 0 {
				base = indent
			}
			if indent < base {
				add(i, "a line left of its block's first entry")
			}
			if indent == base {
				if open == "::rows" {
					textCol = helpTextColumn(indent, body)
					if textCol < 0 {
						add(i, "a row with no gap of two or more spaces between label and text")
					}
				}
				if open == "::list" && !helpListStart.MatchString(body) {
					add(i, `a list entry without "- ", "* " or "1. "`)
				}
			} else if open == "::rows" && textCol >= 0 && indent != textCol {
				// A continuation starts exactly at its entry's text column, so a
				// line meant as its own row or sentence is never glued on.
				add(i, fmt.Sprintf("a row continuation at column %d, not its entry's text column %d", indent, textCol))
			}
			continue
		case "::keep":
			if len([]rune(trimmed)) > keepWidth {
				add(i, fmt.Sprintf("a kept line wider than %d columns", keepWidth))
			}
			continue
		}
		// Unmarked: paragraphs only.
		if helpRuleLine.MatchString(vis) {
			if indent != 0 || !helpHeadingVis.MatchString(trimmed) {
				add(i, "a rule line that is not a heading")
			}
			if prevVis != "" || (i+1 < len(lines) && strings.TrimSpace(helpTag.ReplaceAllString(lines[i+1], "")) != "") {
				add(i, "a heading not alone between blank lines")
			}
		}
		if helpGapRun.MatchString(body) {
			add(i, "two or more spaces inside a line: mark it ::rows or ::keep")
		}
		if helpListStart.MatchString(body) {
			add(i, "a list marker: mark it ::list")
		}
		// A line that is only a command is ambiguous only when it shares its
		// paragraph: it would be joined to the line above or below. Alone
		// between blank lines (or markers) it is a one-line paragraph and
		// nothing can join it, however wide. A lone command after a line
		// ending in "," finishes that sentence.
		if helpLoneUnit(line) {
			joinsAbove := prevVis != "" && !strings.HasSuffix(strings.TrimRight(prevVis, " "), ",")
			joinsBelow := false
			if i+1 < len(lines) {
				next := strings.TrimRight(lines[i+1], " \t")
				nextVis := strings.TrimSpace(helpTag.ReplaceAllString(next, ""))
				joinsBelow = nextVis != "" && !strings.HasPrefix(next, "::") && !helpAction.MatchString(next)
			}
			if joinsAbove || joinsBelow {
				add(i, "a line that is only a command, inside a paragraph: mark it ::keep, or give it blank lines")
			}
		}
		if prevVis != "" && indent != prevIndent && !strings.HasSuffix(strings.TrimRight(prevVis, " "), ":") {
			add(i, "the indent changes inside a paragraph: mark the block, or end the line above with \":\"")
		}
		prevVis, prevIndent = vis, indent
	}
	if open != "" {
		add(len(lines)-1, open+" is never closed")
	}
	return problems
}

func TestHelpMarkupRulesCanFail(t *testing.T) {
	bad := map[string][]string{
		"gap":          {"  look      Look at the room."},
		"list":         {"- an item"},
		"lone command": {`  <ansi fg="command">quaff poultice</ansi>`, `  <ansi fg="command">quaff draught</ansi>`},
		"indent":       {"A paragraph line", "  indented after it"},
		"heading":      {"Text", "━━━ Title ━━━"},
		"open":         {"::rows", "  a  b"},
		"close":        {"::"},
		"nest":         {"::rows", "::keep", "::", "::"},
		"row gap":      {"::rows", "  a b", "::"},
		"list marker":  {"::list", "  item", "::"},
		"keep width":   {"::keep", strings.Repeat("x", 31), "::"},
		// A line in a rows block that is not at its entry's text column: a
		// sentence or a row of its own, never a continuation.
		"row continuation left of text":  {"::rows", `  <ansi fg="command">flee</ansi>  Attempt to escape`, "    Victory - Kill all opponents", "::"},
		"row continuation right of text": {"::rows", "  look  Look at", "          the room.", "::"},
		"row continuation after blank":   {"::rows", "  look  Look at the room.", "", "    It is dark.", "::"},
		"lone command with argument":     {`  <ansi fg="command">set x <a|b></ansi>`, `  <ansi fg="command">set y</ansi>`},
		"lone command above a sentence":  {`<ansi fg="command">help x</ansi>`, "Shows help."},
		"lone command after a sentence":  {"See this.", `<ansi fg="command">help x</ansi>`},
	}
	for name, lines := range bad {
		assert.NotEmpty(t, helpMarkupProblems(lines, 30), name)
	}
	good := []string{
		".: Help for look", "", "The look command shows you what is", "around you.", "", "Usage:",
		"::rows", "  look              Look at the room.", "  look [thing]      Look closely at a",
		"                    colored word.", "::", "::keep", "  quaff poultice", "  quaff draught", "::",
		"::list", "  - Heavy armor (reduces mobility)", "  - Being prone", "::",
		"",
		// A continuation at its entry's text column, after a widened colon label.
		"::rows", "  Note:  Only one number", "         sets the colour.", "::",
		// The text column counts runes: text that starts with "—" is one column.
		"::rows", "  ☺ mercy  — Release them", "           cleanly.", "::",
		// A kept command whose span holds an argument placeholder.
		"::keep", `  <ansi fg="command">set x <a|b></ansi>`, "::",
		"",
		// A lone command that finishes a sentence ending in ",".
		`See <ansi fg="command">help a</ansi>, <ansi fg="command">help b</ansi>,`,
		`<ansi fg="command">help c</ansi>`,
		"",
		// A usage line wider than 30, alone between blank lines: a one-line
		// paragraph that nothing can join.
		`  <ansi fg="command">cast mutation-catalyst [target]</ansi>`,
		"",
		"Explains the command.",
	}
	assert.Empty(t, helpMarkupProblems(good, 30))
	// A kept line is judged at the width the file is shown at: 42 wide passes
	// a file shown at 42 and fails one shown at 30.
	kept42 := []string{"::keep", strings.Repeat("x", 42), "::"}
	assert.Empty(t, helpMarkupProblems(kept42, 42))
	assert.NotEmpty(t, helpMarkupProblems(kept42, 30))
}

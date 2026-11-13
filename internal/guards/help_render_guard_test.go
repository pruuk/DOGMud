package guards

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/stretchr/testify/assert"
)

// helpRenderSetup renders help as the server does: the live world's
// templates and every module's, the real species and spells. osPathFS lets
// os.DirFS read the backslash paths util.FilePath builds on Windows.
func helpRenderSetup(t *testing.T) {
	t.Helper()
	templateRenderSetup(t)
	fss := []fs.ReadFileFS{osPathFS{os.DirFS(dogmudDataFilesRoot)}}
	mods, _ := filepath.Glob(filepath.Join("modules", "*", "files", "datafiles"))
	for _, m := range mods {
		fss = append(fss, osPathFS{os.DirFS(m)})
	}
	templates.SetFSForTest(t, fss...)
	// The shipped help aliases (race and races mean species).
	t.Cleanup(keywords.SeedKeywordsForTest())
	keywords.LoadAliases()
	species.LoadForTest(t)
	spells.LoadSpellFiles()
}

// helpIndexEntry and helpIndexData mirror what Help() passes the bare `help`
// index: category maps of commands, one of them missing its page.
type helpIndexEntry struct {
	Command string
	Type    string
	Missing bool
}

type helpIndexData struct {
	Commands map[string][]helpIndexEntry
	Skills   map[string][]helpIndexEntry
	Admin    map[string][]helpIndexEntry
}

func helpIndexFixture() helpIndexData {
	return helpIndexData{
		Commands: map[string][]helpIndexEntry{
			"":       {{Command: "look", Type: "command"}, {Command: "inventory", Type: "command"}, {Command: "unwritten-page", Type: "command", Missing: true}},
			"combat": {{Command: "attack", Type: "command"}, {Command: "flee", Type: "command"}},
		},
		Skills: map[string][]helpIndexEntry{
			"": {{Command: "search", Type: "skill"}, {Command: "picklock", Type: "skill"}},
		},
		Admin: map[string][]helpIndexEntry{
			"": {{Command: "spawn", Type: "command-admin"}},
		},
	}
}

// helpTopics is every help file's topic, plus the topics whose page needs
// data its name alone does not give. The bare "help" topic is the index,
// which Help() renders with category maps, not through GetHelpContents; it
// is held separately (TestHelpIndexFitsEveryWidth).
func helpTopics(t *testing.T) []string {
	topics := []string{"spell blood-boil", "species all"}
	for name := range helpTemplateFiles(t) {
		if name == "help/help" {
			continue
		}
		topics = append(topics, strings.TrimPrefix(name, "help/"))
	}
	sort.Strings(topics)
	return topics
}

func TestEveryHelpPageFitsEveryWidth(t *testing.T) {
	helpRenderSetup(t)
	for _, topic := range helpTopics(t) {
		for _, width := range templateWidths {
			out, err := usercommands.GetHelpContents(topic, width)
			if err != nil {
				t.Errorf("help %s at %d: %v", topic, width, err)
				continue
			}
			if topic == "spell blood-boil" && !strings.Contains(out, "Blood Boil") {
				t.Errorf("help %s at %d did not render the spell page:\n%s", topic, width, out)
			}
			if got := templates.WidestLine(out); got > width {
				t.Errorf("help %s at %d has a line %d wide:\n%s", topic, width, got, out)
			}
		}
	}
}

func TestHelpIndexFitsEveryWidth(t *testing.T) {
	helpRenderSetup(t)
	for _, width := range templateWidths {
		out, err := templates.ProcessHelp("help/help", helpIndexFixture(), width)
		if err != nil {
			t.Fatalf("help index at %d: %v", width, err)
		}
		if !strings.Contains(out, "unwritten-page") {
			t.Errorf("help index at %d lost the missing entry:\n%s", width, out)
		}
		if got := templates.WidestLine(out); got > width {
			t.Errorf("help index at %d has a line %d wide:\n%s", width, got, out)
		}
	}
}

// The layout pass moves line breaks and drops marker lines; it never loses,
// adds or reorders a word (owner ruling 2026-11-12).
//
// Each file is rendered at the width it is shown at, so every file is
// checked: a page with forms at FullWidth (its base), each form at its own
// width (formFor picks that form there), every other page at 30.
func TestHelpLayoutKeepsEveryWord(t *testing.T) {
	helpRenderSetup(t)
	pages, forms := helpFolderFiles(t)
	pageForms := helpPageForms(forms)
	type shown struct {
		page  string
		width int
	}
	files := map[string]shown{}
	for name := range pages {
		files[name] = shown{name, 30}
		if len(pageForms[name]) > 0 {
			files[name] = shown{name, templates.FullWidth}
		}
	}
	for name := range forms {
		if m := helpFormName.FindStringSubmatch(name); m != nil && m[3] != "" {
			w, _ := strconv.Atoi(m[3])
			files[name] = shown{m[1], w}
		}
	}
	rendered := 0
	for name, at := range files {
		var data any
		if at.page == "help/help" {
			data = helpIndexFixture()
		}
		raw, err := templates.ProcessForWidth(at.page, data, false, at.width)
		if err != nil {
			continue // a page that needs data; its words are held through GetHelpContents above
		}
		rendered++
		for _, width := range templateWidths {
			laid := templates.LayoutHelp(raw, width)
			if !helpWordsKept(helpWords(raw), helpWords(laid), width) {
				t.Errorf("%s at %d: words differ after layout", name, width)
			}
		}
	}
	if rendered < 400 {
		t.Errorf("only %d help pages rendered without data; the word check is not reaching them", rendered)
	}
}

// The word check must be able to fail: a laid-out string that drops a word,
// or swaps two, reads differently from the raw one.
func TestHelpWordsCanFail(t *testing.T) {
	raw := "::rows\n  look  Look at the room.\n::\n"
	good := "  look  Look at\n        the room."
	assert.Equal(t, helpWords(raw), helpWords(good))
	assert.NotEqual(t, helpWords(raw), helpWords("  look  Look at\n        room."), "a dropped word must differ")
	assert.NotEqual(t, helpWords(raw), helpWords("  look  at Look the room."), "a reordered word must differ")

	// The comparison: a token that fits the line must come through whole; only
	// a token wider than the line may be broken at the edge.
	words := func(s string) []string { return strings.Fields(s) }
	assert.True(t, helpWordsKept(words("Help for master's component case"), words("Help for master's component case"), 30))
	assert.False(t, helpWordsKept(words("Help for master's component case"), words("Help for master's component ca se"), 30), "a split of a fitting token must fail")
	assert.False(t, helpWordsKept(words("a b c"), words("a c"), 30), "a dropped word must fail")
	assert.False(t, helpWordsKept(words("a b"), words("a b c"), 30), "an added word must fail")
	assert.False(t, helpWordsKept(words("a b"), words("b a"), 30), "a reordered word must fail")
	long := "(light/moderate/heavy/overburdened/crushed)"
	assert.True(t, helpWordsKept(words("tier "+long+" x"), words("tier (light/moderate/heavy/ov erburdened/crushed) x"), 30), "a split of an over-wide token must pass")
	assert.False(t, helpWordsKept(words("tier "+long+" x"), words("tier (light/moderate/heavy/ov erburdened/crushe x"), 30), "an over-wide token that loses letters must fail")
}

// helpWordsKept reports whether laid carries raw's tokens in order. A raw
// token that fits width must equal the next laid token exactly; one wider
// than width cannot fit, so the layout breaks it at the edge and it may be
// matched by consecutive laid tokens that join back to it exactly.
func helpWordsKept(raw, laid []string, width int) bool {
	j := 0
	for _, tok := range raw {
		if j >= len(laid) {
			return false
		}
		if templates.WidestLine(tok) <= width {
			if laid[j] != tok {
				return false
			}
			j++
			continue
		}
		joined := ""
		for j < len(laid) && len(joined) < len(tok) {
			joined += laid[j]
			j++
		}
		if joined != tok {
			return false
		}
	}
	return j == len(laid)
}

func helpWords(s string) []string {
	keep := []string{}
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		switch l {
		case "::rows", "::list", "::keep", "::":
			continue
		}
		keep = append(keep, l)
	}
	// Rule marks are not words: a heading too wide drops them (decision 7).
	words := []string{}
	for _, w := range strings.Fields(helpTag.ReplaceAllString(strings.Join(keep, "\n"), "")) {
		if strings.Trim(w, "━─") != "" {
			words = append(words, w)
		}
	}
	return words
}

// helpKeptBlocks are the ::keep blocks of a help file's source, each its lines.
func helpKeptBlocks(t *testing.T, path string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	blocks := [][]string{}
	var cur []string
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		switch {
		case line == "::keep":
			cur = []string{}
		case line == "::" && cur != nil:
			blocks = append(blocks, cur)
			cur = nil
		case cur != nil:
			cur = append(cur, line)
		}
	}
	return blocks
}

var helpSwatch = regexp.MustCompile(`<ansi fg="(\d+)" bg="(\d+)"> ?(\d+)</ansi>`)

// helpSwatchGrids are the colour grids of a help colors file: per kept
// block, per line, each swatch as "fg/bg/number".
func helpSwatchGrids(t *testing.T, path string) [][][]string {
	grids := [][][]string{}
	for _, block := range helpKeptBlocks(t, path) {
		grid := [][]string{}
		for _, line := range block {
			row := []string{}
			for _, m := range helpSwatch.FindAllStringSubmatch(line, -1) {
				row = append(row, m[1]+"/"+m[2]+"/"+m[3])
			}
			grid = append(grid, row)
		}
		grids = append(grids, grid)
	}
	return grids
}

// helpSwatchParity reports how form's grids differ from base's: each base
// grid, in order, must be matched by the form's next grid, or by several
// grids of as many rows joined row by row (a grid split in halves one under
// the other), swatch for swatch. "" when they hold the same.
func helpSwatchParity(base, form [][][]string) string {
	j := 0
	for i, want := range base {
		if j >= len(form) {
			return fmt.Sprintf("base grid %d has no form grid", i+1)
		}
		got := make([][]string, len(form[j]))
		for r := range form[j] {
			got[r] = append([]string{}, form[j][r]...)
		}
		j++
		for len(got) > 0 && len(got[0]) < len(want[0]) && j < len(form) && len(form[j]) == len(got) {
			for r := range got {
				got[r] = append(got[r], form[j][r]...)
			}
			j++
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			return fmt.Sprintf("grid %d differs:\n got %v\nwant %v", i+1, got, want)
		}
	}
	if j != len(form) {
		return fmt.Sprintf("the form has %d grids past the base's", len(form)-j)
	}
	return ""
}

// The colours page's narrow forms keep every swatch: each form holds the
// base's numbers and colours in the same order in every grid row, the .w30's
// halves joined back (plan 5, Task 3).
func TestHelpColorsFormsKeepEverySwatch(t *testing.T) {
	pages, forms := helpFolderFiles(t)
	base := helpSwatchGrids(t, pages["help/colors"])
	if len(base) < 4 {
		t.Fatalf("found %d colour grids in help/colors; the parse is not reaching them", len(base))
	}
	checked := 0
	for name, path := range forms {
		if !strings.HasPrefix(name, "help/colors.w") {
			continue
		}
		checked++
		if msg := helpSwatchParity(base, helpSwatchGrids(t, path)); msg != "" {
			t.Errorf("%s: %s", name, msg)
		}
	}
	if checked < 2 {
		t.Errorf("checked %d colours forms, want the .w42 and the .w30", checked)
	}
}

// helpMapRule is a kept line that is only a rule: the narrow map's frame.
var helpMapRule = regexp.MustCompile(`^─+$`)

// helpBaseMaps are the map columns of each framed example map in the base
// page: characters 8 to 72 of each framed row, tags removed.
func helpBaseMaps(t *testing.T, path string) [][][]rune {
	maps := [][][]rune{}
	for _, block := range helpKeptBlocks(t, path) {
		rows := [][]rune{}
		for _, line := range block {
			vis := []rune(helpTag.ReplaceAllString(line, ""))
			if len(vis) != 79 || strings.Contains(string(vis), "-._.-=") {
				continue
			}
			if edge := string(vis[:7]); edge != "( _ __)" && edge != "(__  _)" {
				continue
			}
			rows = append(rows, vis[7:72])
		}
		if len(rows) > 0 {
			maps = append(maps, rows)
		}
	}
	return maps
}

// helpFormMaps are each kept block's lines between its two rule lines, tags
// removed: the narrow form's map rows.
func helpFormMaps(t *testing.T, path string) [][][]rune {
	maps := [][][]rune{}
	for _, block := range helpKeptBlocks(t, path) {
		var rows [][]rune
		for _, line := range block {
			vis := helpTag.ReplaceAllString(line, "")
			if helpMapRule.MatchString(vis) {
				if rows != nil {
					maps = append(maps, rows)
					break
				}
				rows = [][]rune{}
				continue
			}
			if rows != nil {
				rows = append(rows, []rune(vis))
			}
		}
	}
	return maps
}

// helpMapSlice reports how a form's maps differ from the base's: each form
// map, width columns wide, must be the base map's rows cut to width around
// the "@", which sits at width/2 as the mapper centres it (GetLimitedMap).
// "" when they match.
func helpMapSlice(base, form [][][]rune, width int) string {
	if len(form) != len(base) {
		return fmt.Sprintf("%d maps, the base has %d", len(form), len(base))
	}
	for m := range base {
		if len(form[m]) != len(base[m]) {
			return fmt.Sprintf("map %d has %d rows, the base %d", m+1, len(form[m]), len(base[m]))
		}
		at := -1
		for _, row := range base[m] {
			if i := strings.IndexRune(string(row), '@'); i >= 0 {
				at = len([]rune(string(row)[:i]))
			}
		}
		start := at - width/2
		if at < 0 || start < 0 || start+width > len(base[m][0]) {
			return fmt.Sprintf("map %d: no %d-column cut around the @ at %d", m+1, width, at)
		}
		for r := range base[m] {
			want := strings.TrimRight(string(base[m][r][start:start+width]), " ")
			if got := strings.TrimRight(string(form[m][r]), " "); got != want {
				return fmt.Sprintf("map %d row %d:\n got %q\nwant %q", m+1, r+1, got, want)
			}
		}
	}
	return ""
}

// The map examples' narrow forms draw the base's maps the way the narrow
// map command does: each row cut to the form's width around the "@" (plan
// 5, Task 3).
func TestHelpMapExamplesFormsCutTheBaseMaps(t *testing.T) {
	pages, forms := helpFolderFiles(t)
	base := helpBaseMaps(t, pages["help/map-examples"])
	if len(base) != 2 {
		t.Fatalf("found %d framed maps in help/map-examples, want 2; the parse is not reaching them", len(base))
	}
	checked := 0
	for name, path := range forms {
		m := helpFormName.FindStringSubmatch(name)
		if m == nil || m[1] != "help/map-examples" || m[3] == "" {
			continue
		}
		checked++
		width, _ := strconv.Atoi(m[3])
		if msg := helpMapSlice(base, helpFormMaps(t, path), width); msg != "" {
			t.Errorf("%s: %s", name, msg)
		}
	}
	if checked < 2 {
		t.Errorf("checked %d map-examples forms, want the .w42 and the .w30", checked)
	}
}

// HELP_DUMP=<folder> writes every page at 30 and 80 for the owner's report
// (Task 7). It checks nothing.
func TestDumpHelpPages(t *testing.T) {
	dir := os.Getenv("HELP_DUMP")
	if dir == "" {
		t.Skip("set HELP_DUMP to a folder to write the report pages")
	}
	helpRenderSetup(t)
	for _, topic := range helpTopics(t) {
		for _, width := range []int{30, 80} {
			out, _ := usercommands.GetHelpContents(topic, width)
			name := strings.ReplaceAll(topic, " ", "_") + "." + map[int]string{30: "w30", 80: "w80"}[width] + ".txt"
			if err := os.WriteFile(filepath.Join(dir, name), []byte(out), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

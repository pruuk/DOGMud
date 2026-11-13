package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
			if helpWide["help/"+topic] {
				continue
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
func TestHelpLayoutKeepsEveryWord(t *testing.T) {
	helpRenderSetup(t)
	rendered := 0
	for name := range helpTemplateFiles(t) {
		var data any
		if name == "help/help" {
			data = helpIndexFixture()
		}
		raw, err := templates.ProcessForWidth(name, data, false, 30)
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

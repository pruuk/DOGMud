package dialogue

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

var helpTagRe = regexp.MustCompile(`<[^>]+>`)

// helpParagraphs returns the help template's paragraphs as plain text, one
// line each.
func helpParagraphs(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "world", "dogmud", "templates", "help", name+".template"))
	require.NoError(t, err)
	var out []string
	for _, p := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n\n") {
		out = append(out, strings.Join(strings.Fields(helpTagRe.ReplaceAllString(p, "")), " "))
	}
	return out
}

func paragraphsWith(paras []string, word string) string {
	var hit []string
	for _, p := range paras {
		if strings.Contains(p, word) {
			hit = append(hit, p)
		}
	}
	return strings.Join(hit, "\n")
}

// TestHelp_HandInCopyMatchesTheTopics ties `help talk` and `help quests` to
// the shipped dialogue (playtest finding, #375). Some quest steps are
// reported through a marked info line ([*]), not a [?] hand-in line: Velk's
// protection notice for quest 10 is one. And a line that moves a quest on
// drops out of the list once its step is done (its questExcluded names its
// own grant), so "a heard line stays" is not the whole story. While such
// topics ship, each help file must say both.
func TestHelp_HandInCopyMatchesTheTopics(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "_datafiles", "world", "dogmud", "dialogue", "*", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "the glob found no dialogue files, so the check below could not fire")

	markedStep, selfExcluding := "", ""
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		var df DialogueFile
		if yaml.Unmarshal(data, &df) != nil {
			continue
		}
		for _, tp := range df.Topics {
			if tp.Kind == KindInfo && tp.Marked && tp.GrantsQuest != "" && markedStep == "" {
				markedStep = f + " " + tp.Key
			}
			for _, ex := range tp.QuestExcluded {
				if tp.GrantsQuest != "" && ex == tp.GrantsQuest && selfExcluding == "" {
					selfExcluding = f + " " + tp.Key
				}
			}
		}
	}
	require.NotEmpty(t, markedStep, "no shipped [*] quest-step topic: the copy rule below has nothing to pin")
	require.NotEmpty(t, selfExcluding, "no shipped topic hides itself after its step: the copy rule below has nothing to pin")

	for _, name := range []string{"talk", "quests"} {
		paras := helpParagraphs(t, name)
		// The paragraphs that say how a quest is handed in.
		handIn := paragraphsWith(paras, "hand")
		require.NotEmpty(t, handIn, "help %s has no hand-in paragraph", name)
		require.Contains(t, handIn, "[*]",
			"help %s says a quest is finished through [?] only; %s reports a step through a [*] line", name, markedStep)
	}
	talk := helpParagraphs(t, "talk")
	heard := paragraphsWith(talk, "already heard")
	require.Contains(t, heard, "goes away",
		"help talk says every heard line stays; %s drops out once its step is done", selfExcluding)
}

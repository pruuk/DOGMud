package usercommands

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// Owner ruling (2026-11-16): a help page answers to every name its command
// answers to. `quest` runs `quests`, so `help quest` opens the quests page,
// and the same holds for every command alias (inv, mutation, wear, ...).
// A word with its own page or its own help alias keeps it: `help exp` is
// the progression page, not the skills command's.
func TestHelpAnswersToEveryCommandAlias(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	useDogmudTemplates(t)

	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "keywords.yaml"))
	require.NoError(t, err)
	var shipped keywords.Aliases
	require.NoError(t, yaml.Unmarshal(raw, &shipped))
	t.Cleanup(keywords.SeedKeywordsForTest(shipped))

	quests, err := GetHelpContents("quests", 80)
	require.NoError(t, err)
	quest, err := GetHelpContents("quest", 80)
	require.NoError(t, err, "help quest")
	require.Equal(t, quests, quest, "help quest opens the quests page")

	checked := 0
	for target, aliases := range shipped.CommandAliases {
		command, _, _ := strings.Cut(target, " ")
		if !templates.Exists("help/" + command) {
			continue // the command has no page to open
		}
		want, err := GetHelpContents(command, 80)
		if err != nil {
			continue // a page that needs the caller's data (help/help's command list)
		}
		for _, alias := range aliases {
			if keywords.TryHelpAlias(alias) != alias || templates.Exists("help/"+alias) {
				continue // its own help alias or page wins
			}
			if strings.Trim(alias, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" {
				continue // punctuation ('.', '=?', '/w') is cleaned before lookup
			}
			got, err := GetHelpContents(alias, 80)
			require.NoError(t, err, "help %s (alias of %s)", alias, target)
			require.Equal(t, want, got, "help %s should open help %s", alias, command)
			checked++
		}
	}
	require.Greater(t, checked, 10, "the guard must actually check aliases")
}

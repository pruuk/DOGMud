package templates

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const u8DataFilesRoot = `../../_datafiles/world/dogmud`

var u8ActionHelpPaths = []string{
	"help/combat",
	"help/stamina",
	"help/ranged-combat",
	"help/shoot",
	"help/reload",
	"help/grapple",
	"help/sneak",
	"help/throw",
	"help/taunt",
	"help/rally",
	"help/warcry",
	"help/quell",
	"help/defy",
	"help/weapon-combat",
	"help/unarmed-combat",
	"help/bash",
	"help/trip",
	"help/kick",
	"help/rake",
	"help/maul",
	"help/pounce",
	"help/gore",
	"help/drain",
	"help/throttle",
	"help/special",
	"help/conviction",
}

var ansiTagPattern = regexp.MustCompile(`</?ansi(?:\s+[^>]*)?>`)

func useU8DataFiles(t *testing.T) {
	t.Helper()

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(u8DataFilesRoot)
	configs.SetConfigForTest(t, cfg)

	// TestMain registers the default world's filesystem. Force Process through
	// its configured OS path so a same-named default template cannot mask the
	// DOGMud template this acceptance test is meant to exercise.
	originalFileSystems := fileSystems
	fileSystems = []fs.ReadFileFS{emptyFS{}}
	t.Cleanup(func() { fileSystems = originalFileSystems })
}

func processU8Help(t *testing.T, path string) string {
	t.Helper()
	result, err := Process(path, nil)
	require.NoError(t, err, "Process(%q) must load the required DOGMud template", path)
	require.NotContains(t, result, "[TEMPLATE ERROR]", path)
	require.NotEmpty(t, strings.TrimSpace(result), path)
	return result
}

func normalizedHelpText(rendered string) string {
	withoutTags := ansiTagPattern.ReplaceAllString(rendered, "")
	return strings.ToLower(strings.Join(strings.Fields(withoutTags), " "))
}

// Catches a required action-cost helpfile being deleted, renamed, malformed,
// or accidentally satisfied by a same-named template from the default world.
func TestU8ActionAdmissionHelpTemplatesProcess(t *testing.T) {
	useU8DataFiles(t)

	for _, path := range u8ActionHelpPaths {
		t.Run(path, func(t *testing.T) {
			processU8Help(t, path)
		})
	}
}

// Catches the player help reverting to the old policy: voluntary actions must
// pay in full, while the four life-preserving families still resolve without
// their governing skill when short. The literal phrases are authored here,
// independently of the templates under test.
func TestU8ActionAdmissionHelpStatesExactPolicyWithoutTuning(t *testing.T) {
	useU8DataFiles(t)

	expectations := map[string][]string{
		"help/combat":         {"voluntary actions require full payment", "desperate form without the governing skill"},
		"help/stamina":        {"load raises the price", "without their governing skill"},
		"help/ranged-combat":  {"shooting and reloading spend stamina", "require full payment"},
		"help/shoot":          {"spends stamina", "requires full payment"},
		"help/reload":         {"physical exertion that spends stamina", "requires full payment"},
		"help/grapple":        {"initial grapple is a special move", "both participants pay upkeep", "without unarmed combat skill"},
		"help/sneak":          {"spends stamina", "requires full payment"},
		"help/throw":          {"spends stamina", "requires full payment"},
		"help/taunt":          {"spends conviction", "requires full payment"},
		"help/rally":          {"spends conviction", "requires full payment"},
		"help/warcry":         {"spends conviction", "requires full payment"},
		"help/quell":          {"spends conviction", "without spellcasting skill"},
		"help/defy":           {"spends conviction", "without rhetoric skill"},
		"help/weapon-combat":  {"load raises the stamina price", "skill lowers it"},
		"help/unarmed-combat": {"load raises the stamina price", "skill lowers it"},
		"help/bash":           {"spends stamina", "requires full payment"},
		"help/trip":           {"spends stamina", "requires full payment"},
		"help/kick":           {"spends stamina", "requires full payment"},
		"help/rake":           {"spends stamina", "requires full payment"},
		"help/maul":           {"spends stamina", "requires full payment"},
		"help/pounce":         {"spends stamina", "requires full payment"},
		"help/gore":           {"spends stamina", "requires full payment"},
		"help/drain":          {"spends stamina", "requires full payment"},
		"help/throttle":       {"spends stamina", "requires full payment"},
		"help/special":        {"physical moves spend stamina", "rhetoric moves spend conviction", "require full payment"},
		"help/conviction":     {"rhetoric actions require full payment", "quell and defy remain possible"},
	}

	for _, path := range u8ActionHelpPaths {
		t.Run(path, func(t *testing.T) {
			rendered := normalizedHelpText(processU8Help(t, path))
			for _, phrase := range expectations[path] {
				assert.Contains(t, rendered, phrase)
			}
		})
	}

	forbiddenTuning := regexp.MustCompile(`(?i)(?:\b\d+(?:\.\d+)?\s*%|\b\d+(?:\.\d+)?[- ]rounds?\b|\b\d+(?:\.\d+)?\s+(?:points?|ranks?|modifiers?)\b)`)
	for _, path := range u8ActionHelpPaths {
		t.Run(path+"/no-raw-tuning", func(t *testing.T) {
			rendered := processU8Help(t, path)
			assert.Empty(t, forbiddenTuning.FindAllString(rendered, -1),
				"player help must describe costs and timing without raw tuning")
		})
	}
}

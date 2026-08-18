package templates

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
var helpCrossReferencePattern = regexp.MustCompile(`(?i)<ansi\s+fg="command"[^>]*>\s*help\s+([a-z0-9-]+)\s*</ansi>`)
var forbiddenU8NumericTuningPattern = regexp.MustCompile(`(?i)(?:\b\d+(?:\.\d+)?\s*%|\b\d+(?:\.\d+)?[- ]rounds?\b|\b\d+(?:\.\d+)?\s+(?:points?|ranks?|modifiers?)\b)`)
var forbiddenU8WordedTuningPattern = regexp.MustCompile(`(?i)(?:\b(?:zero|one|two|three|four|five|six|seven|eight|nine|ten|half|quarter|first|second|third)\s+(?:percent|points?|ranks?|modifiers?)\b|\b(?:half|quarter)\s+(?:damage|armou?r)\b|\bfirst pound\b|\bunder half\b|\b(?:novice|master(?:ful)?)\s+(?:pays?|costs?)\b)`)

func useU8DataFiles(t *testing.T) {
	t.Helper()
	useU8DataFilesAt(t, u8DataFilesRoot)
}

func useU8DataFilesAt(t *testing.T, dataRoot string) {
	t.Helper()

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(dataRoot)
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

func findForbiddenU8HelpDisclosures(rendered string) []string {
	visible := normalizedHelpText(rendered)
	disclosures := make([]string, 0)
	if strings.Contains(visible, "—") {
		disclosures = append(disclosures, "—")
	}
	disclosures = append(disclosures, forbiddenU8NumericTuningPattern.FindAllString(visible, -1)...)
	disclosures = append(disclosures, forbiddenU8WordedTuningPattern.FindAllString(visible, -1)...)
	return disclosures
}

func u8HelpCrossReferenceTopics(rendered string) []string {
	matches := helpCrossReferencePattern.FindAllStringSubmatch(rendered, -1)
	topics := make([]string, 0, len(matches))
	for _, match := range matches {
		topics = append(topics, strings.ToLower(match[1]))
	}
	return topics
}

func validateU8HelpCrossReferences(sourcePath, rendered string) error {
	for _, topic := range u8HelpCrossReferenceTopics(rendered) {
		result, err := Process("help/"+topic, nil)
		if err != nil {
			return fmt.Errorf("%s emits broken cross-reference %q: %w", sourcePath, "help "+topic, err)
		}
		if strings.TrimSpace(result) == "" || strings.Contains(result, "[TEMPLATE") {
			return fmt.Errorf("%s emits broken cross-reference %q", sourcePath, "help "+topic)
		}
	}
	return nil
}

func TestU8HelpTuningGuardInspectsStyledVisibleText(t *testing.T) {
	styledDisclosure := `Cost: <ansi fg="yellow">one </ansi><ansi fg="skill">rank</ansi>; still.`
	rawDisclosurePattern := regexp.MustCompile(`(?i)\bone\s+ranks?\b`)
	require.Empty(t, rawDisclosurePattern.FindAllString(styledDisclosure, -1),
		"mutation control: a raw-output scan must miss the disclosure split by ANSI tags")

	visible := normalizedHelpText(styledDisclosure)
	assert.Equal(t, "cost: one rank; still.", visible,
		"normalization must preserve word boundaries and punctuation")
	assert.Contains(t, findForbiddenU8HelpDisclosures(styledDisclosure), "one rank",
		"the visible-text guard must catch the styled tuning disclosure")
}

func TestU8CrossReferenceValidationRejectsMissingDOGMudOnlyTopic(t *testing.T) {
	registeredDefaultShoot := false
	for _, registeredFS := range fileSystems {
		if _, err := registeredFS.ReadFile("templates/help/shoot.template"); err == nil {
			registeredDefaultShoot = true
			break
		}
	}
	require.True(t, registeredDefaultShoot,
		"mutation control requires the registered default-world shoot template")

	dataRoot := t.TempDir()
	templatePath := filepath.Join(dataRoot, "templates", "help", "shoot.template")
	require.NoError(t, os.MkdirAll(filepath.Dir(templatePath), 0o755))
	require.NoError(t, os.WriteFile(templatePath, []byte(
		`DOGMud-only shoot fixture: <ansi fg="command">help dogmud-only-missing</ansi>`), 0o600))
	useU8DataFilesAt(t, dataRoot)

	rendered := processU8Help(t, "help/shoot")
	require.Contains(t, rendered, "DOGMud-only shoot fixture",
		"the configured data root must win over the registered default template")
	assert.Equal(t, []string{"dogmud-only-missing"}, u8HelpCrossReferenceTopics(rendered))
	assert.ErrorContains(t, validateU8HelpCrossReferences("help/shoot", rendered),
		`help/shoot emits broken cross-reference "help dogmud-only-missing"`)
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

	for _, path := range u8ActionHelpPaths {
		t.Run(path+"/no-raw-tuning", func(t *testing.T) {
			rendered := processU8Help(t, path)
			assert.Empty(t, findForbiddenU8HelpDisclosures(rendered),
				"visible player help must not use em dashes or disclose raw or worded tuning")
		})
	}
}

func TestU8StaminaHelpDistinguishesFleeFromGrappleCadence(t *testing.T) {
	useU8DataFiles(t)
	rendered := normalizedHelpText(processU8Help(t, "help/stamina"))
	assert.Contains(t, rendered,
		"flee spends once when you issue the command; grapple upkeep spends each continuing round")
}

func TestU8ActionHelpCrossReferencesResolve(t *testing.T) {
	useU8DataFiles(t)

	for _, path := range u8ActionHelpPaths {
		t.Run(path, func(t *testing.T) {
			rendered := processU8Help(t, path)
			if path == "help/shoot" {
				assert.Contains(t, rendered, "Fire a loaded ranged weapon at a target in your room",
					"the validator must inspect DOGMud's shoot template, not the registered default")
				assert.Equal(t, []string{"reload", "ranged-combat", "stamina", "equip"},
					u8HelpCrossReferenceTopics(rendered),
					"all four DOGMud shoot cross-references must be enumerated")
			}
			require.NoError(t, validateU8HelpCrossReferences(path, rendered))
		})
	}
}

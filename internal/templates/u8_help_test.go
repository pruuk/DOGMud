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
	// U10d: the ambush topic is player-facing action help like the rest of this
	// list, so it earns the same two guards: no em dashes or raw/worded tuning
	// in its visible text, and every cross-reference it emits must resolve. It
	// carries no entry in the action-admission expectations map below because
	// it documents no stamina admission policy of its own; the commands that do
	// (shoot, reload, sneak) are already listed above and still assert theirs.
	"help/ambush",
}

// u8HelpExceptions lists help templates deliberately exempt from the
// whole-tree guards below. It is a SHRINKING list: every entry needs a written
// reason, and an entry without one is the old allowlist growing back.
//
// This replaced an allowlist. u8ActionHelpPaths named 28 files out of 454, so
// 426 templates had no parse check, no numeric-disclosure check and no
// cross-reference check at all -- and both files U10d edited that fell outside
// it were exactly where its surviving copy defects lived. Same failure shape as
// `stow` going invisible in the 2026-08-03 helpfile audit.
var u8HelpExceptions = map[string]string{
	// DATA-DRIVEN templates, not static help pages. They are rendered with a
	// payload by the help command (help/help ranges over .Commands; help/spell
	// reads .Name/.Description/.SpellId), so processing them with no data
	// context fails inside text/template rather than revealing a copy defect.
	"help/help":  "data-driven: ranges over .Commands, needs a payload to render",
	"help/spell": "data-driven: reads .Name/.Description/.SpellId from a payload",

	// RESOURCE-BAR LEGENDS. These document a fixed display, not a tunable: the
	// bands ARE the interface. Same class as the `status` stat sheet, which is
	// the standing exception to "no hard numbers".
	"help/health": "resource-bar legend: the percentage bands are the display itself",
	"help/mana":   "resource-bar legend: the percentage bands are the display itself",

	// PLAYER-SUPPLIED COMMAND ARGUMENTS. The number is what the player types.
	// Removing it does not protect a balance value, it breaks the documentation.
	"help/set-wimpy": "documents the numeric argument the player types (set wimpy 10)",
	"help/triggers":  "worked example of a player-authored trigger threshold",

	// RECIPE TABLES. Craft time and skill requirement are the reference a player
	// consults before spending materials; a recipe book without them is useless.
	// NOTE: these mirror the recipe YAML and WILL drift if it changes. Generating
	// them from the data is a filed follow-up, not a U11 change.
	"help/blacksmithing": "recipe table: skill, ingredients and craft rounds are the reference",
	"help/jewelcrafting": "recipe table: skill, ingredients and craft rounds are the reference",

	// SPELL AND BUFF REFERENCE CARDS ("Time: 4 rounds", "Reserve: 1% at tier 0").
	// The no-hard-numbers rule targets combat and spell MESSAGES, where a raw
	// number breaks immersion mid-fight. A reference card a player deliberately
	// looks up is the documented exception, like the status sheet.
	"help/carapace-ward":      "spell/buff reference card: cast time and reserve are its content",
	"help/chitin-brace":       "spell/buff reference card: cast time and reserve are its content",
	"help/chrysalis-bond":     "spell/buff reference card: cast time and reserve are its content",
	"help/chrysalis-sight":    "spell/buff reference card: cast time and reserve are its content",
	"help/chrysalis-stride":   "spell/buff reference card: cast time and reserve are its content",
	"help/firebomb":           "spell/buff reference card: cast time and reserve are its content",
	"help/flashbang":          "spell/buff reference card: cast time and reserve are its content",
	"help/honed-edge":         "spell/buff reference card: cast time and reserve are its content",
	"help/hungering-touch":    "spell/buff reference card: cast time and reserve are its content",
	"help/ironblood":          "spell/buff reference card: cast time and reserve are its content",
	"help/mindweave":          "spell/buff reference card: cast time and reserve are its content",
	"help/predators-instinct": "spell/buff reference card: cast time and reserve are its content",
	"help/rootbind":           "spell/buff reference card: cast time and reserve are its content",
	"help/rootwalker":         "spell/buff reference card: cast time and reserve are its content",
	"help/serpents-edge":      "spell/buff reference card: cast time and reserve are its content",
	"help/shadowweave":        "spell/buff reference card: cast time and reserve are its content",
	"help/spore-mantle":       "spell/buff reference card: cast time and reserve are its content",
	"help/sporeweave":         "spell/buff reference card: cast time and reserve are its content",
	"help/thornguard":         "spell/buff reference card: cast time and reserve are its content",
	"help/toxic-flask":        "spell/buff reference card: cast time and reserve are its content",
	"help/venomgrip":          "spell/buff reference card: cast time and reserve are its content",

	// ITEM STAT BLOCKS. Weight reduction is the reason to buy the item.
	"help/artisans-satchel":       "item stat block: weight reduction is the item's whole point",
	"help/component-bag":          "item stat block: weight reduction is the item's whole point",
	"help/leather-backpack":       "item stat block: weight reduction is the item's whole point",
	"help/masters-component-case": "item stat block: weight reduction is the item's whole point",
	"help/reinforced-travel-pack": "item stat block: weight reduction is the item's whole point",
}

// allHelpTemplatePaths walks every help template and returns them as
// "help/<name>" paths, minus anything in u8HelpExceptions.
func allHelpTemplatePaths(t *testing.T) []string {
	t.Helper()

	dir := filepath.Join(u8DataFilesRoot, "templates", "help")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "help template directory must be readable")

	var paths []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".template") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".template")
		p := "help/" + name
		if reason, skip := u8HelpExceptions[p]; skip {
			t.Logf("skipping %s: %s", p, reason)
			continue
		}
		paths = append(paths, p)
	}

	require.Greater(t, len(paths), 400,
		"expected the whole help tree; a short list means the walk broke and "+
			"the guards silently stopped covering anything")
	return paths
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
	disclosures = append(disclosures, findForbiddenU8TuningNumbers(rendered)...)
	return disclosures
}

// findForbiddenU8TuningNumbers is the half of the disclosure guard that runs
// over the WHOLE help tree: raw or worded tuning values leak internal balance
// numbers to players, which is a standing project rule and not specific to the
// U8 action helpfiles.
//
// The em-dash half deliberately stays scoped to u8ActionHelpPaths (see
// findForbiddenU8HelpDisclosures). Applying it tree-wide would be a new house-
// style decision affecting 154 templates, not a coverage fix, and a mechanical
// substitution across that much prose would damage sentences. Those files are
// filed in docs/audits/2026-08-30-u11-filed-findings.md.
func findForbiddenU8TuningNumbers(rendered string) []string {
	visible := normalizedHelpText(rendered)
	disclosures := make([]string, 0)
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

	for _, path := range allHelpTemplatePaths(t) {
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

	// Iterates the EXPECTATIONS, not the tree. Every authored expectation must
	// be checked; the previous form ranged over u8ActionHelpPaths and looked
	// expectations up, so an expectation whose path was missing from that list
	// was silently never asserted. Widening this one to all 454 templates would
	// add nothing -- 426 of them have no expectations -- while hiding that bug.
	for path := range expectations {
		t.Run(path, func(t *testing.T) {
			rendered := normalizedHelpText(processU8Help(t, path))
			for _, phrase := range expectations[path] {
				assert.Contains(t, rendered, phrase)
			}
		})
	}

	// Tree-wide: no help template may leak a raw or worded tuning value.
	for _, path := range allHelpTemplatePaths(t) {
		t.Run(path+"/no-raw-tuning", func(t *testing.T) {
			rendered := processU8Help(t, path)
			assert.Empty(t, findForbiddenU8TuningNumbers(rendered),
				"visible player help must not disclose raw or worded tuning values")
		})
	}

	// Action help additionally bans em dashes. Scoped on purpose -- see
	// findForbiddenU8TuningNumbers for why this half is not yet tree-wide.
	for _, path := range u8ActionHelpPaths {
		t.Run(path+"/no-em-dash", func(t *testing.T) {
			rendered := processU8Help(t, path)
			assert.Empty(t, findForbiddenU8HelpDisclosures(rendered),
				"visible action help must not use em dashes or disclose tuning")
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

	for _, path := range allHelpTemplatePaths(t) {
		t.Run(path, func(t *testing.T) {
			rendered := processU8Help(t, path)
			if path == "help/shoot" {
				assert.Contains(t, rendered, "Fire a loaded ranged weapon at a target in your room",
					"the validator must inspect DOGMud's shoot template, not the registered default")
				// U10d added the three stealth/cooldown pointers: shooting from
				// hiding is an ambush, and `help shoot` was previously the only
				// place a player could read about aimed fire without ever being
				// told that.
				assert.Equal(t, []string{"reload", "ranged-combat", "stamina", "equip", "sneak", "skullduggery", "ambush", "special"},
					u8HelpCrossReferenceTopics(rendered),
					"every DOGMud shoot cross-reference must be enumerated")
			}
			require.NoError(t, validateU8HelpCrossReferences(path, rendered))
		})
	}
}

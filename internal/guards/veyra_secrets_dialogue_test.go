package guards

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// veyraSecret is one row of the owner-approved table in the tier 3 spec
// (docs/superpowers/specs/completed/2026-11-02-quest-arc-phase7-tier3-design.md, "The
// trades and lines"). The test holds the table itself, so a content edit that
// drifts from it fails here.
type veyraSecret struct {
	key        string
	price      int
	recipes    []string
	notReady   string
	retiredQst int
}

var veyraSecrets = []veyraSecret{
	{"teach_bandolier", 17500, []string{"reinforced-harness", "preservation-runes", "assemble-vitalis-bandolier"},
		"Come back when you've practiced at the loom, the enchanting circle and the alchemy bench.", 79},
	{"teach_blackrazor", 25000, []string{"hungering-guard", "obsidian-edge-resin", "assemble-the-blackrazor"},
		"Come back when you've practiced at the jeweler's bench, the alchemy bench and the forge.", 80},
	{"teach_wayfarer", 12500, []string{"reinforced-frame", "spatial-stitching", "assemble-wayfarers-pack"},
		"Come back when you've practiced at the forge, the enchanting circle and the loom.", 81},
	{"teach_aegis", 20000, []string{"voice-amber-housing", "resonance-lacquer", "assemble-aegis-of-mockery"},
		"Come back when you've practiced at the jeweler's bench, the alchemy bench and the forge.", 82},
	{"teach_thornwall", 15000, []string{"barbed-spike-plates", "anti-corrosion-quench", "assemble-thornwall-harness"},
		"Come back when you've practiced at the forge, the alchemy bench and the loom.", 83},
	{"teach_prism", 20000, []string{"containment-lattice", "nutrient-suspension", "assemble-seething-prism"},
		"Come back when you've practiced at the enchanting circle, the alchemy bench and the jeweler's bench.", 84},
	{"teach_zephyr", 12500, []string{"quicksilver-soles", "windlace-bindings", "assemble-zephyr-treads"},
		"Come back when you've practiced at the alchemy bench, the enchanting circle and the loom.", 85},
	{"teach_choir", 22500, []string{"conductor-core", "choir-focus-gems", "assemble-hollow-choir-staff"},
		"Come back when you've practiced at the forge, the jeweler's bench and the enchanting circle.", 86},
	{"teach_phial", 15000, []string{"reduction-base", "assemble-phial-of-second-birth"},
		"Come back when you've practiced at the cooking fire and the alchemy bench.", 87},
}

// stationWords is how Veyra's lines say each crafting station.
var stationWords = map[string]string{
	"forge":             "the forge",
	"loom":              "the loom",
	"alchemy_bench":     "the alchemy bench",
	"jeweler_bench":     "the jeweler's bench",
	"enchanting_circle": "the enchanting circle",
	"cooking_fire":      "the cooking fire",
}

func loadVeyra(t *testing.T) *dialogue.DialogueFile {
	t.Helper()
	b, err := os.ReadFile("_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml")
	require.NoError(t, err)
	var df dialogue.DialogueFile
	require.NoError(t, yaml.UnmarshalStrict(b, &df))
	return &df
}

func loadShippedRecipes(t *testing.T) map[string]*crafting.RecipeSpec {
	t.Helper()
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	require.NoError(t, configs.ReloadConfig())
	recipes, err := fileloader.LoadAllFlatFiles[string, *crafting.RecipeSpec](configs.GetFilePathsConfig().DataFiles.String() + `/recipes`)
	require.NoError(t, err)
	return recipes
}

// TestVeyraSecretsDialogue pins Veyra's nine sales (#375 phase 7 tier 3b):
// the price, the recipes and the trade line each carries, and who sees which
// answer. A sale is not a quest, so it grants nothing and gates on nothing but
// what the player already knows.
func TestVeyraSecretsDialogue(t *testing.T) {
	df := loadVeyra(t)
	require.Nil(t, df.FindTopic("secrets_list"), "the menu is the price list now")

	var sales []string
	for _, tp := range df.Topics {
		if strings.HasPrefix(tp.Key, "teach_") {
			sales = append(sales, tp.Key)
		}
	}
	require.Len(t, sales, 9, "Veyra's nine sales")

	squash := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	for _, s := range veyraSecrets {
		tp := df.FindTopic(s.key)
		require.NotNil(t, tp, s.key)
		require.Equal(t, dialogue.KindSale, tp.Kind, s.key)
		require.Equal(t, s.price, tp.ChargesGold, "%s price", s.key)
		require.Equal(t, s.recipes, tp.TeachesRecipes, "%s recipes", s.key)
		require.Equal(t, s.notReady, squash(tp.NotReadyText), "%s notReadyText", s.key)
		require.Contains(t, squash(tp.Text), util.FormatNumber(s.price)+" gold", "%s: the pitch must state the price", s.key)
		require.NotEmpty(t, tp.PaidText, s.key)
		require.NotEmpty(t, tp.PlayerLine, s.key)
		require.Empty(t, tp.GrantsQuest, "%s: a sale grants no quest", s.key)
		require.Zero(t, tp.Quest, s.key)
		require.Zero(t, tp.GoldRequired, "%s: Accept is the gate, so the line shows to a player short of gold", s.key)
		require.Empty(t, tp.QuestRequired, s.key)
		require.Empty(t, tp.QuestExcluded, s.key)

		player := func(gold int, skilled bool, known ...string) *dialogue.PlayerState {
			knows := map[string]bool{}
			for _, k := range known {
				knows[k] = true
			}
			return &dialogue.PlayerState{
				HasQuest:       func(string) bool { return false },
				KnowsRecipe:    func(id string) bool { return knows[id] },
				RecipeSkillMet: func(string) bool { return skilled },
				HasGold:        func(n int) bool { return gold >= n },
				ChargeGold:     func(n int) bool { return gold >= n },
				LearnRecipe:    func(id string) { knows[id] = true },
				HasHeard:       func(string) bool { return false },
				MarkHeard:      func(string) {},
			}
		}

		// Listed to everyone who has not bought it, skilled or not, rich or not.
		for _, ps := range []*dialogue.PlayerState{player(0, false), player(s.price, false), player(0, true)} {
			require.True(t, df.Eligible(tp, ps), "%s is listed", s.key)
		}
		// Only a player with the skill for every recipe is marked.
		require.Empty(t, df.TopicMarker(tp, player(s.price, false)), s.key)
		require.Equal(t, "[*]", df.TopicMarker(tp, player(s.price, true)), s.key)

		// Knowing every recipe hides it; knowing all but one does not.
		require.False(t, df.Eligible(tp, player(s.price, true, s.recipes...)), "%s: owned", s.key)
		require.True(t, df.Eligible(tp, player(s.price, true, s.recipes[1:]...)), "%s: part owned", s.key)

		// Buying: the exact price once, every recipe taught. A short purse
		// teaches nothing.
		learned := map[string]bool{}
		purse := s.price
		var charges []int
		recordCharge := func(n int) bool {
			if purse < n {
				return false
			}
			purse -= n
			charges = append(charges, n)
			return true
		}
		ps := player(s.price, true)
		ps.ChargeGold = recordCharge
		ps.LearnRecipe = func(id string) { learned[id] = true }
		require.True(t, df.Apply(tp, ps), s.key)
		require.Equal(t, []int{s.price}, charges, "%s: the exact price, once", s.key)
		require.Len(t, learned, len(s.recipes), s.key)
		learned, charges, purse = map[string]bool{}, nil, s.price-1
		ps = player(s.price-1, true)
		ps.ChargeGold = recordCharge
		ps.LearnRecipe = func(id string) { learned[id] = true }
		require.False(t, df.Apply(tp, ps), s.key)
		require.Empty(t, charges, "%s: a short purse is charged nothing", s.key)
		require.Empty(t, learned, "%s: a short purse teaches nothing", s.key)
	}

	// Blackrazor owners (80's recipes) do not see its line, but still see the rest.
	var knows80 []string
	for _, s := range veyraSecrets {
		if s.key == "teach_blackrazor" {
			knows80 = s.recipes
		}
	}
	ps := &dialogue.PlayerState{
		HasQuest:       func(string) bool { return false },
		KnowsRecipe:    func(id string) bool { return containsString(knows80, id) },
		RecipeSkillMet: func(string) bool { return true },
	}
	require.False(t, df.Eligible(df.FindTopic("teach_blackrazor"), ps))
	require.True(t, df.Eligible(df.FindTopic("teach_bandolier"), ps))
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// Each notReadyText names exactly the stations the secret's recipes use, in
// words and never a number (owner ruling 2026-11-02). The stations are read
// from the shipped recipes, so a recipe moved to another station fails here.
func TestVeyraNotReadyTextNamesTheTrades(t *testing.T) {
	df := loadVeyra(t)
	recipes := loadShippedRecipes(t)
	for _, s := range veyraSecrets {
		tp := df.FindTopic(s.key)
		require.NotNil(t, tp, s.key)

		want := map[string]bool{}
		for _, id := range s.recipes {
			r := recipes[id]
			require.NotNil(t, r, "%s: recipe %q is not shipped", s.key, id)
			words, ok := stationWords[r.Station]
			require.True(t, ok, "%s: recipe %s uses station %q with no words", s.key, id, r.Station)
			want[words] = true
		}
		got := map[string]bool{}
		for _, words := range stationWords {
			if strings.Contains(tp.NotReadyText, words) {
				got[words] = true
			}
		}
		require.Equal(t, sortedKeys(want), sortedKeys(got), "%s: notReadyText names the wrong trades", s.key)
		require.False(t, regexp.MustCompile(`\d`).MatchString(tp.NotReadyText), "%s: no digit in notReadyText", s.key)
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Quests 78 to 87 are retired whole: no quest file, every step in the retired
// section of the manifest, and no shipped quest, dialogue or behavior file
// still names one of their tokens.
func TestVeyraQuestsAreRetired(t *testing.T) {
	// The manifest is read through the configured data folder, which a test
	// binary only points at the shipped world after a reload.
	configs.SetConfigForTest(t, configs.GetConfig())
	require.NoError(t, configs.ReloadConfig())
	retired := quests.RetiredQuests()
	for _, s := range veyraSecrets {
		require.True(t, retired[s.retiredQst], "the quest that sold %s (%d) must be retired", s.key, s.retiredQst)
	}
	for id := 78; id <= 87; id++ {
		require.True(t, retired[id], "quest %d must be retired", id)
		matches, err := filepath.Glob("_datafiles/world/dogmud/quests/" + strconv.Itoa(id) + "-*.yaml")
		require.NoError(t, err)
		require.Empty(t, matches, "quest %d still has a file", id)
	}

	tokenRe := regexp.MustCompile(`(^|[^0-9A-Za-z_.-])(7[89]|8[0-7])-(start|end)\b`)
	for _, dir := range []string{"quests", "dialogue", "behaviors"} {
		root := filepath.Join("_datafiles", "world", "dogmud", dir)
		require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".yaml") {
				return err
			}
			b, rerr := os.ReadFile(path)
			require.NoError(t, rerr)
			if m := tokenRe.FindString(string(b)); m != "" {
				t.Errorf("%s still names a retired quest token %q", path, strings.TrimSpace(m))
			}
			return nil
		}))
	}

	// The probe can fail: the pattern catches a token written the way the old
	// content wrote it.
	require.True(t, tokenRe.MatchString("  - 78-end\n"))
	require.True(t, tokenRe.MatchString("grantsQuest: 87-start"))
	require.False(t, tokenRe.MatchString("a 1978-end year"))
}

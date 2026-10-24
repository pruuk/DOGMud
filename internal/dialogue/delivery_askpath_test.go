package dialogue

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// deliveryCase describes one delivery quest's expected ask-path completion.
type deliveryCase struct {
	name       string
	file       string
	mobId      int
	heldToken  string // quest token the player has at delivery time
	grantToken string // token the ask-path must grant (END directly for chain quests)
	itemId     int    // delivery item the player is holding (consumed)
	topic      string // the word the pre-#375 `ask <npc> <topic>` path used (case label only)
	// optional expected side-effects (zero value = not checked):
	flagKey   string
	flagVal   string
	gold      int
	givesItem int
	rep       map[string]int
	// heard names a warning topic the hand-in requires first (spec R28
	// choice quests). Before it is heard, giving the item must not complete
	// the step.
	heard string
}

// TestDeliveryAskPathCompletes locks the "every reasonable path resolves the
// step" guarantee for item-delivery quests (Malia playtest follow-up,
// 2026-06-29). Since #375 the give path and the talk path are one topic: `give
// <item> <npc>` resolves through ItemTopic to the hand-in that wants the item
// (Task 10), and the same hand-in is on the talk menu. Choosing or giving it
// must complete the step with the delivery's side-effects: the item is
// consumed (requiresItem), the right token is granted (grantsQuest), and any
// flag/gold/rep/item rewards land.
//
// Chain quests grant the END token directly; IsTokenAfter permits skipping
// intermediate steps.
//
// Data-driven over the REAL dialogue files so it catches authoring mistakes.
func TestDeliveryAskPathCompletes(t *testing.T) {
	cases := []deliveryCase{
		{
			name: "Q4 Warden's Report — purse to Tessara", file: "../../_datafiles/world/dogmud/dialogue/dustwalk_road/83.yaml",
			mobId: 83, heldToken: "4-investigate", grantToken: "4-end", itemId: 16, topic: "evidence",
		},
		{
			name: "Q5 Innkeeper's Complaint — ledger to Tolva", file: "../../_datafiles/world/dogmud/dialogue/watchers_crossing/84.yaml",
			mobId: 84, heldToken: "5-start", grantToken: "5-end", itemId: 21, topic: "ledger",
		},
		{
			name: "Q7 Fallow Field — sample to Dorn", file: "../../_datafiles/world/dogmud/dialogue/thornwall_outskirts/89.yaml",
			mobId: 89, heldToken: "7-start", grantToken: "7-end", itemId: 24, topic: "sample",
		},
		{
			name: "Q9 Tithe Audit — ledger to Olen", file: "../../_datafiles/world/dogmud/dialogue/thornwall_city/95.yaml",
			mobId: 95, heldToken: "9-start", grantToken: "9-end", itemId: 29, topic: "ledger",
		},
		{
			name: "Q14 Undertow — bribe ledger to Velk (chain->end)", file: "../../_datafiles/world/dogmud/dialogue/thornwall_city/94.yaml",
			mobId: 94, heldToken: "14-evidence", grantToken: "14-end", itemId: 40036, topic: "ledger",
		},
		{
			name: "Q17 Caravan Guard — pin to master", file: "../../_datafiles/world/dogmud/dialogue/north_road/281.yaml",
			mobId: 281, heldToken: "17-start", grantToken: "17-end", itemId: 54, topic: "pin",
			// The Letter of Introduction (item 51) was removed from the
			// quest by the owner, 2026-10-23 (quest 17 file history).
			flagKey: "17-resolution", flagVal: "combat",
		},
		{
			name: "Q19 Lake Caves — tooth to Drunn", file: "../../_datafiles/world/dogmud/dialogue/stillwater/335.yaml",
			mobId: 335, heldToken: "19-signed", grantToken: "19-end", itemId: 40054, topic: "tooth",
			flagKey: "19-completion", flagVal: "full", gold: 350,
		},
		{
			name: "Q19 Lake Caves — tooth to Arn", file: "../../_datafiles/world/dogmud/dialogue/stillwater/342.yaml",
			mobId: 342, heldToken: "19-signed", grantToken: "19-end", itemId: 40054, topic: "tooth",
			flagKey: "19-completion", flagVal: "full", gold: 350,
		},
		{
			name: "Q20 Ulla — kingfisher to Vella", file: "../../_datafiles/world/dogmud/dialogue/stillwater/355.yaml",
			mobId: 355, heldToken: "20-kingfisher_found", grantToken: "20-vella_journal", itemId: 40060, topic: "kingfisher",
			givesItem: 40061,
		},
		{
			name: "Q20 Ulla — journal to Ulla", file: "../../_datafiles/world/dogmud/dialogue/stillwater/347.yaml",
			mobId: 347, heldToken: "20-vella_journal", grantToken: "20-end", itemId: 40061, topic: "journal",
			flagKey: "20-truth", flagVal: "partial",
		},
		{
			name: "Q60 Long Road — dispatch to Pell", file: "../../_datafiles/world/dogmud/dialogue/hartcharn/9182.yaml",
			mobId: 9182, heldToken: "60-start", grantToken: "60-waybill", itemId: 40071, topic: "dispatch",
		},
		{
			name: "Q60 Long Road — waybill to Verrold", file: "../../_datafiles/world/dogmud/dialogue/kingsbarrow_vale/9198.yaml",
			mobId: 9198, heldToken: "60-waybill", grantToken: "60-manifest", itemId: 40072, topic: "waybill",
		},
		{
			name: "Q60 Long Road — manifest to gate guard", file: "../../_datafiles/world/dogmud/dialogue/new_plymouth_outskirts/9208.yaml",
			mobId: 9208, heldToken: "60-manifest", grantToken: "60-end", itemId: 40073, topic: "manifest",
		},
		{
			name: "Q63 Dock Rat — tally to Constable (chain->end)", file: "../../_datafiles/world/dogmud/dialogue/new_plymouth_docks/9316.yaml",
			mobId: 9316, heldToken: "63-evidence", grantToken: "63-end", itemId: 40100, topic: "tally",
		},
		{
			name: "Q63 Dock Rat — tally to Jesset (chain->end)", file: "../../_datafiles/world/dogmud/dialogue/new_plymouth_docks/9300.yaml",
			mobId: 9300, heldToken: "63-evidence", grantToken: "63-end", itemId: 40100, topic: "tally",
		},
		{
			name: "Q66 Addict's Plight — wrapper to Ysolde", file: "../../_datafiles/world/dogmud/dialogue/new_plymouth_common/9323.yaml",
			// The wrapper hand-in completes the quest: the old ask route
			// granted 66-escort, which nothing ever completed (9323.yaml).
			mobId: 9323, heldToken: "66-start", grantToken: "66-end", itemId: 40110, topic: "wrapper",
		},
		{
			name: "Q67 Bloom Trail — case-file to Constable (chain->end, rep)", file: "../../_datafiles/world/dogmud/dialogue/new_plymouth_docks/9316.yaml",
			mobId: 9316, heldToken: "67-witness", grantToken: "67-end", itemId: 40111, topic: "casefile",
			rep: map[string]int{"bloom_trade": -15},
		},
		{
			name: "Q68 Cooperage — map to Vell (bloodline)", file: "../../_datafiles/world/dogmud/dialogue/new_plymouth_merchant/9349.yaml",
			mobId: 9349, heldToken: "68-choice", grantToken: "68-end", itemId: 40113, topic: "fragment",
			flagKey: "68-allegiance", flagVal: "bloodline", gold: 100, heard: "q68_vell_report",
			rep: map[string]int{"bloodline_domestic": 20, "cooperage_circle": -15},
		},
		{
			name: "Q70 Pre-Founding Web — rubbing to Orin (rep)", file: "../../_datafiles/world/dogmud/dialogue/new_plymouth_crafting/9332.yaml",
			mobId: 9332, heldToken: "70-witness", grantToken: "70-end", itemId: 40118, topic: "rubbing",
			rep: map[string]int{"cooperage_circle": 5},
		},
		{
			name: "Q71 Tribute — ledger page to Ostry", file: "../../_datafiles/world/dogmud/dialogue/new_plymouth_merchant/9347.yaml",
			mobId: 9347, heldToken: "71-ledger", grantToken: "71-end", itemId: 40119, topic: "ledger",
			rep: map[string]int{"cooperage_circle": 5},
		},
		{
			name: "Q74 Undercroft — rubbing to Aldric (keepers)", file: "../../_datafiles/world/dogmud/dialogue/the_confluence/9464.yaml",
			mobId: 9464, heldToken: "74-reveal", grantToken: "74-end", itemId: 40142, topic: "rubbing",
			flagKey: "74-allegiance", flagVal: "keepers", gold: 80, heard: "aldric_terms",
			rep: map[string]int{"keepers": 20, "margin": -10},
		},
		{
			name: "Q74 Undercroft — rubbing to Margin scholar (margin)", file: "../../_datafiles/world/dogmud/dialogue/the_confluence/9454.yaml",
			mobId: 9454, heldToken: "74-reveal", grantToken: "74-end", itemId: 40142, topic: "rubbing",
			flagKey: "74-allegiance", flagVal: "margin", gold: 40, heard: "margin_terms",
			rep: map[string]int{"margin": 20, "keepers": -10},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			data, err := os.ReadFile(tc.file)
			require.NoError(t, err, "must read the real dialogue file")
			var df DialogueFile
			require.NoError(t, yaml.Unmarshal(data, &df), "dialogue YAML must unmarshal")

			hasItem := true
			var granted string
			itemRemoved := false
			flags := map[string]string{}
			gold := 0
			gaveItem := 0
			bumps := map[string]int{}
			heard := map[string]bool{}
			// A real character holds one current step per quest, and HasQuest
			// is true for that step and every step before it
			// (characters.HasQuest via quests.IsTokenAfter).
			questId, _ := splitToken(tc.heldToken)
			steps := questStepsForTest(t, questId)
			current := stepIndex(steps, tc.heldToken)
			require.GreaterOrEqual(t, current, 0, "quest %d has no step %s", questId, tc.heldToken)
			carries := func(id int) bool { return hasItem && id == tc.itemId }
			ps := &PlayerState{
				HasQuest: func(tok string) bool {
					i := stepIndex(steps, tok)
					return i >= 0 && i <= current
				},
				HasItem:     carries,
				CarriesItem: carries,
				RemoveItem:  func(id int) bool { itemRemoved = true; hasItem = false; return true },
				GiveQuest: func(tok string) {
					granted = tok
					if i := stepIndex(steps, tok); i > current {
						current = i
					}
				},
				HasHeard:     func(k string) bool { return heard[k] },
				MarkHeard:    func(k string) { heard[k] = true },
				GiveItem:     func(id int) bool { gaveItem = id; return true },
				GetQuestFlag: func(key string) string { return flags[key] },
				SetQuestFlag: func(key, val string) { flags[key] = val },
				BumpRep:      func(faction string, delta int) { bumps[faction] += delta },
				GiveGold:     func(amount int) { gold += amount },
			}

			require.Equal(t, tc.mobId, df.MobId, "loaded the wrong dialogue file")
			if tc.heard != "" {
				require.NotNil(t, df.FindTopic(tc.heard), "no warning topic %s", tc.heard)
				if early, _ := df.ItemTopic(tc.itemId, ps); early != nil {
					require.NotEqual(t, tc.grantToken, early.GrantsQuest,
						"giving item %d before hearing %s must not make the choice", tc.itemId, tc.heard)
					require.Zero(t, early.RequiresItem, "giving item %d before hearing %s must not take it", tc.itemId, tc.heard)
				}
				heard[HeardKey(tc.mobId, tc.heard)] = true
			}
			tp, wanted := df.ItemTopic(tc.itemId, ps)
			require.True(t, wanted, "%d must want item %d", tc.mobId, tc.itemId)
			require.NotNil(t, tp, "giving item %d on %s must play a topic", tc.itemId, tc.heldToken)
			if strings.HasSuffix(tc.grantToken, "-end") {
				assert.Equal(t, KindHandIn, tp.Kind, "the delivery topic %s ends the quest, so it is the hand-in", tp.Key)
			}
			assert.Equal(t, tc.itemId, tp.RequiresItem, "the delivery topic %s takes the item", tp.Key)
			assert.True(t, df.Eligible(tp, ps), "the same topic is on the talk menu")
			require.True(t, df.Apply(tp, ps), "the delivery topic %s must apply", tp.Key)
			assert.Equal(t, tc.grantToken, granted,
				"delivering item %d must grant %s", tc.itemId, tc.grantToken)
			assert.True(t, itemRemoved, "the delivery item must be consumed by the hand-in")

			if tc.flagKey != "" {
				assert.Equal(t, tc.flagVal, flags[tc.flagKey],
					"flag %s must be set to %s", tc.flagKey, tc.flagVal)
			}
			assert.Equal(t, tc.gold, gold, "gold reward must match the give-path")
			if tc.givesItem != 0 {
				assert.Equal(t, tc.givesItem, gaveItem, "the ask-path must give back item %d", tc.givesItem)
			}
			for faction, delta := range tc.rep {
				assert.Equal(t, delta, bumps[faction], "rep for %s must match the give-path", faction)
			}
		})
	}
}

// splitToken splits "<quest id>-<step>".
func splitToken(tok string) (int, string) {
	id, step, ok := strings.Cut(tok, "-")
	if !ok {
		return 0, ""
	}
	n, err := strconv.Atoi(id)
	if err != nil {
		return 0, ""
	}
	return n, step
}

// stepIndex is the position of tok's step in its quest, or -1 when tok names
// another quest or no step.
func stepIndex(steps []string, tok string) int {
	if len(steps) == 0 {
		return -1
	}
	for i, s := range steps {
		if tok == s {
			return i
		}
	}
	return -1
}

// questStepsForTest reads the shipped quest's step ids as full tokens
// ("4-start", "4-investigate", ...), in order.
func questStepsForTest(t *testing.T, questId int) []string {
	t.Helper()
	files, err := filepath.Glob(fmt.Sprintf("../../_datafiles/world/dogmud/quests/%d-*.yaml", questId))
	require.NoError(t, err)
	require.Len(t, files, 1, "exactly one quest file for quest %d", questId)
	data, err := os.ReadFile(files[0])
	require.NoError(t, err)
	var q struct {
		QuestId int `yaml:"questid"`
		Steps   []struct {
			Id string `yaml:"id"`
		} `yaml:"steps"`
	}
	require.NoError(t, yaml.Unmarshal(data, &q))
	require.Equal(t, questId, q.QuestId)
	var out []string
	for _, s := range q.Steps {
		out = append(out, fmt.Sprintf("%d-%s", questId, s.Id))
	}
	require.NotEmpty(t, out)
	return out
}

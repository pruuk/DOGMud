package dialogue

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// #375 phase 7: Esk's Road (89) is offered by Crier Toke once all eight
// "Report to Toke" bridges are in (90-end to 97-end), no longer by Cleric
// Hadwen; Warden Esk takes the hand-in, and his passage topic opens the arch
// for any Opened player, holding quest 89 or not.

func loadCouleeDialogue(t *testing.T, file string) *DialogueFile {
	t.Helper()
	data, err := os.ReadFile("../../_datafiles/world/dogmud/dialogue/pothole_coulee/" + file)
	require.NoError(t, err)
	var df DialogueFile
	require.NoError(t, yaml.Unmarshal(data, &df))
	return &df
}

// heldState is a player holding exactly the listed tokens.
func heldState(held ...string) *PlayerState {
	set := map[string]bool{}
	for _, h := range held {
		set[h] = true
	}
	return &PlayerState{
		QuestOpen:    func(int) bool { return true },
		HasQuest:     func(tok string) bool { return set[tok] },
		HasItem:      func(int) bool { return false },
		CarriesItem:  func(int) bool { return false },
		GetQuestFlag: func(string) string { return "" },
		HasHeard:     func(string) bool { return false },
		MarkHeard:    func(string) {},
	}
}

func menuKeys(df *DialogueFile, ps *PlayerState) []string {
	var ids []string
	for _, e := range df.Menu("npc", ps).Entries {
		ids = append(ids, e.Id)
	}
	return ids
}

func TestEskPassageWorksWithAndWithoutEsksRoad(t *testing.T) {
	esk := loadCouleeDialogue(t, "9107.yaml")

	without := menuKeys(esk, heldState("30-start", "30-end"))
	require.Contains(t, without, "passage")
	require.NotContains(t, without, "road_esk_end")

	with := menuKeys(esk, heldState("30-start", "30-end", "89-start"))
	require.Contains(t, with, "passage")
	require.Contains(t, with, "road_esk_end")
}

func TestTokeOffersEsksRoadOnlyAfterEveryBridge(t *testing.T) {
	toke := loadCouleeDialogue(t, "9106.yaml")
	held := []string{"30-start", "30-end", "31-start", "31-end"}
	for q := 90; q <= 97; q++ {
		held = append(held, fmt.Sprintf("%d-start", q), fmt.Sprintf("%d-end", q))
	}
	require.Contains(t, menuKeys(toke, heldState(held...)), "road_esk")
	require.NotContains(t, menuKeys(toke, heldState(held[:len(held)-1]...)), "road_esk",
		"without 97-end the offer stays hidden")

	hadwen := loadCouleeDialogue(t, "9100.yaml")
	require.Nil(t, hadwen.FindTopic("road_esk"), "Hadwen no longer offers Esk's Road")
}

// Ruling for #375 phase 7: moving quest 89 closes nothing. Being Opened (30-end)
// is the whole of what the arch asks, as it was before the move, so a living
// save that could leave the coulee still can. Both ways through are held: Esk's
// passage topic with his tree's crossing, and the arch room's own command.
func TestLeavingTheCouleeAsksOnlyToBeOpened(t *testing.T) {
	passage := loadCouleeDialogue(t, "9107.yaml").FindTopic("passage")
	require.NotNil(t, passage)
	require.Equal(t, []string{"30-end"}, passage.QuestRequired)
	require.Empty(t, passage.QuestExcluded)
	require.Empty(t, passage.HeardRequired)

	for _, rel := range []string{"pothole_coulee/9107-warden_esk.yaml", "rooms/pothole_coulee/5215.yaml"} {
		data, err := os.ReadFile("../../_datafiles/world/dogmud/behaviors/" + rel)
		require.NoError(t, err)
		var doc interface{}
		require.NoError(t, yaml.Unmarshal(data, &doc))
		gates := crossingGates(doc)
		require.NotEmpty(t, gates, "%s: no crossing found, so this test proves nothing", rel)
		for _, g := range gates {
			require.Equal(t, []string{"30-end"}, g, rel)
		}
	}
}

// crossingGates returns, for each sequence in a tree that runs move_player, the
// quest tokens its conditions name.
func crossingGates(v interface{}) [][]string {
	var out [][]string
	switch n := v.(type) {
	case map[interface{}]interface{}:
		kids, _ := n["children"].([]interface{})
		moves := false
		gate := []string{}
		for _, c := range kids {
			m, ok := c.(map[interface{}]interface{})
			if !ok {
				continue
			}
			if m["do"] == "move_player" {
				moves = true
			}
			if q, ok := m["quest"].(string); ok && m["type"] == "condition" {
				gate = append(gate, q)
			}
		}
		if moves {
			out = append(out, gate)
		}
		for _, c := range n {
			out = append(out, crossingGates(c)...)
		}
	case []interface{}:
		for _, c := range n {
			out = append(out, crossingGates(c)...)
		}
	}
	return out
}

// What each living save reads from Esk, Toke and Hadwen. Quest 89 used to come
// from Hadwen with no bridges at all, so a save may hold it, or have finished
// it, at any point in the eight trades.
func TestEsksRoadLivingSaves(t *testing.T) {
	esk := loadCouleeDialogue(t, "9107.yaml")
	toke := loadCouleeDialogue(t, "9106.yaml")
	hadwen := loadCouleeDialogue(t, "9100.yaml")

	allBridges := []string{"30-start", "30-end", "31-start", "31-end", "32-start", "35-start", "38-start",
		"41-start", "44-start", "47-start", "50-start", "98-start"}
	for q := 90; q <= 97; q++ {
		allBridges = append(allBridges, fmt.Sprintf("%d-start", q), fmt.Sprintf("%d-end", q))
	}

	t.Run("holds 89-start from Hadwen, no bridges", func(t *testing.T) {
		ps := heldState("30-start", "30-end", "88-start", "89-start")
		keys := menuKeys(esk, ps)
		require.Contains(t, keys, "road_esk_end", "they can still finish")
		require.Contains(t, keys, "passage", "they can still leave")
		_, hints := esk.OpenerFor(ps)
		require.Contains(t, hints, "Esk's Road", "the opener names the [?] line")
		require.Contains(t, hints, "I'm ready. Open the arch for me.")

		keys = menuKeys(toke, ps)
		require.NotContains(t, keys, "road_esk")
		require.NotContains(t, keys, "esk_road_when", "their log already sends them to Esk")
	})

	t.Run("finished 89 or crossed under the old flow", func(t *testing.T) {
		ps := heldState("30-start", "30-end", "88-start", "89-start", "89-end")
		keys := menuKeys(esk, ps)
		require.NotContains(t, keys, "road_esk_end", "no second hand-in")
		require.Contains(t, keys, "passage")
		_, hints := esk.OpenerFor(ps)
		require.NotContains(t, hints, "Esk's Road")

		keys = menuKeys(toke, ps)
		require.NotContains(t, keys, "road_esk", "never offered twice")
	})

	t.Run("Opened with no bridges and no quest 89", func(t *testing.T) {
		ps := heldState("30-start", "30-end")
		require.Contains(t, menuKeys(esk, ps), "passage")
		require.Contains(t, menuKeys(hadwen, ps), "another_way")
		_, hints := esk.OpenerFor(ps)
		require.NotContains(t, hints, "Esk's Road")
	})

	t.Run("not Opened", func(t *testing.T) {
		keys := menuKeys(esk, heldState())
		require.NotContains(t, keys, "passage")
		require.Contains(t, keys, "passage_2")
	})

	t.Run("Toke once all eight are in", func(t *testing.T) {
		offered := heldState(allBridges...)
		require.Contains(t, menuKeys(toke, offered), "road_esk")
		_, hints := toke.OpenerFor(offered)
		require.Contains(t, hints, "Esk's Road")

		took := heldState(append(slices.Clone(allBridges), "89-start")...)
		require.NotContains(t, menuKeys(toke, took), "road_esk")
		_, hints = toke.OpenerFor(took)
		require.Contains(t, hints, "talk esk", "he says where the road goes")
		require.NotContains(t, hints, "When a teacher sends you back", "every teacher already has")

		done := heldState(append(slices.Clone(allBridges), "89-start", "89-end")...)
		require.NotContains(t, menuKeys(toke, done), "road_esk")
		_, hints = toke.OpenerFor(done)
		require.NotContains(t, hints, "Esk's Road", "the road is finished")
		require.Contains(t, hints, "To cross the arch", "the arch stays open to them")
		require.NotContains(t, hints, "When a teacher sends you back", "every teacher already has")
	})
}

// Hadwen's opener names Toke's Road only when it is on his menu, and every
// state can find the other way out.
func TestHadwenOpenerNamesOnlyTheLinesOnHisMenu(t *testing.T) {
	hadwen := loadCouleeDialogue(t, "9100.yaml")
	for name, held := range map[string][]string{
		"no road yet":          {"30-start", "30-end"},
		"Toke's Road taken":    {"30-start", "30-end", "88-start"},
		"footing started":      {"30-start", "30-end", "31-start"},
		"old save, 89 held":    {"30-start", "30-end", "89-start"},
		"old save, 89 done":    {"30-start", "30-end", "89-start", "89-end"},
		"old save, both taken": {"30-start", "30-end", "88-start", "89-start"},
	} {
		t.Run(name, func(t *testing.T) {
			ps := heldState(held...)
			keys := menuKeys(hadwen, ps)
			text, hints := hadwen.OpenerFor(ps)
			require.Contains(t, text, "How does it sit")
			require.NotContains(t, hints, "Esk's Road")
			require.Contains(t, keys, "another_way")
			require.Contains(t, hints, "another way out")
			require.Equal(t, slices.Contains(keys, "road_toke"), strings.Contains(hints, "Toke's Road"),
				"hint %q against menu %v", hints, keys)
			require.Equal(t, slices.Contains(keys, "road_toke"), strings.Contains(text, "One road leads on"),
				"text %q against menu %v", text, keys)
		})
	}
}

package dialogue

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func loadShippedDialogue(t *testing.T, path string) *DialogueFile {
	t.Helper()
	data, err := os.ReadFile("../../_datafiles/world/dogmud/dialogue/" + path)
	require.NoError(t, err)
	var df DialogueFile
	require.NoError(t, yaml.Unmarshal(data, &df))
	return &df
}

// #613: a player who takes quest 11 from Kael first holds 11-start with no
// side. Each side's menu must still lead to the choice: the terms topic opens,
// and hearing it opens the stand topic. Once the side is chosen, that side's
// hand-in opens and the other side's stand closes.
func TestQuest11CanBeFinishedAfterTalkingToKaelFirst(t *testing.T) {
	kael := loadShippedDialogue(t, "ironwind_steppe/240.yaml")
	require.Contains(t, menuKeys(kael, heldState()), "the_problem", "Kael offers the quest")

	sides := []struct {
		file, cause, terms, stand, handin, value, otherStand, otherFile string
	}{
		{"ironwind_steppe/241.yaml", "her_cause", "sylara_terms", "stand_with_sylara", "totem_return", "sylara",
			"stand_with_rhett", "ironwind_steppe/242.yaml"},
		{"ironwind_steppe/242.yaml", "his_cause", "rhett_terms", "stand_with_rhett", "sample_return", "rhett",
			"stand_with_sylara", "ironwind_steppe/241.yaml"},
	}
	for _, s := range sides {
		t.Run(s.value, func(t *testing.T) {
			df := loadShippedDialogue(t, s.file)

			unsided := heldState("11-start")
			keys := menuKeys(df, unsided)
			require.NotContains(t, keys, s.cause, "the offer is closed once the quest is held")
			require.Contains(t, keys, s.terms, "the side's terms are open with no side chosen")
			require.NotContains(t, keys, s.stand, "the choice waits until the terms are heard")

			heard := heldState("11-start")
			heard.HasHeard = func(string) bool { return true }
			require.Contains(t, menuKeys(df, heard), s.stand, "hearing the terms opens the choice")

			sided := heldState("11-start", "11-sided")
			sided.HasHeard = func(string) bool { return true }
			sided.HasItem = func(int) bool { return true }
			sided.CarriesItem = func(int) bool { return true }
			sided.GetQuestFlag = func(key string) string {
				if key == "11-branch" {
					return s.value
				}
				return ""
			}
			require.Contains(t, menuKeys(df, sided), s.handin, "the chosen side takes the hand-in")
			require.NotContains(t, menuKeys(loadShippedDialogue(t, s.otherFile), sided), s.otherStand,
				"the other side's choice is closed")
		})
	}
}

// #290: Marek hands over a replacement notice only to a player without one.
func TestMarekOnlyReplacesALostNotice(t *testing.T) {
	const notice = 30
	marek := loadShippedDialogue(t, "thornwall_city/96.yaml")

	lost := heldState("10-start")
	require.Contains(t, menuKeys(marek, lost), "lost_notice")

	carrying := heldState("10-start")
	carrying.CarriesItem = func(id int) bool { return id == notice }
	require.NotContains(t, menuKeys(marek, carrying), "lost_notice")
}

// A held item on the first step: new players go through the carrying offer and
// Hook 1, and a save from before this shipped gets have_it (spec, "Old saves").
func TestGiversHandleAHeldItem(t *testing.T) {
	for _, c := range []struct {
		file, offer, carrying, handin string
		item                          int
		start, deliver, end           string
	}{
		{"dustwalk_road/83.yaml", "help_quest", "help_quest_carrying", "evidence_handoff", 16, "4-start", "4-evidence", "4-end"},
		{"thornwall_outskirts/89.yaml", "help_quest", "help_quest_carrying", "dornhandoff", 24, "7-start", "7-deliver", "7-end"},
		{"marches_spur_road/250.yaml", "quest_start", "quest_start_carrying", "quest_complete_dialogue", 40039, "15-start", "15-return", "15-end"},
	} {
		df := loadShippedDialogue(t, c.file)
		holding := heldState()
		holding.CarriesItem = func(id int) bool { return id == c.item }
		holding.HasItem = func(id int) bool { return id == c.item }

		keys := menuKeys(df, holding)
		require.Contains(t, keys, c.carrying, c.file)
		require.NotContains(t, keys, c.offer, c.file)
		keys = menuKeys(df, heldState())
		require.Contains(t, keys, c.offer, c.file)
		require.NotContains(t, keys, c.carrying, c.file)
		require.NotEmpty(t, df.FindTopic(c.carrying).Objective, c.file)

		// have_it shows only on the start step with the item.
		onStart := heldState(c.start)
		onStart.CarriesItem = holding.CarriesItem
		onStart.HasItem = holding.HasItem
		require.Contains(t, menuKeys(df, onStart), "have_it", c.file)
		require.NotContains(t, menuKeys(df, onStart), c.handin, "the hand-in follows the last step: "+c.file)
		require.NotContains(t, menuKeys(df, heldState(c.start)), "have_it", c.file)

		delivered := heldState(c.start, c.deliver)
		delivered.CarriesItem = holding.CarriesItem
		delivered.HasItem = holding.HasItem
		require.Contains(t, menuKeys(df, delivered), c.handin, c.file)
		require.NotContains(t, menuKeys(df, delivered), "have_it", c.file)
	}

	// The ledgers cannot be held before their quests, so they have no second
	// offer, only have_it for an old save.
	for _, c := range []struct {
		file, handin   string
		item           int
		start, deliver string
	}{
		{"watchers_crossing/84.yaml", "tolvahandoff", 21, "5-start", "5-deliver"},
		{"thornwall_city/95.yaml", "olenhandoff", 29, "9-start", "9-deliver"},
	} {
		df := loadShippedDialogue(t, c.file)
		onStart := heldState(c.start)
		onStart.CarriesItem = func(id int) bool { return id == c.item }
		onStart.HasItem = onStart.CarriesItem
		require.Contains(t, menuKeys(df, onStart), "have_it", c.file)
		require.NotContains(t, menuKeys(df, onStart), c.handin, c.file)
	}
}

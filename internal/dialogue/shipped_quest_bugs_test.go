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

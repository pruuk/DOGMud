package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/require"
)

// #958: a save on quest 11's retired `sided` step lands on its side's fetch
// step, or on the return step when it already carries the item.
func TestMigrateQuest11SidedStep(t *testing.T) {
	cases := []struct {
		name, branch string
		carry        int
		want         string
	}{
		{"sylara fetching", "sylara", 0, "sylara"},
		{"sylara carrying the totem", "sylara", quest11TotemItemId, "totem"},
		{"sylara carrying samples only", "sylara", quest11SampleItem, "sylara"},
		{"rhett fetching", "rhett", 0, "rhett"},
		{"rhett carrying the samples", "rhett", quest11SampleItem, "samples"},
		{"no side", "", quest11TotemItemId, "start"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ch := &Character{QuestProgress: map[int]string{11: "sided", 12: "sided"}}
			if c.branch != "" {
				ch.QuestFlags = map[string]string{"11-branch": c.branch}
			}
			if c.carry != 0 {
				ch.Items = []items.Item{{ItemId: c.carry}}
			}
			ch.MigrateQuest11SidedStep()
			require.Equal(t, c.want, ch.QuestProgress[11])
			require.Equal(t, "sided", ch.QuestProgress[12], "only quest 11 moves")

			ch.MigrateQuest11SidedStep()
			require.Equal(t, c.want, ch.QuestProgress[11], "a second run changes nothing")
		})
	}
}

func TestMigrateQuest11SidedStep_ShelvedAndUntouched(t *testing.T) {
	ch := &Character{
		QuestFlags:    map[string]string{"11-branch": "rhett"},
		ShelvedQuests: map[int]string{11: "sided"},
	}
	ch.MigrateQuest11SidedStep()
	require.Equal(t, "rhett", ch.ShelvedQuests[11], "a shelved save moves too")
	require.Empty(t, ch.QuestProgress, "nothing is added to active progress")

	for _, step := range []string{"start", "sylara", "end"} {
		ch := &Character{
			QuestFlags:    map[string]string{"11-branch": "rhett"},
			QuestProgress: map[int]string{11: step},
		}
		ch.MigrateQuest11SidedStep()
		require.Equal(t, step, ch.QuestProgress[11], "only the sided step moves")
	}
}

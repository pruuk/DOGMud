package dialogue

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// stepOrder is each quest's steps in order, as their quest files declare
// them, so a test PlayerState answers HasQuest the way the character does:
// a step held, or any step after it.
var velkStepOrder = map[string][]string{
	"8":  {"start", "found", "end"},
	"10": {"start", "report", "end"},
	"14": {"start", "explore", "confront", "loot", "unlock", "evidence", "report", "end"},
	"30": {"start", "rite", "end"},
}

// velkState is a player at the given quest steps ("8": "found"), carrying
// the given items.
func velkState(progress map[string]string, carrying ...int) *PlayerState {
	has := func(token string) bool {
		q, step, ok := strings.Cut(token, "-")
		if !ok {
			return false
		}
		cur, held := progress[q]
		if !held {
			return false
		}
		order := velkStepOrder[q]
		ci, ti := -1, -1
		for i, s := range order {
			if s == cur {
				ci = i
			}
			if s == step {
				ti = i
			}
		}
		return ti >= 0 && ci >= ti
	}
	carries := func(id int) bool {
		for _, c := range carrying {
			if c == id {
				return true
			}
		}
		return false
	}
	return &PlayerState{
		HasQuest:     has,
		HasItem:      carries,
		CarriesItem:  carries,
		GetQuestFlag: func(string) string { return "" },
		HasHeard:     func(string) bool { return false },
		MarkHeard:    func(string) {},
	}
}

// TestVelkOpener_CurrentStepWins walks quests 8 and 10 (and 14's item step)
// through Velk's opener. The playtest found the quest 10 greeting never
// showed while quest 8 was active, because quest 8's variants came first
// (#375). A variant naming an item the player carries for a step in
// progress is the most specific, so it wins; quest 8's own states still
// read as before.
func TestVelkOpener_CurrentStepWins(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/dialogue/thornwall_city/94.yaml")
	require.NoError(t, err)
	var df DialogueFile
	require.NoError(t, yaml.Unmarshal(data, &df))

	const (
		notice = 30    // Marek's protection notice, quest 10
		ledger = 40036 // the bribe ledger, quest 14
	)
	cases := []struct {
		name     string
		progress map[string]string
		carrying []int
		want     string
	}{
		{"nothing", nil, nil, "Civilian."},
		{"8 start", map[string]string{"8": "start"}, nil, "Any news on the missing girl?"},
		{"8 found", map[string]string{"8": "found"}, nil, "What did you learn about Elara?"},
		{"8 end", map[string]string{"8": "end"}, nil, "You did the watch a service"},
		{"10 start with the notice", map[string]string{"10": "start"}, []int{notice}, "carrying a complaint"},
		{"10 start without the notice", map[string]string{"10": "start"}, nil, "Civilian."},
		{"10 report", map[string]string{"10": "report"}, []int{notice}, "Civilian."},
		{"8 start, 10 start with the notice", map[string]string{"8": "start", "10": "start"}, []int{notice}, "carrying a complaint"},
		{"8 found, 10 start with the notice", map[string]string{"8": "found", "10": "start"}, []int{notice}, "carrying a complaint"},
		{"8 start, 10 start without the notice", map[string]string{"8": "start", "10": "start"}, nil, "Any news on the missing girl?"},
		{"8 found, 10 report", map[string]string{"8": "found", "10": "report"}, nil, "What did you learn about Elara?"},
		{"8 end, 10 start with the notice", map[string]string{"8": "end", "10": "start"}, []int{notice}, "carrying a complaint"},
		{"8 start, 14 evidence with the ledger", map[string]string{"8": "start", "14": "evidence"}, []int{ledger}, "I can smell it on you"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, _ := df.OpenerFor(velkState(tc.progress, tc.carrying...))
			require.Contains(t, text, tc.want)
		})
	}
}

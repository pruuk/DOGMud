package quests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestRoomTextProblems(t *testing.T) {
	cases := []struct {
		name        string
		text        string
		wantProblem bool
		mentions    string
	}{
		{"names the player", "{source} unlocks the strongbox.", false, ""},
		{"subjectless fragment", "unlocks the strongbox.", true, "{source}"},
		{"uses target", "{source} glares at {target}.", true, "{target}"},
		{"uses target_plain", "{source} glares at {target_plain}.", true, "{target_plain}"},
		{"uses source_plain", "{source_plain} unlocks the strongbox.", true, "{source_plain}"},
		{"unknown token", "{source} opens {thing}.", true, "{thing}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			problems := RoomTextProblems(c.text)
			if !c.wantProblem {
				assert.Empty(t, problems)
				return
			}
			assert.NotEmpty(t, problems)
			assert.True(t, strings.Contains(strings.Join(problems, " | "), c.mentions),
				"a problem should mention %s, got %v", c.mentions, problems)
		})
	}
}

// roomTextQuest builds the smallest quest Validate accepts, with a room_text
// at the top level of its trigger, nested in a sequence's on_complete, or both.
func roomTextQuest(top, nested string) *Quest {
	q := &Quest{QuestId: 90001, Name: "Room Text Test", Steps: []QuestStep{{Id: "start"}},
		Triggers: []TriggerDef{{Event: "room_interact"}}}
	if top != "" {
		q.Triggers[0].Actions = append(q.Triggers[0].Actions, ActionDef{RoomText: top})
	}
	if nested != "" {
		q.Triggers[0].Actions = append(q.Triggers[0].Actions,
			ActionDef{Sequence: &SequenceDef{OnComplete: []ActionDef{{RoomText: nested}}}})
	}
	return q
}

// Validate runs on every quest file parse (boot) AND before an editor save, so
// putting the rule here refuses a bad line at save time with a reply, instead
// of saving it and letting the next boot panic.
func TestValidate_RefusesSubjectlessRoomText(t *testing.T) {
	err := roomTextQuest("unlocks the strongbox.", "").Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "{source}")
}

func TestValidate_RefusesSubjectlessRoomTextInASequence(t *testing.T) {
	err := roomTextQuest("", "unlocks the strongbox.").Validate()
	require.Error(t, err, "room_text under a sequence's on_complete still reaches the room")
	assert.Contains(t, err.Error(), "on_complete")
}

// TestValidate_RefusesSubjectlessRoomTextThreeSequencesDeep: the engine runs
// nested sequences with no depth limit, so the rule must walk them all. The
// first version stopped two levels down, found by review.
func TestValidate_RefusesSubjectlessRoomTextThreeSequencesDeep(t *testing.T) {
	deep := ActionDef{RoomText: "unlocks the strongbox."}
	for i := 0; i < 3; i++ {
		deep = ActionDef{Sequence: &SequenceDef{OnComplete: []ActionDef{deep}}}
	}
	q := &Quest{QuestId: 90001, Name: "Room Text Test", Steps: []QuestStep{{Id: "start"}},
		Triggers: []TriggerDef{{Event: "room_interact", Actions: []ActionDef{deep}}}}
	require.Error(t, q.Validate())
}

func TestValidate_AcceptsRoomTextNamingTheActor(t *testing.T) {
	assert.NoError(t, roomTextQuest("{source} unlocks the strongbox.", "{source} nods.").Validate())
}

// TestShippedQuestRoomTextFollowsConvention reads the real quest files, walking
// nested sequences too, and requires every shipped quest to pass Validate.
func TestShippedQuestRoomTextFollowsConvention(t *testing.T) {
	files, err := filepath.Glob("../../_datafiles/world/dogmud/quests/*.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, files, "the glob must find the quest files, or this test proves nothing")

	checked := 0
	var walk func(file string, actions []ActionDef, depth int)
	walk = func(file string, actions []ActionDef, depth int) {
		for _, a := range actions {
			if a.RoomText != "" {
				checked++
				assert.Empty(t, RoomTextProblems(a.RoomText), "%s: %q", file, a.RoomText)
			}
			if a.Sequence != nil {
				walk(file, a.Sequence.OnComplete, depth+1)
			}
		}
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		var q Quest
		require.NoError(t, yaml.Unmarshal(data, &q), f)
		assert.NoError(t, q.Validate(), filepath.Base(f))
		for _, tr := range q.Triggers {
			walk(filepath.Base(f), tr.Actions, 0)
		}
	}
	assert.Equal(t, 22, checked,
		"22 quest room_text lines ship; a different count means the inventory moved, so re-read this test")
}

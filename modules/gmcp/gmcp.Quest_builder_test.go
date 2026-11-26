package gmcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// The builder's save path (#958): a quest loaded from its file, sent to the
// editor as JSON, posted back unedited (what the page's gather now does) and
// saved by Build.Quest.Update must be the same quest. Compared as YAML: a
// quest carries an unexported cache Validate fills, so raw struct equality
// would depend on which side was validated.

func loadShippedQuest(t *testing.T, prefix string) quests.Quest {
	t.Helper()
	files, err := filepath.Glob("../../_datafiles/world/dogmud/quests/" + prefix + "-*.yaml")
	require.NoError(t, err)
	require.Len(t, files, 1, "quest file for %s", prefix)
	data, err := os.ReadFile(files[0])
	require.NoError(t, err)
	var q quests.Quest
	require.NoError(t, yaml.Unmarshal(data, &q))
	require.NoError(t, q.Validate())
	return q
}

func TestBuilderSave_ShippedQuestsRoundTripUnchanged(t *testing.T) {
	for _, prefix := range []string{"11", "76"} {
		t.Run(prefix, func(t *testing.T) {
			loaded := loadShippedQuest(t, prefix)
			want, err := yaml.Marshal(loaded)
			require.NoError(t, err)

			// The editor receives the quest as JSON and posts JSON back.
			wire, err := json.Marshal(loaded)
			require.NoError(t, err)
			var posted quests.Quest
			require.NoError(t, json.Unmarshal(wire, &posted))

			w := newFakeQuestWorld()
			cp := loaded
			w.specs[loaded.QuestId] = &cp
			res := buildQuestUpdate(w.deps(), posted)
			require.True(t, res.Ok, res.Error)
			require.Len(t, w.saved, 1)

			got, err := yaml.Marshal(w.saved[0])
			require.NoError(t, err)
			require.Equal(t, string(want), string(got))
		})
	}
}

// Quest 11 carries a branch on its steps and quest 76 a trigger's
// missing_items: the round trip above must be holding them, not an empty
// field on both sides.
func TestBuilderSave_RoundTripHoldsTheFieldsTheEditorUsedToDrop(t *testing.T) {
	q11 := loadShippedQuest(t, "11")
	branched := 0
	destined := 0
	for _, s := range q11.Steps {
		if s.Branch != "" {
			branched++
		}
		if !s.Destination.IsZero() {
			destined++
		}
	}
	require.Positive(t, branched, "quest 11 has branched steps")
	require.Positive(t, destined, "quest 11 has step destinations")

	q76 := loadShippedQuest(t, "76")
	missing := 0
	for _, tr := range q76.Triggers {
		missing += len(tr.Conditions.MissingItems)
	}
	require.Positive(t, missing, "quest 76 has a trigger with missing_items")
}

// The page must gather through questmerge.js (tested by
// tools/jstest/questmerge.test.js), load it before quests.js, and carry no
// map_target control (the field left Go at #375 phase 4).
func TestQuestBuilderPageGathersThroughTheMerge(t *testing.T) {
	src, err := os.ReadFile(questBuilderJS)
	require.NoError(t, err)
	js := string(src)
	for _, want := range []string{"QM.step(", "QM.trigger(", "QM.merge(q,", "QM.merge(q.rewards,", "QM.destinationText("} {
		require.Contains(t, js, want)
	}
	require.NotContains(t, js, "map_target")

	html, err := os.ReadFile("../../_datafiles/html/public/build.html")
	require.NoError(t, err)
	page := string(html)
	merge := strings.Index(page, "static/js/questmerge.js")
	editor := strings.Index(page, "static/js/quests.js")
	require.True(t, merge >= 0 && merge < editor, "build.html must load questmerge.js before quests.js")
}

// The editor door refuses a save that adds a destination finding, the way
// loopVerdict refuses new loop findings, and lets one through that the world
// already had (boot would refuse it, so it is not this save's doing).
func TestBuildQuestUpdate_RefusesAddedDestinationFinding(t *testing.T) {
	w := newFakeQuestWorld()
	w.specs[90020] = fakeQuest(90020, "Dest Test")
	w.specs[90021] = fakeQuest(90021, "Other")
	d := w.deps()
	// A fake rule: a step named "bad" is a finding; "old" is one the world has.
	d.destinations = func(qs []*quests.Quest) ([]questloop.Finding, []string) {
		var fs []questloop.Finding
		for _, q := range qs {
			for _, s := range q.Steps {
				if s.Id == "bad" || s.Id == "old" {
					fs = append(fs, questloop.Finding{Rule: "walk-destination", QuestId: q.QuestId, Where: "step " + s.Id, Msg: "no destination"})
				}
			}
		}
		return fs, nil
	}
	w.specs[90021].Steps = append([]quests.QuestStep{{Id: "old"}}, w.specs[90021].Steps...)

	q := validUpdateReq(90020)
	q.Steps = append([]quests.QuestStep{{Id: "bad"}}, q.Steps...)
	res := buildQuestUpdate(d, q)
	require.False(t, res.Ok, "a save that adds a destination finding is refused")
	require.True(t, strings.Contains(res.Error, "walk-destination"), res.Error)
	require.Empty(t, w.saved)
	require.Zero(t, w.reindexed)

	// A save that leaves the finding count alone passes, even though another
	// quest already has one.
	ok := buildQuestUpdate(d, validUpdateReq(90020))
	require.True(t, ok.Ok, ok.Error)
	require.Len(t, w.saved, 1)
}

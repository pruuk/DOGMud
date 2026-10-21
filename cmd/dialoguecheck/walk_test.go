package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// testdataDir anchors on this file, not the process CWD.
func testdataDir(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

// loadFixture builds an Input from testdata/<name>: dialogue/<zone>/*.yaml
// decoded strictly, quests/*.yaml, and the behaviors, rooms and items the
// real readers find there.
func loadFixture(t *testing.T, name string) Input {
	t.Helper()
	dir := testdataDir(t, name)
	var in Input
	paths, err := filepath.Glob(filepath.Join(dir, "dialogue", "*", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, paths, "fixture %s has no dialogue files", name)
	for _, p := range paths {
		b, err := os.ReadFile(p)
		require.NoError(t, err)
		var df dialogue.DialogueFile
		require.NoError(t, yaml.UnmarshalStrict(b, &df), p)
		in.Files = append(in.Files, dlgFile{Folder: filepath.Base(filepath.Dir(p)), File: &df})
	}
	qpaths, err := filepath.Glob(filepath.Join(dir, "quests", "*.yaml"))
	require.NoError(t, err)
	for _, p := range qpaths {
		b, err := os.ReadFile(p)
		require.NoError(t, err)
		var q quests.Quest
		require.NoError(t, yaml.Unmarshal(b, &q), p)
		in.Quests = append(in.Quests, q)
	}
	require.NotEmpty(t, in.Quests, "fixture %s has no quests", name)
	in.Trees, err = readTrees(dir)
	require.NoError(t, err)
	in.WorldItems, err = readWorldItems(dir)
	require.NoError(t, err)
	in.RoomZones, err = readRoomZones(dir)
	require.NoError(t, err)
	return in
}

func ofClass(fs []Finding, class string) []string {
	var out []string
	for _, f := range fs {
		if f.Class == class {
			out = append(out, f.Text)
		}
	}
	return out
}

func TestWalk_CleanFixturePassesAndReachesTheEnd(t *testing.T) {
	w := newWalker(loadFixture(t, "clean"))
	w.fixpoint()
	w.check()
	require.Empty(t, w.findings)
	// The empty report must come from a walk that got somewhere: the
	// quest's end is reachable and every topic fired.
	require.True(t, w.reach[1][node{idx: 1, chain: true}], "1-end should be reachable, reach = %v", w.reach[1])
	for _, s := range w.srcs {
		require.True(t, s.fired, "%s never fired", s.where)
	}
}

func TestWalk_Quest66StyleSoftLockIsADeadEnd(t *testing.T) {
	fs := Walk(loadFixture(t, "softlock66"))
	dead := ofClass(fs, ClassDeadEnd)
	require.Len(t, dead, 1, "%v", fs)
	require.Contains(t, dead[0], "can reach 66-escort and never 66-end")
	require.Contains(t, dead[0], "quest 66 trigger 0 (item_give): at 66-escort: needs 66-start, excludes 66-escort")
	// The give route still finishes, so the quest as a whole is reachable:
	// only the per-step walk finds the lock.
	require.Empty(t, ofClass(fs, ClassUnstartable))
}

func TestWalk_OfferWithoutAnOwnTokenExcludedIsReOffered(t *testing.T) {
	fs := Walk(loadFixture(t, "reoffer"))
	re := ofClass(fs, ClassReOffer)
	require.Len(t, re, 1, "%v", fs)
	require.Contains(t, re[0], `mob 100 topic "two" grants 2-start`)
	// Quest 3 excludes only 3-start; HasQuest(3-start) holds at 3-end, so
	// that offer is closed after completion and must not be flagged.
	require.NotContains(t, strings.Join(re, "\n"), `"three"`)
}

func TestWalk_StepNothingChecksIsAnOrphan(t *testing.T) {
	fs := Walk(loadFixture(t, "orphan"))
	orph := ofClass(fs, ClassOrphan)
	require.Len(t, orph, 1, "%v", fs)
	require.Contains(t, orph[0], `4-mid (quest 4 "Middle Step") is granted by mob 100 topic "step"`)
	require.Empty(t, ofClass(fs, ClassDeadEnd))
}

func TestWalk_GateNothingMeetsIsAnUnreachableTopic(t *testing.T) {
	fs := Walk(loadFixture(t, "unreachable"))
	un := ofClass(fs, ClassUnreachable)
	require.Len(t, un, 2, "%v", fs)
	joined := strings.Join(un, "\n")
	require.Contains(t, joined, `mob 100 topic "five" (offer): item 777: nothing reachable gives it`)
	require.Contains(t, joined, `mob 100 topic "five_done" (handin): questRequired 5-start is never reached`)
	st := ofClass(fs, ClassUnstartable)
	require.Len(t, st, 1, "%v", fs)
	require.Contains(t, st[0], "quest 5 \"Locked Out\": 5-start is never granted")
}

// The quest 29 shape: a converter ruling drops the only start grant while
// the quest's other parts still expect it to run.
func TestWalk_QuestNothingStartsIsUnstartable(t *testing.T) {
	in := loadFixture(t, "clean")
	topics := in.Files[0].File.Topics
	require.Equal(t, "errand", topics[0].Key)
	in.Files[0].File.Topics = topics[1:]
	fs := Walk(in)
	st := ofClass(fs, ClassUnstartable)
	require.Len(t, st, 1, "%v", fs)
	require.Contains(t, st[0], `quest 1 "Clean Errand": nothing grants 1-start`)
}

// A retired quest has no start grant by design: it is reported as RETIRED,
// not UNSTARTABLE, and its holders still reach the end through the hand-in.
func TestWalk_RetiredQuestIsInformationalNotUnstartable(t *testing.T) {
	in := loadFixture(t, "clean")
	in.Files[0].File.Topics = in.Files[0].File.Topics[1:] // drop the offer
	in.Retired = map[int]bool{1: true}
	fs := Walk(in)
	require.Empty(t, ofClass(fs, ClassUnstartable), "%v", fs)
	require.Empty(t, ofClass(fs, ClassDeadEnd), "%v", fs)
	ret := ofClass(fs, ClassRetired)
	require.Equal(t, []string{`quest 1 "Clean Errand": not startable by design; holders finish through its triggers`}, ret)
	require.False(t, isError(ClassRetired))
}

// Retiring does not switch the other checks off: with no hand-in, a holder
// at 1-start can never reach 1-end, which is still a DEAD END.
func TestWalk_RetiredQuestWhoseHoldersCannotFinishIsADeadEnd(t *testing.T) {
	in := loadFixture(t, "clean")
	in.Files[0].File.Topics = in.Files[0].File.Topics[2:] // drop offer and hand-in
	in.Retired = map[int]bool{1: true}
	fs := Walk(in)
	require.Empty(t, ofClass(fs, ClassUnstartable), "%v", fs)
	dead := ofClass(fs, ClassDeadEnd)
	require.Len(t, dead, 1, "%v", fs)
	require.Contains(t, dead[0], "can reach 1-start and never 1-end")
}

func TestWalk_UnmodeledEventsAndWorldItemsAreAssumed(t *testing.T) {
	fs := Walk(loadFixture(t, "assumed"))
	as := strings.Join(ofClass(fs, ClassAssumed), "\n")
	require.Contains(t, as, "quest 6 trigger 0 (room_enter): event room_enter not modeled")
	require.Contains(t, as, `mob 100 topic "six_done": item 600 assumed obtainable from rooms/zone_a/1000.yaml`)
	require.Empty(t, ofClass(fs, ClassDeadEnd), "%v", fs)
	require.Empty(t, ofClass(fs, ClassUnreachable), "%v", fs)
}

func TestWalk_BehaviorTopicFiresItsTreeGrant(t *testing.T) {
	in := loadFixture(t, "behavior")
	fs := Walk(in)
	require.Empty(t, ofClass(fs, ClassDeadEnd), "%v", fs)
	require.Contains(t, strings.Join(ofClass(fs, ClassAssumed), "\n"), "checked separately")

	// Without behavior: true the topic never fires the tree, and the quest
	// has no other way to end.
	for _, f := range in.Files {
		for i := range f.File.Topics {
			f.File.Topics[i].Behavior = false
		}
	}
	fs = Walk(in)
	dead := ofClass(fs, ClassDeadEnd)
	require.Len(t, dead, 1, "%v", fs)
	require.Contains(t, dead[0], "can reach 7-start and never 7-end; nothing grants a later step of quest 7")
	require.Contains(t, strings.Join(ofClass(fs, ClassAssumed), "\n"), `topic "report" on mob 300 is not behavior: true`)
}

func TestWalk_TreeGrantFiresNoQuestGrantedChain(t *testing.T) {
	fs := Walk(loadFixture(t, "chain"))
	dead := ofClass(fs, ClassDeadEnd)
	require.Len(t, dead, 1, "%v", fs)
	require.Contains(t, dead[0], "can reach 8-mid and never 8-end")
}

func TestParseReport_ReadsDeletionsAndBranchRewrites(t *testing.T) {
	p, err := parseReport("## Triggers to delete\n\n" +
		"- delete quest 66-the_addicts_plight.yaml trigger 0\n" +
		"- delete quest 66-the_addicts_plight.yaml trigger 1 (quest_granted 66-detox chain to 66-end, folded into mob 9323 topic wrapperhandoff)\n\n" +
		"## Behavior branches\n\n" +
		"- `behaviors/pothole_coulee/9100-cleric_hadwen.yaml` branch 2 (keywords [begin ready]): topic begin (R17): event: player_topic, check: topic_is with topic: begin replaces keyword_match; remove the speech action 2 that the topic text now speaks; remove grant_quest 29-start; keep grant_quest 30-end\n" +
		"- `behaviors/thornwall_city/117-barmaid_dal.yaml` branch 1 (keywords [gossip]): topic gossip: speech only: delete the branch\n")
	require.NoError(t, err)
	require.Equal(t, map[int]map[int]bool{66: {0: true, 1: true}}, p.DeleteTriggers)
	require.Equal(t, map[string]map[int]branchPlan{
		"behaviors/pothole_coulee/9100-cleric_hadwen.yaml": {2: {Topics: []string{"begin"}, RemoveGrants: []string{"29-start"}}},
	}, p.Branches)
}

func TestReport_ZoneFilterHidesOtherZonesAndSetsTheExitStatus(t *testing.T) {
	fs := []Finding{
		{Class: ClassDeadEnd, Zones: []string{"zone_b"}, Text: "elsewhere"},
		{Class: ClassOrphan, Zones: []string{"zone_a"}, Text: "here"},
		{Class: ClassAssumed, Zones: []string{unzoned}, Text: "nowhere"},
	}
	var out bytes.Buffer
	require.Equal(t, 0, report(&out, fs, "zone_a"), out.String())
	require.Contains(t, out.String(), "ORPHAN STEP: here")
	require.NotContains(t, out.String(), "elsewhere")
	require.NotContains(t, out.String(), "nowhere")
	require.Contains(t, out.String(), "(2 findings outside the filter not shown)")
	out.Reset()
	require.Equal(t, 1, report(&out, fs, ""), out.String())
	require.Contains(t, out.String(), "DEAD END: elsewhere")
}

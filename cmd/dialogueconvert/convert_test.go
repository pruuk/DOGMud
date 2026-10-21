package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

const fixtureWorld = "testdata/world"

// convertFixtures converts the whole fixture world. Every test reads its own
// mob out of the one result, and the reconciliation test reads all of them.
func convertFixtures(t *testing.T) *Result {
	t.Helper()
	in, err := LoadInput(fixtureWorld)
	require.NoError(t, err)
	return Convert(in)
}

// mustTopic returns the converted topic, failing the test when it is absent.
func mustTopic(t *testing.T, res *Result, mobId int, key string) *dialogue.Topic {
	t.Helper()
	tp := res.Topic(mobId, key)
	require.NotNil(t, tp, "mob %d has no topic %q; has %v", mobId, key, res.Keys(mobId))
	return tp
}

// anyLine reports whether some line contains every fragment.
func anyLine(lines []string, fragments ...string) bool {
	for _, l := range lines {
		ok := true
		for _, f := range fragments {
			if !strings.Contains(l, f) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// inlineInput builds an Input from YAML strings, for cases that would upset
// the fixture world's reconciliation.
func inlineInput(t *testing.T, dialogues map[int]string, questFiles map[string]string) *Input {
	t.Helper()
	in := NewInput()
	for mobId, src := range dialogues {
		require.NoError(t, in.AddDialogue("inline_zone", filepath.Join("inline_zone", "x.yaml"), []byte(src)), "mob %d", mobId)
	}
	for name, src := range questFiles {
		require.NoError(t, in.AddQuest(name, []byte(src)))
	}
	return in
}

func TestConvert_NodeKinds(t *testing.T) {
	res := convertFixtures(t)

	offer := mustTopic(t, res, 7001, "field_help")
	assert.Equal(t, dialogue.KindOffer, offer.Kind)
	assert.Equal(t, 7, offer.Quest)
	assert.False(t, offer.Marked)
	assert.Equal(t, "The Fallow Field", offer.Label, "offer label is the quest title")
	assert.Equal(t, "7-start", offer.GrantsQuest)
	assert.Equal(t, []string{"7-start", "7-end"}, offer.QuestExcluded)
	assert.Equal(t, "Find out what is killing the field.", offer.Text)
	assert.Equal(t, "Look at the soil.", offer.Hints)
	assert.Equal(t, "friendly", offer.MoodChange)
	assert.Empty(t, offer.PlayerLine)

	mid := mustTopic(t, res, 7001, "pelts_done")
	assert.Equal(t, dialogue.KindInfo, mid.Kind)
	assert.Equal(t, 0, mid.Quest)
	assert.True(t, mid.Marked, "a mid-quest grant is a marked info topic")
	assert.Equal(t, "", mid.Label)
	assert.Equal(t, []string{"7-start"}, mid.QuestRequired)
	assert.Equal(t, []string{"7-pelts"}, mid.QuestExcluded)
	assert.Equal(t, map[string]string{"7-path": "hunter"}, mid.QuestFlagRequired)
	assert.Equal(t, 24, mid.RequiresItem)
	assert.Equal(t, 25, mid.GivesItem)
	require.NotNil(t, mid.SetsQuestFlag)
	assert.Equal(t, dialogue.QuestFlagSet{Key: "7-path", Value: "done"}, *mid.SetsQuestFlag)
	assert.Equal(t, []dialogue.RepBump{{Faction: "farmers", Delta: 3}}, mid.BumpsRep)
	assert.Equal(t, 12, mid.GivesGold)
	assert.Equal(t, "7-pelts", mid.GrantsQuest)

	handin := mustTopic(t, res, 7001, "field_done")
	assert.Equal(t, dialogue.KindHandIn, handin.Kind)
	assert.Equal(t, 7, handin.Quest)
	assert.Equal(t, "The Fallow Field", handin.Label)
	assert.Equal(t, 99, handin.ItemExcluded)
	assert.Equal(t, 40, handin.MasterworkRequired)
	assert.Equal(t, "7-end", handin.GrantsQuest)

	plain := mustTopic(t, res, 7001, "weather")
	assert.Equal(t, dialogue.KindInfo, plain.Kind)
	assert.False(t, plain.Marked)
	assert.Empty(t, plain.GrantsQuest)
	assert.Equal(t, "Ask about the well.", plain.Hints)

	assert.Equal(t, []string{"field_help", "pelts_done", "field_done", "weather"}, res.Keys(7001),
		"authored order is kept")
	out := res.Files[7001].Output()
	assert.Equal(t, "Fixture Zone", out.Zone)
	assert.Equal(t, "neutral", out.DefaultMood)
	assert.Equal(t, []dialogue.Greeting{{Text: "The farmer looks up from the furrow.", Moods: []string{"neutral"}}}, out.Greetings)
	require.NotNil(t, out.Opener)
	assert.Equal(t, "Field's fallow. Has been all year.", out.Opener.Text)
}

func TestConvert_RequiresToHeardRequired(t *testing.T) {
	res := convertFixtures(t)

	assert.Equal(t, []string{"a"}, mustTopic(t, res, 7002, "b").HeardRequired, "requires naming a node id")
	assert.Equal(t, []string{"c"}, mustTopic(t, res, 7002, "d").HeardRequired, "requires naming an unlock goes to the unlocking node")
	assert.Empty(t, mustTopic(t, res, 7002, "a").HeardRequired)
	assert.Empty(t, mustTopic(t, res, 7002, "c").HeardRequired, "unlocks are dropped")
	assert.True(t, anyLine(res.Report.Requires, "mob 7002", "node d", `"x"`, "node c"), "D's mapping is reported: %v", res.Report.Requires)
}

func TestConvert_RequiresOnAnOfferIsReported(t *testing.T) {
	res := convertFixtures(t)

	f := mustTopic(t, res, 7003, "f")
	assert.Empty(t, f.HeardRequired)
	assert.True(t, anyLine(res.Report.Requires, "mob 7003", "node f", "requires names an offer"), "%v", res.Report.Requires)
}

func TestConvert_RootAndVariants(t *testing.T) {
	res := convertFixtures(t)

	out := res.Files[7004].Output()
	require.NotNil(t, out.Opener)
	assert.Equal(t, "Welcome to the mill.", out.Opener.Text)
	assert.Equal(t, "Ask about the grain.", out.Opener.Hints)
	require.Len(t, out.Opener.Variants, 1, "the granting variant leaves the opener")
	v := out.Opener.Variants[0]
	assert.Equal(t, "Back about the field?", v.Text)
	assert.Equal(t, "Tell me about the pelts.", v.Hints)
	assert.Equal(t, []string{"7-start"}, v.QuestRequired)
	assert.Equal(t, []string{"7-end"}, v.QuestExcluded)

	vt := mustTopic(t, res, 7004, "variant_2")
	assert.Equal(t, dialogue.KindOffer, vt.Kind)
	assert.Equal(t, 7, vt.Quest)
	assert.Equal(t, "7-start", vt.GrantsQuest)
	assert.Equal(t, []string{"7-start", "7-end"}, vt.QuestExcluded)
	assert.Equal(t, "You look like you need work. The field needs saving.", vt.Text)
	assert.True(t, anyLine(res.Report.Notes, "mob 7004", "variant 2", "variant_2"), "%v", res.Report.Notes)
}

func TestConvert_PatternClasses(t *testing.T) {
	res := convertFixtures(t)

	assert.Equal(t, []string{"missing_case", "crime_wave"}, res.Keys(7005),
		"catch-all, greeting and overlap are dropped")
	other := mustTopic(t, res, 7005, "crime_wave")
	assert.Equal(t, dialogue.KindInfo, other.Kind)
	assert.False(t, other.Marked)
	assert.Equal(t, "", other.Label)
	assert.Equal(t, "There is always crime in the alleys.\n---\nWe patrol when we can.", other.Text)
	assert.Equal(t, []string{"7-start"}, other.QuestRequired, "pattern gates are copied")
	assert.Equal(t, "friendly", other.MoodChange)

	// Counts over this one file: build it alone so the other fixtures do not add in.
	src, err := os.ReadFile(filepath.Join(fixtureWorld, "dialogue", "fixture_zone", "7005.yaml"))
	require.NoError(t, err)
	in := NewInput()
	require.NoError(t, in.AddDialogue("fixture_zone", "fixture_zone/7005.yaml", src))
	one := Convert(in)
	assert.Equal(t, map[string]int{"catchall": 1, "greeting": 1, "overlap": 1, "other": 1}, one.Report.Patterns)
	assert.True(t, anyLine(one.Report.Dropped, "mob 7005", "pattern 1", "catchall"), "%v", one.Report.Dropped)
	assert.True(t, anyLine(one.Report.Dropped, "mob 7005", "pattern 2", "greeting"), "%v", one.Report.Dropped)
	assert.True(t, anyLine(one.Report.Dropped, "mob 7005", "pattern 3", "overlap"), "%v", one.Report.Dropped)
}

func TestConvert_TreelessOpener(t *testing.T) {
	res := convertFixtures(t)

	out := res.Files[7006].Output()
	require.NotNil(t, out.Opener)
	assert.Equal(t, "Sit a moment. What tore this time?\n---\nBex. Cloth or leather, I can fix it.", out.Opener.Text,
		"every greeting response, folded by the zone pass")
	assert.Empty(t, out.Topics, "the catch-all is dropped")
	assert.Equal(t, "friendly", out.DefaultMood)
}

func TestConvert_TreelessOpenerFallsBackToTheCatchAll(t *testing.T) {
	in := inlineInput(t, map[int]string{8001: `
mobid: 8001
zone: Inline Zone
defaultMood: neutral
patterns:
  - keywords: ["road"]
    responses: ["The road is long."]
  - keywords: [""]
    responses: ["I keep the bar.", "Ask someone else."]
`}, nil)
	res := Convert(in)
	out := res.Files[8001].Output()
	require.NotNil(t, out.Opener)
	assert.Equal(t, "I keep the bar.\n---\nAsk someone else.", out.Opener.Text)
	assert.Equal(t, []string{"road"}, res.Keys(8001))
}

func TestConvert_ChargesGetGoldRequired(t *testing.T) {
	res := convertFixtures(t)

	tp := mustTopic(t, res, 7007, "buy_map")
	assert.Equal(t, 10, tp.ChargesGold)
	assert.Equal(t, 10, tp.GoldRequired)
}

func TestConvert_ItemGiveMergesIntoTopic(t *testing.T) {
	res := convertFixtures(t)

	tp := mustTopic(t, res, 94, "protection_report")
	assert.Equal(t, []dialogue.RepBump{{Faction: "thornwall_watch", Delta: 5}}, tp.BumpsRep, "bump_rep appended")
	assert.Equal(t, "10-report", tp.GrantsQuest)
	assert.Equal(t, 30, tp.RequiresItem)
	assert.Equal(t, []string{"10-start"}, tp.QuestRequired)
	assert.Equal(t, []string{"10-report", "10-end"}, tp.QuestExcluded)
	assert.Equal(t, "You have evidence of a protection racket? Show me.", tp.Text, "text is the zone pass's to reconcile")
	assert.Equal(t, "friendly", tp.MoodChange)
	assert.Equal(t, 0, tp.GivesGold)
	assert.Nil(t, tp.SetsQuestFlag)
	assert.Equal(t, []string{"protection_report"}, res.Keys(94), "no extra topic")

	assert.True(t, anyLine(res.Report.MergeLines, "mob 94", "protection_report", "A protection notice? Let me see that.", "This is enough to act on."),
		"%v", res.Report.MergeLines)
	assert.True(t, anyLine(res.Report.MergeLines, "mob 94", "protection_report", "added bumpsRep thornwall_watch 5"),
		"an effect the merge adds is named: %v", res.Report.MergeLines)
	assert.True(t, anyLine(res.Report.Triggers, "delete quest 10-the_drowning_posts_debt.yaml trigger 0"), "%v", res.Report.Triggers)
}

func TestConvert_ItemGiveMirrorEffectsAreNotDoubled(t *testing.T) {
	// The ask route and the give route were alternatives: a topic that already
	// carries the trigger's reward must not pay it twice.
	in := inlineInput(t, map[int]string{335: `
mobid: 335
zone: Inline Zone
defaultMood: neutral
patterns: []
tree:
  root: {text: "Bounty office."}
  nodes:
    - id: drunnhandoff
      triggers: ["proof"]
      requiresItem: 40054
      grantsQuest: "19-end"
      givesGold: 350
      givesItem: 5
      setsQuestFlag: {key: "19-completion", value: "full"}
      bumpsRep: [{faction: watch, delta: 5}]
      text: "Paid in full."
`}, map[string]string{"19-bounty.yaml": `
questid: 19
name: The Lake Caves Bounty
steps: [{id: start}, {id: end}]
triggers:
  - event: item_give
    mob: 335
    item: 40054
    conditions: {has: ["19-start"], missing: ["19-end"]}
    actions:
      - consume_item: 40054
      - grant: "19-end"
      - set_flag: {key: "19-completion", value: "full"}
      - give_gold: 350
      - give_item: 5
      - bump_rep: {faction: watch, delta: 5}
`})
	res := Convert(in)
	tp := mustTopic(t, res, 335, "drunnhandoff")
	assert.Equal(t, 350, tp.GivesGold, "same gold is not summed")
	assert.Equal(t, 5, tp.GivesItem)
	assert.Equal(t, []dialogue.RepBump{{Faction: "watch", Delta: 5}}, tp.BumpsRep, "same bump is not appended twice")
	assert.Equal(t, dialogue.QuestFlagSet{Key: "19-completion", Value: "full"}, *tp.SetsQuestFlag)
	assert.Empty(t, res.Report.Clashes)
	assert.False(t, anyLine(res.Report.MergeLines, "added"), "nothing was added: %v", res.Report.MergeLines)
	assert.True(t, anyLine(res.Report.Triggers, "delete quest 19-bounty.yaml trigger 0"), "%v", res.Report.Triggers)
}

func TestConvert_ItemGiveClashesAreReportedAndHeld(t *testing.T) {
	in := inlineInput(t, map[int]string{335: `
mobid: 335
zone: Inline Zone
defaultMood: neutral
patterns: []
tree:
  root: {text: "Bounty office."}
  nodes:
    - id: drunnhandoff
      triggers: ["proof"]
      requiresItem: 40054
      grantsQuest: "19-end"
      givesGold: 300
      setsQuestFlag: {key: "19-completion", value: "half"}
      text: "Paid."
`}, map[string]string{"19-bounty.yaml": `
questid: 19
name: The Lake Caves Bounty
steps: [{id: start}, {id: end}]
triggers:
  - event: item_give
    mob: 335
    item: 40054
    actions:
      - grant: "19-end"
      - set_flag: {key: "19-completion", value: "full"}
      - give_gold: 350
`})
	res := Convert(in)
	tp := mustTopic(t, res, 335, "drunnhandoff")
	assert.Equal(t, 300, tp.GivesGold, "a clash leaves the topic as authored")
	assert.Equal(t, "half", tp.SetsQuestFlag.Value)
	assert.True(t, anyLine(res.Report.Clashes, "19-bounty.yaml trigger 0", "give_gold 350", "givesGold 300"), "%v", res.Report.Clashes)
	assert.True(t, anyLine(res.Report.Clashes, "19-bounty.yaml trigger 0", "set_flag"), "%v", res.Report.Clashes)
	assert.True(t, anyLine(res.Report.HeldTriggers, "quest 19-bounty.yaml trigger 0"), "%v", res.Report.HeldTriggers)
	assert.False(t, anyLine(res.Report.Triggers, "19-bounty.yaml"), "a held trigger is not listed for deletion")
}

func TestConvert_ItemGiveGrantMismatchIsNotMerged(t *testing.T) {
	in := inlineInput(t, map[int]string{94: `
mobid: 94
zone: Inline Zone
defaultMood: neutral
patterns: []
tree:
  root: {text: "Civilian."}
  nodes:
    - id: velkhandoff
      triggers: ["ledger"]
      requiresItem: 40036
      grantsQuest: "14-end"
      text: "Names. Dates."
`}, map[string]string{"14-undertow.yaml": `
questid: 14
name: The Undertow
steps: [{id: start}, {id: report}, {id: end}]
triggers:
  - event: item_give
    mob: 94
    item: 40036
    actions:
      - grant: "14-report"
      - bump_rep: {faction: watch, delta: 5}
`})
	res := Convert(in)
	tp := mustTopic(t, res, 94, "velkhandoff")
	assert.Empty(t, tp.BumpsRep, "nothing merges across a grant mismatch")
	assert.Equal(t, []string{"velkhandoff"}, res.Keys(94))
	assert.True(t, anyLine(res.Report.Clashes, "14-undertow.yaml trigger 0", "grant mismatch", "14-report", "14-end"), "%v", res.Report.Clashes)
	assert.True(t, anyLine(res.Report.HeldTriggers, "14-undertow.yaml trigger 0"), "%v", res.Report.HeldTriggers)
	assert.Equal(t, 1, res.Report.GrantsIn[srcItemGive])
	assert.Equal(t, 0, res.Report.GrantsOut[srcItemGive], "the unmerged grant shows in the reconciliation")
	assert.False(t, res.Report.Reconciled())
}

func TestConvert_ItemGiveWithNoTopicCreatesOne(t *testing.T) {
	res := convertFixtures(t)

	tp := mustTopic(t, res, 9116, "give_10009")
	assert.Equal(t, dialogue.KindHandIn, tp.Kind, "kind from the grant token")
	assert.Equal(t, 35, tp.Quest)
	assert.Equal(t, "First Heat", tp.Label)
	assert.Equal(t, []string{"35-start"}, tp.QuestRequired)
	assert.Equal(t, []string{"35-end"}, tp.QuestExcluded, "missing plus the grant and end token, once each")
	assert.Equal(t, 10009, tp.RequiresItem)
	assert.Equal(t, 10009, tp.GivesItem)
	assert.Equal(t, "35-end", tp.GrantsQuest)
	assert.Equal(t, "Yours. You made it.", tp.Text)
	assert.Equal(t, []string{"forge", "give_10009"}, res.Keys(9116))
	assert.True(t, anyLine(res.Report.Triggers, "delete quest 35-first_heat.yaml trigger 0"), "%v", res.Report.Triggers)
}

func TestConvert_UnmappableActionsAreReported(t *testing.T) {
	res := convertFixtures(t)

	tp := mustTopic(t, res, 9360, "give_40115")
	assert.Equal(t, dialogue.KindInfo, tp.Kind)
	assert.True(t, tp.Marked)
	assert.Equal(t, []string{"69-start"}, tp.QuestRequired)
	assert.Equal(t, []string{"69-gallery", "69-end"}, tp.QuestExcluded)
	assert.True(t, anyLine(res.Report.Unmappable, "quest 69-the_gallery_cipher.yaml", "trigger 0", "action 1", "actor", "Dross takes the rubbing"),
		"%v", res.Report.Unmappable)
	assert.True(t, anyLine(res.Report.HeldTriggers, "69-the_gallery_cipher.yaml trigger 0"), "%v", res.Report.HeldTriggers)
}

func TestConvert_DialogueTrigger(t *testing.T) {
	res := convertFixtures(t)

	tp := mustTopic(t, res, 9491, "asked_28")
	assert.Equal(t, dialogue.KindInfo, tp.Kind)
	assert.True(t, tp.Marked)
	assert.Equal(t, []string{"28-said"}, tp.QuestRequired)
	assert.Equal(t, []string{"28-asked", "28-end"}, tp.QuestExcluded)
	assert.Equal(t, "28-asked", tp.GrantsQuest)
	assert.True(t, anyLine(res.Report.Triggers, "delete quest 28-waking_to_gaius.yaml trigger 0"), "%v", res.Report.Triggers)
	assert.True(t, anyLine(res.Report.Notes, "mob 9491", "asked_28", "world"),
		"topics gated on the granted token are named for the zone pass: %v", res.Report.Notes)
}

func TestConvert_BehaviorAskBranches(t *testing.T) {
	res := convertFixtures(t)

	require.NotNil(t, res.Files[9571], "a behavior-only mob gets a new file")
	assert.Equal(t, "stillwater", res.Files[9571].Folder)
	out := res.Files[9571].Output()
	assert.Equal(t, 9571, out.MobId)
	assert.Equal(t, "Stillwater", out.Zone)
	assert.True(t, anyLine(res.Report.NewFiles, "dialogue/stillwater/9571.yaml"), "%v", res.Report.NewFiles)

	assert.Equal(t, []string{"to_confluence", "to_plymouth"}, res.Keys(9571),
		"one ferry topic per destination; the greeting branch is the opener, not a topic")
	for _, key := range []string{"to_confluence", "to_plymouth"} {
		tp := mustTopic(t, res, 9571, key)
		assert.Equal(t, dialogue.KindInfo, tp.Kind)
		assert.True(t, tp.Behavior, key)
	}
	require.NotNil(t, out.Opener, "a file with no opener takes its first ungated speech-only greeting branch")
	assert.Equal(t, "I book passage on the water.\n---\n<ansi fg=\"181\">  [Try: ask agent passage plymouth]</ansi>", out.Opener.Text)
	assert.Empty(t, out.Opener.Variants)

	path := "behaviors/stillwater/9571-the_ferry_agent.yaml"
	assert.True(t, anyLine(res.Report.Behavior, path, "branch 1", "to_plymouth", "destination: plymouth"), "%v", res.Report.Behavior)
	assert.True(t, anyLine(res.Report.Behavior, path, "branch 1", "to_confluence", "destination: confluence"), "%v", res.Report.Behavior)
	assert.True(t, anyLine(res.Report.Behavior, path, "branch 2", "became the opener", "delete the branch"), "%v", res.Report.Behavior)
	assert.False(t, anyLine(res.Report.NoOpener, "mob 9571"), "%v", res.Report.NoOpener)

	assert.Nil(t, res.Topic(7001, "ask_1"), "trees under behaviors/rooms are not mob trees")
	assert.False(t, anyLine(res.Report.Behavior, "behaviors/rooms"), "%v", res.Report.Behavior)
}

func TestConvert_InstanceZoneBranches(t *testing.T) {
	res := convertFixtures(t)

	tp := mustTopic(t, res, 9553, "open_crash")
	assert.True(t, tp.Behavior)
	assert.Equal(t, dialogue.KindInfo, tp.Kind)
	assert.Equal(t, 0, tp.RequiresItem, "the disc is a reusable key: no topic requiresItem")
	assert.Equal(t, 40168, tp.CarriesItem, "shown only to disc holders, never taken")
	assert.Equal(t, "eastern_highlands", res.Files[9553].Folder)
	assert.True(t, anyLine(res.Report.Behavior, "9553-the_threshold_keeper.yaml", "open_crash", "zone: crash", "guidance:", "player_has_item"),
		"%v", res.Report.Behavior)

	fallback := mustTopic(t, res, 9553, "crash")
	assert.Equal(t, 40168, fallback.ItemExcluded, "the no-disc answer hides from disc holders")
	assert.Equal(t, 0, fallback.CarriesItem)
	assert.True(t, anyLine(res.Report.Fallbacks, "9553-the_threshold_keeper.yaml", "branch 2", "branch 1", "itemExcluded 40168"),
		"%v", res.Report.Fallbacks)
	assert.True(t, anyLine(res.Report.NoOpener, "mob 9553"), "no greeting branch: reported: %v", res.Report.NoOpener)
}

func TestConvert_BehaviorFallbacks(t *testing.T) {
	res := convertFixtures(t)
	path := "behaviors/fixture_zone/9601-a_warden.yaml"

	out := res.Files[9601].Output()
	require.NotNil(t, out.Opener)
	assert.Equal(t, "Hello.\n---\nMind the arch.", out.Opener.Text)
	require.Len(t, out.Opener.Variants, 1, "the gated greeting above it becomes a variant")
	assert.Equal(t, "Welcome back.", out.Opener.Variants[0].Text)
	assert.Equal(t, []string{"40-end"}, out.Opener.Variants[0].QuestRequired)
	assert.Equal(t, []string{"passage", "passage_2", "begin", "begin_2"}, res.Keys(9601))

	open := mustTopic(t, res, 9601, "passage")
	assert.Equal(t, []string{"40-start"}, open.QuestRequired)
	notYet := mustTopic(t, res, 9601, "passage_2")
	assert.Equal(t, []string{"40-start"}, notYet.QuestExcluded, "inverse of the branch above")
	assert.True(t, anyLine(res.Report.Fallbacks, path, "branch 4", "branch 3", "questExcluded 40-start"), "%v", res.Report.Fallbacks)

	assert.Empty(t, mustTopic(t, res, 9601, "begin_2").QuestExcluded)
	assert.True(t, anyLine(res.Report.Clashes, path, "branch 6", "branch 5", "state_equals", "could not gate"), "%v", res.Report.Clashes)

	assert.True(t, anyLine(res.Report.Suffixed, "mob 9601", "passage_2", "passage"), "%v", res.Report.Suffixed)
	assert.True(t, anyLine(res.Report.Suffixed, "mob 9601", "begin_2", "begin"), "%v", res.Report.Suffixed)
}

func TestConvert_RequiresPicksTheEarliestUnlocker(t *testing.T) {
	res := convertFixtures(t)

	assert.Equal(t, []string{"people"}, mustTopic(t, res, 7008, "needs").HeardRequired,
		"people unlocks exile_story and comes first, so hearing people was enough")
	assert.Equal(t, []string{"people"}, mustTopic(t, res, 7008, "exile_story").HeardRequired)
	assert.True(t, anyLine(res.Report.Requires, "mob 7008", "node needs", "exile_story", "people", "earliest"), "%v", res.Report.Requires)
}

func TestConvert_PatternReasons(t *testing.T) {
	res := convertFixtures(t)

	assert.True(t, anyLine(res.Report.Dropped, "mob 7009", "pattern 1", "live fallback, breadcrumb candidate", "find_her"),
		"%v", res.Report.Dropped)
	assert.True(t, anyLine(res.Report.Dropped, "mob 7005", "pattern 3", "overlap", "missing_case"), "%v", res.Report.Dropped)
	assert.False(t, anyLine(res.Report.Dropped, "mob 7005", "live fallback"), "an ungated node was the answer")

	assert.NotNil(t, res.Topic(7009, "hello"), "still emitted as a topic")
	assert.True(t, anyLine(res.Report.FoldIntoOpener, "mob 7009", "hello", "tavern"), "%v", res.Report.FoldIntoOpener)
	assert.False(t, anyLine(res.Report.FoldIntoOpener, "mob 7005"), "%v", res.Report.FoldIntoOpener)

	assert.True(t, anyLine(res.Report.Dropped, "mob 7009", "pattern 3", "greeting", "moods [grateful]", "moodChange friendly", "questFlagRequired", "7-path"),
		"%v", res.Report.Dropped)
}

func TestConvert_BehaviorGrantQuestBecomesOffer(t *testing.T) {
	res := convertFixtures(t)

	offer := mustTopic(t, res, 9600, "work")
	assert.Equal(t, dialogue.KindOffer, offer.Kind)
	assert.Equal(t, 40, offer.Quest)
	assert.Equal(t, "The Ferryman's Errand", offer.Label)
	assert.Equal(t, "40-start", offer.GrantsQuest)
	assert.Equal(t, []string{"40-start", "40-end"}, offer.QuestExcluded, "the grant and end token keep it from being re-offered")
	assert.Equal(t, "Carry this to the far pier for me.", offer.Text)
	assert.False(t, offer.Behavior)

	act := mustTopic(t, res, 9600, "pier")
	assert.True(t, act.Behavior)
	assert.Equal(t, []string{"40-start"}, act.QuestRequired)
	assert.Equal(t, []string{"40-end"}, act.QuestExcluded)
	assert.Equal(t, "Step aboard, then.", act.Text)

	path := "behaviors/fixture_zone/9600-a_ferryman.yaml"
	assert.True(t, anyLine(res.Report.Behavior, path, "branch 1", "work", "delete the branch"), "%v", res.Report.Behavior)
	assert.True(t, anyLine(res.Report.Behavior, path, "branch 2", "pier", "player_topic", "topic_is"), "%v", res.Report.Behavior)
	assert.Nil(t, res.Files[9600].Output().Opener)
	assert.True(t, anyLine(res.Report.NoOpener, "mob 9600"), "%v", res.Report.NoOpener)
}

func TestConvert_BehaviorBranchWithoutKeywordsIsFlagged(t *testing.T) {
	// The bandit leader's ask branch ran on every ask to cancel his
	// countdown; a topic runs only when chosen, so the owner decides.
	in := NewInput()
	in.MobZones[254] = "Marches Spur Road"
	require.NoError(t, in.AddBehavior("behaviors/marches_spur_road/254-bandit_leader.yaml", 254, []byte(`
tree:
  type: selector
  children:
    - type: sequence
      event: player_ask
      children:
        - type: action
          do: set_state
          key: negotiating
          value: "true"
        - type: condition
          check: random_chance
          percent: 0
`)))
	res := Convert(in)
	tp := mustTopic(t, res, 254, "ask_1")
	assert.True(t, tp.Behavior)
	assert.True(t, anyLine(res.Report.Clashes, "254-bandit_leader.yaml", "branch 1", "no keyword_match", "every ask"), "%v", res.Report.Clashes)
	assert.True(t, anyLine(res.Report.Behavior, "254-bandit_leader.yaml", "ask_1", "random_chance", "set_state"), "%v", res.Report.Behavior)
}

func TestConvert_LabelsLeftEmptyForTheZonePass(t *testing.T) {
	res := convertFixtures(t)
	tmp := t.TempDir()
	require.NoError(t, WriteOutput(res, tmp))

	raw, err := os.ReadFile(filepath.Join(tmp, "dialogue", "fixture_zone", "7001.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), `label: ""`, "an empty label is written so the validator fails until it is filled")
	for _, gone := range []string{"patterns:", "tree:", "memory:", "triggers:"} {
		assert.NotContains(t, string(raw), gone)
	}

	var df dialogue.DialogueFile
	require.NoError(t, yaml.UnmarshalStrict(raw, &df), "output loads as a DialogueFile")
	require.Len(t, df.Topics, 4)
	assert.Equal(t, "", df.Topics[3].Label)
	assert.Equal(t, "weather", df.Topics[3].Key)

	_, err = os.Stat(filepath.Join(tmp, "dialogue", "stillwater", "9571.yaml"))
	assert.NoError(t, err, "new behavior-only files are written")
	report, err := os.ReadFile(filepath.Join(tmp, "report.md"))
	require.NoError(t, err)
	for _, section := range []string{"## Counts", "## Requires mappings", "## Unmappable actions", "## Merge lines",
		"## Triggers to delete", "## Behavior branches", "## New files"} {
		assert.Contains(t, string(report), section)
	}
}

func TestConvert_ReconciliationCounts(t *testing.T) {
	res := convertFixtures(t)
	r := res.Report

	// Input side, counted from the fixtures by hand.
	want := map[string]int{
		srcNodeStart: 2, // 7001 field_help, 7003 e
		srcNodeEnd:   1, // 7001 field_done
		srcNodeMid:   2, // 7001 pelts_done, 94 protection_report
		srcVariant:   1, // 7004 variant 2
		srcItemGive:  3, // quests 10, 35, 69
		srcDialogue:  1, // quest 28
		srcBehavior:  1, // 9600 work
	}
	assert.Equal(t, want, nonZero(r.GrantsIn), "grants by source, input side")
	assert.Equal(t, nonZero(r.GrantsIn), nonZero(r.GrantsOut), "every component maps")
	assert.Equal(t, 11, r.GrantsInTotal())
	assert.Equal(t, r.GrantsInTotal(), r.GrantsOutTotal(), "and the totals agree")
	assert.Equal(t, 10, r.TopicsCarryingGrants, "the item_give grant merged into protection_report shares its topic")
	assert.True(t, r.Reconciled())

	md := r.Markdown()
	assert.Contains(t, md, "| total | 11 | 11 |")
	assert.Contains(t, md, "Reconciled: yes")
}

func TestOutDirInsideTheRepoIsRefused(t *testing.T) {
	repo := t.TempDir()
	assert.Error(t, checkOutDir(filepath.Join(repo, "out"), repo))
	assert.Error(t, checkOutDir(repo, repo))
	assert.NoError(t, checkOutDir(filepath.Join(t.TempDir(), "convert1"), repo))
}

func TestPatternClass(t *testing.T) {
	triggers := map[string]bool{"missing": true}
	assert.Equal(t, "catchall", patternClass(legacyPattern{Keywords: []string{""}}, triggers))
	assert.Equal(t, "catchall", patternClass(legacyPattern{Keywords: []string{" "}}, triggers))
	assert.Equal(t, "greeting", patternClass(legacyPattern{Keywords: []string{"Hello", "good day"}}, triggers))
	assert.Equal(t, "overlap", patternClass(legacyPattern{Keywords: []string{"hello", "missing"}}, triggers))
	assert.Equal(t, "other", patternClass(legacyPattern{Keywords: []string{"hello", "crime"}}, triggers))
}

func TestSlug(t *testing.T) {
	assert.Equal(t, "field_help", slug("Field-Help"))
	assert.Equal(t, "good_day", slug(" good day "))
	assert.Equal(t, "a_b", slug("a'b"))
}

func nonZero(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m {
		if v != 0 {
			out[k] = v
		}
	}
	return out
}

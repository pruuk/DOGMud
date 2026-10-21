package main

import (
	"os"
	"path/filepath"
	"strconv"
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

func TestConvert_RequiresOnAnOfferBecomesAQuestGate(t *testing.T) {
	// Settled ruling (mobs 95, 96): requires naming an offer becomes a gate on
	// the quest that offer starts.
	res := convertFixtures(t)

	f := mustTopic(t, res, 7003, "f")
	assert.Empty(t, f.HeardRequired)
	assert.Equal(t, []string{"40-start"}, f.QuestRequired)
	assert.True(t, anyLine(res.Report.Requires, "mob 7003", "node f", "requires names an offer", "questRequired 40-start"), "%v", res.Report.Requires)
	assert.False(t, anyLine(res.Report.Requires, "mob 7003", "no heardRequired written"), "%v", res.Report.Requires)
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
		"catch-all, greeting and an overlap the node already says are dropped")
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
	assert.Equal(t, 0, tp.RequiresItem, "the dagger is shown, never taken")
	assert.Equal(t, 0, tp.GivesItem)
	assert.Equal(t, 10009, tp.CarriesItem)
	assert.Equal(t, "35-end", tp.GrantsQuest)
	assert.Equal(t, "Yours. You made it.", tp.Text)
	assert.Equal(t, []string{"forge", "give_10009"}, res.Keys(9116))
	assert.True(t, anyLine(res.Report.Triggers, "delete quest 35-first_heat.yaml trigger 0"), "%v", res.Report.Triggers)
}

func TestConvert_ItemShownAndReturnedIsCarriesItem(t *testing.T) {
	// Settled ruling (Rusk's dagger, Dross's rubbing): a give route that takes
	// the item and hands the same item back is a carriesItem gate, merged or
	// created.
	in := inlineInput(t, map[int]string{9116: `
mobid: 9116
zone: Inline Zone
defaultMood: neutral
patterns: []
tree:
  root: {text: "Forge's hot."}
  nodes:
    - id: dagger
      triggers: ["dagger"]
      questRequired: ["35-craft"]
      questExcluded: ["35-end"]
      requiresItem: 10009
      grantsQuest: "35-end"
      text: "Let me see it."
`}, map[string]string{"35-first_heat.yaml": `
questid: 35
name: First Heat
steps: [{id: start}, {id: craft}, {id: end}]
triggers:
  - event: item_give
    mob: 9116
    item: 10009
    conditions: {has: ["35-craft"], missing: ["35-end"]}
    actions:
      - consume_item: 10009
      - give_item: 10009
      - grant: "35-end"
`})
	res := Convert(in)
	tp := mustTopic(t, res, 9116, "dagger")
	assert.Equal(t, 10009, tp.CarriesItem)
	assert.Equal(t, 0, tp.RequiresItem)
	assert.Equal(t, 0, tp.GivesItem)
	assert.True(t, anyLine(res.Report.Rulings, "mob 9116 topic dagger", "carriesItem 10009", "shown, never taken"), "%v", res.Report.Rulings)
	assert.True(t, anyLine(res.Report.Triggers, "delete quest 35-first_heat.yaml trigger 0"), "%v", res.Report.Triggers)
}

func TestConvert_GiveNarrationIsDropped(t *testing.T) {
	// R16: an actor line only the old give route printed is dropped, named in
	// the report, and does not hold the trigger.
	res := convertFixtures(t)

	tp := mustTopic(t, res, 9360, "give_40115")
	assert.Equal(t, dialogue.KindInfo, tp.Kind)
	assert.True(t, tp.Marked)
	assert.Equal(t, []string{"69-start"}, tp.QuestRequired)
	assert.Equal(t, []string{"69-gallery", "69-end"}, tp.QuestExcluded)
	assert.True(t, anyLine(res.Report.DroppedNarration, "quest 69-the_gallery_cipher.yaml trigger 0", "action 1", "actor", "Dross takes the rubbing"),
		"%v", res.Report.DroppedNarration)
	assert.False(t, anyLine(res.Report.Unmappable, "69-the_gallery_cipher.yaml"), "%v", res.Report.Unmappable)
	assert.False(t, anyLine(res.Report.HeldTriggers, "69-the_gallery_cipher.yaml"), "%v", res.Report.HeldTriggers)
	assert.True(t, anyLine(res.Report.Triggers, "delete quest 69-the_gallery_cipher.yaml trigger 0"), "%v", res.Report.Triggers)
}

func TestConvert_UnmappableActionsAreReported(t *testing.T) {
	in := inlineInput(t, map[int]string{9360: `
mobid: 9360
zone: Inline Zone
defaultMood: neutral
patterns: []
tree:
  root: {text: "Gallery."}
  nodes: []
`}, map[string]string{"69-gallery.yaml": `
questid: 69
name: The Gallery Cipher
steps: [{id: start}, {id: gallery}, {id: end}]
triggers:
  - event: item_give
    mob: 9360
    item: 40115
    conditions: {has: ["69-start"], missing: ["69-gallery"]}
    actions:
      - grant: "69-gallery"
      - teleport: 5000
`})
	res := Convert(in)
	mustTopic(t, res, 9360, "give_40115")
	assert.True(t, anyLine(res.Report.Unmappable, "quest 69-gallery.yaml", "trigger 0", "action 1", "teleport"), "%v", res.Report.Unmappable)
	assert.True(t, anyLine(res.Report.HeldTriggers, "69-gallery.yaml trigger 0"), "%v", res.Report.HeldTriggers)
}

func TestConvert_ReportRouteBecomesDirectEnd(t *testing.T) {
	// Settled ruling (quests 14, 63, 65, 67): a give route granting a token
	// whose quest_granted chain grants the end merges into the -end hand-in;
	// the give trigger and its chain are both deleted.
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
      questRequired: ["14-evidence"]
      questExcluded: ["14-end"]
      requiresItem: 40036
      grantsQuest: "14-end"
      bumpsRep: [{faction: bloom_trade, delta: -15}]
      text: "Names. Dates."
`}, map[string]string{"14-undertow.yaml": `
questid: 14
name: The Undertow
steps: [{id: start}, {id: evidence}, {id: report}, {id: end}]
triggers:
  - event: item_give
    mob: 94
    item: 40036
    conditions: {has: ["14-evidence"], missing: ["14-report"]}
    actions:
      - grant: "14-report"
      - npc_say:
          mob: 94
          lines:
            - {delay: 1, text: "A bribe ledger? Let me see that."}
  - event: quest_granted
    quest_token: "14-report"
    conditions: {missing: ["14-end"]}
    actions:
      - bump_rep: {faction: bloom_trade, delta: -15}
      - set_flag: {key: "14-outcome", value: "arrests"}
      - grant: "14-end"
`})
	res := Convert(in)
	tp := mustTopic(t, res, 94, "velkhandoff")
	assert.Equal(t, "14-end", tp.GrantsQuest)
	assert.Equal(t, dialogue.KindHandIn, tp.Kind)
	assert.Equal(t, []dialogue.RepBump{{Faction: "bloom_trade", Delta: -15}}, tp.BumpsRep, "the chain's same bump is not doubled")
	require.NotNil(t, tp.SetsQuestFlag, "a chain effect the topic lacks is added")
	assert.Equal(t, dialogue.QuestFlagSet{Key: "14-outcome", Value: "arrests"}, *tp.SetsQuestFlag)
	assert.Empty(t, res.Report.Clashes)
	assert.Empty(t, res.Report.HeldTriggers)
	assert.True(t, anyLine(res.Report.MergeLines, "mob 94 topic velkhandoff", "A bribe ledger?"), "%v", res.Report.MergeLines)
	assert.True(t, anyLine(res.Report.MergeLines, "mob 94 topic velkhandoff", "14-report", "14-end", "chain"), "%v", res.Report.MergeLines)
	assert.True(t, anyLine(res.Report.Triggers, "delete quest 14-undertow.yaml trigger 0"), "%v", res.Report.Triggers)
	assert.True(t, anyLine(res.Report.Triggers, "delete quest 14-undertow.yaml trigger 1", "quest_granted 14-report"), "%v", res.Report.Triggers)
	assert.Equal(t, 1, res.Report.GrantsOut[srcItemGive])
	assert.True(t, res.Report.Reconciled())
}

func TestConvert_SharedChainIsDeletedOnce(t *testing.T) {
	// Quest 63: two give routes to two mobs share one quest_granted chain.
	// The chain is listed once, and the list keeps trigger order.
	handoff := func(mob int) string {
		return `
mobid: ` + strconv.Itoa(mob) + `
zone: Inline Zone
defaultMood: neutral
patterns: []
tree:
  root: {text: "Docks."}
  nodes:
    - id: tallyhandoff
      triggers: ["tally"]
      questRequired: ["63-evidence"]
      questExcluded: ["63-end"]
      requiresItem: 40100
      grantsQuest: "63-end"
      text: "Two columns."
`
	}
	in := inlineInput(t, map[int]string{9316: handoff(9316), 9300: handoff(9300)}, map[string]string{"63-dock_rat.yaml": `
questid: 63
name: Dock Rat
steps: [{id: start}, {id: evidence}, {id: report}, {id: end}]
triggers:
  - event: item_give
    mob: 9316
    item: 40100
    actions: [{grant: "63-report"}]
  - event: item_give
    mob: 9300
    item: 40100
    actions: [{grant: "63-report"}]
  - event: quest_granted
    quest_token: "63-report"
    actions: [{grant: "63-end"}]
`})
	res := Convert(in)
	var lines []string
	for _, l := range res.Report.Triggers {
		lines = append(lines, strings.SplitN(l, " (", 2)[0])
	}
	assert.Equal(t, []string{
		"delete quest 63-dock_rat.yaml trigger 0",
		"delete quest 63-dock_rat.yaml trigger 1",
		"delete quest 63-dock_rat.yaml trigger 2",
	}, lines)
	assert.Equal(t, 2, res.Report.GrantsOut[srcItemGive])
	assert.True(t, res.Report.Reconciled())
}

func TestConvert_GiveBeforeAcceptingIsDeleted(t *testing.T) {
	// R15 (quests 15, 16): the give route that starts the quest is deleted,
	// not merged; the hand-in keeps its -end grant; the chain that completed
	// the quest on -start (a live bug) is deleted too.
	in := inlineInput(t, map[int]string{250: `
mobid: 250
zone: Inline Zone
defaultMood: neutral
patterns: []
tree:
  root: {text: "Freight."}
  nodes:
    - id: freight_offer
      triggers: ["freight"]
      questExcluded: ["15-start", "15-end"]
      grantsQuest: "15-start"
      text: "My crate went missing."
    - id: quest_complete_dialogue
      triggers: ["crate"]
      questRequired: ["15-start"]
      questExcluded: ["15-end"]
      requiresItem: 40039
      grantsQuest: "15-end"
      text: "That is the one."
`}, map[string]string{"15-freight.yaml": `
questid: 15
name: The Peddler's Overdue Freight
steps: [{id: start}, {id: end}]
triggers:
  - event: item_give
    mob: 250
    item: 40039
    conditions: {has: ["15-start"], missing: ["15-end"]}
    actions:
      - grant: "15-end"
  - event: item_give
    mob: 250
    item: 40039
    conditions: {missing: ["15-start"]}
    actions:
      - grant: "15-start"
      - npc_say:
          mob: 250
          lines:
            - {delay: 1, text: "My freight! Where did you find it?"}
  - event: quest_granted
    quest_token: "15-start"
    conditions: {missing: ["15-end"]}
    actions:
      - grant: "15-end"
`})
	res := Convert(in)
	assert.Equal(t, []string{"freight_offer", "quest_complete_dialogue"}, res.Keys(250), "no topic from the early give")
	tp := mustTopic(t, res, 250, "quest_complete_dialogue")
	assert.Equal(t, "15-end", tp.GrantsQuest)
	assert.Equal(t, 40039, tp.RequiresItem)
	assert.Empty(t, res.Report.Clashes)
	assert.Empty(t, res.Report.HeldTriggers)
	r := res.Report
	assert.True(t, anyLine(r.Triggers, "delete quest 15-freight.yaml trigger 0"), "%v", r.Triggers)
	assert.True(t, anyLine(r.Triggers, "delete quest 15-freight.yaml trigger 1", "R15"), "%v", r.Triggers)
	assert.True(t, anyLine(r.Triggers, "delete quest 15-freight.yaml trigger 2", "R15", "quest_granted 15-start", "live bug"), "%v", r.Triggers)
	assert.True(t, anyLine(r.RulingGrants, "15-freight.yaml trigger 1", "15-start", "R15"), "%v", r.RulingGrants)
	assert.True(t, anyLine(r.Rulings, "15-freight.yaml trigger 1", "R15", "My freight!"), "the dropped line is named: %v", r.Rulings)
	assert.Equal(t, 2, r.GrantsIn[srcItemGive])
	assert.Equal(t, 1, r.GrantsOut[srcItemGive])
	assert.Equal(t, 1, r.GrantsByRuling[srcItemGive])
	assert.True(t, r.Reconciled())
	assert.Contains(t, r.Markdown(), "| item_give | 2 | 1 | 1 |")
}

func TestConvert_ChainedSecondGrantBecomesTheEnd(t *testing.T) {
	// Quest 66 (settled): the give route granted the escort step and a second
	// token whose chain ends the quest. The hand-in becomes one -end topic with
	// the chain's flag; the skipped step's grants are named as by ruling.
	in := inlineInput(t, map[int]string{9323: `
mobid: 9323
zone: Inline Zone
defaultMood: neutral
patterns: []
tree:
  root: {text: "Sit down."}
  nodes:
    - id: wrapperhandoff
      triggers: ["wrapper"]
      questRequired: ["66-start"]
      questExcluded: ["66-escort", "66-end"]
      requiresItem: 40110
      grantsQuest: "66-escort"
      text: "Copper-flower note."
`}, map[string]string{"66-plight.yaml": `
questid: 66
name: The Addict's Plight
steps: [{id: start}, {id: escort}, {id: detox}, {id: end}]
triggers:
  - event: item_give
    mob: 9323
    item: 40110
    conditions: {has: ["66-start"], missing: ["66-escort"]}
    actions:
      - grant: "66-escort"
      - npc_say:
          mob: 9323
          lines:
            - {delay: 1, text: "This wrapper. I know this smell."}
      - grant: "66-detox"
  - event: quest_granted
    quest_token: "66-detox"
    conditions: {missing: ["66-end"]}
    actions:
      - set_flag: {key: "66-addict-fate", value: "saved"}
      - grant: "66-end"
`})
	res := Convert(in)
	tp := mustTopic(t, res, 9323, "wrapperhandoff")
	assert.Equal(t, dialogue.KindHandIn, tp.Kind)
	assert.Equal(t, 66, tp.Quest)
	assert.False(t, tp.Marked)
	assert.Equal(t, "The Addict's Plight", tp.Label)
	assert.Equal(t, "66-end", tp.GrantsQuest)
	require.NotNil(t, tp.SetsQuestFlag)
	assert.Equal(t, dialogue.QuestFlagSet{Key: "66-addict-fate", Value: "saved"}, *tp.SetsQuestFlag)
	assert.Equal(t, []string{"66-escort", "66-end"}, tp.QuestExcluded)

	r := res.Report
	assert.Empty(t, r.Unmappable)
	assert.Empty(t, r.Clashes)
	assert.Empty(t, r.HeldTriggers)
	assert.True(t, anyLine(r.Triggers, "delete quest 66-plight.yaml trigger 0"), "%v", r.Triggers)
	assert.True(t, anyLine(r.Triggers, "delete quest 66-plight.yaml trigger 1", "quest_granted 66-detox"), "%v", r.Triggers)
	assert.True(t, anyLine(r.RulingGrants, "mob 9323 topic wrapperhandoff", "66-escort", "66-end"), "%v", r.RulingGrants)
	assert.True(t, anyLine(r.RulingGrants, "66-plight.yaml trigger 0", "66-escort"), "%v", r.RulingGrants)
	assert.Equal(t, map[string]int{srcNodeMid: 1, srcItemGive: 2}, nonZero(r.GrantsIn))
	assert.Equal(t, map[string]int{srcItemGive: 1}, nonZero(r.GrantsOut))
	assert.Equal(t, map[string]int{srcNodeMid: 1, srcItemGive: 1}, nonZero(r.GrantsByRuling))
	assert.True(t, r.Reconciled())
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

	assert.True(t, anyLine(res.Report.OverlapKept, "mob 7009", "pattern 1", "live fallback, breadcrumb candidate", "find_her", "topic missing"),
		"%v", res.Report.OverlapKept)
	assert.Equal(t, "Ask the captain.", mustTopic(t, res, 7009, "missing").Text, "its text is not the node's, so it is kept")
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

// keywordlessAsk is an ask branch with no keyword_match: it ran on every ask.
const keywordlessAsk = `
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
`

func TestConvert_BehaviorBranchWithoutKeywordsIsFlagged(t *testing.T) {
	// A branch that ran on every ask becomes a topic that runs only when
	// chosen, so it is flagged.
	in := NewInput()
	in.MobZones[9602] = "Fixture Zone"
	require.NoError(t, in.AddBehavior("behaviors/fixture_zone/9602-a_guard.yaml", 9602, []byte(keywordlessAsk)))
	res := Convert(in)
	tp := mustTopic(t, res, 9602, "ask_1")
	assert.True(t, tp.Behavior)
	assert.True(t, anyLine(res.Report.Clashes, "9602-a_guard.yaml", "branch 1", "no keyword_match", "every ask"), "%v", res.Report.Clashes)
	assert.True(t, anyLine(res.Report.Behavior, "9602-a_guard.yaml", "ask_1", "random_chance", "set_state"), "%v", res.Report.Behavior)
}

func TestConvert_BanditLeaderAskIsDropped(t *testing.T) {
	// R19: the bandit leader's talk option is dropped; nothing in
	// conversation stops his ambush.
	in := NewInput()
	in.MobZones[254] = "Marches Spur Road"
	require.NoError(t, in.AddBehavior("behaviors/marches_spur_road/254-bandit_leader.yaml", 254, []byte(keywordlessAsk)))
	res := Convert(in)
	assert.Empty(t, res.Keys(254), "no topic")
	assert.Nil(t, res.Files[254], "no new file for a mob the ruling leaves without topics")
	assert.Empty(t, res.Report.Clashes)
	assert.True(t, anyLine(res.Report.Behavior, "254-bandit_leader.yaml", "branch 1", "R19", "delete the branch"), "%v", res.Report.Behavior)
	assert.True(t, anyLine(res.Report.Rulings, "254-bandit_leader.yaml", "R19"), "%v", res.Report.Rulings)
}

func TestConvert_HadwenRiteIsOneBeginTopic(t *testing.T) {
	// R17: Hadwen's two begin branches (mid-rite answer and the rite) answer
	// one behavior topic gated on 30-start and not 30-end. The topic carries
	// no grant: the tree keeps granting 30-end at the rite's close and stops
	// granting 29-start.
	in := NewInput()
	in.MobZones[9100] = "Pothole Coulee"
	require.NoError(t, in.AddBehavior("behaviors/pothole_coulee/9100-cleric_hadwen.yaml", 9100, []byte(`
tree:
  type: selector
  children:
    - type: sequence
      event: player_ask
      children:
        - type: condition
          check: keyword_match
          keywords: [begin, ready]
        - type: condition
          check: state_equals
          key: rite_running
          value: "1"
        - type: action
          do: say
          text: Hush, now.
    - type: sequence
      event: player_ask
      children:
        - type: condition
          check: keyword_match
          keywords: [begin, ready]
        - type: condition
          check: player_has_quest
          quest: "30-start"
        - type: condition
          check: player_missing_quest
          quest: "30-end"
        - type: condition
          check: state_equals
          key: rite_running
          value: ""
        - type: action
          do: set_state
          key: rite_running
          value: "1"
        - type: action
          do: say
          text: Then we begin.
        - type: action
          do: grant_quest
          quest: "30-end"
        - type: action
          do: grant_quest
          quest: "29-start"
`)))
	res := Convert(in)
	assert.Equal(t, []string{"begin"}, res.Keys(9100))
	tp := mustTopic(t, res, 9100, "begin")
	assert.True(t, tp.Behavior)
	assert.Equal(t, dialogue.KindInfo, tp.Kind)
	assert.Empty(t, tp.GrantsQuest, "the tree grants 30-end, not the topic")
	assert.Equal(t, []string{"30-start"}, tp.QuestRequired)
	assert.Equal(t, []string{"30-end"}, tp.QuestExcluded)
	assert.Equal(t, "Then we begin.", tp.Text)

	r := res.Report
	assert.Empty(t, r.Clashes, "no owner decision left")
	assert.True(t, anyLine(r.Behavior, "9100-cleric_hadwen.yaml", "branch 1", "topic begin", "R17", "state_equals"), "%v", r.Behavior)
	assert.True(t, anyLine(r.Behavior, "9100-cleric_hadwen.yaml", "branch 2", "topic begin", "R17", "remove grant_quest 29-start", "keep grant_quest 30-end"), "%v", r.Behavior)
	assert.True(t, anyLine(r.Rulings, "9100-cleric_hadwen.yaml", "R17"), "%v", r.Rulings)
	assert.True(t, anyLine(r.RulingGrants, "9100-cleric_hadwen.yaml", "29-start", "dropped", "R17"), "%v", r.RulingGrants)
	assert.True(t, anyLine(r.RulingGrants, "9100-cleric_hadwen.yaml", "30-end", "stays in the tree", "R17"), "%v", r.RulingGrants)
	assert.Equal(t, 2, r.GrantsIn[srcBehavior])
	assert.Equal(t, 0, r.GrantsOut[srcBehavior])
	assert.Equal(t, 2, r.GrantsByRuling[srcBehavior])
	assert.True(t, r.Reconciled())
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
	assert.Contains(t, md, "| source | input | output | by ruling |")
	assert.Contains(t, md, "| total | 11 | 11 | 0 |")
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

// overlapInput is one tree file with an overlap pattern of each outcome.
func overlapInput(t *testing.T) *Input {
	t.Helper()
	return inlineInput(t, map[int]string{8101: `
mobid: 8101
zone: Inline Zone
defaultMood: neutral
patterns:
  - keywords: ["lake", "husband"]
    questFlagRequired: {"9-path": "lake"}
    questExcluded: ["9-end"]
    moodChange: sad
    responses:
      - "He went into the lake twelve years ago. He never came out."
  - keywords: ["water"]
    responses:
      - "The <ansi fg=\"cyan\">lake</ansi> is COLD. Nobody swims!"
  - keywords: ["Water", "lake"]
    responses:
      - "The lake is cold. Nobody swims."
      - "Fish there are big as dogs."
  - keywords: ["ferry"]
    responses:
      - "The ferry runs at dawn."
tree:
  root:
    text: "Mind the reeds."
  nodes:
    - id: lake
      triggers: ["lake", "water"]
      text: "The lake is cold. Nobody swims."
    - id: ferry_talk
      triggers: ["ferry"]
      questRequired: ["9-start"]
      text: "Take the ferry north."
`}, nil)
}

func TestConvert_OverlapWithUniqueTextBecomesATopic(t *testing.T) {
	res := Convert(overlapInput(t))

	tp := mustTopic(t, res, 8101, "husband")
	assert.Equal(t, dialogue.KindInfo, tp.Kind)
	assert.Equal(t, "", tp.Label, "the zone pass writes the label")
	assert.Equal(t, "He went into the lake twelve years ago. He never came out.", tp.Text)
	assert.Equal(t, "sad", tp.MoodChange)

	fallback := mustTopic(t, res, 8101, "ferry")
	assert.Equal(t, "The ferry runs at dawn.", fallback.Text, "the answer outside the gated node's gates")
	assert.Empty(t, fallback.QuestRequired)

	assert.Equal(t, []string{"lake", "ferry_talk", "husband", "ferry"}, res.Keys(8101))
	assert.Equal(t, map[string]int{"topic": 2, "merged": 1, "covered": 1}, res.Report.OverlapOutcomes)
}

func TestConvert_OverlapGatesCarryOver(t *testing.T) {
	res := Convert(overlapInput(t))

	tp := mustTopic(t, res, 8101, "husband")
	assert.Equal(t, map[string]string{"9-path": "lake"}, tp.QuestFlagRequired)
	assert.Equal(t, []string{"9-end"}, tp.QuestExcluded)
}

func TestConvert_OverlapWhoseTextTheNodeHasStaysDropped(t *testing.T) {
	res := Convert(overlapInput(t))

	assert.Nil(t, res.Topic(8101, "water"), "pattern 2 says what node lake says")
	assert.True(t, anyLine(res.Report.Dropped, "mob 8101", "pattern 2", "overlap", "text already in", "lake", "dropped"),
		"%v", res.Report.Dropped)
	assert.False(t, anyLine(res.Report.Dropped, "mob 8101", "pattern 1"), "%v", res.Report.Dropped)
}

func TestConvert_OverlapWithTheSameKeywordsMergesIntoTheNode(t *testing.T) {
	res := Convert(overlapInput(t))

	lake := mustTopic(t, res, 8101, "lake")
	assert.Equal(t, "The lake is cold. Nobody swims."+joinSep+"Fish there are big as dogs.", lake.Text,
		"only the response the node lacks is added")
}

func TestConvert_OverlapKeptIsReported(t *testing.T) {
	res := Convert(overlapInput(t))
	r := res.Report

	assert.True(t, anyLine(r.OverlapKept, "mob 8101 pattern 1", "[lake husband]", "topic husband"), "%v", r.OverlapKept)
	assert.True(t, anyLine(r.OverlapKept, "mob 8101 pattern 3", "[Water lake]", "merged into topic lake"), "%v", r.OverlapKept)
	assert.True(t, anyLine(r.OverlapKept, "mob 8101 pattern 4", "[ferry]", "topic ferry", "live fallback"), "%v", r.OverlapKept)
	assert.False(t, anyLine(r.OverlapKept, "pattern 2"), "%v", r.OverlapKept)
	md := r.Markdown()
	assert.Contains(t, md, "## Overlap patterns kept\n\n- mob 8101 pattern 1")
	assert.Contains(t, md, "- overlap kept as a topic: 2\n- overlap merged into a node topic: 1\n- overlap dropped, text already in the node: 1\n")
	assert.True(t, r.Reconciled())
}

func TestConvert_OverlapParaphraseIsFlagged(t *testing.T) {
	res := Convert(inlineInput(t, map[int]string{8102: `
mobid: 8102
zone: Inline Zone
defaultMood: neutral
patterns:
  - keywords: ["mill"]
    responses:
      - "The old mill burned down last winter, wheel and all."
  - keywords: ["mill", "grain"]
    responses:
      - "Grain comes up from Greenford on the barge."
tree:
  root:
    text: "Busy day."
  nodes:
    - id: mill
      triggers: ["mill"]
      questRequired: ["9-start"]
      text: "The mill burned down last winter. Wheel and all, gone."
`}, nil))
	r := res.Report
	assert.Equal(t, map[string]int{"topic": 2}, nonZero(r.OverlapOutcomes))
	assert.True(t, anyLine(r.OverlapKept, "mob 8102 pattern 1", "zone pass: check for a paraphrase"), "%v", r.OverlapKept)
	assert.False(t, anyLine(r.OverlapKept, "mob 8102 pattern 2", "paraphrase"), "%v", r.OverlapKept)
	assert.Equal(t, "Grain comes up from Greenford on the barge.", mustTopic(t, res, 8102, "grain").Text)
}

func TestNormText(t *testing.T) {
	assert.Equal(t, "the lake is cold nobody swims", normText("The <ansi fg=\"cyan\">lake</ansi> is COLD.  Nobody swims!"))
	assert.Equal(t, "dont go", normText("Don't go"))
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

package dialogue

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// testWorld is a World over two quests: 7 (The Wolf Hunt, steps start,
// pelts, end) and 9 (The Lost Purse, steps start, end), flag 7-mood=calm,
// items 77 and 900.
func testWorld(files ...*DialogueFile) World {
	quests := map[int]QuestInfo{
		7: {Title: "The Wolf Hunt", StartToken: "7-start", EndToken: "7-end"},
		9: {Title: "The Lost Purse", StartToken: "9-start", EndToken: "9-end"},
	}
	tokens := map[string]int{"7-start": 7, "7-pelts": 7, "7-end": 7, "9-start": 9, "9-found": 9, "9-end": 9}
	return World{
		Files:        files,
		QuestById:    func(id int) (QuestInfo, bool) { q, ok := quests[id]; return q, ok },
		QuestOfToken: func(tok string) (QuestInfo, bool) { id, ok := tokens[tok]; return quests[id], ok },
		FlagDeclared: func(k, v string) bool { return k == "7-mood" && v == "calm" },
		ItemExists:   func(id int) bool { return id == 77 || id == 900 },
	}
}

func problems(t *testing.T, src string) []string {
	t.Helper()
	ps := ValidateWorld(testWorld(loadFixture(t, src)))
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.String())
	}
	return out
}

func requireProblem(t *testing.T, src, want string) {
	t.Helper()
	got := problems(t, src)
	for _, g := range got {
		if strings.Contains(g, want) {
			return
		}
	}
	t.Fatalf("expected a problem containing %q, got:\n%s", want, strings.Join(got, "\n"))
}

func TestValidateWorld_CleanFixturePasses(t *testing.T) {
	require.Empty(t, problems(t, menuFixtureYAML))
}

// Semicolons truncate spoken text, but hints reach the player through
// SendText, never a mob's say, so a semicolon in a hint is not a problem.
// Ported from the old ValidateDialogueFile test (#375 Task 21).
func TestValidateWorld_HintsAreExemptFromTheSemicolonRule(t *testing.T) {
	src := strings.Replace(menuFixtureYAML, "opener:\n  text: \"Hello.\"\n", "opener:\n  text: \"Hello.\"\n  hints: \"Ask; or listen.\"\n", 1)
	src = strings.Replace(src, "    text: \"Wet.\"\n", "    text: \"Wet.\"\n    hints: \"Rain; mostly.\"\n", 1)
	src = strings.Replace(src, "      text: \"Back again?\"\n", "      text: \"Back again?\"\n      hints: \"Again; yes.\"\n", 1)
	require.Equal(t, 3, strings.Count(src, "hints:"), "the fixture edits did not apply")
	require.Empty(t, problems(t, src))
}

func breakFixture(old, nu string) string { return strings.Replace(menuFixtureYAML, old, nu, 1) }

// emDash is built from its code point so no dash character sits in source.
var emDash = string(rune(0x2014))

func TestValidateWorld_Rules(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"bad key", breakFixture("key: weather", "key: Weather Talk"), `key must match`},
		{"duplicate key", breakFixture("key: rumor", "key: weather"), `duplicate key`},
		{"bad kind", breakFixture("kind: info\n    label: \"How is", "kind: chat\n    label: \"How is"), `kind must be info, offer or handin`},
		{"offer without playerLine", breakFixture(`    playerLine: "Do you want the wolves dealt with?"`+"\n", ""), `offer topics need quest and playerLine`},
		{"marked offer", breakFixture("    breadcrumb: \"Come back", "    marked: true\n    breadcrumb: \"Come back"), `marked belongs on info topics only`},
		{"breadcrumb on info", breakFixture("    text: \"Wet.\"", "    text: \"Wet.\"\n    breadcrumb: \"Later.\""), `breadcrumb belongs on offer topics only`},
		{"offer grants wrong token", breakFixture("grantsQuest: 7-start", "grantsQuest: 7-pelts"), `offer must grant "7-start"`},
		{"handin grants wrong token", breakFixture("grantsQuest: 7-end", "grantsQuest: 7-pelts"), `handin must grant "7-end"`},
		{"missing quest", breakFixture("quest: 7\n    label: \"The Wolf Hunt\"\n    playerLine: \"Do", "quest: 70\n    label: \"The Wolf Hunt\"\n    playerLine: \"Do"), `quest 70 does not exist`},
		{"label not title", breakFixture(`label: "The Wolf Hunt"`, `label: "Wolves"`), `label must be the quest title`},
		{"long label", breakFixture(`label: "How is the weather?"`, `label: "`+strings.Repeat("a", 51)+`"`), `50`},
		{"dash in label", breakFixture(`label: "How is the weather?"`, "label: \"How is it "+emDash+" wet?\""), `dash`},
		{"dash in breadcrumb", breakFixture(`breadcrumb: "Come back when you can hold a spear."`, "breadcrumb: \"Come back "+emDash+" later.\""), `dash`},
		{"duplicate info label", breakFixture(`label: "Heard any rumors?"`, `label: "How is the weather?"`), `label repeats topic`},
		{"heardRequired not info", breakFixture("heardRequired: [rumor]", "heardRequired: [hunt]"), `heardRequired "hunt" is not an info topic`},
		{"heardRequired cycle", breakFixture("    text: \"Wolves in the hills.\"", "    text: \"Wolves in the hills.\"\n    heardRequired: [wolves]"), `heardRequired cycle`},
		{"charge without gold gate", breakFixture("    goldRequired: 5\n", "    chargesGold: 5\n"), `chargesGold 5 needs goldRequired of at least 5`},
		{"info topic with quest", breakFixture("    text: \"Wet.\"", "    text: \"Wet.\"\n    quest: 7"), `quest and playerLine belong on offer and handin topics only`},
		{"info topic with playerLine", breakFixture("    text: \"Wet.\"", "    text: \"Wet.\"\n    playerLine: \"Rain?\""), `quest and playerLine belong on offer and handin topics only`},
		{"breadcrumb on handin", breakFixture("    text: \"Good work.\"", "    text: \"Good work.\"\n    breadcrumb: \"Later.\""), `topic "hunt_done": breadcrumb belongs on offer topics only`},
		{"long playerLine", breakFixture(`playerLine: "Do you want the wolves dealt with?"`, `playerLine: "`+strings.Repeat("a", 71)+`"`), `detail over 70`},
		{"dash in playerLine", breakFixture(`playerLine: "Do you want the wolves dealt with?"`, "playerLine: \"Do you "+emDash+" want it?\""), `em or en dash`},
		{"semicolon in greeting", breakFixture("opener:\n", "greetings:\n  - text: \"Hi; there.\"\nopener:\n"), `greeting 1: semicolons`},
		{"semicolon in opener", breakFixture(`text: "Hello."`, `text: "Hello; there."`), `opener: semicolons`},
		{"semicolon in opener variant", breakFixture(`text: "Back again?"`, `text: "Back; again?"`), `opener variant 1: semicolons`},
		{"semicolon in breadcrumb", breakFixture(`breadcrumb: "Come back when you can hold a spear."`, `breadcrumb: "Come back; later."`), `breadcrumb: semicolons`},
		{"semicolon in label", breakFixture(`label: "How is the weather?"`, `label: "How is; the weather?"`), `topic "weather" label: semicolons`},
		{"semicolon in playerLine", breakFixture(`playerLine: "I have the pelts."`, `playerLine: "I have; the pelts."`), `topic "hunt_done" playerLine: semicolons`},
		{"bad token in opener variant", breakFixture("- questRequired: [7-start]", "- questRequired: [7-nope]"), `opener variant 1: quest token "7-nope" does not exist`},
		{"bad token in grantsQuest", breakFixture("grantsQuest: 7-start", "grantsQuest: 7-nope"), `quest token "7-nope" does not exist`},
		{"bad token in questExcluded", breakFixture("questExcluded: [7-end]", "questExcluded: [7-nope, 7-end]"), `quest token "7-nope" does not exist`},
		{"unknown itemExcluded", breakFixture("    goldRequired: 5\n", "    goldRequired: 5\n    itemExcluded: 4243\n"), `item 4243 does not exist`},
		{"unknown carriesItem", breakFixture("    goldRequired: 5\n", "    goldRequired: 5\n    carriesItem: 4244\n"), `item 4244 does not exist`},
		{"unknown givesItem", breakFixture("    goldRequired: 5\n", "    goldRequired: 5\n    givesItem: 4244\n"), `item 4244 does not exist`},
		{"undeclared questFlagExcluded", breakFixture("    text: \"Wet.\"", "    text: \"Wet.\"\n    questFlagExcluded: {7-mood: furious}"), `quest flag "7-mood"="furious" is not declared`},
		{"undeclared setsQuestFlag", breakFixture("    text: \"Wet.\"", "    text: \"Wet.\"\n    setsQuestFlag: {key: 7-mood, value: gone}"), `setsQuestFlag "7-mood"="gone" is not declared`},
		{"offer without quest", breakFixture("    quest: 7\n    label: \"The Wolf Hunt\"\n    playerLine: \"Do", "    label: \"The Wolf Hunt\"\n    playerLine: \"Do"), `offer topics need quest and playerLine`},
		{"duplicate info label by case and space", breakFixture(`label: "Heard any rumors?"`, `label: "HOW IS THE WEATHER? "`), `label repeats topic "weather"`},
		{"info label equals quest title", breakFixture(`label: "Heard any rumors?"`, `label: "the wolf hunt"`), `label repeats the quest title of topic "hunt"`},
		{"semicolon", breakFixture(`text: "Wet."`, `text: "Wet; very."`), `topic "weather" text: semicolons`},
		{"grant not excluded", breakFixture("questExcluded: [7-start, 7-end]", "questExcluded: [7-end]"), `must also appear in questExcluded`},
		{"end token not excluded", breakFixture("questExcluded: [7-start, 7-end]", "questExcluded: [7-start]"), `must include the end token "7-end"`},
		{"unknown token", breakFixture("questRequired: [7-pelts]", "questRequired: [7-nope]"), `quest token "7-nope" does not exist`},
		{"unknown item", breakFixture("    goldRequired: 5\n", "    goldRequired: 5\n    requiresItem: 4242\n"), `item 4242 does not exist`},
		{"undeclared flag", breakFixture("    text: \"Wet.\"", "    text: \"Wet.\"\n    questFlagRequired: {7-mood: angry}"), `quest flag "7-mood"="angry" is not declared`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.NotEqual(t, menuFixtureYAML, c.src, "the fixture edit did not apply")
			requireProblem(t, c.src, c.want)
		})
	}
}

func TestValidateWorld_BehaviorTopicsMatchTheTree(t *testing.T) {
	df := loadFixture(t, breakFixture("    text: \"Wet.\"", "    text: \"Wet.\"\n    behavior: true"))
	w := testWorld(df)
	w.BehaviorTopics = map[int][]string{50: {"ferry"}}
	var got []string
	for _, p := range ValidateWorld(w) {
		got = append(got, p.String())
	}
	joined := strings.Join(got, "\n")
	require.Contains(t, joined, `behavior: true but the mob's tree has no topic_is "weather"`)
	require.Contains(t, joined, `topic_is "ferry" names no topic on this mob`)
}

// An archetype's topic_is serves a behavior topic (the runtime runs the
// archetype when the per-mob tree fails), and an archetype key with no topic
// on this mob is no problem: the archetype is shared (#375 review M5).
func TestValidateWorld_ArchetypeTopicsServeABehaviorTopic(t *testing.T) {
	df := loadFixture(t, breakFixture("    text: \"Wet.\"", "    text: \"Wet.\"\n    behavior: true"))
	w := testWorld(df)
	w.ArchetypeTopics = map[int][]string{50: {"weather", "some_other_mobs_topic"}}
	require.Empty(t, ValidateWorld(w))

	w.ArchetypeTopics = map[int][]string{50: {"some_other_mobs_topic"}}
	ps := ValidateWorld(w)
	require.Len(t, ps, 1)
	require.Contains(t, ps[0].String(), `behavior: true but the mob's tree has no topic_is "weather"`)
}

func TestValidateWorld_ItemGiveGrantsAreRefused(t *testing.T) {
	w := testWorld(loadFixture(t, menuFixtureYAML))
	w.ItemGiveGrants = []string{"quest 10 trigger 2"}
	ps := ValidateWorld(w)
	require.Len(t, ps, 1)
	require.Contains(t, ps[0].String(), "item_give triggers may not grant")
	require.Contains(t, ps[0].String(), fmt.Sprint("quest 10 trigger 2"))
}

func TestValidateWorld_BehaviorTopicOnMobWithoutAFile(t *testing.T) {
	w := testWorld(loadFixture(t, menuFixtureYAML))
	w.BehaviorTopics = map[int][]string{50: {}, 99: {"x"}, 12: {"y"}}
	var got []string
	for _, p := range ValidateWorld(w) {
		got = append(got, p.String())
	}
	require.Equal(t, []string{
		"mob 12 behavior tree: topic_is on mob 12, which has no dialogue file",
		"mob 99 behavior tree: topic_is on mob 99, which has no dialogue file",
	}, got)
}

func TestValidateWorld_ItemGiveGrantsAreSorted(t *testing.T) {
	w := testWorld()
	w.ItemGiveGrants = []string{"quest 9 trigger 0", "quest 10 trigger 2"}
	ps := ValidateWorld(w)
	require.Len(t, ps, 2)
	require.Contains(t, ps[0].String(), "quest 10 trigger 2")
	require.Contains(t, ps[1].String(), "quest 9 trigger 0")
}

func TestDecodeFile(t *testing.T) {
	df, err := decodeFile("dir/50.yaml", []byte(menuFixtureYAML))
	require.NoError(t, err)
	require.Equal(t, 50, df.MobId)

	variantGrant := strings.Replace(menuFixtureYAML, "- questRequired: [7-start]", "- grantsQuest: 7-start\n      questRequired: [7-start]", 1)
	require.NotEqual(t, menuFixtureYAML, variantGrant)
	_, err = decodeFile("dir/50.yaml", []byte(variantGrant))
	require.ErrorContains(t, err, "grantsQuest")

	stray := strings.Replace(menuFixtureYAML, "topics:\n", "bogus: 1\ntopics:\n", 1)
	require.NotEqual(t, menuFixtureYAML, stray)
	_, err = decodeFile("dir/50.yaml", []byte(stray))
	require.ErrorContains(t, err, "bogus")

	multi := strings.Replace(stray, "topics:\n", "bogus2: 1\ntopics:\n", 1)
	_, err = decodeFile("dir/50.yaml", []byte(multi))
	require.Error(t, err)
	require.NotContains(t, err.Error(), "\n")
	require.NotContains(t, err.Error(), ";")
	require.Contains(t, err.Error(), " | ")

	_, err = decodeFile("dir/51.yaml", []byte(menuFixtureYAML))
	require.ErrorContains(t, err, "does not match the file name")

	_, err = decodeFile("dir/rat.yaml", []byte(menuFixtureYAML))
	require.ErrorContains(t, err, "not a mob id")
}

func TestTokenStepValid(t *testing.T) {
	q := &quests.Quest{QuestId: 7, Steps: []quests.QuestStep{{Id: "start"}, {Id: "pelts"}, {Id: "end"}}}
	require.True(t, tokenStepValid(q, "pelts"))
	require.True(t, tokenStepValid(q, "end"))
	require.False(t, tokenStepValid(q, "all+"))
	require.False(t, tokenStepValid(q, ""))
	require.False(t, tokenStepValid(q, "nope"))
	require.False(t, tokenStepValid(nil, "start"))
}

func TestGrantsInActions(t *testing.T) {
	require.False(t, grantsInActions(nil))
	require.False(t, grantsInActions([]quests.ActionDef{{GiveGold: 5}}))
	require.True(t, grantsInActions([]quests.ActionDef{{Grant: "7-start"}}))
	nested := []quests.ActionDef{{Sequence: &quests.SequenceDef{OnComplete: []quests.ActionDef{{Grant: "7-end"}}}}}
	require.True(t, grantsInActions(nested))
}

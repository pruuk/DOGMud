package dialogue

import (
	"fmt"
	"strings"
	"testing"

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
		{"semicolon", breakFixture(`text: "Wet."`, `text: "Wet; very."`), `semicolons`},
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
	w.BehaviorTopics = func(mobId int) []string { return []string{"ferry"} }
	var got []string
	for _, p := range ValidateWorld(w) {
		got = append(got, p.String())
	}
	joined := strings.Join(got, "\n")
	require.Contains(t, joined, `behavior: true but the mob's tree has no topic_is "weather"`)
	require.Contains(t, joined, `topic_is "ferry" names no topic on this mob`)
}

func TestValidateWorld_ItemGiveGrantsAreRefused(t *testing.T) {
	w := testWorld(loadFixture(t, menuFixtureYAML))
	w.ItemGiveGrants = []string{"quest 10 trigger 2"}
	ps := ValidateWorld(w)
	require.Len(t, ps, 1)
	require.Contains(t, ps[0].String(), "item_give triggers may not grant")
	require.Contains(t, ps[0].String(), fmt.Sprint("quest 10 trigger 2"))
}

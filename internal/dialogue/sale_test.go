package dialogue

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/stretchr/testify/require"
)

func saleTopic() *Topic {
	return &Topic{
		Key: "teach_x", Kind: KindSale, Label: "Secret of X", PlayerLine: "Teach me?", Text: "pitch",
		NotReadyText: "Come back.", PaidText: "Paid.",
		Effects: Effects{ChargesGold: 100, TeachesRecipes: []string{"a", "b"}},
	}
}

func saleState(known map[string]bool, skilled map[string]bool) *PlayerState {
	return &PlayerState{
		HasQuest:       func(string) bool { return false },
		KnowsRecipe:    func(id string) bool { return known[id] },
		RecipeSkillMet: func(id string) bool { return skilled[id] },
	}
}

func saleFile(t *Topic) *DialogueFile { return &DialogueFile{MobId: 1, Topics: []Topic{*t}} }

func TestSale_HiddenOnlyWhenEveryRecipeIsKnown(t *testing.T) {
	tp := saleTopic()
	df := saleFile(tp)
	require.True(t, df.Eligible(tp, saleState(nil, nil)), "no skill: still listed")
	require.True(t, df.Eligible(tp, saleState(map[string]bool{"a": true}, nil)), "knows part of it")
	require.False(t, df.Eligible(tp, saleState(map[string]bool{"a": true, "b": true}, nil)))
}

func TestSaleReady_NeedsEverySkill(t *testing.T) {
	tp := saleTopic()
	require.False(t, SaleReady(tp, saleState(nil, nil)))
	require.False(t, SaleReady(tp, saleState(nil, map[string]bool{"a": true})), "one of two")
	require.True(t, SaleReady(tp, saleState(nil, map[string]bool{"a": true, "b": true})))
	require.False(t, SaleReady(tp, &PlayerState{}), "a nil callback fails closed")
}

func TestSale_MarkerOnlyWhenReady(t *testing.T) {
	tp := saleTopic()
	df := saleFile(tp)
	require.Empty(t, df.TopicMarker(tp, saleState(nil, nil)))
	require.Equal(t, menu.MarkerInfo, df.TopicMarker(tp, saleState(nil, map[string]bool{"a": true, "b": true})))
}

// A sale line reads as its label with the player's line under it, like an offer,
// and is never dimmed: it stays on the menu until every recipe is known.
func TestSale_MenuLineIsNotDimmed(t *testing.T) {
	tp := saleTopic()
	df := saleFile(tp)
	m := df.Menu("Veyra", saleState(nil, map[string]bool{"a": true, "b": true}))
	require.Len(t, m.Entries, 1)
	e := m.Entries[0]
	require.Equal(t, "Secret of X", e.Label)
	require.Equal(t, "Teach me?", e.Detail)
	require.False(t, e.Dim)
	require.Equal(t, menu.MarkerInfo, e.Marker)
}

func TestSale_ApplyChargesThenTeaches(t *testing.T) {
	tp := saleTopic()
	df := saleFile(tp)
	var learned []string
	paid := 0
	ps := saleState(nil, map[string]bool{"a": true, "b": true})
	ps.LearnRecipe = func(id string) { learned = append(learned, id) }
	ps.ChargeGold = func(n int) bool { paid = n; return true }
	require.True(t, df.Apply(tp, ps))
	require.Equal(t, 100, paid)
	require.Equal(t, []string{"a", "b"}, learned)
}

func TestSale_RefusedChargeTeachesNothing(t *testing.T) {
	tp := saleTopic()
	df := saleFile(tp)
	var learned []string
	ps := saleState(nil, map[string]bool{"a": true, "b": true})
	ps.LearnRecipe = func(id string) { learned = append(learned, id) }
	ps.ChargeGold = func(int) bool { return false }
	require.False(t, df.Apply(tp, ps))
	require.Empty(t, learned)
}

const saleFixtureYAML = `mobid: 50
zone: testzone
topics:
  - key: teach_x
    kind: sale
    label: "Secret of X"
    playerLine: "Will you teach me X?"
    text: "X is not for everyone."
    notReadyText: "Come back when you have practiced."
    paidText: "Paid. It is yours."
    chargesGold: 100
    teachesRecipes: [real-a, real-b]
`

func saleWorld(t *testing.T, src string) World {
	w := testWorld(loadFixture(t, src))
	w.RecipeExists = func(id string) bool { return id == "real-a" || id == "real-b" }
	return w
}

func saleProblems(t *testing.T, src string) string {
	t.Helper()
	out := ""
	for _, p := range ValidateWorld(saleWorld(t, src)) {
		out += p.String() + "\n"
	}
	return out
}

func TestValidateWorld_SaleFixtureIsClean(t *testing.T) {
	require.Empty(t, saleProblems(t, saleFixtureYAML))
}

func saleBreak(old, nu string) string {
	return strings.Replace(saleFixtureYAML, old, nu, 1)
}

func TestValidateWorld_SaleRules(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"no recipes", saleBreak("    teachesRecipes: [real-a, real-b]\n", ""), `sale topics need teachesRecipes`},
		{"unknown recipe", saleBreak("[real-a, real-b]", "[real-a, typo-b]"), `teachesRecipes "typo-b" is not a recipe`},
		{"no price", saleBreak("    chargesGold: 100\n", ""), `sale topics need chargesGold`},
		{"no notReadyText", saleBreak(`    notReadyText: "Come back when you have practiced."`+"\n", ""), `sale topics need playerLine, notReadyText and paidText`},
		{"no paidText", saleBreak(`    paidText: "Paid. It is yours."`+"\n", ""), `sale topics need playerLine, notReadyText and paidText`},
		{"no playerLine", saleBreak(`    playerLine: "Will you teach me X?"`+"\n", ""), `sale topics need playerLine, notReadyText and paidText`},
		{"grantsQuest", saleBreak("    chargesGold: 100\n", "    chargesGold: 100\n    grantsQuest: 7-start\n    questExcluded: [7-start, 7-end]\n"), `a sale is not a quest`},
		{"goldRequired", saleBreak("    chargesGold: 100\n", "    chargesGold: 100\n    goldRequired: 100\n"), `a sale takes no goldRequired`},
		{"quest", saleBreak("    chargesGold: 100\n", "    chargesGold: 100\n    quest: 7\n"), `a sale is not a quest`},
		{"dash in notReadyText", saleBreak("practiced.", "practiced "+emDash+" now."), `em or en dash`},
		{"semicolon in notReadyText", saleBreak("practiced.", "practiced; now."), `notReadyText: semicolons`},
		{"semicolon in paidText", saleBreak("It is yours.", "It is; yours."), `paidText: semicolons`},
		{"dash in paidText", saleBreak("It is yours.", "It "+emDash+" yours."), `em or en dash`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.NotEqual(t, saleFixtureYAML, c.src, "the fixture edit did not apply")
			require.Contains(t, saleProblems(t, c.src), c.want)
		})
	}
}

func TestValidateWorld_SaleFieldsBelongToSalesOnly(t *testing.T) {
	for _, extra := range []string{
		"    teachesRecipes: [real-a]\n",
		"    notReadyText: \"No.\"\n",
		"    paidText: \"Paid.\"\n",
	} {
		src := breakFixture("    text: \"Wet.\"\n", "    text: \"Wet.\"\n"+extra)
		require.NotEqual(t, menuFixtureYAML, src)
		requireProblem(t, src, "teachesRecipes, notReadyText and paidText belong on sale topics only")
	}
}

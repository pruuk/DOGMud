package dialogue

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// fakePlayer is a PlayerState over plain sets, for topic tests.
type fakePlayer struct {
	tokens map[string]bool
	items  map[int]bool
	heard  map[string]bool
	gold   int
}

func (f *fakePlayer) ps() *PlayerState {
	return &PlayerState{
		HasQuest:     func(t string) bool { return f.tokens[t] },
		HasItem:      func(id int) bool { return f.items[id] },
		HasHeard:     func(k string) bool { return f.heard[k] },
		MarkHeard:    func(k string) { f.heard[k] = true },
		HasGold:      func(n int) bool { return f.gold >= n },
		GetQuestFlag: func(string) string { return "" },
		GiveQuest:    func(t string) { f.tokens[t] = true },
		RemoveItem:   func(id int) bool { delete(f.items, id); return true },
		ChargeGold: func(n int) bool {
			if f.gold < n {
				return false
			}
			f.gold -= n
			return true
		},
		GiveGold: func(n int) { f.gold += n },
	}
}

func newFake() *fakePlayer {
	return &fakePlayer{tokens: map[string]bool{}, items: map[int]bool{}, heard: map[string]bool{}}
}

func loadFixture(t *testing.T, src string) *DialogueFile {
	t.Helper()
	var df DialogueFile
	require.NoError(t, yaml.UnmarshalStrict([]byte(src), &df))
	return &df
}

const menuFixtureYAML = `mobid: 50
zone: testzone
opener:
  text: "Hello."
  variants:
    - questRequired: [7-start]
      text: "Back again?"
topics:
  - key: weather
    kind: info
    label: "How is the weather?"
    text: "Wet."
  - key: rumor
    kind: info
    label: "Heard any rumors?"
    text: "Wolves in the hills."
    marked: true
  - key: wolves
    kind: info
    label: "Where are the wolves?"
    text: "North ridge."
    heardRequired: [rumor]
  - key: hunt
    kind: offer
    quest: 7
    label: "The Wolf Hunt"
    playerLine: "Do you want the wolves dealt with?"
    text: "Bring me three pelts."
    breadcrumb: "Come back when you can hold a spear."
    goldRequired: 5
    questExcluded: [7-start, 7-end]
    grantsQuest: 7-start
  - key: hunt_done
    kind: handin
    quest: 7
    label: "The Wolf Hunt"
    playerLine: "I have the pelts."
    text: "Good work."
    questRequired: [7-pelts]
    questExcluded: [7-end]
    grantsQuest: 7-end
`

func TestEligible_GatesAndHeardRequired(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	wolves := df.FindTopic("wolves")
	require.False(t, df.Eligible(wolves, f.ps()), "wolves needs rumor heard")
	f.heard["50.rumor"] = true
	require.True(t, df.Eligible(wolves, f.ps()))

	hunt := df.FindTopic("hunt")
	require.False(t, df.Eligible(hunt, f.ps()), "needs 5 gold")
	f.gold = 5
	require.True(t, df.Eligible(hunt, f.ps()))
	f.tokens["7-start"] = true
	require.False(t, df.Eligible(hunt, f.ps()), "questExcluded once started")
}

func TestEligible_NilHasHeardFailsHeardRequiredClosed(t *testing.T) {
	df := loadFixture(t, menuFixtureYAML)
	ps := newFake().ps()
	ps.HasHeard = nil
	require.False(t, df.Eligible(df.FindTopic("wolves"), ps))
}

func TestMarker_PriorityHandInOverOfferOverInfo(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	require.Equal(t, menu.MarkerInfo, df.Marker(f.ps()), "only the marked rumor is lit")
	f.gold = 5
	require.Equal(t, menu.MarkerQuest, df.Marker(f.ps()))
	f.tokens["7-start"], f.tokens["7-pelts"] = true, true
	require.Equal(t, menu.MarkerHandIn, df.Marker(f.ps()))
	f.tokens["7-end"] = true
	f.heard["50.rumor"] = true
	require.Equal(t, "", df.Marker(f.ps()), "everything heard or done")
}

func TestOpenerFor_FirstPassingVariantElseRoot(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	text, _ := df.OpenerFor(f.ps())
	require.Equal(t, "Hello.", text)
	f.tokens["7-start"] = true
	text, _ = df.OpenerFor(f.ps())
	require.Equal(t, "Back again?", text)
}

func TestBreadcrumbs_OnlyForHiddenOffersNotYetTaken(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	require.Equal(t, []string{"Come back when you can hold a spear."}, df.Breadcrumbs(f.ps()))
	f.gold = 5
	require.Empty(t, df.Breadcrumbs(f.ps()), "eligible offers have no breadcrumb")
	f.gold = 0
	f.tokens["7-start"] = true
	require.Empty(t, df.Breadcrumbs(f.ps()), "a taken quest never breadcrumbs")
}

func TestMenu_QuestsFirstThenInfoInAuthoredOrderHeardDim(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	f.gold = 5
	f.heard["50.weather"] = true
	m := df.Menu("Hunter Bram", f.ps())
	require.Equal(t, "Talking with Hunter Bram", m.Heading)
	require.Equal(t, "Leave", m.Leave)
	ids := []string{}
	for _, e := range m.Entries {
		ids = append(ids, e.Id)
	}
	require.Equal(t, []string{"hunt", "weather", "rumor"}, ids)
	require.Equal(t, menu.MarkerQuest, m.Entries[0].Marker)
	require.Equal(t, "Do you want the wolves dealt with?", m.Entries[0].Detail)
	require.True(t, m.Entries[1].Dim, "heard weather is dim")
	require.Equal(t, menu.MarkerInfo, m.Entries[2].Marker)
}

func TestItemTopic_EligibleFirstElseWanted(t *testing.T) {
	df := loadFixture(t, `mobid: 51
zone: testzone
topics:
  - key: early
    kind: info
    label: "Here is the purse."
    text: "Not yet."
    questRequired: [9-start]
    requiresItem: 77
    grantsQuest: 9-found
    questExcluded: [9-found, 9-end]
`)
	f := newFake()
	f.items[77] = true
	tp, wanted := df.ItemTopic(77, f.ps())
	require.Nil(t, tp)
	require.True(t, wanted, "a topic wants it, none eligible: hand it back")
	f.tokens["9-start"] = true
	tp, wanted = df.ItemTopic(77, f.ps())
	require.NotNil(t, tp)
	require.True(t, wanted)
	tp, wanted = df.ItemTopic(78, f.ps())
	require.Nil(t, tp)
	require.False(t, wanted)
}

func TestNilFileIsSafe(t *testing.T) {
	var df *DialogueFile
	require.Equal(t, "", df.Marker(newFake().ps()))
	require.Nil(t, df.FindTopic("x"))
	text, hints := df.OpenerFor(nil)
	require.Equal(t, "", text+hints)
}

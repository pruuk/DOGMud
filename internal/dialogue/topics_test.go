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
		QuestOpen:    func(int) bool { return true },
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

const carriesFixtureYAML = `mobid: 57
zone: testzone
topics:
  - key: open_crash
    kind: info
    label: "Open the way down."
    text: "Then hold the disc up."
    carriesItem: 77
`

func TestEligible_CarriesItemNeedsTheItemAnywhere(t *testing.T) {
	df := loadFixture(t, carriesFixtureYAML)
	tp := df.FindTopic("open_crash")
	f := newFake()
	require.False(t, df.Eligible(tp, f.ps()), "hidden without the item")
	f.items[77] = true
	require.True(t, df.Eligible(tp, f.ps()), "HasItem stands in when CarriesItem is nil")

	worn := newFake().ps()
	worn.CarriesItem = func(id int) bool { return id == 77 }
	require.True(t, df.Eligible(tp, worn), "CarriesItem counts worn and pouched items")

	none := newFake().ps()
	none.HasItem, none.CarriesItem = nil, nil
	require.False(t, df.Eligible(tp, none), "no item callback fails closed")
	require.True(t, df.Eligible(tp, nil), "a nil PlayerState skips the gates")
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

func TestMarker_HandInBeatsEligibleOfferAndMarkedInfo(t *testing.T) {
	df := loadFixture(t, `mobid: 55
zone: testzone
topics:
  - key: tip
    kind: info
    label: "Any news?"
    text: "None."
    marked: true
  - key: new_job
    kind: offer
    quest: 8
    label: "A New Job"
    playerLine: "Got work?"
    text: "Yes."
    questExcluded: [8-start]
    grantsQuest: 8-start
  - key: old_job
    kind: handin
    quest: 6
    label: "The Old Job"
    playerLine: "It is done."
    text: "Good."
    questRequired: [6-done]
    questExcluded: [6-end]
    grantsQuest: 6-end
`)
	f := newFake()
	f.tokens["6-done"] = true
	require.Equal(t, menu.MarkerHandIn, df.Marker(f.ps()), "hand-in outranks offer and info")
	f.tokens["6-end"] = true
	require.Equal(t, menu.MarkerQuest, df.Marker(f.ps()), "offer outranks info")
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
  - key: first_try
    kind: info
    label: "Is this your purse?"
    text: "Not yet either."
    questRequired: [9-never]
    requiresItem: 77
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
	require.Equal(t, "early", tp.Key, "skips the ineligible topic listed first")
	require.True(t, wanted)
	tp, wanted = df.ItemTopic(78, f.ps())
	require.Nil(t, tp)
	require.False(t, wanted)
}

// A shown item (carriesItem) is wanted by give just as a handed one is (R8):
// give resolves as choosing the topic, and Apply never takes the item. The
// first matching topic in authored order wins, whichever field matched.
func TestItemTopic_CarriesItemWantsTheItemToo(t *testing.T) {
	df := loadFixture(t, `mobid: 51
zone: testzone
topics:
  - key: shown
    kind: info
    label: "Look at this dagger."
    text: "Fine work."
    questRequired: [35-start]
    carriesItem: 77
  - key: handed
    kind: handin
    label: "Take the dagger."
    playerLine: "Here."
    text: "Thanks."
    requiresItem: 77
`)
	f := newFake()
	f.items[77] = true
	tp, wanted := df.ItemTopic(77, f.ps())
	require.NotNil(t, tp)
	require.Equal(t, "handed", tp.Key, "the shown topic is ineligible, so the next match wins")
	require.True(t, wanted)

	f.tokens["35-start"] = true
	tp, wanted = df.ItemTopic(77, f.ps())
	require.NotNil(t, tp)
	require.Equal(t, "shown", tp.Key, "authored order decides between the two kinds")
	require.True(t, wanted)

	f.items[77] = false
	tp, wanted = df.ItemTopic(77, f.ps())
	require.Nil(t, tp, "a shown topic needs the item carried")
	require.True(t, wanted, "but it still wants the item: hand it back")
}

func TestNilFileIsSafe(t *testing.T) {
	var df *DialogueFile
	require.Equal(t, "", df.Marker(newFake().ps()))
	require.Nil(t, df.FindTopic("x"))
	require.Equal(t, "", df.TopicMarker(nil, nil))
	require.Equal(t, "", df.TopicMarker(&Topic{Kind: KindOffer}, nil))
	live := loadFixture(t, menuFixtureYAML)
	require.Equal(t, "", live.TopicMarker(nil, nil))
	text, hints := df.OpenerFor(nil)
	require.Equal(t, "", text+hints)
}

// A topic the player's quest is waiting on shows [*] even when already heard
// and not authored `marked` (#375 phase 3b slice 1). The Objective callback
// is the only difference between the cases.
func TestTopicMarker_ObjectiveShowsEvenWhenHeard(t *testing.T) {
	df := loadFixture(t, `mobid: 60
zone: testzone
topics:
  - key: directions
    kind: info
    label: "Which way?"
    text: "North."
  - key: weather
    kind: info
    label: "Weather?"
    text: "Wet."
`)
	f := newFake()
	f.heard["60.directions"] = true
	f.heard["60.weather"] = true
	want := func(mobId int, key string) bool { return mobId == 60 && key == "directions" }

	ps := f.ps()
	require.Equal(t, "", df.TopicMarker(df.FindTopic("directions"), ps), "nil Objective: a heard topic is unmarked")
	require.Equal(t, "", df.Marker(ps))

	ps.Objective = func(int, string) bool { return false }
	require.Equal(t, "", df.TopicMarker(df.FindTopic("directions"), ps), "Objective false: unmarked")

	ps.Objective = want
	require.Equal(t, menu.MarkerInfo, df.TopicMarker(df.FindTopic("directions"), ps), "the objective topic is lit though heard")
	require.Equal(t, "", df.TopicMarker(df.FindTopic("weather"), ps), "another heard topic stays unmarked")
	require.Equal(t, menu.MarkerInfo, df.Marker(ps), "the NPC's marker shows the objective")
}

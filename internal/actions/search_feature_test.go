package actions

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Targeted search (`search <feature>`, search_feature.go).

// featureRoom is a room with a noun, an alias, a hidden noun and a hidden
// container, none discovered.
func featureRoom(roomId int) *rooms.Room {
	baubles.ResetWindow(roomId) // windows are package state; start clean
	r := newSearchTestRoom(roomId)
	r.Nouns = map[string]string{
		"writing table": "A scarred writing table, its drawer stuck half open.",
		"desk":          ":writing table",
		"hearth":        "A wide stone hearth, cold and full of old ash.",
	}
	r.HiddenNouns = map[string]rooms.HiddenNoun{
		"loose brick": {Description: "A brick that shifts under your hand.", HiddenDescription: "One brick sits proud of the rest."},
	}
	r.Containers = map[string]rooms.Container{
		"strongbox": {Hidden: true},
		"barrel":    {},
	}
	return r
}

func TestFindSearchFeature(t *testing.T) {
	room := featureRoom(9601)
	actor := newSearchFakeActor("Looker", room, true, 7601)

	cases := []struct {
		typed string
		name  string
		kind  FeatureKind
		ok    bool
	}{
		{"hearth", "hearth", FeatureNoun, true},
		{"under the hearth", "hearth", FeatureNoun, true},
		{"The HEARTH", "hearth", FeatureNoun, true},
		{"desk", "writing table", FeatureNoun, true}, // alias resolves to its noun
		{"table", "writing table", FeatureNoun, true},
		{"in the barrel", "barrel", FeatureContainer, true},
		{"brick", "", "", false},     // hidden noun, not discovered
		{"strongbox", "", "", false}, // hidden container, not discovered
		{"chandelier", "", "", false},
		{"the", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		f, ok := FindSearchFeature(actor.char, room, c.typed)
		if ok != c.ok || f.Name != c.name || f.Kind != c.kind {
			t.Errorf("%q: got %+v %v, want %q %q %v", c.typed, f, ok, c.name, c.kind, c.ok)
		}
	}

	// Once discovered, hidden things can be searched like any other.
	actor.char.AddDiscovery(room.RoomId, "loose brick")
	actor.char.AddDiscovery(room.RoomId, "strongbox")
	if f, ok := FindSearchFeature(actor.char, room, "brick"); !ok || f.Name != "loose brick" || f.Kind != FeatureHiddenNoun || f.Description == "" {
		t.Errorf("discovered hidden noun: %+v %v", f, ok)
	}
	if f, ok := FindSearchFeature(actor.char, room, "strongbox"); !ok || f.Kind != FeatureContainer {
		t.Errorf("discovered container: %+v %v", f, ok)
	}
}

// Words that name no feature (an undiscovered hidden noun among them) are
// a plain search, exactly as `search <anything>` always was: every tier,
// the room's bauble roll, and identical words to the player either way.
func TestSearchFeature_NothingByThatNameIsAPlainSearch(t *testing.T) {
	pinConfigForTest(t)
	room := featureRoom(9602)
	actor := newSearchFakeActor("Guesser", room, true, 7602)
	var features []string
	h := stubBaubleSearch(t, false, actor)
	searchBaubleRoll = func(o baubles.FindOpts) (baubles.ValueTier, bool) {
		h.rolls++
		features = append(features, o.Feature)
		return baubles.TierCheap, false
	}

	hidden := Search(actor, SearchOptions{Feature: "brick"})
	hiddenSaid := append([]string(nil), actor.sent...)
	actor.sent = nil
	actor.char.Cooldowns = nil
	nonsense := Search(actor, SearchOptions{Feature: "chandelier"})

	if !hidden.FeatureNotFound || !nonsense.FeatureNotFound || hidden.Feature != "" || h.rolls != 2 {
		t.Fatalf("plain searches: %+v %+v rolls=%d", hidden, nonsense, h.rolls)
	}
	if features[0] != "" || features[1] != "" {
		t.Fatalf("the room's own roll: %q", features)
	}
	if len(hiddenSaid) == 0 || hiddenSaid[0] != actor.sent[0] || !strings.Contains(hiddenSaid[0], "snoop around") {
		t.Fatalf("an undiscovered hidden noun reads exactly like nonsense: %q vs %q", hiddenSaid, actor.sent)
	}
}

// A find in a feature: its own window key, a request that names the feature
// and carries its authored description, and a won award.
func TestSearchFeature_FindNamesTheFeature(t *testing.T) {
	pinConfigForTest(t)
	room := featureRoom(9603)
	actor := newSearchFakeActor("Rummager", room, true, 7603)
	canCarry(actor)
	h := stubBaubleSearch(t, true, actor)

	var opts baubles.FindOpts
	searchBaubleRoll = func(o baubles.FindOpts) (baubles.ValueTier, bool) {
		h.rolls++
		opts = o
		return baubles.TierCheap, true
	}
	var asked baubles.GenRequest
	baubles.SetGenerator(func(ctx context.Context, req baubles.GenRequest) (baubles.GenResult, error) {
		asked = req
		return baubles.GenResult{}, context.Canceled // falls back to a generic trinket
	}, nil)

	result := Search(actor, SearchOptions{Feature: "under the desk"})

	if result.Feature != "writing table" || !result.BaubleFound || h.rolls != 1 {
		t.Fatalf("result %+v rolls=%d", result, h.rolls)
	}
	if opts.Feature != "writing table" {
		t.Fatalf("the roll uses the feature's own window: %q", opts.Feature)
	}
	if asked.Container != "writing table" || asked.ContainerDescription != room.Nouns["writing table"] {
		t.Fatalf("the request names the feature: %q / %q", asked.Container, asked.ContainerDescription)
	}
	if len(actor.awards) != 1 || !actor.awards[0].won {
		t.Fatalf("a find trains search: %+v", actor.awards)
	}
	if !searchSaid(actor, "You search the") || !searchSaid(actor, "working it loose") || searchSaid(actor, "You find nothing of interest.") {
		t.Fatalf("messages: %q", actor.sent)
	}
	if _, has := actor.char.FindInBackpack("trinket"); !has {
		t.Fatal("the find is delivered")
	}
}

// A feature search is NOT the whole room search (owner, 2026-10-26,
// overturning 2026-09-26/27): what the room hides elsewhere (here a hidden
// noun tucked into nothing and a hidden container) is not found, no contest
// is rolled, and the shared special-move cooldown is claimed.
func TestSearchFeature_IsNotTheWholeRoomSearch(t *testing.T) {
	pinConfigForTest(t)
	room := featureRoom(9604)
	actor := newSearchFakeActor("Rummager2", room, true, 7604)
	actor.char.Stats.Perception.ValueAdj = 100000 // would win any contest it rolled
	h := stubBaubleSearch(t, false, actor)

	result := Search(actor, SearchOptions{Feature: "hearth"})

	if result.Feature != "hearth" || h.rolls != 1 || result.BaubleFound {
		t.Fatalf("result %+v rolls=%d", result, h.rolls)
	}
	if actor.char.HasDiscovery(room.RoomId, "loose brick") || actor.char.HasDiscovery(room.RoomId, "strongbox") {
		t.Fatal("a feature search finds nothing elsewhere in the room")
	}
	if len(actor.awards) != 0 {
		t.Fatalf("a search that rolled no contest does not train: %+v", actor.awards)
	}
	if !searchSaid(actor, "You search the") || !searchSaid(actor, "You find nothing of interest.") {
		t.Fatalf("messages: %q", actor.sent)
	}
	if SpecialMoveReady(actor.char) {
		t.Fatal("the search claims the shared special-move cooldown")
	}
}

// Mobs never search features: the option is ignored and the room is searched.
func TestSearchFeature_MobSearchesTheRoom(t *testing.T) {
	pinConfigForTest(t)
	room := featureRoom(9605)
	mob := newSearchFakeActor("Scout", room, false, 0)
	h := stubBaubleSearch(t, true, nil)

	result := Search(mob, SearchOptions{Feature: "hearth"})

	if result.Feature != "" || result.FeatureNotFound || h.rolls != 0 {
		t.Fatalf("a mob's search is the room search, with no bauble roll: %+v rolls=%d", result, h.rolls)
	}
}

func TestRoomSearchFeatures(t *testing.T) {
	got := RoomSearchFeatures(featureRoom(9606))
	want := []string{"barrel", "hearth", "loose brick", "strongbox", "writing table"}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, f := range got {
		if f.Name != want[i] {
			t.Fatalf("sorted, aliases left out: %+v", got)
		}
	}
}

// Only the BAUBLE roll is limited per feature: a feature offers one per
// BaubleFeatureWindowMinutes, by anyone. Searching it again is never
// refused; the search then quietly takes the room's roll, exactly as a
// plain search would.
func TestSearchFeature_BaubleRollOncePerHour(t *testing.T) {
	pinConfigForTest(t)
	room := featureRoom(9607)
	first := newSearchFakeActor("First", room, true, 7607)
	second := newSearchFakeActor("Second", room, true, 7608)
	var features []string
	h := stubBaubleSearch(t, false, first)
	searchBaubleRoll = func(o baubles.FindOpts) (baubles.ValueTier, bool) {
		h.rolls++
		features = append(features, o.Feature)
		return baubles.TierCheap, false
	}

	if r := Search(first, SearchOptions{Feature: "hearth"}); r.FeatureSearched {
		t.Fatalf("first search: %+v", r)
	}
	first.char.Cooldowns = nil
	again := Search(first, SearchOptions{Feature: "hearth"})
	other := Search(second, SearchOptions{Feature: "the hearth"})
	if !again.FeatureSearched || !other.FeatureSearched || again.OnCooldown || other.OnCooldown {
		t.Fatalf("searched again, by anyone: not refused, the feature's roll spent: %+v %+v", again, other)
	}
	if len(features) != 3 || features[0] != "hearth" || features[1] != "" || features[2] != "" {
		t.Fatalf("the feature's roll, then the room's: %q", features)
	}
	second.char.Cooldowns = nil
	if r := Search(second, SearchOptions{Feature: "barrel"}); r.FeatureSearched || features[3] != "barrel" {
		t.Fatalf("another feature has its own roll: %+v %q", r, features)
	}
	if searchSaid(first, "already searched") || searchSaid(second, "searched through recently") {
		t.Fatal("no refusal messages: searching is never limited, only baubles")
	}
}

// Indoors with one of the household about, a find stays where it was found,
// on the feature, and belongs to the household.
func TestSearchFeature_HouseholdFindStaysOnTheFeature(t *testing.T) {
	pinConfigForTest(t)
	room := featureRoom(9608)
	actor := newSearchFakeActor("Guest", room, true, 7609)
	canCarry(actor)
	stubBaubleSearch(t, true, actor)
	martha := newSearchTestMob(8801, "Martha", 9608)
	findHouseholdResidents = func(*rooms.Room) []*mobs.Mob { return []*mobs.Mob{martha} }
	when := time.Unix(5_000_000, 0)
	baubleNow = func() time.Time { return when }

	result := Search(actor, SearchOptions{Feature: "hearth"})

	if !result.BaubleFound {
		t.Fatalf("result %+v", result)
	}
	if _, has := actor.char.FindInBackpack("trinket"); has {
		t.Fatal("a household's find is not pocketed")
	}
	if len(room.Items) != 1 {
		t.Fatalf("it stays in the room: %+v", room.Items)
	}
	itm := room.Items[0]
	if itm.BaubleSpot != "on the hearth" || !itm.BaubleBelongsTo(9608) || itm.BaubleLeftAt != when.Unix() {
		t.Fatalf("on the feature, the household's, and timed: %+v", itm)
	}
	if got := itm.BaubleSpotSuffix(); got == "" {
		t.Fatal("it is listed with where it lies")
	}
	if !searchSaid(actor, "belongs to this household") || !searchSaid(actor, "Martha") || !searchSaid(actor, "(on the hearth)") {
		t.Fatalf("the finder is told why it stays: %q", actor.sent)
	}
	rec, _ := baubles.Get(itm.Bauble)
	if !rec.Household || rec.FoundIn != "hearth" || rec.Stolen {
		t.Fatalf("record: %+v", rec)
	}
}

// With one of the household about when the search is made, both the
// feature's roll and the room's roll are a household's, so their tier comes
// from the household weights; with nobody about, neither is.
func TestSearch_HouseholdFindsRollAsTheHouseholds(t *testing.T) {
	pinConfigForTest(t)
	room := featureRoom(9611)
	actor := newSearchFakeActor("Guest", room, true, 7612)
	stubBaubleSearch(t, false, actor)
	martha := newSearchTestMob(8803, "Martha", 9611)
	var residents []*mobs.Mob
	findHouseholdResidents = func(*rooms.Room) []*mobs.Mob { return residents }
	var rolls []baubles.FindOpts
	searchBaubleRoll = func(o baubles.FindOpts) (baubles.ValueTier, bool) {
		rolls = append(rolls, o)
		return ``, false
	}

	residents = []*mobs.Mob{martha}
	_ = Search(actor, SearchOptions{Feature: "hearth"}) // the feature's roll
	actor.char.Cooldowns = nil
	_ = Search(actor, SearchOptions{}) // the room's roll
	residents = nil
	actor.char.Cooldowns = nil
	_ = Search(actor, SearchOptions{})

	if len(rolls) != 3 {
		t.Fatalf("three rolls: %+v", rolls)
	}
	if rolls[0].Feature == `` || !rolls[0].Household {
		t.Fatalf("the feature's roll is the household's: %+v", rolls[0])
	}
	if rolls[1].Feature != `` || !rolls[1].Household {
		t.Fatalf("the room's roll is the household's: %+v", rolls[1])
	}
	if rolls[2].Household {
		t.Fatalf("nobody about, no household: %+v", rolls[2])
	}
}

// A find rolled as a household's stays theirs even when nobody of the
// household is about by the time it is delivered: its richer tier is for
// stealing, never for picking up.
func TestSearch_AHouseholdsFindStaysTheirsWhenTheyStepOut(t *testing.T) {
	pinConfigForTest(t)
	room := featureRoom(9612)
	actor := newSearchFakeActor("Guest", room, true, 7613)
	canCarry(actor)
	h := stubBaubleSearch(t, true, actor)
	martha := newSearchTestMob(8804, "Martha", 9612)
	asked := 0
	findHouseholdResidents = func(*rooms.Room) []*mobs.Mob {
		asked++
		if asked == 1 {
			return []*mobs.Mob{martha} // at the search
		}
		return nil // gone by the delivery
	}

	_ = Search(actor, SearchOptions{})

	if len(h.deliveries) != 1 || !h.deliveries[0].Household {
		t.Fatalf("the delivery carries the household: %+v", h.deliveries)
	}
	if _, has := actor.char.FindInBackpack("trinket"); has {
		t.Fatal("not pocketed")
	}
	if len(room.Items) != 1 || !room.Items[0].BaubleBelongsTo(9612) {
		t.Fatalf("left in the room as the household's: %+v", room.Items)
	}
	rec, _ := baubles.Get(room.Items[0].Bauble)
	if !rec.Household {
		t.Fatalf("the record is the household's: %+v", rec)
	}
	if !searchSaid(actor, "It belongs to this household, so you leave it where it lies.") || searchSaid(actor, "Martha") {
		t.Fatalf("told it is the household's, without naming anyone: %q", actor.sent)
	}
}

// Nobody home: the find is pocketed as usual. Too heavy: it is left on the
// feature, not the household's, and still timed.
func TestSearchFeature_TooHeavyIsLeftOnTheFeature(t *testing.T) {
	pinConfigForTest(t)
	room := featureRoom(9609)
	actor := newSearchFakeActor("Weak", room, true, 7610) // Strength 0: carries nothing
	stubBaubleSearch(t, true, actor)

	_ = Search(actor, SearchOptions{Feature: "barrel"})

	if len(room.Items) != 1 {
		t.Fatalf("left in the room: %+v", room.Items)
	}
	itm := room.Items[0]
	if itm.BaubleSpot != "beside the barrel" || itm.BaubleHousehold != 0 || itm.BaubleLeftAt == 0 {
		t.Fatalf("beside the container, nobody's, timed: %+v", itm)
	}
	if !searchSaid(actor, "leave it beside the barrel") {
		t.Fatalf("messages: %q", actor.sent)
	}
}

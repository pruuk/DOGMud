package actions

import (
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// #375 phase 3a, search rework (owner, 2026-10-26): `search <feature>`
// searches only that feature, plain `search` the whole room at worse odds,
// and both claim the shared special-move cooldown.

// searchRollLog records the difficulty of every static search roll.
type searchRollLog struct{ difficulties []float64 }

func (l *searchRollLog) count(d float64) int {
	n := 0
	for _, x := range l.difficulties {
		if x == d {
			n++
		}
	}
	return n
}

// pinSearchRoll makes every static search roll win (or lose) and records its
// difficulty. Opposed rolls (hiders) are not affected.
func pinSearchRoll(t *testing.T, win bool) *searchRollLog {
	t.Helper()
	log := &searchRollLog{}
	orig := searchRoll
	searchRoll = func(score, difficulty float64) contest.Result {
		log.difficulties = append(log.difficulties, difficulty)
		return contest.Result{Success: win}
	}
	t.Cleanup(func() { searchRoll = orig })
	return log
}

// splitRoom has two features, a hidden noun tucked into each, one with no
// `on`, a chained hidden noun tucked into a hidden one, a secret exit, a
// hidden container, and a hidden mob when mobInstId is not 0 (only a search
// that must never spot it may have one: a spotted mob needs the room
// registries these tests do not seed).
func splitRoom(t *testing.T, roomId, mobInstId int) *rooms.Room {
	t.Helper()
	r := newSearchTestRoom(roomId)
	if mobInstId != 0 {
		r = hiddenMobRoom(t, roomId, mobInstId, 0)
	}
	r.Nouns = map[string]string{
		"altar":  "A low stone altar, its top worn smooth.",
		"hearth": "A wide stone hearth, cold and full of old ash.",
	}
	r.HiddenNouns = map[string]rooms.HiddenNoun{
		"loose flagstone": {Description: "A flagstone.", HiddenDescription: "One flagstone rocks under your hand.", On: "altar"},
		"scratched rune":  {Description: "A rune.", HiddenDescription: "A rune is scratched into the stone.", On: "hearth"},
		"old coins":       {Description: "Coins.", HiddenDescription: "A few old coins lie in the dust."},
		"inner seam":      {Description: "A seam.", HiddenDescription: "A thin seam runs under the flagstone.", On: "loose flagstone"},
	}
	r.Exits = map[string]exit.RoomExit{"north": {Secret: true}}
	r.Containers = map[string]rooms.Container{"strongbox": {Hidden: true}}
	return r
}

func searchDifficulties(t *testing.T) (room, feature float64) {
	t.Helper()
	b := configs.GetBalanceConfig()
	room, feature = float64(b.SearchRoomHiddenDifficulty), float64(b.SearchFeatureHiddenDifficulty)
	if room != 175 || feature != 125 {
		t.Fatalf("fixture: search difficulties %v / %v, want the shipped 175 / 125", room, feature)
	}
	return room, feature
}

// `search altar` rolls only the hidden nouns tucked into the altar, at the
// feature difficulty, plus the altar's bauble roll. Nothing else in the room.
func TestSearchFeature_FindsOnlyItsOwnFeaturesSecrets(t *testing.T) {
	pinConfigForTest(t)
	_, featureD := searchDifficulties(t)
	room := splitRoom(t, 9701, 8701)
	actor := newSearchFakeActor("Prober", room, true, 7701)
	actor.char.Stats.Perception.ValueAdj = 100000
	h := stubBaubleSearch(t, false, actor)
	log := pinSearchRoll(t, true)

	result := Search(actor, SearchOptions{Feature: "altar"})

	if !reflect.DeepEqual(result.HiddenNounsFound, []string{"loose flagstone"}) {
		t.Fatalf("a feature search finds only what is tucked into it: %q", result.HiddenNounsFound)
	}
	if !reflect.DeepEqual(log.difficulties, []float64{featureD}) {
		t.Fatalf("one roll, at the feature difficulty: %v", log.difficulties)
	}
	if len(result.HiddenExitsFound)+len(result.HiddenContainersFound)+len(result.StashedItemsFound)+
		len(result.HiddenPlayersFound)+len(result.HiddenMobsFound) != 0 {
		t.Fatalf("a feature search finds nothing elsewhere in the room: %+v", result)
	}
	if actor.char.HasDiscovery(room.RoomId, "strongbox") || actor.char.HasDiscovery(room.RoomId, secretExitDiscoveryKey("north")) {
		t.Fatal("no container or exit is discovered")
	}
	if h.rolls != 1 {
		t.Fatalf("the feature's bauble roll is taken: rolls=%d", h.rolls)
	}
	if !searchSaid(actor, "One flagstone rocks under your hand.") {
		t.Fatalf("the find prints its hidden description: %q", actor.sent)
	}
	if len(actor.awards) != 1 || !actor.awards[0].won {
		t.Fatalf("a feature search that rolled trains: %+v", actor.awards)
	}

	// The chain: once the flagstone is found it is a feature too, and
	// searching it finds what is tucked into it.
	actor.char.Cooldowns = nil
	log.difficulties = nil
	result = Search(actor, SearchOptions{Feature: "loose flagstone"})
	if !reflect.DeepEqual(result.HiddenNounsFound, []string{"inner seam"}) || !reflect.DeepEqual(log.difficulties, []float64{featureD}) {
		t.Fatalf("searching a found hidden noun finds its own secret: %q at %v", result.HiddenNounsFound, log.difficulties)
	}
}

// A feature with nothing tucked into it rolls no contest at all.
func TestSearchFeature_NothingTuckedRollsNothing(t *testing.T) {
	pinConfigForTest(t)
	room := splitRoom(t, 9702, 8702) // a feature search never spots the hider
	room.Nouns["shelf"] = "A bare shelf."
	actor := newSearchFakeActor("Prober2", room, true, 7702)
	stubBaubleSearch(t, false, actor)
	log := pinSearchRoll(t, true)

	result := Search(actor, SearchOptions{Feature: "shelf"})

	if len(log.difficulties) != 0 || result.FoundAnything() || len(actor.awards) != 0 {
		t.Fatalf("rolls %v, result %+v, awards %+v", log.difficulties, result, actor.awards)
	}
	if !searchSaid(actor, "You find nothing of interest.") {
		t.Fatalf("messages: %q", actor.sent)
	}
}

// Plain search works through the whole room: every tier as before, hidden
// nouns at the room difficulty, each found one announced with the thorough
// line naming where it was tucked.
func TestSearch_PlainSearchesTheWholeRoomAtTheRoomDifficulty(t *testing.T) {
	pinConfigForTest(t)
	roomD, featureD := searchDifficulties(t)
	room := splitRoom(t, 9703, 0)
	actor := newSearchFakeActor("Sweeper", room, true, 7703)
	stubBaubleSearch(t, false, actor)
	log := pinSearchRoll(t, true)

	result := Search(actor, SearchOptions{})

	if len(result.HiddenExitsFound) != 1 || len(result.HiddenContainersFound) != 1 {
		t.Fatalf("plain search still finds exits and containers: %+v", result)
	}
	want := []string{"loose flagstone", "old coins", "scratched rune"}
	if !reflect.DeepEqual(result.HiddenNounsFound, want) {
		t.Fatalf("hidden nouns found %q, want %q (the chained one waits for its parent)", result.HiddenNounsFound, want)
	}
	if got := log.count(roomD); got != 3 {
		t.Fatalf("hidden nouns roll at the room difficulty: %d rolls at %v in %v", got, roomD, log.difficulties)
	}
	if log.count(featureD) != 2 { // the secret exit and the container keep their own 125
		t.Fatalf("exit and container rolls: %v", log.difficulties)
	}
	for _, line := range []string{
		`You work through the room inch by inch. Tucked into the <ansi fg="noun">altar</ansi>, you find a <ansi fg="noun">loose flagstone</ansi>.`,
		`You work through the room inch by inch. Tucked into the <ansi fg="noun">hearth</ansi>, you find a <ansi fg="noun">scratched rune</ansi>.`,
		`You work through the room inch by inch. You find some <ansi fg="noun">old coins</ansi>.`,
		"One flagstone rocks under your hand.",
	} {
		if !searchSaid(actor, line) {
			t.Errorf("missing %q in %q", line, actor.sent)
		}
	}

	// Next plain search: the flagstone is known now, so the seam beneath it
	// can be found, and the line names the flagstone.
	actor.char.Cooldowns = nil
	actor.sent = nil
	result = Search(actor, SearchOptions{})
	if !reflect.DeepEqual(result.HiddenNounsFound, []string{"inner seam"}) {
		t.Fatalf("second search: %q", result.HiddenNounsFound)
	}
	if !searchSaid(actor, `Tucked into the <ansi fg="noun">loose flagstone</ansi>, you find an <ansi fg="noun">inner seam</ansi>.`) {
		t.Fatalf("messages: %q", actor.sent)
	}
}

// Both searches claim the shared special-move cooldown, players and mobs.
func TestSearch_ClaimsTheSharedSpecialMoveCooldown(t *testing.T) {
	pinConfigForTest(t)

	t.Run("a special move blocks a search", func(t *testing.T) {
		for _, feature := range []string{"", "altar"} {
			room := splitRoom(t, 9704, 0)
			actor := newSearchFakeActor("Basher", room, true, 7704)
			stubBaubleSearch(t, false, actor)
			log := pinSearchRoll(t, true)
			if !ClaimSpecialMove(actor.char) {
				t.Fatal("fixture: the special move is free")
			}
			result := Search(actor, SearchOptions{Feature: feature})
			if !result.OnCooldown || len(log.difficulties) != 0 || result.FoundAnything() {
				t.Fatalf("search %q after a special move: %+v rolls %v", feature, result, log.difficulties)
			}
			if !searchSaid(actor, "You need a moment to recover before you can search.") {
				t.Fatalf("the cooldown refusal: %q", actor.sent)
			}
		}
	})

	t.Run("a search blocks a special move", func(t *testing.T) {
		for _, feature := range []string{"", "altar"} {
			room := splitRoom(t, 9705, 0)
			actor := newSearchFakeActor("Searcher", room, true, 7705)
			stubBaubleSearch(t, false, actor)
			pinSearchRoll(t, false)
			if Search(actor, SearchOptions{Feature: feature}).OnCooldown {
				t.Fatal("fixture: the first search runs")
			}
			if ClaimSpecialMove(actor.char) {
				t.Fatalf("search %q must hold the special-move cooldown", feature)
			}
		}
	})

	t.Run("a mob's search too", func(t *testing.T) {
		mob := newSearchMobActor("Scout", newSearchTestRoom(9706), 8706)
		if Search(mob, SearchOptions{}).OnCooldown {
			t.Fatal("fixture: the mob's search runs")
		}
		if ClaimSpecialMove(mob.char) {
			t.Fatal("a mob's search holds the special-move cooldown")
		}
		if !Search(mob, SearchOptions{}).OnCooldown {
			t.Fatal("and is refused while it cools down")
		}
	})
}

// Plain search rolls each quest search spot at the room difficulty and, on a
// success, names it with the thorough line; a feature search never rolls them.
func TestSearch_PlainSearchRollsQuestFeatures(t *testing.T) {
	pinConfigForTest(t)
	roomD, _ := searchDifficulties(t)
	const line = `You work through the room inch by inch, and your search leads you to the <ansi fg="noun">hearth</ansi>.`

	t.Run("found", func(t *testing.T) {
		actor := newSearchFakeActor("Quester", newSearchTestRoom(9707), true, 7707)
		stubBaubleSearch(t, false, actor)
		log := pinSearchRoll(t, true)
		result := Search(actor, SearchOptions{QuestFeatures: []string{"hearth"}})
		if !reflect.DeepEqual(result.QuestFeaturesFound, []string{"hearth"}) || !reflect.DeepEqual(log.difficulties, []float64{roomD}) {
			t.Fatalf("found %q rolls %v", result.QuestFeaturesFound, log.difficulties)
		}
		if !searchSaid(actor, line) || searchSaid(actor, "You find nothing of interest.") {
			t.Fatalf("messages: %q", actor.sent)
		}
	})
	t.Run("missed", func(t *testing.T) {
		actor := newSearchFakeActor("Quester2", newSearchTestRoom(9708), true, 7708)
		stubBaubleSearch(t, false, actor)
		pinSearchRoll(t, false)
		result := Search(actor, SearchOptions{QuestFeatures: []string{"hearth"}})
		if len(result.QuestFeaturesFound) != 0 || searchSaid(actor, line) || !searchSaid(actor, "You find nothing of interest.") {
			t.Fatalf("result %+v messages %q", result, actor.sent)
		}
	})
	t.Run("a feature search ignores them", func(t *testing.T) {
		room := newSearchTestRoom(9709)
		room.Nouns = map[string]string{"hearth": "A hearth."}
		actor := newSearchFakeActor("Quester3", room, true, 7709)
		stubBaubleSearch(t, false, actor)
		log := pinSearchRoll(t, true)
		result := Search(actor, SearchOptions{Feature: "hearth", QuestFeatures: []string{"hearth"}})
		if len(result.QuestFeaturesFound) != 0 || len(log.difficulties) != 0 {
			t.Fatalf("result %+v rolls %v", result, log.difficulties)
		}
	})
}

// "You find nothing of interest." is the same words whether nothing is there
// or every roll failed, for plain and feature searches alike.
func TestSearch_FailedAndEmptySearchesReadTheSame(t *testing.T) {
	pinConfigForTest(t)

	run := func(name string, room *rooms.Room, uid int, opts SearchOptions) []string {
		actor := newSearchFakeActor(name, room, true, uid)
		stubBaubleSearch(t, false, actor)
		pinSearchRoll(t, false)
		Search(actor, opts)
		return actor.sent
	}

	rich := splitRoom(t, 9710, 0)
	bare := newSearchTestRoom(9711)
	bare.Nouns = map[string]string{"altar": "A low stone altar, its top worn smooth."}

	failed := run("Seeker", rich, 7710, SearchOptions{QuestFeatures: []string{"hearth"}})
	empty := run("Seeker", bare, 7711, SearchOptions{})
	if !reflect.DeepEqual(failed, empty) {
		t.Errorf("plain search: failed %q vs empty %q", failed, empty)
	}

	failed = run("Seeker", rich, 7712, SearchOptions{Feature: "altar"})
	empty = run("Seeker", bare, 7713, SearchOptions{Feature: "altar"})
	if !reflect.DeepEqual(failed, empty) {
		t.Errorf("feature search: failed %q vs empty %q", failed, empty)
	}
}

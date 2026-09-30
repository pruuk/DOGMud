package actions

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Search's bauble tier (docs/baubles Phases 3 and 4). The dice and the roll
// window are the baubles package's (tested there). Here the roll is stubbed,
// delivery runs in line (no goroutine, no lock, no delay), and the recipient
// is the test's fake actor.

type baubleHarness struct {
	rolls      int
	deliveries []BaubleDelivery
}

// stubBaubleSearch makes every bauble roll find (or not) and delivers finds
// in line to actor. With actor nil the finder counts as offline.
func stubBaubleSearch(t *testing.T, found bool, actor *searchFakeActor) *baubleHarness {
	t.Helper()
	seedBaubleSale(t) // carrier + a temp catalog (sell_bauble_test.go)
	baubles.SetGenerator(nil, nil)
	h := &baubleHarness{}

	origRoll, origStart, origWho, origRoom := searchBaubleRoll, startBaubleDelivery, findBaubleRecipient, findBaubleRoom
	origResidents, origNow := findHouseholdResidents, baubleNow
	// Nobody keeps house unless a test says so, and the find room is the
	// actor's (no room registry in these tests).
	findHouseholdResidents = func(*rooms.Room) []*mobs.Mob { return nil }
	findBaubleRoom = func(roomId int) *rooms.Room {
		if actor != nil && actor.room != nil && actor.room.RoomId == roomId {
			return actor.room
		}
		return nil
	}
	searchBaubleRoll = func(o baubles.FindOpts) (baubles.ValueTier, bool) {
		h.rolls++
		return baubles.TierAverage, found
	}
	startBaubleDelivery = func(d BaubleDelivery) {
		h.deliveries = append(h.deliveries, d)
		d.MinDelay = 0
		d.run(false)
	}
	findBaubleRecipient = func(userId int) (baubleRecipient, bool) {
		if actor == nil || userId != actor.userId {
			return baubleRecipient{}, false
		}
		return baubleRecipient{char: actor.char, room: actor.room, send: func(s string) { actor.sent = append(actor.sent, s) }}, true
	}
	t.Cleanup(func() {
		searchBaubleRoll, startBaubleDelivery, findBaubleRecipient, findBaubleRoom = origRoll, origStart, origWho, origRoom
		findHouseholdResidents, baubleNow = origResidents, origNow
		baubles.SetGenerator(nil, nil)
	})
	return h
}

// canCarry gives the fake character enough strength to carry a bauble.
func canCarry(a *searchFakeActor) {
	a.char.Stats.Strength.ValueAdj = 50
}

func TestSearch_Bauble_NoKeyDeliversAGenericTrinket(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Finder", newSearchTestRoom(9501), true, 7101)
	canCarry(actor)
	h := stubBaubleSearch(t, true, actor)

	result := Search(actor, SearchOptions{})

	if h.rolls != 1 || !result.BaubleFound || len(h.deliveries) != 1 {
		t.Fatalf("rolls=%d found=%v deliveries=%d", h.rolls, result.BaubleFound, len(h.deliveries))
	}
	if !result.FoundAnything() || searchSaid(actor, "You find nothing of interest.") {
		t.Error("a find must not also be reported as nothing")
	}
	if !searchSaid(actor, "working it loose") {
		t.Errorf("the search tells the player it is still going: %q", actor.sent)
	}
	itm, has := actor.char.FindInBackpack("trinket")
	if !has || !itm.IsBauble() {
		t.Fatal("the generic trinket is in the pack")
	}
	rec, _ := baubles.Get(itm.Bauble)
	if rec.Name != "Trinket" || rec.Generator != baubles.GeneratorLocal || rec.Status != baubles.StatusFallback || rec.Source != baubles.SourceSearch {
		t.Fatalf("with no generator every find is a generic trinket: %+v", rec)
	}
	if !baubles.TierAverage.Range().Contains(rec.Value) || rec.RoomId != 9501 || rec.FoundByUserId != 7101 {
		t.Fatalf("value and provenance: %+v", rec)
	}
	if !searchSaid(actor, "You pocket it.") {
		t.Errorf("delivery is announced: %q", actor.sent)
	}
}

func TestSearch_Bauble_ModelNamesTheFind(t *testing.T) {
	pinConfigForTest(t)
	room := newSearchTestRoom(9502)
	room.Title = "The Toymaker's Back Room"
	room.Description = `Shelves of <ansi fg="red">half-painted</ansi> toys.`
	room.Nouns = map[string]string{"shelves": "x", "toys": "y"}
	actor := newSearchFakeActor("Finder2", room, true, 7102)
	canCarry(actor)
	stubBaubleSearch(t, true, actor)

	var asked baubles.GenRequest
	baubles.SetGenerator(func(ctx context.Context, req baubles.GenRequest) (baubles.GenResult, error) {
		asked = req
		return baubles.GenResult{Reply: baubles.Reply{
			Name: "Painted Wooden Horse", NameSimple: "horse", Material: "pine",
			Description: "A child's toy horse, its red paint flaking from the mane.",
			WeightLbs:   0.6, Value: 12,
		}, Model: "gpt-test", PromptVersion: 1}, nil
	}, nil)

	_ = Search(actor, SearchOptions{})

	if asked.RoomTitle != room.Title || asked.Tier != baubles.TierAverage || len(asked.RoomNouns) != 2 || asked.Source != baubles.SourceSearch {
		t.Fatalf("the request carries the room: %+v", asked)
	}
	itm, has := actor.char.FindInBackpack("horse")
	if !has {
		t.Fatalf("the named bauble is in the pack: %q", actor.sent)
	}
	rec, _ := baubles.Get(itm.Bauble)
	if rec.Generator != baubles.GeneratorOpenAI || rec.Status != baubles.StatusReady || rec.Model != "gpt-test" || rec.WeightLbs != 0.6 {
		t.Fatalf("record: %+v", rec)
	}
	if !searchSaid(actor, "Painted Wooden Horse") {
		t.Errorf("the reveal names it: %q", actor.sent)
	}
}

func TestSearch_Bauble_TooHeavyIsLeftOnTheFloor(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Weakling", newSearchTestRoom(9503), true, 7103) // Strength 0: carries nothing
	stubBaubleSearch(t, true, actor)

	_ = Search(actor, SearchOptions{})

	if len(actor.room.Items) != 1 || !actor.room.Items[0].IsBauble() {
		t.Fatalf("the bauble is left on the floor: %+v", actor.room.Items)
	}
	if !searchSaid(actor, "carrying too much") {
		t.Errorf("the player is told why: %q", actor.sent)
	}
}

func TestSearch_Bauble_FinderOfflineLeavesItWhereFound(t *testing.T) {
	pinConfigForTest(t)
	room := newSearchTestRoom(9504)
	actor := newSearchFakeActor("Gone", room, true, 7104)
	stubBaubleSearch(t, true, nil) // no recipient: logged off before delivery
	findBaubleRoom = func(roomId int) *rooms.Room {
		if roomId == 9504 {
			return room
		}
		return nil
	}

	_ = Search(actor, SearchOptions{})
	if len(room.Items) != 1 || !room.Items[0].IsBauble() {
		t.Fatalf("an absent finder's bauble waits in the room it was found in: %+v", room.Items)
	}
}

// A find trains search: it is a won search, even in a room with nothing
// else to roll against (owner ruling, 2026-09-26).
func TestSearch_Bauble_AFindTrainsSearch(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("LuckyOne", newSearchTestRoom(9505), true, 7105)
	canCarry(actor)
	stubBaubleSearch(t, true, actor)

	_ = Search(actor, SearchOptions{})
	if len(actor.awards) != 1 || !actor.awards[0].won {
		t.Fatalf("a bauble find is one won award, got %+v", actor.awards)
	}
	if _, n := actor.awardedCandidate(string(skills.Search)); n != 1 {
		t.Errorf("the award names the search skill %d times, want 1", n)
	}
}

// A bauble roll that finds nothing pays nothing: otherwise every room would
// be a progression candidate, since every room offers bauble rolls.
func TestSearch_Bauble_AMissedRollAwardsNothing(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Unlucky", newSearchTestRoom(9516), true, 7131)
	h := stubBaubleSearch(t, false, actor)

	_ = Search(actor, SearchOptions{})
	if h.rolls != 1 || len(actor.awards) != 0 {
		t.Fatalf("rolled but found nothing in an empty room: rolls=%d awards=%+v", h.rolls, actor.awards)
	}
}

// With contests lost, a find still makes the one award a win.
func TestSearch_Bauble_AFindWinsAlongsideLostContests(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("LuckyLoser", searchRoomWithHiddenNouns(9506, 3), true, 7106)
	suppressSearchFinds(actor)
	canCarry(actor)
	stubBaubleSearch(t, true, actor)

	result := Search(actor, SearchOptions{})
	if len(result.HiddenNounsFound) > 0 {
		t.Skip("fixture beat a five-sigma roll and found a noun; nothing to assert")
	}
	if !result.BaubleFound {
		t.Fatal("the bauble was found")
	}
	if len(actor.awards) != 1 || !actor.awards[0].won {
		t.Fatalf("still ONE award per search, and a win: %+v", actor.awards)
	}
}

// The roll is told the searcher's skill and the room's biome.
func TestSearch_Bauble_RollGetsSkillAndBiome(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Skilled", newSearchTestRoom(9517), true, 7132)
	actor.char.Skills = map[string]int{string(skills.Search): 50}
	stubBaubleSearch(t, false, actor)
	var got baubles.FindOpts
	searchBaubleRoll = func(o baubles.FindOpts) (baubles.ValueTier, bool) {
		got = o
		return ``, false
	}

	_ = Search(actor, SearchOptions{})
	if got.SkillFactor != 1 || got.Place.RoomId != 9517 {
		t.Fatalf("search at the soft cap is skill factor 1: %+v", got)
	}
}

func TestBaubleSkillFactor(t *testing.T) {
	pinConfigForTest(t)
	c := &characters.Character{}
	if BaubleSkillFactor(c) != 0 || BaubleSkillFactor(nil) != 0 {
		t.Fatal("untrained is 0")
	}
	c.Skills = map[string]int{string(skills.Search): 50}
	if BaubleSkillFactor(c) != 1 {
		t.Fatalf("soft cap is 1: %v", BaubleSkillFactor(c))
	}
	c.Skills[string(skills.Search)] = 200
	if BaubleSkillFactor(c) != 1 {
		t.Fatal("capped at 1")
	}
	c.Skills[string(skills.Search)] = 12 // sqrt(12/50) is about 0.49
	if f := BaubleSkillFactor(c); f < 0.48 || f > 0.5 {
		t.Fatalf("square-root curve: %v", f)
	}
}

func TestSearch_Bauble_MobsNeverRoll(t *testing.T) {
	pinConfigForTest(t)
	h := stubBaubleSearch(t, true, nil)

	room := newSearchTestRoom(9507)
	mob := newSearchTestMob(8901, "Scavenger", 9507)
	actor := newSearchMobActor("Scavenger", room, mob.InstanceId)

	if result := Search(actor, SearchOptions{}); h.rolls != 0 || result.BaubleFound {
		t.Fatal("mob searches never roll for baubles")
	}
}

func TestSearch_Bauble_ExcludedRoomKindsNeverRoll(t *testing.T) {
	pinConfigForTest(t)
	h := stubBaubleSearch(t, true, nil)

	bank := newSearchTestRoom(9508)
	bank.IsBank = true
	storage := newSearchTestRoom(9509)
	storage.IsStorage = true
	charRoom := newSearchTestRoom(9510)
	charRoom.IsCharacterRoom = true

	for i, room := range []*rooms.Room{bank, storage, charRoom} {
		actor := newSearchFakeActor("Excluded", room, true, 7110+i)
		_ = Search(actor, SearchOptions{})
	}
	if h.rolls != 0 {
		t.Fatalf("banks, storage and character rooms never roll; got %d rolls", h.rolls)
	}
	if !baubleRoomAllowed(newSearchTestRoom(9511)) {
		t.Fatal("an ordinary room is allowed")
	}

	// A private room (a housing lodging) never offers baubles either.
	rooms.SetPrivateRoomCheck(func(roomId int) bool { return roomId == 9514 })
	defer rooms.SetPrivateRoomCheck(nil)
	if baubleRoomAllowed(newSearchTestRoom(9514)) {
		t.Fatal("a private room offered baubles")
	}
}

// The anti-leak property holds with the bauble tier in place: a failed
// bauble roll changes nothing the player can see.
func TestSearch_Bauble_FailedRollIsInvisible(t *testing.T) {
	pinConfigForTest(t)
	fruitless := newSearchFakeActor("NoLuck", searchRoomWithHiddenNouns(9512, 3), true, 7120)
	suppressSearchFinds(fruitless)
	empty := newSearchFakeActor("NoLuckBare", newSearchTestRoom(9513), true, 7121)
	h := stubBaubleSearch(t, false, nil)

	rf := Search(fruitless, SearchOptions{})
	_ = Search(empty, SearchOptions{})
	if rf.FoundAnything() {
		t.Skip("fixture beat a five-sigma roll and found something; nothing to compare")
	}
	if h.rolls != 2 || len(h.deliveries) != 0 {
		t.Fatalf("both searched, neither found: rolls=%d deliveries=%d", h.rolls, len(h.deliveries))
	}
	if !reflect.DeepEqual(fruitless.sent, empty.sent) {
		t.Errorf("failed and bare searches must read identically:\n failed: %q\n bare:   %q", fruitless.sent, empty.sent)
	}
	if !searchSaid(empty, "You find nothing of interest.") {
		t.Error("the no-find line is unchanged")
	}
}

// The delivery waits out MinDelay even when naming is instant, so a generic
// trinket and a model-named find arrive at the same pace.
func TestBaubleDelivery_WaitsTheMinimumDelay(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Patient", newSearchTestRoom(9514), true, 7130)
	canCarry(actor)
	stubBaubleSearch(t, true, actor)

	d := BaubleDelivery{
		Request:  BaubleRequest(actor.room, baubles.TierCheap, baubles.SourceSearch, ""),
		UserId:   7130,
		MinDelay: 150 * time.Millisecond,
	}
	start := time.Now()
	d.run(false)
	if time.Since(start) < 150*time.Millisecond {
		t.Fatal("delivery must not arrive before MinDelay")
	}
	if _, has := actor.char.FindInBackpack("trinket"); !has {
		t.Fatal("delivered after the wait")
	}
}

func TestBaubleRequest_OnlyAuthoredRoomText(t *testing.T) {
	pinConfigForTest(t)
	room := newSearchTestRoom(9515)
	room.Title = "Market Square"
	room.Description = "A busy square."
	room.Signs = []rooms.Sign{{DisplayText: "PLAYER SCRIBBLE"}}
	req := BaubleRequest(room, baubles.TierRare, baubles.SourceSearch, "barrel")
	if req.RoomTitle != "Market Square" || req.Container != "barrel" || req.Tier != baubles.TierRare {
		t.Fatalf("request: %+v", req)
	}
	all := append([]string{req.RoomTitle, req.RoomDescription, req.Container, req.TimeOfDay}, req.RoomNouns...)
	for _, v := range all {
		if strings.Contains(v, "PLAYER SCRIBBLE") {
			t.Fatal("signs never reach the model")
		}
	}
}

func TestSearchResult_FoundByContestExcludesBaubles(t *testing.T) {
	r := SearchResult{BaubleFound: true}
	if !r.FoundAnything() || r.foundByContest() {
		t.Fatal("a bauble is a find but not a contest win")
	}
	r.HiddenNounsFound = []string{"x"}
	if !r.foundByContest() {
		t.Fatal("a noun is a contest win")
	}
}

// Copyover or shutdown while a find is still being named (the player was
// told "Something glints..."): the flush delivers it now, as the fallback
// it would have been (the corpus, or a generic trinket), and the delivery
// goroutine stands down, so the find arrives exactly once.
func TestFlush_FinishesAFindStillBeingNamed(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Hasty", newSearchTestRoom(9520), true, 7140)
	canCarry(actor)
	stubBaubleSearch(t, true, actor)
	naming := make(chan struct{})
	baubles.SetGenerator(func(ctx context.Context, req baubles.GenRequest) (baubles.GenResult, error) {
		close(naming)
		<-ctx.Done() // the model has not answered when the flush comes
		return baubles.GenResult{}, ctx.Err()
	}, nil)

	d := BaubleDelivery{Request: BaubleRequest(actor.room, baubles.TierCheap, baubles.SourceSearch, ""), UserId: 7140}
	done := make(chan struct{})
	go func() { d.run(false); close(done) }()
	<-naming

	if n := FlushBaubleDeliveries(); n != 1 {
		t.Fatalf("one find finished by the flush, got %d", n)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the delivery goroutine stands down once flushed")
	}
	count := 0
	for _, it := range actor.char.Items {
		if it.IsBauble() {
			count++
		}
	}
	if count != 1 || PendingBaubleDeliveries() != 0 {
		t.Fatalf("exactly one find, nothing left pending: %d found, %d pending", count, PendingBaubleDeliveries())
	}
	if _, has := actor.char.FindInBackpack("trinket"); !has {
		t.Fatal("unnamed: a generic trinket")
	}
}

// A naming that came back before the flush is kept, and the flush does not
// wait out the reveal delay.
func TestFlush_KeepsANamingThatCameBack(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Lucky", newSearchTestRoom(9521), true, 7141)
	canCarry(actor)
	stubBaubleSearch(t, true, actor)
	baubles.SetGenerator(func(ctx context.Context, req baubles.GenRequest) (baubles.GenResult, error) {
		return baubles.GenResult{Reply: baubles.Reply{
			Name: "Painted Wooden Horse", NameSimple: "horse", Material: "pine",
			Description: "A child's toy horse.", WeightLbs: 0.6, Value: 3,
		}, Model: "gpt-test", PromptVersion: 1}, nil
	}, nil)

	d := BaubleDelivery{Request: BaubleRequest(actor.room, baubles.TierCheap, baubles.SourceSearch, ""), UserId: 7141, MinDelay: time.Minute}
	done := make(chan struct{})
	go func() { d.run(false); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		named := false
		pendingFinds.Lock()
		for _, p := range pendingFinds.m {
			_, named = p.result()
		}
		pendingFinds.Unlock()
		if named {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("never named")
		}
		time.Sleep(5 * time.Millisecond)
	}

	start := time.Now()
	FlushBaubleDeliveries()
	<-done
	if time.Since(start) > 5*time.Second {
		t.Fatal("the flush does not wait out the reveal delay")
	}
	if _, has := actor.char.FindInBackpack("horse"); !has {
		t.Fatal("the naming that came back is the one delivered")
	}
}

// A finder gone offline, from a room that can no longer be loaded: nothing
// is minted, so no record exists for a find that is nowhere.
func TestDeliver_NowhereToPutItMintsNothing(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Ghost", newSearchTestRoom(9522), true, 7142)
	stubBaubleSearch(t, true, nil) // finder offline
	findBaubleRoom = func(int) *rooms.Room { return nil }
	before := baubles.Count()
	d := BaubleDelivery{Request: BaubleRequest(actor.room, baubles.TierCheap, baubles.SourceSearch, ""), UserId: 7142}
	d.run(false)
	if baubles.Count() != before {
		t.Fatalf("no ghost record: %d records, was %d", baubles.Count(), before)
	}
}

// origStartBaubleDelivery is the production starter, taken at package
// initialisation, before any test or init() replaces it.
var origStartBaubleDelivery = startBaubleDelivery

// A find is tracked the moment it is started, before its goroutine runs: a
// copyover in the same pass of the game loop still finishes it.
func TestFlush_FindsAFindWhoseGoroutineHasNotStarted(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Early", newSearchTestRoom(9523), true, 7143)
	canCarry(actor)
	stubBaubleSearch(t, true, actor)
	startBaubleDelivery = origStartBaubleDelivery // the production starter, goroutine and all

	util.LockMud() // the game loop's lock: the goroutine cannot deliver
	startBaubleDelivery(BaubleDelivery{Request: BaubleRequest(actor.room, baubles.TierCheap, baubles.SourceSearch, ""), UserId: 7143})
	n := FlushBaubleDeliveries() // same pass, same lock
	util.UnlockMud()
	if n != 1 {
		t.Fatalf("the find started a moment ago is finished by the flush: %d", n)
	}
	deadline := time.Now().Add(5 * time.Second)
	for PendingBaubleDeliveries() != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	count := 0
	for _, it := range actor.char.Items {
		if it.IsBauble() {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("delivered exactly once: %d", count)
	}
}

// loadTestCorpus gives the baubles package a small fallback corpus for one
// test: bare cheap and average pools (the test rooms have no biome) and a
// pocket pool.
func loadTestCorpus(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	seed := filepath.Join(dir, "bauble-corpus.yaml")
	text := `entries:
  cheap:
    - name: Bent Tin Thimble
      name_simple: thimble
      description: A tin thimble, pressed a little out of shape by a careless heel.
      material: tin
      weight_lbs: 0.1
      value: 3
  average:
    - name: Painted Wooden Spool
      name_simple: spool
      description: A wooden thread spool painted with a band of faded blue.
      material: wood
      weight_lbs: 0.2
      value: 12
  pocket-cheap:
    - name: Brass Snuff Spoon
      name_simple: spoon
      description: A tiny brass spoon for snuff, its bowl no bigger than a fingernail.
      material: brass
      weight_lbs: 0.1
      value: 2
`
	if err := os.WriteFile(seed, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := baubles.LoadCorpusFrom(seed, filepath.Join(dir, "corpus.promoted.yaml"))
	t.Cleanup(baubles.ClearCorpusForTest)
	if rep.Seed != 3 {
		t.Fatalf("test corpus: %+v", rep)
	}
}

func TestSearch_Bauble_NoKeyDrawsFromTheCorpus(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Finder", newSearchTestRoom(9540), true, 7160)
	canCarry(actor)
	stubBaubleSearch(t, true, actor) // every roll finds, tier average
	loadTestCorpus(t)

	Search(actor, SearchOptions{})

	itm, has := actor.char.FindInBackpack("spool")
	if !has || !itm.IsBauble() {
		t.Fatalf("the corpus entry is in the pack: %q", actor.sent)
	}
	rec, _ := baubles.Get(itm.Bauble)
	if rec.Name != "Painted Wooden Spool" || rec.Generator != baubles.GeneratorCorpus || rec.Status != baubles.StatusReady || rec.Model != "corpus:average" {
		t.Fatalf("record: %+v", rec)
	}
}

// A find still being named at a flush is finished from the corpus.
func TestFlush_UnnamedFindDrawsFromTheCorpus(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Hasty", newSearchTestRoom(9541), true, 7161)
	canCarry(actor)
	stubBaubleSearch(t, true, actor)
	loadTestCorpus(t)
	naming := make(chan struct{})
	baubles.SetGenerator(func(ctx context.Context, req baubles.GenRequest) (baubles.GenResult, error) {
		close(naming)
		<-ctx.Done()
		return baubles.GenResult{}, ctx.Err()
	}, nil)

	d := BaubleDelivery{Request: BaubleRequest(actor.room, baubles.TierCheap, baubles.SourceSearch, ""), UserId: 7161}
	done := make(chan struct{})
	go func() { d.run(false); close(done) }()
	<-naming

	if n := FlushBaubleDeliveries(); n != 1 {
		t.Fatalf("one find finished by the flush, got %d", n)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the delivery goroutine stands down once flushed")
	}
	if _, has := actor.char.FindInBackpack("thimble"); !has {
		t.Fatal("unnamed at the flush: drawn from the corpus")
	}
}

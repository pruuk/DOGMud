package actions

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crimes"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/knowledge"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// A player's pickpocket of an NPC (steal_pocket.go): the roll at once, the
// outcome after a Dexterity-scaled pause, and a bauble among the loot. The
// outcome of the roll is passed straight to startPocketAttempt here, so
// each test sees the branch it is about; the roll itself is steal_test.go's.

// origRunPocketAttempt is the production runner (goroutine and pause),
// taken at package initialisation, before the test init replaces it.
var origRunPocketAttempt = runPocketAttempt

type pocketHarness struct {
	room  *rooms.Room
	thief *searchFakeActor
	mark  *mobs.Mob
	asked []baubles.GenRequest
}

func setupPocket(t *testing.T, roomId int, userId int) *pocketHarness {
	t.Helper()
	pinConfigForTest(t)
	seedBaubleSale(t) // the carrier and a temp catalog
	baubles.SetGenerator(nil, nil)
	h := &pocketHarness{room: newSearchTestRoom(roomId)}
	h.thief = newSearchFakeActor("Nimble", h.room, true, userId)
	h.thief.char = characters.New()
	h.thief.char.Name = "Nimble"
	h.thief.char.Stats.Dexterity.ValueAdj = 100
	h.thief.char.Skills[string(skills.Skullduggery)] = 2
	canCarry(h.thief)
	h.mark = newStealTestMob(9800+roomId%100, 40, 1)
	h.mark.Character.Name = "a harried clerk"
	h.mark.Character.RoomId = roomId
	mobs.SetInstanceForTest(h.mark.InstanceId, h.mark)

	origRun, origRoll := runPocketAttempt, pocketBaubleRoll
	t.Cleanup(func() {
		mobs.SetInstanceForTest(h.mark.InstanceId, nil)
		runPocketAttempt, pocketBaubleRoll = origRun, origRoll
		baubles.SetGenerator(nil, nil)
	})
	return h
}

// name installs a generator that names every find (after wait) and
// records what it was asked.
func (h *pocketHarness) name(t *testing.T, wait time.Duration) {
	baubles.SetGenerator(func(ctx context.Context, req baubles.GenRequest) (baubles.GenResult, error) {
		h.asked = append(h.asked, req)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return baubles.GenResult{}, ctx.Err()
		}
		return baubles.GenResult{Reply: baubles.Reply{
			Name: "Tarnished Brass Thimble", NameSimple: "thimble", Material: "brass",
			Description: "A dented brass thimble, warm from a pocket.", WeightLbs: 0.1, Value: 3,
		}, Model: "gpt-test", PromptVersion: 4}, nil
	}, nil)
}

// paused runs attempts on the production runner with this pause.
func paused(d time.Duration, grace time.Duration) {
	runPocketAttempt = func(p *pocketAttempt) StealResult {
		p.delay, p.grace = d, grace
		return origRunPocketAttempt(p)
	}
}

func waitSettled(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for PendingPocketAttempts() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the pickpocket never resolved")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (h *pocketHarness) baubleCarried() (items.Item, bool) {
	for _, it := range h.thief.char.Items {
		if it.IsBauble() {
			return it, true
		}
	}
	return items.Item{}, false
}

func said(a *searchFakeActor, part string) int {
	n := 0
	for _, s := range a.sent {
		if strings.Contains(s, part) {
			n++
		}
	}
	return n
}

// Quicker hands are faster: StealPocketSeconds (3) at Dexterity 100, scaled
// by 100/Dexterity, between 1.5 and 6 seconds.
func TestPocketDelayScalesWithDexterity(t *testing.T) {
	pinConfigForTest(t)
	for dex, want := range map[int]time.Duration{
		100: 3 * time.Second, 150: 2 * time.Second, 200: 1500 * time.Millisecond, 1000: 1500 * time.Millisecond,
		50: 6 * time.Second, 10: 6 * time.Second, 0: 6 * time.Second,
	} {
		if got := PocketDelay(dex); got != want {
			t.Errorf("Dexterity %d: %v, want %v", dex, got, want)
		}
	}
}

// The attempt is announced at once and nothing moves until the pause ends;
// then the loot, with the ordinary success line.
func TestPickpocketHoldsTheOutcomeForThePause(t *testing.T) {
	h := setupPocket(t, 9601, 7601)
	paused(150*time.Millisecond, 0)
	goldBefore := h.thief.char.Gold

	util.LockMud()
	res := startPocketAttempt(h.thief, h.mark, true)
	util.UnlockMud()
	if !res.Pending || said(h.thief, "You attempt to pick") != 1 || said(h.thief, "successfully steal") != 0 {
		t.Fatalf("announced, and held back: %+v %q", res, h.thief.sent)
	}
	util.LockMud() // game state is read under the lock the reveal takes
	moved := h.thief.char.Gold != goldBefore || h.mark.Character.Gold != 40
	util.UnlockMud()
	if moved {
		t.Fatal("nothing moves during the pause")
	}
	waitSettled(t)
	if h.thief.char.Gold <= goldBefore || said(h.thief, "successfully steal") != 1 {
		t.Fatalf("revealed after the pause: gold %d, %q", h.thief.char.Gold, h.thief.sent)
	}
}

// A mark carrying a bauble loses it to a successful pickpocket, in the same
// line as the rest, and it is recorded as stolen.
func TestPickpocketTakesTheBaubleTheMarkCarries(t *testing.T) {
	h := setupPocket(t, 9602, 7602)
	h.room.Zone = "Ashwick"
	itm, rec, err := baubles.Mint(baubles.MintOpts{Tier: baubles.TierCheap, Source: baubles.SourcePickpocket,
		Result: &baubles.GenResult{Reply: baubles.Reply{Name: "Bent Copper Ring", NameSimple: "ring", Description: "A copper ring, bent out of true.", WeightLbs: 0.1, Value: 2}, Generator: baubles.GeneratorOpenAI}})
	if err != nil {
		t.Fatal(err)
	}
	intoPocket(h.mark, itm)
	pocketBaubleRoll = func(*rooms.Room) bool { t.Fatal("a mark with a bauble makes no new one"); return false }

	res := startPocketAttempt(h.thief, h.mark, true)
	got, ok := h.baubleCarried()
	if !res.Succeeded || !ok || got.Bauble != rec.Id {
		t.Fatalf("the mark's bauble is taken: %+v", res)
	}
	if _, still := carriedBauble(h.mark); still {
		t.Fatal("and is no longer the mark's")
	}
	if said(h.thief, "Bent Copper Ring") != 1 || said(h.thief, "successfully steal") != 1 {
		t.Fatalf("in the one success line: %q", h.thief.sent)
	}
	if r, _ := baubles.Get(rec.Id); !r.Stolen || r.StolenByUserId != 7602 || r.StolenFromName != "a harried clerk" || r.StolenZone != "Ashwick" {
		t.Fatalf("recorded as stolen: %+v", r)
	}
}

// A bauble a player gave the mark is not the mark's own: picked back out of
// its pocket, it is taken (the steal is still a steal) but does not become
// stolen goods, so a fence pays no premium for it. A bauble stolen from
// someone else before the gift stays that someone's stolen goods.
func TestPickpocketTakesBackAGiftWithoutMakingItStolenGoods(t *testing.T) {
	h := setupPocket(t, 9606, 7606)
	h.room.Zone = "Ashwick"
	h.mark.MobId = 77 // gifts are marked by mob id, as thefts are
	pocketBaubleRoll = func(*rooms.Room) bool { return false }
	mint := func(name string) items.Item {
		itm, _, err := baubles.Mint(baubles.MintOpts{Tier: baubles.TierCheap, Source: baubles.SourceSearch,
			Result: &baubles.GenResult{Reply: baubles.Reply{Name: name, NameSimple: "ring", Description: "A ring.", WeightLbs: 0.1, Value: 2}, Generator: baubles.GeneratorOpenAI}})
		if err != nil {
			t.Fatal(err)
		}
		return itm
	}

	gift := mint("Bent Copper Ring")
	if StolenBaubleGiven(h.thief, h.mark, gift) {
		t.Fatal("a gift is not a return")
	}
	intoPocket(h.mark, gift)
	if res := startPocketAttempt(h.thief, h.mark, true); !res.Succeeded {
		t.Fatalf("the steal succeeds: %+v", res)
	}
	if got, ok := h.baubleCarried(); !ok || got.Bauble != gift.Bauble {
		t.Fatal("the gift is taken back")
	}
	if r, _ := baubles.Get(gift.Bauble); r.Stolen || r.StolenGoods() {
		t.Fatalf("but it is not the mark's stolen goods: %+v", r)
	}

	// Stolen from another mob first, then given to this one: still the
	// first owner's stolen goods after the pickpocket.
	h.thief.char.Items = nil
	loot := mint("Jade Ring")
	if !baubles.MarkStolen(loot.Bauble, baubles.Theft{ByUserId: 7606, FromMob: 4242, Zone: "Ashwick"}, time.Now().Add(-time.Hour)) {
		t.Fatal("fixture")
	}
	StolenBaubleGiven(h.thief, h.mark, loot)
	intoPocket(h.mark, loot)
	if res := startPocketAttempt(h.thief, h.mark, true); !res.Succeeded {
		t.Fatalf("the steal succeeds: %+v", res)
	}
	if r, _ := baubles.Get(loot.Bauble); !r.StolenGoods() || r.StolenFromMob != 4242 {
		t.Fatalf("still stolen from its first owner: %+v", r)
	}
}

// A mark with no bauble turns one up when the roll says so: named from
// the mark and the place (never the thief), pocket-sized, handed over with
// the rest of the loot.
func TestPickpocketTurnsUpAPocketSizedBauble(t *testing.T) {
	h := setupPocket(t, 9603, 7603)
	h.name(t, 0)
	pocketBaubleRoll = func(*rooms.Room) bool { return true }

	res := startPocketAttempt(h.thief, h.mark, true)
	got, ok := h.baubleCarried()
	if !res.Succeeded || !ok {
		t.Fatalf("a bauble among the loot: %+v", res)
	}
	rec, _ := baubles.Get(got.Bauble)
	if rec.Name != "Tarnished Brass Thimble" || rec.Source != baubles.SourcePickpocket || rec.WeightLbs > 1.0 || !rec.Stolen {
		t.Fatalf("named, pickpocketed, pocket-sized, stolen: %+v", rec)
	}
	if len(h.asked) != 1 || h.asked[0].Source != baubles.SourcePickpocket || h.asked[0].Victim != "a harried clerk" || h.asked[0].FinderUserId != 7603 {
		t.Fatalf("asked as a pickpocket of the mark: %+v", h.asked)
	}
	if said(h.thief, "Tarnished Brass Thimble") != 1 {
		t.Fatalf("in the success line: %q", h.thief.sent)
	}
}

// A bauble made for a pickpocket takes the pickpocket weights, not the
// search weights: stolen, so richer. Two sets that each allow one tier only
// show which was used.
func TestPickpocketBaubleTakesThePickpocketWeights(t *testing.T) {
	h := setupPocket(t, 9616, 7616)
	cfg := configs.GetConfig()
	cfg.Balance.BaubleTierWeightCheap, cfg.Balance.BaubleTierWeightAverage, cfg.Balance.BaubleTierWeightRare = 1, 0, 0
	cfg.Balance.BaublePickpocketTierWeightCheap, cfg.Balance.BaublePickpocketTierWeightAverage, cfg.Balance.BaublePickpocketTierWeightRare = 0, 0, 1
	configs.SetConfigForTest(t, cfg)
	h.name(t, 0)
	pocketBaubleRoll = func(*rooms.Room) bool { return true }

	res := startPocketAttempt(h.thief, h.mark, true)
	got, ok := h.baubleCarried()
	if !res.Succeeded || !ok {
		t.Fatalf("a bauble among the loot: %+v", res)
	}
	if len(h.asked) != 1 || h.asked[0].Tier != baubles.TierRare {
		t.Fatalf("asked for the pickpocket weights' tier: %+v", h.asked)
	}
	if rec, _ := baubles.Get(got.Bauble); rec.Tier != baubles.TierRare {
		t.Fatalf("minted at that tier: %+v", rec)
	}
}

// No bauble without the roll; none on a failed attempt, which is caught
// at the reveal.
func TestPickpocketWithoutLuckOrSuccessTurnsUpNothing(t *testing.T) {
	h := setupPocket(t, 9604, 7604)
	h.name(t, 0)
	before := baubles.Count()
	pocketBaubleRoll = func(*rooms.Room) bool { return false }
	if res := startPocketAttempt(h.thief, h.mark, true); !res.Succeeded || baubles.Count() != before {
		t.Fatalf("no roll, no bauble: %+v", res)
	}
	delete(h.thief.char.Cooldowns, skills.Skullduggery.String("steal"))
	pocketBaubleRoll = func(*rooms.Room) bool { return true }
	res := startPocketAttempt(h.thief, h.mark, false)
	if !res.Detected || baubles.Count() != before || len(h.asked) != 0 {
		t.Fatalf("a failed attempt makes nothing, and is caught: %+v", res)
	}
	if said(h.thief, "catches you in the act") != 1 {
		t.Fatalf("caught: %q", h.thief.sent)
	}
}

// Nothing is made in a companion's pocket: its name may be a player's.
func TestNoBaubleFromACompanionsPocket(t *testing.T) {
	h := setupPocket(t, 9605, 7605)
	h.name(t, 0)
	pocketBaubleRoll = func(*rooms.Room) bool { return true }
	h.mark.Character.Charm(1, -1, ``)
	startPocketAttempt(h.thief, h.mark, true)
	if _, ok := h.baubleCarried(); ok || len(h.asked) != 0 {
		t.Fatal("no bauble from a companion")
	}
}

// The naming runs during the pause. One a little slower than the pause is
// waited for (BaublePickpocketGraceSecs); one that never comes is given
// up at the grace's end, and the find is a generic trinket.
func TestPickpocketWaitsForTheNamingThenGivesUp(t *testing.T) {
	h := setupPocket(t, 9606, 7606)
	pocketBaubleRoll = func(*rooms.Room) bool { return true }
	h.name(t, 200*time.Millisecond)
	paused(50*time.Millisecond, 2*time.Second)
	startPocketAttempt(h.thief, h.mark, true)
	waitSettled(t)
	if got, ok := h.baubleCarried(); !ok {
		t.Fatal("delivered")
	} else if r, _ := baubles.Get(got.Bauble); r.Name != "Tarnished Brass Thimble" {
		t.Fatalf("a naming inside the grace is waited for: %+v", r)
	}

	h2 := setupPocket(t, 9607, 7607)
	pocketBaubleRoll = func(*rooms.Room) bool { return true }
	h2.name(t, time.Hour)
	paused(50*time.Millisecond, 100*time.Millisecond)
	start := time.Now()
	startPocketAttempt(h2.thief, h2.mark, true)
	waitSettled(t)
	if time.Since(start) > 2*time.Second {
		t.Fatal("the grace is not exceeded")
	}
	if got, ok := h2.baubleCarried(); !ok {
		t.Fatal("still delivered")
	} else if r, _ := baubles.Get(got.Bauble); r.Name != "Trinket" || r.Source != baubles.SourcePickpocket || r.WeightLbs > 1.0 {
		t.Fatalf("a generic, pocket-sized trinket: %+v", r)
	}
}

// A thief who walks off during the pause loses the chance: nothing taken,
// nothing caught. A bauble already named for it stays in the mark's pocket,
// and the next successful attempt takes that one.
func TestPickpocketChanceLostWhenTheThiefLeaves(t *testing.T) {
	h := setupPocket(t, 9608, 7608)
	h.name(t, 0)
	pocketBaubleRoll = func(*rooms.Room) bool { return true }
	paused(150*time.Millisecond, time.Second)
	gold := h.thief.char.Gold

	util.LockMud()
	startPocketAttempt(h.thief, h.mark, true)
	h.thief.room = newSearchTestRoom(9699) // walks away
	util.UnlockMud()
	waitSettled(t)
	if h.thief.char.Gold != gold || said(h.thief, "successfully steal") != 0 || said(h.thief, "lose your chance") != 1 {
		t.Fatalf("nothing taken: %q", h.thief.sent)
	}
	if b, ok := carriedBauble(h.mark); !ok {
		t.Fatal("the named bauble stays in the mark's pocket")
	} else if r, _ := baubles.Get(b.Bauble); r.Name != "Tarnished Brass Thimble" {
		t.Fatalf("named: %+v", r)
	}

	h.thief.room = h.room
	runPocketAttempt = resolvePocketInLine
	pocketBaubleRoll = func(*rooms.Room) bool { t.Fatal("the mark has one now"); return false }
	startPocketAttempt(h.thief, h.mark, true)
	if _, ok := h.baubleCarried(); !ok {
		t.Fatal("the next attempt takes it")
	}
}

// Copyover in the middle of the pause: the flush reveals it at once, and
// the attempt's goroutine stands down, so it resolves exactly once.
func TestFlushRevealsAPickpocketInItsPause(t *testing.T) {
	h := setupPocket(t, 9609, 7609)
	paused(time.Minute, 0)
	util.LockMud()
	startPocketAttempt(h.thief, h.mark, true)
	n := FlushPocketAttempts()
	util.UnlockMud()
	if n != 1 {
		t.Fatalf("revealed by the flush: %d", n)
	}
	waitSettled(t)
	if said(h.thief, "successfully steal") != 1 {
		t.Fatalf("exactly once: %q", h.thief.sent)
	}
}

// origPocketThief is the production lookup, taken before the test init
// replaces it.
var origPocketThief = pocketThief

// Training comes with the reveal, never before it (a skill-up line then
// would give the roll away). A failed attempt is caught at the reveal, not
// before, and walking off does not dodge it (owner ruling 2026-09-29).
func TestPickpocketAwardsAndCatchesAtTheReveal(t *testing.T) {
	h := setupPocket(t, 9610, 7610)
	paused(100*time.Millisecond, 0)
	util.LockMud()
	startPocketAttempt(h.thief, h.mark, false)
	early := len(h.thief.awards)
	caughtEarly := said(h.thief, "catches you in the act")
	util.UnlockMud()
	if early != 0 || caughtEarly != 0 {
		t.Fatalf("nothing before the reveal: %d awards, caught %d", early, caughtEarly)
	}
	waitSettled(t)
	if len(h.thief.awards) != 1 || h.thief.awards[0].won || said(h.thief, "catches you in the act") != 1 {
		t.Fatalf("caught, and a lost award, at the reveal: %+v %q", h.thief.awards, h.thief.sent)
	}

	h2 := setupPocket(t, 9611, 7611)
	seedPocketRooms(t, h2.room)
	crimes := stubPocketCrime(t)
	paused(100*time.Millisecond, 0)
	util.LockMud()
	startPocketAttempt(h2.thief, h2.mark, false)
	h2.thief.room = newSearchTestRoom(9698) // walks off
	util.UnlockMud()
	waitSettled(t)
	if len(h2.thief.awards) != 1 || h2.thief.awards[0].won || said(h2.thief, "felt your hand") != 1 ||
		said(h2.thief, "lose your chance") != 0 || len(*crimes) != 1 {
		t.Fatalf("walked off, still caught, trained on the loss: %+v %q crimes=%v", h2.thief.awards, h2.thief.sent, *crimes)
	}
}

// stubPocketCrime records the crimes a pickpocket caught away from the mark
// raises (pocketCrime), instead of reaching the faction books.
func stubPocketCrime(t *testing.T) *[]int {
	t.Helper()
	got := &[]int{}
	orig := pocketCrime
	pocketCrime = func(userId int, m *mobs.Mob, room *rooms.Room, _ messaging.SightDecision) {
		*got = append(*got, userId)
	}
	t.Cleanup(func() { pocketCrime = orig })
	return got
}

// seedPocketRooms makes rooms loadable, as real ones are: the reveal loads
// the theft room and the mark's room by ID (rooms.LoadRoom).
func seedPocketRooms(t *testing.T, rs ...*rooms.Room) {
	t.Helper()
	m := map[int]*rooms.Room{}
	for _, r := range rs {
		m[r.RoomId] = r
	}
	t.Cleanup(rooms.SeedRoomsForTest(m, nil))
}

// A failed roll is caught however the pause ends (owner ruling 2026-09-29).
// A thief who walked off or logged out, or whose mark moved, is caught away
// from the mark: the mark cries thief in its own room and the crime is
// recorded, and nobody is attacked. One still beside it is caught in the
// act. A mark that has gone catches nobody.
func TestPickpocketFailedRollIsCaughtHoweverThePauseEnds(t *testing.T) {
	cases := map[string]struct {
		before func(h *pocketHarness, p *pocketAttempt)
		caught bool
		crimes int    // raised away from the mark (pocketCrime)
		told   string // a line the thief is told exactly once
	}{
		`walked off`: {func(h *pocketHarness, p *pocketAttempt) { h.thief.room = newSearchTestRoom(9696) }, true, 1, "felt your hand"},
		`logged out`: {func(h *pocketHarness, p *pocketAttempt) { p.actor = nil }, true, 1, "You attempt to pick"},
		`mark moved`: {func(h *pocketHarness, p *pocketAttempt) { h.mark.Character.RoomId = 9695 }, true, 1, "felt your hand"},
		`still here`: {func(h *pocketHarness, p *pocketAttempt) {}, true, 0, "catches you in the act"},
		`mark gone`:  {func(h *pocketHarness, p *pocketAttempt) { mobs.SetInstanceForTest(h.mark.InstanceId, nil) }, false, 0, "lose your chance"},
	}
	for name, c := range cases {
		c := c
		h := setupPocket(t, 9617, 7617)
		seedPocketRooms(t, h.room, newSearchTestRoom(9695))
		crimes := stubPocketCrime(t)
		runPocketAttempt = func(p *pocketAttempt) StealResult {
			c.before(h, p)
			return resolvePocketInLine(p)
		}
		res := startPocketAttempt(h.thief, h.mark, false)
		if res.Detected != c.caught || len(*crimes) != c.crimes || said(h.thief, c.told) != 1 {
			t.Errorf("%v: detected %v (want %v), crimes %v (want %d), told %q", name, res.Detected, c.caught, *crimes, c.crimes, h.thief.sent)
		}
		if name == `logged out` && (len(h.thief.sent) != 1 || len(h.thief.awards) != 0) {
			t.Errorf("logged out: told and trained nothing after the attempt line: %q %+v", h.thief.sent, h.thief.awards)
		}
	}
}

// A failed roll whose thief and mark have both left the theft room, and are
// together somewhere else at the reveal, is not a catch in the act: the
// crime belongs to the theft room, and the bystanders where they meet saw
// no theft, so none of them identifies the thief. The mark, which can see
// the thief now, still cries thief and attacks.
func TestPickpocketCaughtTogetherElsewhereIsNotInTheAct(t *testing.T) {
	h := setupPocket(t, 9618, 7618)
	seedTheftFaction(t)
	h.room.Lamp = rooms.LampPtr(90)
	h.room.AddMob(h.mark.InstanceId)
	h.mark.MobId = 9718
	h.mark.Groups = []string{"thornwall_citizens"}
	elsewhere := &rooms.Room{RoomId: 9694, Lamp: rooms.LampPtr(90)}
	seedPocketRooms(t, h.room, elsewhere)
	bystander := newStealTestMob(9890, 0, 100)
	bystander.MobId = 9790
	bystander.Character.RoomId = elsewhere.RoomId
	bystander.Groups = []string{"thornwall_citizens"}
	mobs.SetInstanceForTest(bystander.InstanceId, bystander)
	t.Cleanup(func() { mobs.SetInstanceForTest(bystander.InstanceId, nil) })
	elsewhere.AddMob(bystander.InstanceId)
	events.DrainQueuedInputsForTest(h.mark.InstanceId)

	runPocketAttempt = func(p *pocketAttempt) StealResult {
		h.thief.room = elsewhere
		h.room.RemoveMob(h.mark.InstanceId)
		h.mark.Character.RoomId = elsewhere.RoomId
		elsewhere.AddMob(h.mark.InstanceId)
		hideRhetoricActor(t, h.thief.GetCharacter()) // hid during the pause
		return resolvePocketInLine(p)
	}
	res := startPocketAttempt(h.thief, h.mark, false)
	if !res.Detected || said(h.thief, "catches you in the act") != 0 || said(h.thief, "felt your hand") != 1 {
		t.Fatalf("caught, but not in the act: %+v %q", res, h.thief.sent)
	}
	if h.thief.GetCharacter().Awareness.State() == awareness.Hidden {
		t.Fatal("the mark beside the thief reveals them before it attacks")
	}
	got := crimes.AllForFaction("thornwall_citizens", false)
	if len(got) != 1 {
		t.Fatalf("one theft recorded: %+v", got)
	}
	if got[0].RoomId != h.room.RoomId || got[0].HadExternalWitness || got[0].Perpetrator.Type != crimes.PerpPlayer {
		t.Fatalf("in the theft room, the mark its only witness: %+v", *got[0])
	}
	subject := knowledge.PlayerSubject(7618)
	if knowledge.Get(int(bystander.MobId), subject) != nil {
		t.Fatal("the bystander where they met saw no theft, and learns nothing")
	}
	if r := knowledge.Get(int(h.mark.MobId), subject); r == nil || len(r.CrimesWitnessed) != 1 {
		t.Fatalf("the mark learns who robbed it: %+v", r)
	}
	attacks := 0
	for _, in := range events.DrainQueuedInputsForTest(h.mark.InstanceId) {
		if in == "attack @7618" {
			attacks++
		}
	}
	if attacks != 1 {
		t.Fatalf("the mark, beside the thief, attacks: %d", attacks)
	}
}

// The mark's sight of the thief is taken at the attempt, when it felt the
// hand, not at the reveal. A thief whose own carried light was all that lit
// the theft room, and who walked off with it before the reveal, was seen
// by that light: the mark still names them.
func TestPickpocketMarkJudgedByTheLightAtTheAttempt(t *testing.T) {
	const torchId = 9731
	h := setupPocket(t, 9619, 7619)
	seedTheftFaction(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		torchId: {ConditionId: torchId, Name: "Test Torch", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 56}}},
	}))
	thiefUser := users.NewTestUser(7619, "nimble", "Nimble", 97619)
	thiefUser.Character.Conditions.AddCondition(torchId, false)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7619: thiefUser}))

	noSky := 0.0
	h.room.SkyLight = &noSky
	h.room.AddPlayer(7619)
	h.room.AddMob(h.mark.InstanceId)
	h.mark.MobId = 9719
	h.mark.Groups = []string{"thornwall_citizens"}
	elsewhere := newSearchTestRoom(9693)
	seedPocketRooms(t, h.room, elsewhere)
	if !messaging.CanSeeClearly(&h.mark.Character, h.room) {
		t.Fatalf("fixture: the thief's torch lights the theft room (%d)", h.room.LightLevel())
	}

	runPocketAttempt = func(p *pocketAttempt) StealResult {
		h.thief.room = elsewhere // walks off, and the torch with them
		h.room.RemovePlayer(7619)
		if messaging.CanSeeShapes(&h.mark.Character, h.room) {
			t.Fatalf("fixture: the theft room is dark once the thief has gone (%d)", h.room.LightLevel())
		}
		return resolvePocketInLine(p)
	}
	if res := startPocketAttempt(h.thief, h.mark, false); !res.Detected {
		t.Fatalf("caught: %+v", res)
	}
	got := crimes.AllForFaction("thornwall_citizens", false)
	if len(got) != 1 {
		t.Fatalf("one theft recorded: %+v", got)
	}
	if got[0].Perpetrator.Type != crimes.PerpPlayer || got[0].Perpetrator.Id != 7619 {
		t.Fatalf("the mark names the thief it saw by the torch: %+v", *got[0])
	}
	if r := knowledge.Get(int(h.mark.MobId), knowledge.PlayerSubject(7619)); r == nil {
		t.Fatal("and learns who robbed it")
	}
}

// A thief downed or dead beside the mark at the reveal is still caught, but
// the mark does not attack them: there is nobody left to fight. A thief on
// their feet is attacked, as ever.
func TestPickpocketMarkDoesNotAttackAFallenThief(t *testing.T) {
	for name, c := range map[string]struct {
		fall    func(h *pocketHarness)
		attacks int
	}{
		`standing`: {func(h *pocketHarness) {}, 1},
		`downed`:   {func(h *pocketHarness) { h.thief.char.Health = 0 }, 0},
		`dead`: {func(h *pocketHarness) {
			if err := h.thief.char.Life.TransitionToDead(life.DeadData{}, state.TransitionReason{}); err != nil {
				t.Fatal(err)
			}
		}, 0},
	} {
		h := setupPocket(t, 9620, 7620)
		events.DrainQueuedInputsForTest(h.mark.InstanceId)
		runPocketAttempt = func(p *pocketAttempt) StealResult {
			c.fall(h)
			return resolvePocketInLine(p)
		}
		res := startPocketAttempt(h.thief, h.mark, false)
		if !res.Detected || said(h.thief, "catches you in the act") != 1 {
			t.Errorf("%s: caught in the act: %+v %q", name, res, h.thief.sent)
		}
		attacks := 0
		for _, in := range events.DrainQueuedInputsForTest(h.mark.InstanceId) {
			if in == "attack @7620" {
				attacks++
			}
		}
		if attacks != c.attacks {
			t.Errorf("%s: %d attacks, want %d", name, attacks, c.attacks)
		}
	}
}

// A mark that has gone, or died, by the reveal: the chance is lost, and the
// thief is told so.
func TestPickpocketMarkGoneLosesTheChance(t *testing.T) {
	for name, gone := range map[string]func(h *pocketHarness){
		`moved`: func(h *pocketHarness) { h.mark.Character.RoomId = 9697 },
		`gone`:  func(h *pocketHarness) { mobs.SetInstanceForTest(h.mark.InstanceId, nil) },
	} {
		h := setupPocket(t, 9612, 7612)
		gold := h.thief.char.Gold
		runPocketAttempt = func(p *pocketAttempt) StealResult {
			gone(h)
			return resolvePocketInLine(p)
		}
		res := startPocketAttempt(h.thief, h.mark, true)
		if res.Succeeded || h.thief.char.Gold != gold || said(h.thief, "lose your chance at") != 1 {
			t.Fatalf("%s: chance lost, and said: %+v %q", name, res, h.thief.sent)
		}
	}
}

// One pickpocket at a time, whatever the cooldown.
func TestOnePickpocketAtATime(t *testing.T) {
	h := setupPocket(t, 9613, 7613)
	paused(time.Minute, 0)
	util.LockMud()
	startPocketAttempt(h.thief, h.mark, true)
	delete(h.thief.char.Cooldowns, skills.Skullduggery.String("steal"))
	res := Steal(h.thief, StealOptions{TargetMobInstanceId: h.mark.InstanceId})
	FlushPocketAttempts()
	util.UnlockMud()
	waitSettled(t)
	if res.Reason != "busy" || said(h.thief, "still in someone's pocket") != 1 {
		t.Fatalf("a second attempt waits for the first: %+v", res)
	}
}

// A mark that was ever charmed keeps the name a player may have given it:
// nothing is made in its pocket.
func TestNoBaubleFromAFormerCompanionsPocket(t *testing.T) {
	h := setupPocket(t, 9614, 7614)
	h.name(t, 0)
	pocketBaubleRoll = func(*rooms.Room) bool { return true }
	h.mark.Character.EverCharmed = true
	startPocketAttempt(h.thief, h.mark, true)
	if _, ok := h.baubleCarried(); ok || len(h.asked) != 0 {
		t.Fatal("no bauble from a former companion")
	}
}

// The production lookup finds the thief where they are now, or not at all.
func TestPocketThiefIsLookedUpAgain(t *testing.T) {
	now := &rooms.Room{RoomId: 9615}
	restoreRooms := rooms.SeedRoomsForTest(map[int]*rooms.Room{9615: now}, nil)
	defer restoreRooms()
	u := users.NewUserRecord(7615, 0)
	u.Character = characters.New()
	u.Character.RoomId = 9615
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{7615: u})
	defer restoreUsers()

	a, ok := origPocketThief(&pocketAttempt{userId: 7615})
	if !ok || a.GetUserId() != 7615 || a.GetRoom() == nil || a.GetRoom().RoomId != 9615 {
		t.Fatalf("found where they are: %v %v", a, ok)
	}
	if _, ok := origPocketThief(&pocketAttempt{userId: 7616}); ok {
		t.Fatal("an absent thief is not found")
	}
}

// Through the steal command itself: the roll is made, but nothing is
// awarded until the reveal (whatever the roll).
func TestStealAwardsAtTheRevealNotTheRoll(t *testing.T) {
	h := setupPocket(t, 9616, 7616)
	paused(100*time.Millisecond, 0)
	util.LockMud()
	res := Steal(h.thief, StealOptions{TargetMobInstanceId: h.mark.InstanceId})
	early := len(h.thief.awards)
	util.UnlockMud()
	if !res.Pending || early != 0 {
		t.Fatalf("pending, nothing awarded at the roll: %+v, %d awards", res, early)
	}
	waitSettled(t)
	if len(h.thief.awards) != 1 {
		t.Fatalf("one award, at the reveal: %d", len(h.thief.awards))
	}
}

// A pocket bauble whose naming never came back is drawn from the corpus's
// pocket pool.
func TestPickpocketUnnamedBaubleDrawsFromTheCorpus(t *testing.T) {
	loadTestCorpus(t)
	p := &pocketAttempt{
		req:   baubles.GenRequest{Tier: baubles.TierCheap, Source: baubles.SourcePickpocket, Place: baubles.Place{Zone: "nowhere"}},
		randn: func(int) int { return 0 },
	}
	res, named := p.naming()
	if named || res.Generator != baubles.GeneratorCorpus || res.Reply.Name != "Brass Snuff Spoon" {
		t.Fatalf("got %+v (named %v)", res, named)
	}
}

// A mark asleep at the attempt saw nothing (owner ruling 2026-09-29): a
// failed roll still wakes it to the loss, but when the thief has walked
// off by the reveal it cannot say who robbed it, however well lit the
// theft room was.
func TestPickpocketSleepingMarkSawNothing(t *testing.T) {
	h := setupPocket(t, 9621, 7621)
	seedTheftFaction(t)
	h.room.Lamp = rooms.LampPtr(90)
	h.room.AddMob(h.mark.InstanceId)
	h.mark.MobId = 9721
	h.mark.Groups = []string{"thornwall_citizens"}
	elsewhere := newSearchTestRoom(9695)
	seedPocketRooms(t, h.room, elsewhere)
	t.Cleanup(seedSleepCondition(t))
	if err := h.mark.Character.AddCondition(sleepConditionId, true); err != nil {
		t.Fatal(err)
	}
	if !messaging.CanSeeClearly(nil, h.room) || messaging.ParticipantSight(&h.mark.Character, h.room) != messaging.SightFull {
		t.Fatalf("fixture: the theft room is lit (%d)", h.room.LightLevel())
	}

	runPocketAttempt = func(p *pocketAttempt) StealResult {
		h.thief.room = elsewhere
		return resolvePocketInLine(p)
	}
	if res := startPocketAttempt(h.thief, h.mark, false); !res.Detected {
		t.Fatalf("caught: %+v", res)
	}
	got := crimes.AllForFaction("thornwall_citizens", false)
	if len(got) != 1 {
		t.Fatalf("one theft recorded: %+v", got)
	}
	if got[0].Perpetrator.Type == crimes.PerpPlayer {
		t.Fatalf("a mark asleep at the attempt names nobody: %+v", *got[0])
	}
	if r := knowledge.Get(int(h.mark.MobId), knowledge.PlayerSubject(7621)); r != nil {
		t.Fatalf("and learns nothing of the thief: %+v", r)
	}
}

// A player's pickpocket of a mob spends the skullduggery cooldown at the
// roll, not at the reveal: the steal returns pending with the cooldown
// already armed for the configured 60 seconds, and a second try is refused.
// The won/lost cooldown tests in steal_cooldown_test.go use a mob thief,
// whose outcome is immediate; this is the player path through the pause.
func TestPickpocketByAPlayerArmsTheStealCooldown(t *testing.T) {
	h := setupPocket(t, 9617, 7617)
	pinStealCooldown(t)
	paused(50*time.Millisecond, 0)
	util.LockMud()
	res := Steal(h.thief, StealOptions{TargetMobInstanceId: h.mark.InstanceId})
	armed := h.thief.char.GetCooldown(stealKey())
	util.UnlockMud()
	waitSettled(t)
	if !res.Pending {
		t.Fatalf("a player's pickpocket pauses: %+v", res)
	}
	if armed != 15 {
		t.Fatalf("cooldown at the roll = %d rounds, want 15 (60 real seconds at 4-second rounds)", armed)
	}
	util.LockMud()
	again := Steal(h.thief, StealOptions{TargetMobInstanceId: h.mark.InstanceId})
	util.UnlockMud()
	if !again.OnCooldown {
		t.Fatalf("a second pickpocket inside the cooldown is refused: %+v", again)
	}
}

package actions

import (
	"os"
	"path/filepath"
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
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// Stolen baubles after the theft (docs/baubles Phase 6c): heat and fences
// at the sale, and the owner's side, recognition and returns. Built on the
// sale harness (sell_test.go, sell_bauble_test.go): room 1, a legacy
// merchant (template 2, instance 301) and a player seller (user 1).

// stolenTestNow is the tests' fixed clock.
var stolenTestNow = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

// pinStolenClock sets every clock the stolen-bauble code reads.
func pinStolenClock(t *testing.T, now time.Time) {
	t.Helper()
	origSale, origStolen, origShelf := baubleNowForSale, stolenNow, shops.ShelfNow
	baubleNowForSale = func() time.Time { return now }
	stolenNow = func() time.Time { return now }
	shops.ShelfNow = func() time.Time { return now }
	t.Cleanup(func() { baubleNowForSale, stolenNow, shops.ShelfNow = origSale, origStolen, origShelf })
}

// stolenBauble makes a bauble stolen by userId from mob template fromMob, at.
func stolenBauble(t *testing.T, name string, noun string, value int, fromMob int, userId int, at time.Time) items.Item {
	t.Helper()
	it := newBauble(t, name, noun, value, baubles.StatusReady)
	require.True(t, baubles.MarkStolen(it.Bauble, baubles.Theft{ByUserId: userId, RoomId: 1, Zone: "TestZone", FromMob: fromMob, FromName: "Merchant"}, at))
	return it
}

// ─── Heat and fences ────────────────────────────────────────────────────────

func TestStolenBauble_HonestMerchantRefusesAHotOne(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	hot := stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenTestNow.Add(-time.Hour))
	require.True(t, char.StoreItem(hot))

	offer := BaubleOfferFrom(hot, merchantInstance())
	assert.Equal(t, 0, offer.Price)
	assert.Equal(t, baubleSayHot, offer.Refusal, "the refusal names why, and hints at a fence")

	res := Sell(seller, SellOptions{ItemName: "thimble", Quantity: 1})
	assert.Equal(t, 0, res.Sold)
	assert.Equal(t, 0, char.Gold)
	_, still := char.FindInBackpack("thimble")
	assert.True(t, still)
}

// Three days after the theft the bauble has cooled: an honest merchant buys
// it at the honest price. One second short of that it is still hot.
func TestStolenBauble_HeatLastsThreeDays(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()

	stolenAt := stolenTestNow
	it := stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenAt)

	cfg := configs.GetConfig()
	cfg.Balance.BaubleStolenHeatHours = 72
	configs.SetConfigForTest(t, cfg)

	pinStolenClock(t, stolenAt.Add(72*time.Hour-time.Second))
	assert.Equal(t, baubleSayHot, BaubleOfferFrom(it, merchantInstance()).Refusal, "still hot a second short of three days")

	pinStolenClock(t, stolenAt.Add(72*time.Hour))
	assert.Equal(t, 6, BaubleOfferFrom(it, merchantInstance()).Price, "cooled: the honest price, value times ShopBuyRatio")
}

// A fence pays BaubleFenceBuyPct (60%) of the value for any stolen bauble,
// hot or cold, and the honest price for an honest one.
func TestStolenBauble_FencePaysSixtyPercentForStolenGoods(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)
	merchantInstance().Groups = []string{`merchant`, `Fence`}
	require.True(t, IsFence(merchantInstance()), "group match ignores case")

	hot := stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenTestNow.Add(-time.Hour))
	cold := stolenBauble(t, "Bone Dice", "dice", 12, 99, 1, stolenTestNow.Add(-30*24*time.Hour))
	honest := newBauble(t, "Painted Wooden Horse", "horse", 12, baubles.StatusReady)

	assert.Equal(t, 8, BaubleOfferFrom(hot, merchantInstance()).Price, "60% of 12, rounded up")
	assert.Equal(t, 8, BaubleOfferFrom(cold, merchantInstance()).Price, "stolen is stolen to a fence, hot or not")
	assert.Equal(t, 6, BaubleOfferFrom(honest, merchantInstance()).Price, "honest goods at the honest price")
	assert.Greater(t, FencePrice(12), BaublePrice(12), "a fence pays more for stolen goods than an honest merchant pays for anything")

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(hot))
	purse := merchantInstance().Character.Gold
	res := Sell(seller, SellOptions{ItemName: "thimble", Quantity: 1})
	require.Equal(t, 1, res.Sold, "res=%+v", res)
	assert.Equal(t, 8, char.Gold)
	assert.Equal(t, purse-8, merchantInstance().Character.Gold, "a fence who keeps a shop pays from its own purse")
	rec, _ := baubles.Get(hot.Bauble)
	assert.Equal(t, baubles.StatusSold, rec.Status)
	assert.Equal(t, 8, rec.SoldValue)
}

// Given back to its owner, a bauble is no longer stolen goods: a fence
// pays the honest price for it.
func TestStolenBauble_AReturnEndsTheFencePremium(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)
	merchantInstance().Groups = []string{`fence`}

	it := stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenTestNow.Add(-time.Hour))
	require.Equal(t, 8, BaubleOfferFrom(it, merchantInstance()).Price)
	require.True(t, baubles.MarkReturned(it.Bauble, 1, nil, stolenTestNow))
	assert.Equal(t, 6, BaubleOfferFrom(it, merchantInstance()).Price)
}

// A shop that buys no trinkets says so, even for a hot one: the stolen-goods
// line (and its hint at a fence) is only for a merchant that would
// otherwise buy it.
func TestStolenBauble_ATrinketShyShopSaysSoFirst(t *testing.T) {
	seedBaubleSale(t)
	pinStolenClock(t, stolenTestNow)
	hot := stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenTestNow.Add(-time.Hour))
	smith := &shops.ShopInventory{CraftSupport: `smithing`}
	assert.Equal(t, baubleSayNotBuyer, baubleOfferFor(hot, smith, false, "TestZone").Refusal)
	general := &shops.ShopInventory{CraftSupport: `general`}
	assert.Equal(t, baubleSayHot, baubleOfferFor(hot, general, false, "TestZone").Refusal)
}

// With an honest merchant and a fence in one room, a stolen bauble goes to
// the fence (the better offer) and an honest one to whichever pays more.
func TestStolenBauble_SoldToTheBestOfferInTheRoom(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)

	fence := seedFence(t, 1000)
	room := rooms.LoadRoom(1)

	cold := stolenBauble(t, "Bone Dice", "dice", 12, 99, 1, stolenTestNow.Add(-30*24*time.Hour))
	m, _ := resolveMerchant(room, cold, true)
	require.NotNil(t, m)
	assert.Equal(t, fence.InstanceId, m.InstanceId, "the fence pays 8 against the merchant's 6")

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(cold))
	res := Sell(seller, SellOptions{ItemName: "dice", Quantity: 1})
	require.Equal(t, 1, res.Sold)
	assert.Equal(t, 8, char.Gold)
}

// A bauble is hot only in the area it was stolen in: an honest merchant
// anywhere else buys it at once, at the honest price. A city of several
// zones (BaubleHeatAreas) is one area.
func TestStolenBauble_HotOnlyWhereItWasStolen(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)
	cfg := configs.GetConfig()
	cfg.Balance.BaubleHeatAreas = map[string][]string{`Test City`: {`TestZone`, `Test Docks`}}
	configs.SetConfigForTest(t, cfg)

	steal := func(zone string) items.Item {
		it := newBauble(t, "Tarnished Brass Thimble", "thimble", 12, baubles.StatusReady)
		require.True(t, baubles.MarkStolen(it.Bauble, baubles.Theft{ByUserId: 1, RoomId: 9, Zone: zone, FromMob: 99}, stolenTestNow.Add(-time.Hour)))
		return it
	}
	assert.Equal(t, 6, BaubleOfferFrom(steal("Greenford"), merchantInstance()).Price, "stolen in another town: sells here at the honest price")
	assert.Equal(t, baubleSayHot, BaubleOfferFrom(steal("Test Docks"), merchantInstance()).Refusal, "another quarter of the same city: still hot")
	assert.Equal(t, baubleSayHot, BaubleOfferFrom(steal("testzone"), merchantInstance()).Refusal, "zone names match whatever their case")
	assert.Equal(t, baubleSayHot, BaubleOfferFrom(steal(""), merchantInstance()).Refusal, "a theft whose zone is unknown is hot everywhere")

	// Picking a merchant for a sale reads the room's zone too: nobody here
	// will take one hot here, and the honest merchant takes one hot elsewhere.
	room := rooms.LoadRoom(1)
	m, _ := resolveMerchant(room, steal("Test Docks"), true)
	assert.Nil(t, m, "no willing merchant for a bauble hot here")
	m, _ = resolveMerchant(room, steal("Greenford"), true)
	assert.NotNil(t, m)

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(steal("Greenford")))
	res := Sell(seller, SellOptions{ItemName: "thimble", Quantity: 1})
	require.Equal(t, 1, res.Sold, "res=%+v", res)
	assert.Equal(t, 6, char.Gold)
}

func TestStolenBauble_FenceGroupsComeFromConfig(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	merchantInstance().Groups = []string{`smuggler`}
	assert.False(t, IsFence(merchantInstance()))

	cfg := configs.GetConfig()
	cfg.Balance.BaubleFenceGroups = configs.ConfigSliceString{`fence`, `smuggler`}
	configs.SetConfigForTest(t, cfg)
	assert.True(t, IsFence(merchantInstance()))
	assert.False(t, IsFence(nil))
}

// `sell all` at an honest merchant sells the honest bauble and quietly
// keeps the hot one.
func TestStolenBauble_SellAllLeavesTheHotOne(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenTestNow.Add(-time.Hour))))
	require.True(t, char.StoreItem(newBauble(t, "Painted Wooden Horse", "horse", 12, baubles.StatusReady)))

	res := Sell(seller, SellOptions{SellAllSellable: true})
	assert.Equal(t, 1, res.Sold)
	assert.Equal(t, 6, char.Gold)
	_, still := char.FindInBackpack("thimble")
	assert.True(t, still, "the hot one is kept")
}

// ─── Recognition ────────────────────────────────────────────────────────────

type recognitionHarness struct {
	room   *rooms.Room
	thief  Actor
	caught []*mobs.Mob
	rolls  int
}

// setupRecognition puts the seller (user 1) in room 1 with the merchant
// (template 2) and makes the recognition roll win (or lose).
func setupRecognition(t *testing.T, wins bool) *recognitionHarness {
	t.Helper()
	seedBaubleSale(t)
	t.Cleanup(seedSellRoom(t))
	t.Cleanup(seedSellMerchant(t, 1000))
	pinStolenClock(t, stolenTestNow)

	h := &recognitionHarness{room: rooms.LoadRoom(1)}
	h.thief = newSellerActor(t, true)
	origCarriers, origRoll, origCaught := stolenCarriers, recognitionRoll, stolenCaught
	stolenCarriers = func(*rooms.Room, int) []Actor { return []Actor{h.thief} }
	recognitionRoll = func(owner, carrier *characters.Character, room *rooms.Room) bool {
		h.rolls++
		return wins
	}
	stolenCaught = func(_ Actor, m *mobs.Mob, _ *rooms.Room) { h.caught = append(h.caught, m) }
	t.Cleanup(func() { stolenCarriers, recognitionRoll, stolenCaught = origCarriers, origRoll, origCaught })
	return h
}

// The owner recognises its hot bauble on the thief: a catch, once per
// theft. A fresh theft of the same bauble can be recognised again.
func TestStolenBauble_OwnerRecognisesItOnceATheft(t *testing.T) {
	h := setupRecognition(t, true)
	it := stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 2, 1, stolenTestNow.Add(-time.Hour))
	require.True(t, h.thief.GetCharacter().StoreItem(it))

	recognizeIn(h.room, 1, 0) // the thief walks in
	require.Len(t, h.caught, 1)
	assert.Equal(t, merchantInstance(), h.caught[0], "caught by the owner")
	rec, _ := baubles.Get(it.Bauble)
	assert.True(t, rec.RecognizedSinceTheft())

	recognizeIn(h.room, 1, 0)
	assert.Len(t, h.caught, 1, "once per theft")

	pinStolenClock(t, stolenTestNow.Add(2*time.Minute))
	require.True(t, baubles.MarkStolen(it.Bauble, baubles.Theft{ByUserId: 1, RoomId: 1, FromMob: 2}, stolenTestNow.Add(time.Minute)))
	recognizeIn(h.room, 0, 301) // the owner walks in
	assert.Len(t, h.caught, 2, "stolen again, recognisable again")
}

func TestStolenBauble_NoRecognitionWithoutCause(t *testing.T) {
	cases := map[string]func(t *testing.T, h *recognitionHarness) items.Item{
		"the roll is lost": nil, // handled below
		"cold": func(t *testing.T, h *recognitionHarness) items.Item {
			return stolenBauble(t, "Bone Dice", "dice", 12, 2, 1, stolenTestNow.Add(-8*24*time.Hour))
		},
		"someone else's": func(t *testing.T, h *recognitionHarness) items.Item {
			return stolenBauble(t, "Bone Dice", "dice", 12, 77, 1, stolenTestNow.Add(-time.Hour))
		},
		"never stolen": func(t *testing.T, h *recognitionHarness) items.Item {
			return newBauble(t, "Bone Dice", "dice", 12, baubles.StatusReady)
		},
		"owner asleep": func(t *testing.T, h *recognitionHarness) items.Item {
			t.Cleanup(seedSleepCondition(t))
			require.NoError(t, merchantInstance().Character.AddCondition(sleepConditionId, true))
			return stolenBauble(t, "Bone Dice", "dice", 12, 2, 1, stolenTestNow.Add(-time.Hour))
		},
		"owner blinded": func(t *testing.T, h *recognitionHarness) items.Item {
			owner := &merchantInstance().Character
			owner.Perception = characters.New().Perception
			require.NoError(t, owner.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
			return stolenBauble(t, "Bone Dice", "dice", 12, 2, 1, stolenTestNow.Add(-time.Hour))
		},
		"a dark room": func(t *testing.T, h *recognitionHarness) items.Item {
			h.room.SkyLight, h.room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)
			require.Equal(t, messaging.SightNone, messaging.ParticipantSight(&merchantInstance().Character, h.room), "fixture: pitch dark")
			return stolenBauble(t, "Bone Dice", "dice", 12, 2, 1, stolenTestNow.Add(-time.Hour))
		},
		"owner moved on": func(t *testing.T, h *recognitionHarness) items.Item {
			merchantInstance().Character.RoomId = 2
			return stolenBauble(t, "Bone Dice", "dice", 12, 2, 1, stolenTestNow.Add(-time.Hour))
		},
		"carried by someone other than the thief": func(t *testing.T, h *recognitionHarness) items.Item {
			return stolenBauble(t, "Bone Dice", "dice", 12, 2, 42, stolenTestNow.Add(-time.Hour))
		},
		"given back since": func(t *testing.T, h *recognitionHarness) items.Item {
			it := stolenBauble(t, "Bone Dice", "dice", 12, 2, 1, stolenTestNow.Add(-time.Hour))
			require.True(t, baubles.MarkReturned(it.Bauble, 1, nil, stolenTestNow.Add(-time.Minute)))
			return it
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			wins := name != "the roll is lost"
			h := setupRecognition(t, wins)
			var it items.Item
			if mk == nil {
				it = stolenBauble(t, "Bone Dice", "dice", 12, 2, 1, stolenTestNow.Add(-time.Hour))
			} else {
				it = mk(t, h)
			}
			require.True(t, h.thief.GetCharacter().StoreItem(it))
			recognizeIn(h.room, 0, 301)
			assert.Empty(t, h.caught)
			rec, _ := baubles.Get(it.Bauble)
			assert.False(t, rec.RecognizedSinceTheft())
		})
	}
}

// In dim light an owner who sees only shapes still knows its own bauble on
// a figure (the crime is then recorded against an unknown perpetrator by
// thiefCaught's witness count, like any theft seen only as shapes).
//
// The real thiefCaught runs, so the crime is read back from the log: its
// perpetrator is unknown. A clear-sighted observer (nightvision) reads the
// room line exactly as sent, so "a figure" there comes from the owner's
// dim sight, not from the observer's own.
func TestStolenBauble_RecognisedInDimLight(t *testing.T) {
	const observerId = 7401
	const nightEyes = 7402
	h := setupRecognition(t, true)
	configureStagedAdmissionFaction(t) // the thornwall_citizens faction, temp crime log
	merchantInstance().Groups = []string{`thornwall_citizens`}
	stolenCaught = func(a Actor, m *mobs.Mob, r *rooms.Room) {
		h.caught = append(h.caught, m)
		thiefCaught(a, m, r)
	}

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		nightEyes: {
			ConditionId: nightEyes,
			Name:        "Test Night Eyes",
			Flags:       []conditions.Flag{conditions.NightVision},
			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectNightVisionStrength: {Literal: 30}},
		},
	}))
	observer := users.NewTestUser(observerId, "watcher", "Watcher", 97401)
	observer.Character.RoomId = 1
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{observerId: observer}))
	require.True(t, observer.Character.Conditions.AddCondition(nightEyes, true))
	h.room.AddPlayer(observerId)
	t.Cleanup(func() { h.room.RemovePlayer(observerId) })
	events.DrainQueuedMessagesForTest(observerId)

	h.room.SkyLight, h.room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(30)
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(&merchantInstance().Character, h.room), "fixture: the owner sees shapes only")
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(observer.Character, h.room), "fixture: the observer sees clearly")

	it := stolenBauble(t, "Bone Dice", "dice", 12, 2, 1, stolenTestNow.Add(-time.Hour))
	require.True(t, h.thief.GetCharacter().StoreItem(it))
	recognizeIn(h.room, 1, 0)
	require.Len(t, h.caught, 1)

	thefts := 0
	for _, c := range crimes.AllForFaction(`thornwall_citizens`, true) {
		if c.Kind != crimes.KindTheft {
			continue
		}
		thefts++
		assert.Equal(t, crimes.PerpUnknown, c.Perpetrator.Type,
			"an owner who sees only shapes cannot name the thief: %+v", c.Perpetrator)
	}
	assert.Equal(t, 1, thefts, "the recognition is recorded as one theft")

	var line string
	for _, l := range events.DrainQueuedMessagesForTest(observerId) {
		if strings.Contains(l, `That's mine! Thief!`) {
			line = l
		}
	}
	require.NotEmpty(t, line, "the observer hears the owner")
	assert.Contains(t, line, `points at a figure.`, "the owner names nobody")
	assert.NotContains(t, line, h.thief.GetCharacter().Name, "nor the thief's name")
}

// A household's bauble taken with nobody watching belongs to the whole
// household: any of them recognises it, but only at home.
func TestStolenBauble_TheHouseholdRecognisesItAtHome(t *testing.T) {
	h := setupRecognition(t, true)
	origMember := householdMember
	householdMember = func(m *mobs.Mob, room *rooms.Room) bool { return m.InstanceId == 301 }
	t.Cleanup(func() { householdMember = origMember })

	it := newBauble(t, "Small Child's Doll", "doll", 3, baubles.StatusReady)
	require.True(t, baubles.MarkStolen(it.Bauble, baubles.Theft{ByUserId: 1, RoomId: 1}, stolenTestNow.Add(-time.Hour)))
	require.True(t, h.thief.GetCharacter().StoreItem(it))

	recognizeIn(h.room, 1, 0)
	assert.Len(t, h.caught, 1)

	elsewhere := newBauble(t, "Wooden Spoon", "spoon", 3, baubles.StatusReady)
	require.True(t, baubles.MarkStolen(elsewhere.Bauble, baubles.Theft{ByUserId: 1, RoomId: 55}, stolenTestNow.Add(-time.Hour)))
	require.True(t, h.thief.GetCharacter().StoreItem(elsewhere))
	recognizeIn(h.room, 1, 0)
	assert.Len(t, h.caught, 1, "a household recognises only what was taken from its own home")

	// A passer-by who counts as a household member elsewhere (home room not
	// this house) is not this household.
	merchantInstance().HomeRoomId = 5
	third := newBauble(t, "Tin Cup", "cup", 3, baubles.StatusReady)
	require.True(t, baubles.MarkStolen(third.Bauble, baubles.Theft{ByUserId: 1, RoomId: 1}, stolenTestNow.Add(-time.Hour)))
	require.True(t, h.thief.GetCharacter().StoreItem(third))
	recognizeIn(h.room, 1, 0)
	assert.Len(t, h.caught, 1, "a passer-by is not the household")
	assert.False(t, StolenBaubleGiven(h.thief, merchantInstance(), third), "nor can take it back for them")
}

// The recognition roll is the owner's noticing score against the
// carrier's steal score: a sharp-eyed owner beats a clumsy carrier far
// more often than the other way round.
func TestStolenRecognitionRollPitsEyesAgainstHands(t *testing.T) {
	defer seedSellRoom(t)()
	room := rooms.LoadRoom(1)
	sharp, dull := newChar(), newChar()
	sharp.Stats.Perception.ValueAdj = 300
	dull.Stats.Perception.ValueAdj = 20
	clumsy, deft := newChar(), newChar()
	clumsy.Stats.Dexterity.ValueAdj = 20
	deft.Stats.Dexterity.ValueAdj = 300

	rate := func(owner, carrier *characters.Character) int {
		won := 0
		for i := 0; i < 400; i++ {
			if stolenRecognitionRoll(owner, carrier, room) {
				won++
			}
		}
		return won
	}
	assert.Greater(t, rate(sharp, clumsy), rate(dull, deft)+200)
}

// ─── Returns ────────────────────────────────────────────────────────────────

// Every perCatch returns earn back exactly one catch, and no single return
// is worth as much as a catch.
func TestReturnShare_ThreeReturnsEqualOneCatch(t *testing.T) {
	assert.Equal(t, []int{1, 2, 2}, []int{returnShare(0, 5, 3), returnShare(1, 5, 3), returnShare(2, 5, 3)})
	for catch := 1; catch <= 40; catch++ {
		for prior := 0; prior < 12; prior += 3 {
			sum := 0
			for i := 0; i < 3; i++ {
				share := returnShare(prior+i, catch, 3)
				assert.Less(t, share, catch+1)
				if catch >= 3 {
					assert.Less(t, share, catch, "catch %d: a return is smaller than a catch", catch)
				}
				sum += share
			}
			assert.Equal(t, catch, sum, "catch %d: three returns are one catch", catch)
		}
	}
	assert.Equal(t, 0, returnShare(0, 0, 3), "no reputation lost to a catch, none to earn back")
	assert.Equal(t, 0, returnShare(0, 5, 0))
}

type returnHarness struct {
	giver   Actor
	bumps   []int
	catches int    // open theft crimes naming the thief in the faction's log
	since   uint64 // the round of the oldest of them
}

func setupReturns(t *testing.T) *returnHarness {
	t.Helper()
	seedBaubleSale(t)
	t.Cleanup(seedSellRoom(t))
	t.Cleanup(seedSellMerchant(t, 1000))
	cfg := configs.GetConfig()
	cfg.Balance.CrimeRepDeltaTheft = -5
	configs.SetConfigForTest(t, cfg)

	h := &returnHarness{giver: newSellerActor(t, true), catches: 1}
	origFactions, origBump, origCatches := ownerFactions, returnRepBump, theftCatches
	ownerFactions = func(*mobs.Mob) []string { return []string{`thornwall_citizens`} }
	theftCatches = func(userId int, faction string) (int, uint64) {
		assert.Equal(t, 1, userId)
		assert.Equal(t, `thornwall_citizens`, faction)
		return h.catches, h.since
	}
	returnRepBump = func(fid string, userId int, delta int) {
		assert.Equal(t, `thornwall_citizens`, fid)
		assert.Equal(t, 1, userId)
		h.bumps = append(h.bumps, delta)
	}
	t.Cleanup(func() { ownerFactions, returnRepBump, theftCatches = origFactions, origBump, origCatches })
	return h
}

// A thief caught once earns it back with three returns, 1, 2 and 2: one
// catch of 5. A fourth return earns nothing: there is nothing left to earn
// back until they are caught again.
func TestStolenBauble_ReturnsEarnBackACatch(t *testing.T) {
	h := setupReturns(t)
	pinStolenClock(t, stolenTestNow)
	give := func() {
		it := stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 2, 1, stolenTestNow.Add(-time.Hour))
		assert.True(t, StolenBaubleGiven(h.giver, merchantInstance(), it))
		rec, _ := baubles.Get(it.Bauble)
		assert.False(t, rec.Hot(stolenTestNow), "given back, it cools")
	}
	for i := 0; i < 3; i++ {
		give()
	}
	assert.Equal(t, []int{1, 2, 2}, h.bumps)
	assert.Equal(t, 5, h.bumps[0]+h.bumps[1]+h.bumps[2], "three returns are one catch (CrimeRepDeltaTheft -5)")

	give()
	assert.Len(t, h.bumps, 3, "nothing left to earn back")

	h.catches = 2 // caught again
	give()
	assert.Equal(t, []int{1, 2, 2, 1}, h.bumps, "a second catch can be earned back the same way")
}

// A thief never caught earns nothing by returning (no reputation from
// nothing); a bauble earns credit once ever; only its thief is credited;
// and giving it to anyone else is no return at all.
func TestStolenBauble_ReturnsCannotFarmReputation(t *testing.T) {
	h := setupReturns(t)
	pinStolenClock(t, stolenTestNow)

	h.catches = 0
	uncaught := stolenBauble(t, "Glass Marble", "marble", 4, 2, 1, stolenTestNow.Add(-time.Hour))
	assert.True(t, StolenBaubleGiven(h.giver, merchantInstance(), uncaught), "still a return")
	assert.Empty(t, h.bumps, "never caught: nothing to earn back")
	rec, _ := baubles.Get(uncaught.Bauble)
	assert.False(t, rec.Hot(stolenTestNow), "an uncredited return still cools it")
	assert.Empty(t, rec.ReturnCreditFactions, "and records no credit")

	h.catches = 1
	first := stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 2, 1, stolenTestNow.Add(-time.Hour))
	assert.True(t, StolenBaubleGiven(h.giver, merchantInstance(), first))
	assert.Equal(t, []int{1}, h.bumps)

	// The same bauble, stolen and given back again: no credit twice.
	later := stolenTestNow.Add(time.Hour)
	pinStolenClock(t, later)
	require.True(t, baubles.MarkStolen(first.Bauble, baubles.Theft{ByUserId: 1, RoomId: 1, FromMob: 2}, later.Add(-time.Minute)))
	assert.True(t, StolenBaubleGiven(h.giver, merchantInstance(), first))
	assert.Equal(t, []int{1}, h.bumps, "a bauble earns credit once")

	// Someone else's theft, given back: a return, no credit to the giver.
	other := stolenBauble(t, "Glass Marble", "marble", 4, 2, 42, later.Add(-time.Hour))
	assert.True(t, StolenBaubleGiven(h.giver, merchantInstance(), other))
	assert.Equal(t, []int{1}, h.bumps, "only the thief is credited")

	// Given to a mob it was not taken from: not a return, still hot.
	notTheirs := stolenBauble(t, "Brass Key", "key", 4, 77, 1, later.Add(-time.Hour))
	assert.False(t, StolenBaubleGiven(h.giver, merchantInstance(), notTheirs))
	rec, _ = baubles.Get(notTheirs.Bauble)
	assert.True(t, rec.Hot(later))
	assert.False(t, StolenBaubleGiven(h.giver, merchantInstance(), items.New(sellTestItemId)), "not a bauble")
}

// Returns are counted over the same stretch as the catches: a thief whose
// three returns earned back an old catch, who then served a sentence (the
// catch resolved) and was caught again, earns back the new catch in full.
func TestStolenBauble_ReturnsCountSinceTheOldestOpenCatch(t *testing.T) {
	h := setupReturns(t)
	pinStolenClock(t, stolenTestNow)
	origRound := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCountForTest(origRound) })

	give := func(round uint64) {
		util.SetRoundCountForTest(round)
		it := stolenBauble(t, "Glass Marble", "marble", 4, 2, 1, stolenTestNow.Add(-time.Hour))
		require.True(t, StolenBaubleGiven(h.giver, merchantInstance(), it))
	}
	h.catches, h.since = 1, 100
	for r := uint64(110); r < 140; r += 10 {
		give(r)
	}
	give(150)
	assert.Equal(t, []int{1, 2, 2}, h.bumps, "the first catch earned back; nothing more")

	h.catches, h.since = 1, 500 // served, then caught again
	give(510)
	assert.Equal(t, []int{1, 2, 2, 1}, h.bumps, "the new catch starts afresh")
}

// A return that earns nothing (a catch of 1 split in thirds pays 0, 0, 1)
// still counts, so the remainder lands on a later return.
func TestStolenBauble_AReturnWorthNothingStillCounts(t *testing.T) {
	h := setupReturns(t)
	cfg := configs.GetConfig()
	cfg.Balance.CrimeRepDeltaTheft = -1
	configs.SetConfigForTest(t, cfg)
	pinStolenClock(t, stolenTestNow)

	for i := 0; i < 3; i++ {
		it := stolenBauble(t, "Glass Marble", "marble", 4, 2, 1, stolenTestNow.Add(-time.Hour))
		StolenBaubleGiven(h.giver, merchantInstance(), it)
	}
	assert.Equal(t, []int{1}, h.bumps, "0, 0, 1: only the third pays, and it does")
}

// A carrier's score is the steal score: Dexterity, skullduggery at
// SkillWeight, and the hidden bonus when hidden.
func TestCarrierScoreIsTheStealScore(t *testing.T) {
	pinConfigForTest(t)
	full := configs.GetConfig()
	full.Balance.StealHiddenBonus = 25
	configs.SetConfigForTest(t, full)
	cfg := configs.GetBalanceConfig()
	a := newSearchFakeActor("Nimble", newSearchTestRoom(9701), true, 7701)
	a.char.Stats.Dexterity.ValueAdj = 100
	a.char.Skills = map[string]int{`skullduggery`: 4}
	plain := carrierScore(a.char)
	assert.Equal(t, 100+4*float64(cfg.SkillWeight), plain)

	hide(t, a)
	assert.Equal(t, plain+float64(cfg.StealHiddenBonus), carrierScore(a.char), "hiding helps")
}

// identifiedTheftCatches counts the open thefts a faction's log names this
// player the identified perpetrator of; not resolved ones (a sentence
// served gives the reputation back), not other players', not other
// crimes, not unidentified figures.
func TestIdentifiedTheftCatchesReadsTheCrimesLog(t *testing.T) {
	t.Setenv("DOGMUD_FACTIONS_CRIMES_DIR_OVERRIDE", t.TempDir())
	crimes.ClearCache()
	t.Cleanup(crimes.ClearCache)

	victim := &mobs.Mob{MobId: 2, InstanceId: 301}
	victim.Character.Name = "Merchant"
	const fid = `test_catches_guild`
	me := crimes.Perpetrator{Type: crimes.PerpPlayer, Id: 1}
	record := func(kind crimes.Kind, perp crimes.Perpetrator) []int {
		return crimes.Record([]string{fid}, kind, perp, victim, 301, 1, "TestZone", false)
	}
	origRound := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCountForTest(origRound) })
	util.SetRoundCountForTest(100)
	resolved := record(crimes.KindTheft, me)
	util.SetRoundCountForTest(200)
	record(crimes.KindTheft, me)
	util.SetRoundCountForTest(300)
	record(crimes.KindTheft, me)
	record(crimes.KindTheft, crimes.Perpetrator{Type: crimes.PerpPlayer, Id: 42})
	record(crimes.KindAssault, me)
	record(crimes.KindTheft, crimes.Perpetrator{Type: crimes.PerpUnknown})
	require.Len(t, resolved, 1)
	crimes.Resolve(fid, resolved[0], "stale")

	n, since := identifiedTheftCatches(1, fid)
	assert.Equal(t, 2, n, "the open thefts; a resolved one (served, or gone stale) no longer counts")
	assert.Equal(t, uint64(200), since, "the oldest OPEN catch sets the window")
	n, _ = identifiedTheftCatches(42, fid)
	assert.Equal(t, 1, n)
	n, since = identifiedTheftCatches(1, `another_guild`)
	assert.Equal(t, 0, n)
	assert.Equal(t, uint64(0), since)
}

// seedFence puts a fence who keeps a legacy shop (as every fence does:
// they are shopkeepers) in room 1 beside the merchant of seedSellMerchant,
// with gold in its purse.
func seedFence(t *testing.T, gold int) *mobs.Mob {
	t.Helper()
	const instId = 302
	m := &mobs.Mob{MobId: 3, InstanceId: instId, HomeRoomId: 1, Zone: "TestZone", Groups: []string{`fence`}}
	m.Character.Name = "Siv"
	m.Character.RoomId = 1
	m.Character.Gold = gold
	m.Character.Conditions = conditions.New()
	m.Character.Shop = characters.Shop{{ItemId: sellTestItemId, Price: 100}}
	mobs.SetInstanceForTest(instId, m)
	room := rooms.LoadRoom(1)
	room.AddMob(instId)
	t.Cleanup(func() {
		room.RemoveMob(instId)
		mobs.SetInstanceForTest(instId, nil)
	})
	return m
}

// The best offer in the room is the best one the buyer can pay: a fence
// with too little gold for its offer does not hide an honest merchant who
// can pay less. A mob's sale draws on no merchant's gold, so for one the
// fence is still the best offer.
func TestStolenBauble_BestOfferSkipsABuyerWhoCannotPay(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)
	fence := seedFence(t, 5)
	room := rooms.LoadRoom(1)

	cold := stolenBauble(t, "Bone Dice", "dice", 12, 99, 1, stolenTestNow.Add(-30*24*time.Hour))
	m, _ := resolveMerchant(room, cold, true)
	require.NotNil(t, m)
	assert.Equal(t, merchantInstance().InstanceId, m.InstanceId, "the fence offers 8 but has 5")
	m, _ = resolveMerchant(room, cold, false)
	require.NotNil(t, m)
	assert.Equal(t, fence.InstanceId, m.InstanceId, "a mob seller is not paid from the fence's gold")

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(cold))
	res := Sell(seller, SellOptions{ItemName: "dice", Quantity: 1})
	require.Equal(t, 1, res.Sold, "res=%+v", res)
	assert.Equal(t, 6, char.Gold, "sold to the merchant who could pay")
	assert.Equal(t, 5, fence.Character.Gold)

	// Nobody who can pay: the sale is refused, not made on credit.
	merchantInstance().Character.Gold = 0
	require.True(t, char.StoreItem(stolenBauble(t, "Pewter Dice", "dice", 12, 99, 1, stolenTestNow.Add(-30*24*time.Hour))))
	res = Sell(seller, SellOptions{ItemName: "dice", Quantity: 1})
	assert.Equal(t, 0, res.Sold)
	assert.Equal(t, SellStopMerchantBroke, res.Reason)
}

// Selling several baubles of one name, each goes to the best offer for
// it: stolen goods to the fence, and once the fence runs short, the rest
// to a merchant who can still pay.
func TestStolenBauble_SellChoosesTheBuyerPerItem(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)
	fence := seedFence(t, 1000)

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	// An honest one first (6 from either; a tie goes to the merchant, who
	// stands first), then a stolen one (8 from the fence, 6 elsewhere).
	require.True(t, char.StoreItem(newBauble(t, "Painted Wooden Thimble", "thimble", 12, baubles.StatusReady)))
	require.True(t, char.StoreItem(stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenTestNow.Add(-30*24*time.Hour))))
	res := Sell(seller, SellOptions{ItemName: "thimble", Quantity: 2})
	require.Equal(t, 2, res.Sold, "res=%+v", res)
	assert.Equal(t, 14, char.Gold, "6 for the honest one, 8 for the stolen one")
	assert.Equal(t, 992, fence.Character.Gold, "the fence bought the stolen one")
	assert.Equal(t, 994, merchantInstance().Character.Gold, "the merchant bought the honest one")

	// The fence can pay for one stolen bauble; the second goes to the
	// merchant rather than stopping the sale.
	fence.Character.Gold = 8
	char.Gold = 0
	require.True(t, char.StoreItem(stolenBauble(t, "Bone Dice", "dice", 12, 99, 1, stolenTestNow.Add(-30*24*time.Hour))))
	require.True(t, char.StoreItem(stolenBauble(t, "Jade Dice", "dice", 12, 99, 1, stolenTestNow.Add(-30*24*time.Hour))))
	res = Sell(seller, SellOptions{ItemName: "dice", Quantity: 2})
	require.Equal(t, 2, res.Sold, "res=%+v", res)
	assert.Equal(t, 14, char.Gold, "8 from the fence, then 6 from the merchant")
	assert.Equal(t, 0, fence.Character.Gold)
}

// A fence is a shopkeeper: with a living-economy shop it pays from the
// shop's gold, like any shopkeeper, and its own purse is not touched.
func TestStolenBauble_AFencePaysFromItsShopGold(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)()
	pinStolenClock(t, stolenTestNow)
	merchantInstance().Groups = []string{`fence`}

	shops.IsolateForTest(t)
	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, StartingGold: 1000, CraftSupport: shops.CraftSupportGeneral})

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenTestNow.Add(-time.Hour))))
	res := Sell(seller, SellOptions{ItemName: "thimble", Quantity: 1})
	require.Equal(t, 1, res.Sold, "res=%+v", res)
	assert.Equal(t, 8, char.Gold)
	assert.Equal(t, 992, si.Gold, "paid from the shop's gold")
	assert.Equal(t, 0, merchantInstance().Character.Gold, "the purse is not the till")
}

// A fence-only shop (no craft_support) refuses ordinary vendor loot, even
// an item it stocks, so its gold is kept for baubles; it still buys a
// bauble. A general shop would buy the sword (the control in the sabotage
// check: give this shop CraftSupportGeneral and the first assertion fails).
func TestStolenBauble_AFenceShopRefusesOrdinaryLootButBuysBaubles(t *testing.T) {
	seedBaubleSale(t)
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		items.BaubleItemId: {
			ItemId:     items.BaubleItemId,
			Name:       "Curious Trinket",
			NameSimple: "trinket",
			Type:       items.Object,
			Subtype:    items.Mundane,
			Weight:     0.2,
			Value:      1,
			NotSalable: true,
		},
		sellTestItemId: {
			ItemId:           sellTestItemId,
			Name:             "iron sword",
			Type:             items.Weapon,
			Value:            100,
			VendorCategories: []string{shops.CraftSupportBlacksmithing},
		},
	})()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)() // stocks the iron sword
	pinStolenClock(t, stolenTestNow)
	merchantInstance().Groups = []string{`fence`}

	shops.IsolateForTest(t)
	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, StartingGold: 1000})

	seller := newSellerActor(t, true, sellTestItemId)
	char := seller.GetCharacter()
	res := Sell(seller, SellOptions{ItemName: "sword", Quantity: 1})
	assert.Equal(t, 0, res.Sold, "a fence-only shop buys no ordinary loot: res=%+v", res)
	assert.Equal(t, 1000, si.Gold, "the till is untouched")
	assert.Equal(t, 0, char.Gold)

	require.True(t, char.StoreItem(stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenTestNow.Add(-time.Hour))))
	res = Sell(seller, SellOptions{ItemName: "thimble", Quantity: 1})
	require.Equal(t, 1, res.Sold, "the fence still buys a bauble: res=%+v", res)
	assert.Equal(t, 8, char.Gold, "the fence premium on a stolen bauble")
	assert.Equal(t, 992, si.Gold, "paid from the shop's gold")
}

// Every town has a fence in it or a zone or two away (owner ruling,
// 2026-09-28), and every fence is a non-hostile, non-combatant shopkeeper.
// Read from the world's mob files (a mob's folder is its zone), so moving
// or dropping a fence is caught. Thornwall City's fence is Fence Dealer Siv
// (104); Torvan Cresk (249) is not a fence, since quest 14 (The Undertow)
// has players fight him for the strongbox key.
//
// fenceOnlyShops are the fences whose shops PR #175 opened only for the
// trade. They carry no craft_support, so they buy no ordinary loot and keep
// their gold for baubles (owner ruling, 2026-09-29). Fences that were
// traders before (Siv, Mother Coyle, the Hawker, Wick Orrel, Varro) keep the
// craft_support they had.
var fenceOnlyShops = map[string]bool{
	`Sly Tam`:               true,
	`Ysolde`:                true,
	`Peddler Malk`:          true,
	`A River-Road Smuggler`: true,
}

func TestEveryTownHasAFenceNearby(t *testing.T) {
	// sourceDir (consider_no_progression_test.go), not a relative path:
	// another test in this package changes the working directory.
	files, err := filepath.Glob(filepath.Join(sourceDir(t), "..", "..", "_datafiles", "world", "dogmud", "mobs", "*", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "the world's mob files")
	fencesIn := map[string][]string{}
	seenFenceOnly := map[string]bool{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		// Decoded loosely: mob files carry fields of many shapes, and only
		// three matter here.
		var mob map[string]interface{}
		require.NoError(t, yaml.Unmarshal(data, &mob), f)
		groups, _ := mob[`groups`].([]interface{})
		for _, g := range groups {
			if g != `fence` {
				continue
			}
			name := f
			if c, ok := mob[`character`].(map[interface{}]interface{}); ok {
				if n, ok := c[`name`].(string); ok {
					name = n
				}
			}
			hostile, _ := mob[`hostile`].(bool)
			assert.False(t, hostile, "%s: a fence must be someone you can deal with", name)
			// Every fence is a shopkeeper (owner ruling 14): it pays from
			// persisted shop gold, and cannot be attacked or robbed of it.
			nonCombatant, _ := mob[`non_combatant`].(bool)
			assert.True(t, nonCombatant, "%s: a fence is non_combatant", name)
			craft, _ := mob[`craft_support`].(string)
			if fenceOnlyShops[name] {
				assert.Empty(t, craft, "%s: a fence-only shop has no craft_support, so it buys no ordinary loot", name)
				seenFenceOnly[name] = true
			} else {
				assert.NotEmpty(t, craft, "%s: a fence that was already a trader keeps its craft_support", name)
			}
			shop, _ := mob[`character`].(map[interface{}]interface{})[`shop`].([]interface{})
			assert.NotEmpty(t, shop, "%s: a fence keeps a shop", name)
			zone := filepath.Base(filepath.Dir(f))
			fencesIn[zone] = append(fencesIn[zone], name)
		}
	}

	for name := range fenceOnlyShops {
		assert.True(t, seenFenceOnly[name], "%s: listed as a fence-only shop but not found as a fence", name)
	}
	assert.NotContains(t, fencesIn[`thornwall_city`], `Torvan Cresk`,
		"Torvan Cresk is not a fence: quest 14 has players fight him (owner ruling)")
	assert.Contains(t, fencesIn[`thornwall_city`], `Fence Dealer Siv`,
		"Thornwall City's fence is Fence Dealer Siv")

	nearby := map[string][]string{
		`thornwall_city`:    {`thornwall_city`},
		`new_plymouth`:      {`new_plymouth_common`, `new_plymouth_docks`, `new_plymouth_merchant`, `new_plymouth_old_quarter`, `new_plymouth_outskirts`},
		`stillwater`:        {`stillwater`, `stillwater_marsh`, `north_road`, `north_road_north`},
		`hartcharn`:         {`hartcharn`, `greywater_flats`, `the_empty_reach`},
		`the_confluence`:    {`the_confluence`, `river_road`, `east_road_to_greenford`},
		`greenford`:         {`greenford`, `east_road_to_greenford`, `the_confluence`},
		`pothole_coulee`:    {`pothole_coulee`, `ironwind_steppe`, `thornwall_city`},
		`kilnreach_works`:   {`kilnreach_works`, `kingsbarrow_vale`, `new_plymouth_outskirts`},
		`watchers_crossing`: {`watchers_crossing`, `thornwall_outskirts`, `marches_spur_road`, `dustwalk_road`},
		`ashwick`:           {`ashwick`, `marches_spur_road`},
	}
	for town, zones := range nearby {
		var found []string
		for _, z := range zones {
			found = append(found, fencesIn[z]...)
		}
		assert.NotEmpty(t, found, "%s has no fence in or near it", town)
	}
}

// A name matching a bauble and real items sells the real ones to the
// merchant who would buy them, not to the fence that took the bauble.
func TestStolenBauble_RealItemsKeepTheirBuyerAmongBaubles(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)
	fence := seedFence(t, 1000) // stocks iron swords too, so it would buy them

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	// Named exactly "Sword", the bauble is the strongest match, so it goes
	// first (to the fence, stolen), then the two iron swords.
	require.True(t, char.StoreItem(stolenBauble(t, "Sword", "sword", 12, 99, 1, stolenTestNow.Add(-30*24*time.Hour))))
	require.True(t, char.StoreItem(items.New(sellTestItemId)))
	require.True(t, char.StoreItem(items.New(sellTestItemId)))

	purse := merchantInstance().Character.Gold
	res := Sell(seller, SellOptions{ItemName: "sword", Quantity: 3})
	require.Equal(t, 3, res.Sold, "res=%+v", res)
	assert.Equal(t, 992, fence.Character.Gold, "the fence bought the stolen bauble and nothing else")
	assert.Less(t, merchantInstance().Character.Gold, purse, "the merchant bought the swords")
}

// A hot bauble a player sells to a fence's living shop is shelved but held
// out of sight until its heat ends everywhere: StolenAt plus
// BaubleStolenHeatHours (baubles slice D, spec test 8).
func TestStolenBauble_AHotOneIsShelvedHeldUntilItCools(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)()
	pinStolenClock(t, stolenTestNow)
	cfg := configs.GetConfig()
	cfg.Balance.BaubleStolenHeatHours = 72
	configs.SetConfigForTest(t, cfg)
	merchantInstance().Groups = []string{`fence`}

	shops.IsolateForTest(t)
	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, StartingGold: 1000, CraftSupport: shops.CraftSupportGeneral})

	stolenAt := stolenTestNow.Add(-time.Hour)
	hot := stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenAt)
	seller := newSellerActor(t, true)
	require.True(t, seller.GetCharacter().StoreItem(hot))
	res := Sell(seller, SellOptions{ItemName: "thimble", Quantity: 1})
	require.Equal(t, 1, res.Sold, "res=%+v", res)

	require.Len(t, si.AffixedStock, 1, "a player's sale of an average bauble is shelved")
	e := si.AffixedStock[0]
	assert.Equal(t, hot.Bauble, e.Item.Bauble)
	assert.Equal(t, 12, e.Price, "relisted at its catalog value")
	assert.True(t, e.HoldUntil.Equal(stolenAt.Add(72*time.Hour)), "held until the heat ends: %v", e.HoldUntil)
	assert.True(t, e.AddedAt.Equal(stolenTestNow), "shelved on the shelf clock: %v", e.AddedAt)
	assert.Equal(t, 1, si.HeldCount(stolenTestNow))
	assert.Empty(t, si.ListedIndexes(stolenTestNow), "not listed while held")
}

// A shop's backroom holds at most ShopAffixedStockCap hot baubles (owner
// ruling 6, spec test 6). Full, it turns away another hot shelvable one in
// the fence's voice, as an interest refusal (not Broke), and still buys a
// cold one (listed at once) and a cheap hot one (destroyed, never held).
func TestStolenBauble_AFullBackroomRefusesHotGoods(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)()
	pinStolenClock(t, stolenTestNow)
	cfg := configs.GetConfig()
	cfg.Balance.ShopAffixedStockCap = 2
	cfg.Balance.BaubleStolenHeatHours = 72
	configs.SetConfigForTest(t, cfg)
	merchantInstance().Groups = []string{`fence`}

	shops.IsolateForTest(t)
	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, StartingGold: 1000, CraftSupport: shops.CraftSupportGeneral})
	later := stolenTestNow.Add(24 * time.Hour)
	for _, name := range []string{"held one", "held two"} {
		si.AffixedStock = append(si.AffixedStock, shops.AffixedStockEntry{
			Item:  items.Item{ItemId: sellTestItemId, Affixed: true, Spec: &items.ItemSpec{Name: name, Value: 10}},
			Price: 10, AddedAt: stolenTestNow, HoldUntil: later,
		})
	}

	hot := stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenTestNow.Add(-time.Hour))
	offer := BaubleOfferFrom(hot, merchantInstance())
	assert.Equal(t, 0, offer.Price)
	assert.Equal(t, baubleSayBackroomFull, offer.Refusal)
	assert.False(t, offer.Broke, "an interest refusal: the next merchant in the room is tried")

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(hot))
	res := Sell(seller, SellOptions{ItemName: "thimble", Quantity: 1})
	assert.Equal(t, 0, res.Sold, "res=%+v", res)
	assert.Len(t, si.AffixedStock, 2)

	cold := stolenBauble(t, "Bone Dice", "dice", 12, 99, 1, stolenTestNow.Add(-30*24*time.Hour))
	cheapHot := stolenBauble(t, "Brass Button", "button", 4, 99, 1, stolenTestNow.Add(-time.Hour))
	require.True(t, char.StoreItem(cold))
	require.True(t, char.StoreItem(cheapHot))
	require.Equal(t, 1, Sell(seller, SellOptions{ItemName: "dice", Quantity: 1}).Sold, "a cold one still sells")
	require.Equal(t, 1, Sell(seller, SellOptions{ItemName: "button", Quantity: 1}).Sold, "a cheap hot one still sells")
	require.Len(t, si.AffixedStock, 3, "the cold one is shelved; the cheap hot one is not shelved at all")
	assert.Equal(t, cold.Bauble, si.AffixedStock[2].Item.Bauble)
	assert.Equal(t, 2, si.HeldCount(stolenTestNow), "the backroom is unchanged")
}

// An honest shop buys a bauble that is hot only in another heat area (not
// HotIn here) and holds it, so its backroom can fill too. Full, it refuses
// the next one in an honest voice, never the fence's line about hot goods
// (controller decision, plan review 2026-09-30).
func TestStolenBauble_AnHonestShopWithAFullBackroomHasNoRoom(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)()
	pinStolenClock(t, stolenTestNow)
	cfg := configs.GetConfig()
	cfg.Balance.ShopAffixedStockCap = 2
	cfg.Balance.BaubleStolenHeatHours = 72
	configs.SetConfigForTest(t, cfg)

	shops.IsolateForTest(t)
	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, StartingGold: 1000, CraftSupport: shops.CraftSupportGeneral})
	held := shops.AffixedStockEntry{
		Item:  items.Item{ItemId: sellTestItemId, Affixed: true, Spec: &items.ItemSpec{Name: "held", Value: 10}},
		Price: 10, AddedAt: stolenTestNow, HoldUntil: stolenTestNow.Add(24 * time.Hour),
	}
	si.AffixedStock = []shops.AffixedStockEntry{held, held}

	elsewhere := newBauble(t, "Bone Dice", "dice", 12, baubles.StatusReady)
	require.True(t, baubles.MarkStolen(elsewhere.Bauble, baubles.Theft{ByUserId: 1, RoomId: 1, Zone: "Faraway", FromMob: 99, FromName: "Merchant"}, stolenTestNow.Add(-time.Hour)))

	offer := BaubleOfferFrom(elsewhere, merchantInstance())
	assert.Equal(t, 0, offer.Price)
	assert.Equal(t, baubleSayNoRoom, offer.Refusal, "an honest shop does not talk about moving hot goods")
	assert.False(t, offer.Broke)

	si.AffixedStock = si.AffixedStock[:1]
	assert.Equal(t, 6, BaubleOfferFrom(elsewhere, merchantInstance()).Price, "with room, it buys a bauble that is hot only elsewhere")
}

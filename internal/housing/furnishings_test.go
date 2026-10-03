package housing

import (
	"strings"
	"testing"
)

// Beds and crafting stations: bought from the landlord, placed by the owner
// in a room of their own lodging, one of each to a room, kept in the house
// record, laid over the room by the overlay, and usable by anyone let in.

func stubStations(t *testing.T) {
	t.Helper()
	prev := stationTypes
	stationTypes = func() []string { return []string{`alchemy_bench`, `forge`, `loom`} }
	t.Cleanup(func() { stationTypes = prev })
}

func TestFurnish_BedIsPlacedKeptAndLaidOver(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferBed)
	home := w.homeOf(t, u)
	if u.Character.Gold != 100000-500-250 {
		t.Fatalf("a bed deed did not cost 250: gold %d", u.Character.Gold)
	}

	UseItem(u, w.room(home), itemOf(u, testBedId), ``, `bed deed`)
	r := w.room(home)
	if !RoomHasBed(home) || r.Nouns[`bed`] == `` || !strings.Contains(r.Description, `>bed<`) {
		t.Fatalf("the bed was not placed: has=%v nouns=%v desc=%q", RoomHasBed(home), r.Nouns, r.Description)
	}
	if itemOf(u, testBedId).ItemId != 0 {
		t.Error("the deed was not spent")
	}

	// Laying the room again (any house change) does not say it twice.
	applyLayout(r)
	applyLayout(r)
	if n := strings.Count(r.Description, `>bed<`); n != 1 {
		t.Errorf("the bed is described %d times after re-laying", n)
	}

	// A restart rebuilds the room from its template: the record brings it back.
	w.reload(t)
	back := w.room(home)
	if !RoomHasBed(home) || back.Nouns[`bed`] == `` || strings.Count(back.Description, `>bed<`) != 1 {
		t.Errorf("the bed did not survive a restart: %q", back.Description)
	}
}

func TestFurnish_StationSetsTheRoomsStation(t *testing.T) {
	setup(t)
	stubStations(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferStation)
	home := w.homeOf(t, u)
	if u.Character.Gold != 100000-500-1500 {
		t.Fatalf("a station deed did not cost 1500: gold %d", u.Character.Gold)
	}

	// Not a station: refused, deed kept.
	UseItem(u, w.room(home), itemOf(u, testStationId), `bathtub`, `station deed bathtub`)
	if w.room(home).Station != `` || itemOf(u, testStationId).ItemId == 0 {
		t.Fatal("an unknown station was installed or the deed spent")
	}

	UseItem(u, w.room(home), itemOf(u, testStationId), `alchemy bench`, `station deed alchemy bench`)
	r := w.room(home)
	if r.Station != `alchemy_bench` || r.Nouns[`bench`] == `` {
		t.Fatalf("station = %q, nouns %v", r.Station, r.Nouns)
	}
	if !strings.Contains(r.Description, `An alchemy <ansi fg="itemname">bench</ansi>`) {
		t.Errorf("description: %q", r.Description)
	}
	w.reload(t)
	if got := w.room(home).Station; got != `alchemy_bench` {
		t.Errorf("after a restart the station is %q", got)
	}
}

func TestFurnish_OneOfEachToARoom(t *testing.T) {
	setup(t)
	stubStations(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferBed)
	home := w.homeOf(t, u)
	UseItem(u, w.room(home), itemOf(u, testBedId), ``, `bed deed`)

	// The only room has a bed: no second bed deed is sold.
	if o := offerMap(u)[OfferBed]; o.Available {
		t.Errorf("a bed offered with a bed in every room: %+v", o)
	}
	gold := u.Character.Gold
	buyKey(u, OfferBed)
	if u.Character.Gold != gold || itemOf(u, testBedId).ItemId != 0 {
		t.Fatal("a second bed deed was sold for a one-room lodging")
	}

	// Two rooms: one bed deed may be carried for the second, not two.
	buyKey(u, OfferExtension)
	UseItem(u, w.room(home), itemOf(u, testDeedId), `north`, `deed north`)
	buyKey(u, OfferBed)
	buyKey(u, OfferBed)
	n := 0
	for _, itm := range u.Character.Items {
		if itm.ItemId == testBedId {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("carrying %d bed deeds for one bare room", n)
	}
	// Used in the room that has one, it is refused and kept.
	UseItem(u, w.room(home), itemOf(u, testBedId), ``, `bed deed`)
	if itemOf(u, testBedId).ItemId == 0 {
		t.Error("a bed deed was spent in a room that already has a bed")
	}

	// A station and a bed share a room happily; two stations do not.
	buyKey(u, OfferStation)
	buyKey(u, OfferStation)
	UseItem(u, w.room(home), itemOf(u, testStationId), `forge`, `station deed forge`)
	UseItem(u, w.room(home), itemOf(u, testStationId), `loom`, `station deed loom`)
	if got := w.room(home).Station; got != `forge` {
		t.Errorf("station = %q, want the first one, forge", got)
	}
	h, _ := HouseOf(1, testBldgId)
	if len(h.Stations) != 1 || !h.HasBed(home) {
		t.Errorf("house record: stations %+v beds %v", h.Stations, h.Beds)
	}
}

func TestFurnish_OnlyTheOwnerPlacesThem(t *testing.T) {
	setup(t)
	stubStations(t)
	w := newContainerWorld(t)
	owner := w.lodger(t, 1, `Alice`, OfferBed)
	home := w.homeOf(t, owner)

	// A guest holding a bed deed of their own cannot put it in Alice's room.
	guest := w.lodger(t, 2, `Bob`, OfferBed)
	UseItem(guest, w.room(home), itemOf(guest, testBedId), ``, `bed deed`)
	if RoomHasBed(home) || itemOf(guest, testBedId).ItemId == 0 {
		t.Error("someone other than the owner placed a bed")
	}
	// Nor outside a lodging.
	UseItem(owner, alleyRoom(), itemOf(owner, testBedId), ``, `bed deed`)
	if itemOf(owner, testBedId).ItemId == 0 {
		t.Error("a bed deed was spent outside the lodging")
	}
	// Nor with another building's deed.
	other := itemOf(owner, testBedId)
	other.ItemId = 77 // the Quillhouse's bed deed
	owner.Character.Items = append(owner.Character.Items, other)
	UseItem(owner, w.room(home), other, ``, `bed deed`)
	if RoomHasBed(home) {
		t.Error("another building's bed deed worked here")
	}
}

func TestFurnish_VacantUnitIsBare(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferBed)
	home := w.homeOf(t, u)
	r := w.room(home)
	desc := r.Description
	UseItem(u, r, itemOf(u, testBedId), ``, `bed deed`)

	// The tenancy ends (staff removed it) while the room stays loaded.
	ResetForTest()
	AddBuildingForTest(testBuilding())
	ApplyOverlay(r)
	if _, has := r.Nouns[`bed`]; has || r.Description != desc || r.Station != `` {
		t.Errorf("a vacant unit kept the last lodger's bed: %q %v", r.Description, r.Nouns)
	}
}

func TestFurnish_MatchOffer(t *testing.T) {
	for req, want := range map[string]string{
		`bed`: OfferBed, `bed deed`: OfferBed, `a bed`: OfferBed,
		`station`: OfferStation, `station deed`: OfferStation, `crafting station`: OfferStation, `forge`: OfferStation,
		`deed`: OfferExtension, `container deed`: OfferContainer, `strongbox`: OfferStrongbox,
	} {
		if got, _ := MatchOffer(req, true); got != want {
			t.Errorf("buy %q = %q, want %q", req, got, want)
		}
	}
}

func TestFurnish_ListAndBadRecords(t *testing.T) {
	setup(t)
	u := testUser(1, 100000, 0)
	if o := offerMap(u)[OfferBed]; o.Available || o.Note != `Needs a home here first` {
		t.Errorf("bed offered to someone with no home: %+v", o)
	}
	buy(t, u)
	m := offerMap(u)
	if m[OfferBed].Price != 250 || m[OfferStation].Price != 1500 {
		t.Errorf("prices: bed %d station %d", m[OfferBed].Price, m[OfferStation].Price)
	}

	for name, h := range map[string]House{
		`bed elsewhere`:     {RoomIds: []int{testUnitA}, Beds: []int{testUnitB}},
		`two beds`:          {RoomIds: []int{testUnitA}, Beds: []int{testUnitA, testUnitA}},
		`two stations`:      {RoomIds: []int{testUnitA}, Stations: []HouseStation{{testUnitA, `forge`}, {testUnitA, `loom`}}},
		`not a station id`:  {RoomIds: []int{testUnitA}, Stations: []HouseStation{{testUnitA, `Forge!`}}},
		`station elsewhere`: {RoomIds: []int{testUnitA}, Stations: []HouseStation{{testUnitB, `forge`}}},
	} {
		if err := h.checkLinks(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := (House{RoomIds: []int{testUnitA}, Beds: []int{testUnitA}, Stations: []HouseStation{{testUnitA, `alchemy_bench`}}}).checkLinks(); err != nil {
		t.Errorf("a good furnished house was rejected: %v", err)
	}
}

func TestFurnish_ContainerCannotTakeAFurnishingName(t *testing.T) {
	for _, name := range []string{`bed`, `forge`, `bench`, `loom`} {
		if validContainerName(name) == nil {
			t.Errorf("a container may be called %q", name)
		}
	}
}

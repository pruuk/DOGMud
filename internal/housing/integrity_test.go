package housing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A data reload rebuilds the registry from the house files. What players did
// since the last capture is only in the live rooms, so it is captured first
// and never thrown away.
func TestReload_KeepsWhatWasNotYetCaptured(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	w.place(t, u, testBoxId, home, `chest`)
	r := w.room(home)

	// Gold landed on the floor and in the chest with no command after it.
	r.Gold = 75
	chest := r.Containers[`chest`]
	chest.Gold = 12
	r.Containers[`chest`] = chest

	reloadHouses()
	if r.Gold != 75 || r.Containers[`chest`].Gold != 12 {
		t.Fatalf("the reload threw away live state: floor %d, chest %d", r.Gold, r.Containers[`chest`].Gold)
	}
	if f, _ := floorOf(t, 1, home); f.Gold != 75 {
		t.Error("the live floor was not written before the reload")
	}
}

// When the capture before a reload cannot be written, the live room is newer
// than its file, so the reload lays out the room but leaves its contents.
func TestReload_FailedCaptureKeepsTheLiveRoom(t *testing.T) {
	dir := setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	r := w.room(home)
	r.Gold = 40

	// Make the house file unwritable: its folder becomes a file.
	bdir := filepath.Join(dir, testBldgId)
	data, _ := os.ReadFile(filepath.Join(bdir, `6470.yaml`))
	keep := filepath.Join(t.TempDir(), `6470.yaml`)
	os.WriteFile(keep, data, 0o644)
	os.RemoveAll(bdir)
	os.WriteFile(bdir, []byte(`x`), 0o644)
	failed := captureAllLoaded()
	if !failed[home] {
		t.Fatal("setup: the capture should have failed")
	}
	// Put the old file back, as it was on disk, and reload with the failure.
	os.Remove(bdir)
	os.MkdirAll(bdir, 0o755)
	os.WriteFile(filepath.Join(bdir, `6470.yaml`), data, 0o644)
	ResetForTest()
	AddBuildingForTest(testBuilding())
	loadHouses()
	applyAllOverlays(failed)
	if r.Gold != 40 {
		t.Errorf("a reload after a failed capture overwrote the live floor: %d", r.Gold)
	}
}

// reloadHouses is LoadDataFiles for the test building (no authored files).
func reloadHouses() {
	rebuild(map[string]Building{testBldgId: testBuilding()}, nil)
}

// An unreadable house file hides which rooms it owned, so the building sells
// nothing until staff repair it: no homes, no deeds, no extension built.
func TestCorruptFile_FreezesSales(t *testing.T) {
	dir := setup(t)
	a := testUser(1, 100000, 0)
	buy(t, a)
	buyKey(a, OfferExtension)
	deed, _ := deedIn(a)

	// Another lodger's file (entry B) is corrupt.
	os.WriteFile(filepath.Join(dir, testBldgId, `6471.yaml`), []byte("room_ids: [6471, 64"), 0o644)
	ResetForTest()
	AddBuildingForTest(testBuilding())
	loadHouses()
	if _, ok := Frozen(testBldgId); !ok {
		t.Fatal("not frozen")
	}

	b := testUser(3, 100000, 0)
	if res, _ := buy(t, b); res == PurchaseOk {
		t.Error("a home was sold while a house file is unreadable")
	}
	entry := unitRoom(testUnitA)
	ApplyOverlay(entry)
	UseItem(a, entry, deed, `north`, `deed north`)
	if h, _ := HouseOf(1, testBldgId); len(h.RoomIds) != 1 {
		t.Error("an extension was built while a house file is unreadable")
	}
	if o := offerMap(b)[OfferHome]; o.Available {
		t.Errorf("home offered while frozen: %+v", o)
	}
}

// A lodger whose readable file was rejected is not sold a second home.
func TestHeldOwner_IsNotSoldASecondHome(t *testing.T) {
	dir := setup(t)
	os.MkdirAll(filepath.Join(dir, testBldgId), 0o755)
	// Readable but wrong: a doorway to a room outside the house.
	os.WriteFile(filepath.Join(dir, testBldgId, `6470.yaml`), []byte(
		"building_id: test_lodgings\nowner_user_id: 1\ntier_id: simple\nroom_ids: [6470]\nprice_paid: 500\nlinks:\n- {from: 6470, direction: north, to: 6999}\n"), 0o644)
	loadHouses()
	if res, _ := buy(t, testUser(1, 100000, 0)); res == PurchaseOk {
		t.Error("the owner of a held house bought a second home")
	}
	if res, _ := buy(t, testUser(2, 100000, 0)); res != PurchaseOk {
		t.Errorf("someone else could not buy: %v", res)
	}
}

func TestCheckLinks_DuplicateUnreachableOrOverlappingRooms(t *testing.T) {
	cases := map[string]struct {
		h    House
		want string
	}{
		`listed twice`: {House{RoomIds: []int{testUnitA, testUnitA}}, `listed twice`},
		`unreachable`:  {House{RoomIds: []int{testUnitA, testUnitB}}, `no doorway`},
		// A ring of four doorways that comes back one room over: 6474 is
		// placed where the entry is.
		`same place`: {House{RoomIds: []int{6470, 6471, 6472, 6473, 6474}, Links: []RoomLink{
			{From: 6470, Direction: `north`, To: 6471},
			{From: 6471, Direction: `east`, To: 6472},
			{From: 6472, Direction: `south`, To: 6473},
			{From: 6473, Direction: `west`, To: 6474},
		}}, `same place`},
	}
	for name, c := range cases {
		err := c.h.checkLinks()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error about %q", name, err, c.want)
		}
	}
	ok := House{RoomIds: []int{testUnitA, testUnitB}, Links: []RoomLink{{From: testUnitA, Direction: `north`, To: testUnitB}}}
	if err := ok.checkLinks(); err != nil {
		t.Errorf("a good house was rejected: %v", err)
	}
}

// Any change to a saved field of an item left in a lodging is noticed, not
// only a change of which items are there.
func TestCapture_NoticesAnItemChangedInPlace(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	r := w.room(home)
	r.Items = append(r.Items, trinket(testKeyId))
	Capture(r)

	r.Items[0].Uses = 0
	r.Items[0].MakerName = `Alice`
	if !Capture(r) {
		t.Fatal("an in-place change was not captured")
	}
	if f, _ := floorOf(t, 1, home); f.Items[0].MakerName != `Alice` {
		t.Error("the record does not hold the change")
	}
	if Capture(r) {
		t.Error("an unchanged floor was written again")
	}
}

package housing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// containerWorld routes the live-room and mob seams to rooms and mobs the
// test builds, so the guards and capture run without a room manager.
type containerWorld struct {
	*guestWorld
	rooms   map[int]*rooms.Room
	mobAt   map[int]int  // mob instance -> room
	charmed map[int]int  // mob instance -> the user it follows
	saved   map[int]bool // users saveUser was called for
}

func newContainerWorld(t *testing.T) *containerWorld {
	t.Helper()
	w := &containerWorld{guestWorld: newGuestWorld(t), rooms: map[int]*rooms.Room{}, mobAt: map[int]int{}, charmed: map[int]int{}, saved: map[int]bool{}}
	prevRoom, prevLoaded, prevMobRoom, prevCharm, prevSave := liveRoom, roomLoaded, mobRoom, mobCharmedBy, saveUser
	liveRoom = func(id int) *rooms.Room { return w.rooms[id] }
	roomLoaded = func(id int) bool { _, ok := w.rooms[id]; return ok }
	mobRoom = func(id int) (int, bool) { r, ok := w.mobAt[id]; return r, ok }
	mobCharmedBy = func(id, userId int) bool { return w.charmed[id] == userId }
	saveUser = func(u *users.UserRecord) { w.saved[u.UserId] = true }
	t.Cleanup(func() {
		liveRoom, roomLoaded, mobRoom, mobCharmedBy, saveUser = prevRoom, prevLoaded, prevMobRoom, prevCharm, prevSave
	})
	return w
}

// room returns the live unit room, building it through the load overlay the
// first time, as LoadRoomInstance would.
func (w *containerWorld) room(id int) *rooms.Room {
	if r, ok := w.rooms[id]; ok {
		return r
	}
	r := unitRoom(id)
	ApplyOverlay(r)
	w.rooms[id] = r
	return r
}

// reload drops the live rooms, as a restart does, and reloads the houses
// from disk.
func (w *containerWorld) reload(t *testing.T) {
	t.Helper()
	w.rooms = map[int]*rooms.Room{}
	ResetForTest()
	AddBuildingForTest(testBuilding())
	if _, held := loadHouses(); held != 0 {
		t.Fatalf("reload held %d houses", held)
	}
}

// lodger is an online player who owns a home and holds one deed of the kind.
func (w *containerWorld) lodger(t *testing.T, id int, name string, offer string) *users.UserRecord {
	t.Helper()
	u := w.player(id, name, 100000)
	if res, _ := buy(t, u); res != PurchaseOk {
		t.Fatalf("setup: home purchase %v", res)
	}
	u.Character.RoomId = w.homeOf(t, u)
	buyKey(u, offer)
	return u
}

func (w *containerWorld) homeOf(t *testing.T, u *users.UserRecord) int {
	t.Helper()
	h, ok := HouseOf(u.UserId, testBldgId)
	if !ok {
		t.Fatal("setup: no house")
	}
	return h.EntryRoom()
}

// place uses the lodger's deed of itemId in roomId under name.
func (w *containerWorld) place(t *testing.T, u *users.UserRecord, itemId int, roomId int, name string) {
	t.Helper()
	UseItem(u, w.room(roomId), itemOf(u, itemId), name, `deed `+name)
	if _, ok := w.room(roomId).Containers[name]; !ok {
		t.Fatalf("setup: %s was not placed", name)
	}
}

func trinket(id int) items.Item {
	itm := items.New(testKeyId) // any seeded spec will do
	itm.ItemId = id
	return itm
}

func houseContainer(t *testing.T, userId int, name string) HouseContainer {
	t.Helper()
	h, _ := HouseOf(userId, testBldgId)
	for _, c := range h.Containers {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("house has no container %q: %+v", name, h.Containers)
	return HouseContainer{}
}

// ── The list ───────────────────────────────────────────────────────────────

func TestOffers_ContainersPricedAndNeedAHome(t *testing.T) {
	setup(t)
	u := testUser(1, 100000, 0)
	o := offerMap(u)
	if o[OfferContainer].Available || o[OfferStrongbox].Available {
		t.Error("containers offered to someone with no home")
	}
	buy(t, u)
	o = offerMap(u)
	if !o[OfferContainer].Available || o[OfferContainer].Price != 250 {
		t.Errorf("container = %+v, want available at 250", o[OfferContainer])
	}
	if !o[OfferStrongbox].Available || o[OfferStrongbox].Price != 500 {
		t.Errorf("strongbox = %+v, want available at 500", o[OfferStrongbox])
	}

	gold := u.Character.Gold
	buyKey(u, OfferContainer)
	buyKey(u, OfferStrongbox)
	if !has(u, testBoxId) || !has(u, testSafeId) {
		t.Fatal("deeds not handed over")
	}
	if spent := gold - u.Character.Gold; spent != 750 {
		t.Errorf("charged %d, want 750", spent)
	}
}

func TestBuyContainer_RefusedWithoutAHome(t *testing.T) {
	setup(t)
	u := testUser(1, 100000, 0)
	buyKey(u, OfferContainer)
	buyKey(u, OfferStrongbox)
	if has(u, testBoxId) || has(u, testSafeId) || u.Character.Gold != 100000 {
		t.Error("a homeless player was sold a container deed")
	}
}

func TestMatchOffer_Containers(t *testing.T) {
	cases := map[string]string{
		`container deed`:       OfferContainer,
		`a container`:          OfferContainer,
		`deed for a container`: OfferContainer,
		`strongbox`:            OfferStrongbox,
		`strongbox deed`:       OfferStrongbox,
		`lockable container`:   OfferStrongbox,
		`private chest`:        OfferStrongbox,
		`deed`:                 OfferExtension,
		`extension deed`:       OfferExtension,
	}
	for ask, want := range cases {
		if got, _ := MatchOffer(ask, true); got != want {
			t.Errorf("MatchOffer(%q) = %q, want %q", ask, got, want)
		}
	}
}

// ── Placing ────────────────────────────────────────────────────────────────

func TestUseContainerDeed_PlacesANamedContainerAndSavesIt(t *testing.T) {
	dir := setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)

	if !UseItem(u, w.room(home), itemOf(u, testBoxId), `mug`, `container deed mug`) {
		t.Fatal("deed not handled")
	}
	if _, ok := w.room(home).Containers[`mug`]; !ok {
		t.Fatal("the live room has no mug")
	}
	if has(u, testBoxId) {
		t.Error("the deed should be used up")
	}
	c := houseContainer(t, 1, `mug`)
	if c.RoomId != home || c.OwnerOnly {
		t.Errorf("record = %+v", c)
	}
	data, err := os.ReadFile(filepath.Join(dir, testBldgId, `6470.yaml`))
	if err != nil || !strings.Contains(string(data), `name: mug`) {
		t.Errorf("house file does not hold the mug: %v\n%s", err, data)
	}
}

func TestUseContainerDeed_PromptsForTheName(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferStrongbox)
	home := w.homeOf(t, u)

	UseItem(u, w.room(home), itemOf(u, testSafeId), ``, `strongbox deed`)
	if len(w.room(home).Containers) != 0 || !has(u, testSafeId) {
		t.Fatal("placed before a name was given")
	}
	if u.GetPrompt() == nil {
		t.Fatal("no name prompt")
	}
}

func TestUseContainerDeed_OnlyInYourOwnHome(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	alice := w.lodger(t, 1, `Alice`, OfferContainer)
	bob := w.lodger(t, 2, `Bob`, OfferStrongbox)
	letIn(t, alice, bob) // even a guest in Alice's home

	places := map[string]*rooms.Room{
		`the street`:      worldRoom(),
		`the alley`:       alleyRoom(),
		`a vacant unit`:   unitRoom(testUnitC),
		`another's house`: w.room(w.homeOf(t, alice)),
	}
	for where, r := range places {
		before := len(r.Containers)
		UseItem(bob, r, itemOf(bob, testSafeId), `safe`, `strongbox deed safe`)
		if len(r.Containers) != before || !has(bob, testSafeId) {
			t.Errorf("%s: the strongbox deed took", where)
		}
	}
	UseItem(alice, worldRoom(), itemOf(alice, testBoxId), `mug`, `container deed mug`)
	if !has(alice, testBoxId) {
		t.Error("the container deed took in the street")
	}
	for _, id := range []int{1, 2} {
		if h, _ := HouseOf(id, testBldgId); len(h.Containers) != 0 {
			t.Errorf("house %d gained a container: %+v", id, h.Containers)
		}
	}
}

func TestUseContainerDeed_NameRules(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	w.room(home) // the live room, with its door exit and door noun

	for _, bad := range []string{`x`, `two words`, `mug2`, `all`, `door`, `north`, strings.Repeat(`a`, 17), `<b>`} {
		UseItem(u, w.room(home), itemOf(u, testBoxId), bad, `container deed `+bad)
		if !has(u, testBoxId) {
			t.Fatalf("%q was accepted as a name", bad)
		}
	}
	w.place(t, u, testBoxId, home, `mug`)
	buyKey(u, OfferContainer)
	UseItem(u, w.room(home), itemOf(u, testBoxId), `mug`, `container deed mug`)
	if !has(u, testBoxId) {
		t.Error("a second mug in the same room was accepted")
	}
	// Upper case is folded, not refused.
	UseItem(u, w.room(home), itemOf(u, testBoxId), `Vase`, `container deed Vase`)
	if _, ok := w.room(home).Containers[`vase`]; !ok {
		t.Error("Vase was not placed as vase")
	}
}

func TestUseContainerDeed_LimitIsPerRoom(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	w.place(t, u, testBoxId, home, `mug`)
	buyKey(u, OfferStrongbox)
	w.place(t, u, testSafeId, home, `safe`)

	// The fixture allows 2 per room. A one-room lodging is now full.
	if o := offerMap(u); o[OfferContainer].Available || o[OfferStrongbox].Available {
		t.Error("still offered with every room full")
	}
	u.Character.StoreItem(items.New(testBoxId)) // one bought before the room filled
	UseItem(u, w.room(home), itemOf(u, testBoxId), `vase`, `container deed vase`)
	if !has(u, testBoxId) || len(w.room(home).Containers) != 2 {
		t.Fatal("a third container was placed in a room past the per-room limit (2)")
	}

	// A second room has its own allowance: no limit for the house as a whole.
	buyKey(u, OfferExtension)
	UseItem(u, w.room(home), itemOf(u, testDeedId), `north`, `deed north`)
	h, _ := HouseOf(1, testBldgId)
	if len(h.RoomIds) != 2 {
		t.Fatal("setup: no second room")
	}
	if o := offerMap(u); !o[OfferContainer].Available {
		t.Error("not offered though the new room has space")
	}
	back := h.RoomIds[1]
	w.place(t, u, testBoxId, back, `vase`)
	buyKey(u, OfferContainer)
	w.place(t, u, testBoxId, back, `shelf`)
	if h, _ := HouseOf(1, testBldgId); len(h.Containers) != 4 {
		t.Errorf("house holds %d containers, want 4 across two rooms", len(h.Containers))
	}
}

// ── Contents: capture, persistence, load ───────────────────────────────────

func TestCapture_WritesContentsToTheHouseAndSurvivesARestart(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	w.place(t, u, testBoxId, home, `mug`)

	// "put spoon in mug": the command changes the live room, then world.go
	// calls AfterUserCommand.
	r := w.room(home)
	mug := r.Containers[`mug`]
	spoon := trinket(testKeyId)
	mug.Items = append(mug.Items, spoon)
	mug.Gold = 12
	r.Containers[`mug`] = mug
	AfterUserCommand(1, home)

	c := houseContainer(t, 1, `mug`)
	if len(c.Items) != 1 || !c.Items[0].Equals(spoon) || c.Gold != 12 {
		t.Fatalf("record = %+v", c)
	}
	if !w.saved[1] {
		t.Error("the player was not saved alongside the house")
	}

	w.reload(t)
	back := w.room(home).Containers[`mug`]
	// Item UUIDs are minted fresh on every load, so compare what persists.
	if len(back.Items) != 1 || back.Items[0].ItemId != spoon.ItemId || back.Items[0].UUID.IsNil() || back.Gold != 12 {
		t.Fatalf("after restart the mug holds %+v", back)
	}
	if Capture(w.room(home)) {
		t.Error("a freshly loaded room differs from its own record")
	}

	// Taking it out again is captured too.
	back.Items, back.Gold = nil, 0
	w.room(home).Containers[`mug`] = back
	AfterUserCommand(1, home)
	if c := houseContainer(t, 1, `mug`); len(c.Items) != 0 || c.Gold != 0 {
		t.Errorf("record after emptying = %+v", c)
	}
}

func TestCapture_NoChangeWritesNothing(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	w.place(t, u, testBoxId, home, `mug`)
	if Capture(w.room(home)) {
		t.Error("capture reported a change when nothing changed")
	}
	w.saved = map[int]bool{}
	AfterUserCommand(1, home)
	if w.saved[1] {
		t.Error("the player was saved though nothing changed")
	}
}

func TestCapture_OnAutosaveAndForMobs(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	w.place(t, u, testBoxId, home, `mug`)
	r := w.room(home)

	mug := r.Containers[`mug`]
	mug.Gold = 5
	r.Containers[`mug`] = mug
	CaptureOnSave(r)
	if houseContainer(t, 1, `mug`).Gold != 5 {
		t.Error("autosave did not capture")
	}

	w.mobAt[900] = home
	mug.Gold = 9
	r.Containers[`mug`] = mug
	AfterMobCommand(900, home)
	if houseContainer(t, 1, `mug`).Gold != 9 {
		t.Error("a mob's command was not captured")
	}
}

func TestCapture_RestoresAContainerMissingFromTheRoom(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	w.place(t, u, testBoxId, home, `mug`)
	r := w.room(home)
	mug := r.Containers[`mug`]
	mug.Gold = 3
	r.Containers[`mug`] = mug
	Capture(r)

	delete(r.Containers, `mug`)
	Capture(r)
	if got, ok := r.Containers[`mug`]; !ok || got.Gold != 3 {
		t.Errorf("mug not restored from the record: %+v %v", got, ok)
	}
	if houseContainer(t, 1, `mug`).Gold != 3 {
		t.Error("the record lost the mug's contents")
	}
}

func TestLoad_HouseRecordIsTheOnlySourceOfContainers(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	w.place(t, u, testBoxId, home, `mug`)

	// Whatever an old instance save put in the room is replaced.
	r := unitRoom(home)
	r.Containers = map[string]rooms.Container{`crate`: {Gold: 999}, `mug`: {Gold: 999}}
	ApplyOverlay(r)
	if _, stale := r.Containers[`crate`]; stale || r.Containers[`mug`].Gold != 0 {
		t.Errorf("containers after load = %+v", r.Containers)
	}

	// A vacant unit holds none.
	vacant := unitRoom(testUnitC)
	vacant.Containers = map[string]rooms.Container{`crate`: {Gold: 999}}
	ApplyOverlay(vacant)
	if len(vacant.Containers) != 0 {
		t.Errorf("vacant unit kept %+v", vacant.Containers)
	}
}

func TestLayoutRefresh_KeepsLiveContents(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	w.place(t, u, testBoxId, home, `mug`)
	r := w.room(home)
	mug := r.Containers[`mug`]
	mug.Gold = 7 // not yet captured
	r.Containers[`mug`] = mug

	buyKey(u, OfferRedecorate)
	const text = `A snug room with a mug on the sill.`
	voucher := itemOf(u, testVoucherId)
	UseItem(u, r, voucher, text, `voucher `+text)
	u.GetPrompt().GetNextQuestion().Answer(`yes`)
	UseItem(u, r, voucher, text, `voucher `+text)
	if r.Description != text {
		t.Fatal("setup: the voucher did not take")
	}
	if r.Containers[`mug`].Gold != 7 {
		t.Errorf("redecorating reset the mug: %+v", r.Containers[`mug`])
	}

	buyKey(u, OfferExtension)
	UseItem(u, r, itemOf(u, testDeedId), `north`, `deed north`)
	if _, ok := r.Exits[`north`]; !ok {
		t.Fatal("setup: the deed did not take")
	}
	if r.Containers[`mug`].Gold != 7 {
		t.Errorf("extending reset the mug: %+v", r.Containers[`mug`])
	}
}

func TestLoad_BadContainersAreHeld(t *testing.T) {
	for name, bad := range map[string]HouseContainer{
		`room not in house`: {RoomId: testUnitC, Name: `mug`},
		`bad name`:          {RoomId: testUnitA, Name: `Mug 2`},
	} {
		t.Run(name, func(t *testing.T) {
			setup(t)
			h := House{BuildingId: testBldgId, OwnerUserId: 1, RoomIds: []int{testUnitA}, Containers: []HouseContainer{bad}}
			if err := h.checkLinks(); err == nil {
				t.Error("checkLinks accepted it")
			}
		})
	}
	setup(t)
	dup := House{BuildingId: testBldgId, OwnerUserId: 1, RoomIds: []int{testUnitA},
		Containers: []HouseContainer{{RoomId: testUnitA, Name: `mug`}, {RoomId: testUnitA, Name: `mug`}}}
	if err := dup.checkLinks(); err == nil {
		t.Error("two mugs in one room accepted")
	}
}

// ── Who may use them ───────────────────────────────────────────────────────

func strongboxHouse(t *testing.T) (*containerWorld, *users.UserRecord, *users.UserRecord, int) {
	t.Helper()
	setup(t)
	w := newContainerWorld(t)
	alice := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, alice)
	w.place(t, alice, testBoxId, home, `mug`)
	buyKey(alice, OfferStrongbox)
	w.place(t, alice, testSafeId, home, `coffer`)
	bob := w.player(2, `Bob`, 1000)
	letIn(t, alice, bob)
	bob.Character.RoomId = home
	return w, alice, bob, home
}

func TestStrongbox_PlacedHydratedAndRestoredShut(t *testing.T) {
	w, _, _, home := strongboxHouse(t)
	if c := w.room(home).Containers[`coffer`]; !c.IsSealedShut() {
		t.Fatalf("a placed strongbox is not shut: %+v", c)
	}
	if c := w.room(home).Containers[`mug`]; c.Sealed || c.Lock.IsLocked() {
		t.Fatalf("a shared container is sealed: %+v", c)
	}
	w.reload(t)
	if c := w.room(home).Containers[`coffer`]; !c.IsSealedShut() {
		t.Fatal("a strongbox is not shut after a restart")
	}
	delete(w.room(home).Containers, `coffer`)
	Capture(w.room(home))
	if c := w.room(home).Containers[`coffer`]; !c.IsSealedShut() {
		t.Fatal("a restored strongbox is not shut")
	}
}

func TestStrongbox_OpensForTheOwnersCommandOnly(t *testing.T) {
	w, alice, bob, home := strongboxHouse(t)
	r := w.room(home)

	shut := OpenStrongboxesForUser(bob.UserId, home)
	if !r.Containers[`coffer`].IsSealedShut() {
		t.Fatal("a guest's command opened the strongbox")
	}
	shut()

	shut = OpenStrongboxesForUser(alice.UserId, home)
	if r.Containers[`coffer`].Lock.IsLocked() {
		t.Fatal("the owner's command did not open the strongbox")
	}
	// A nested command sees it open already and must not shut it early.
	inner := OpenStrongboxesForUser(alice.UserId, home)
	inner()
	if r.Containers[`coffer`].Lock.IsLocked() {
		t.Fatal("a nested command shut the strongbox under the outer one")
	}
	shut()
	if !r.Containers[`coffer`].IsSealedShut() {
		t.Fatal("the strongbox stayed open after the owner's command")
	}
	if r.Containers[`mug`].Lock.IsLocked() {
		t.Error("shutting strongboxes locked the shared container")
	}
}

func TestStrongbox_StaffAndOnlyInHouses(t *testing.T) {
	w, _, _, home := strongboxHouse(t)
	prev := isStaff
	isStaff = func(id int) bool { return id == 3 }
	defer func() { isStaff = prev }()
	shut := OpenStrongboxesForUser(3, home)
	if w.room(home).Containers[`coffer`].Lock.IsLocked() {
		t.Error("staff could not open it")
	}
	shut()
	OpenStrongboxesForUser(1, 5615)() // not a house: nothing to do, no panic
}

func TestStrongbox_Mobs(t *testing.T) {
	w, alice, bob, home := strongboxHouse(t)
	r := w.room(home)
	w.charmed[900] = alice.UserId
	w.charmed[901] = bob.UserId

	shut := OpenStrongboxesForMob(901, home)
	if !r.Containers[`coffer`].IsSealedShut() {
		t.Error("a guest's companion opened the strongbox")
	}
	shut()
	shut = OpenStrongboxesForMob(902, home)
	if !r.Containers[`coffer`].IsSealedShut() {
		t.Error("a stray mob opened the strongbox")
	}
	shut()
	shut = OpenStrongboxesForMob(900, home)
	if r.Containers[`coffer`].Lock.IsLocked() {
		t.Error("the owner's companion could not open it")
	}
	shut()
	if !r.Containers[`coffer`].IsSealedShut() {
		t.Error("left open after the companion's command")
	}
}

// Deeds already bought and carried count against the free corners, so no deed
// is sold that could never be placed.
func TestBuyContainer_CountsCarriedDeeds(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer) // carries one; the room takes 2
	buyKey(u, OfferStrongbox)                    // carries two: every corner promised
	gold := u.Character.Gold
	buyKey(u, OfferContainer)
	if u.Character.Gold != gold {
		t.Error("sold a deed with every free corner already promised to a carried deed")
	}
	if o := offerMap(u); o[OfferContainer].Available {
		t.Errorf("offered with no free corner left: %+v", o[OfferContainer])
	}
}

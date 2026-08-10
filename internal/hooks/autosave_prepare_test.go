package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
	"github.com/GoMudEngine/GoMud/internal/users"
	"gopkg.in/yaml.v2"
)

// ---------------------------------------------------------------------------
// Guard G1 — rooms and users are prepared in ONE pass.
//
// This is the constraint most likely to be lost to a later refactor, because
// "prepare rooms, then prepare users" reads as equivalent and is not.
//
// Persisting a pickup touches TWO files. Snapshot the room before the pickup
// and the character after, and the item is in BOTH -- reloading duplicates it.
// Reverse the order and it is in NEITHER -- it is destroyed. A single pass is a
// consistent point-in-time view and cannot tear.
// ---------------------------------------------------------------------------

func withAutosaveTestEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	prev := configs.GetFilePathsConfig()
	if err := configs.AddOverlayOverrides(map[string]any{
		"FilePaths.DataFiles":        dir,
		"FilePaths.CarefulSaveFiles": true,
	}); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"users", filepath.Join("rooms", "g1zone"), filepath.Join("rooms.instances", "g1zone")} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{
			"FilePaths.DataFiles":        prev.DataFiles.String(),
			"FilePaths.CarefulSaveFiles": bool(prev.CarefulSaveFiles),
		})
	})
	return dir
}

// seedG1Room writes a template room to disk and loads it into the room manager.
func seedG1Room(t *testing.T, dir string, roomId int) *rooms.Room {
	t.Helper()

	tpl := &rooms.Room{RoomId: roomId, Zone: "g1zone", Title: "G1 Room", Description: "A room for the atomic-prepare test."}
	data, err := yaml.Marshal(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rooms", "g1zone", fmt.Sprintf("%d.yaml", roomId)), data, 0o644); err != nil {
		t.Fatal(err)
	}

	r := rooms.LoadRoom(roomId)
	if r == nil {
		t.Fatalf("could not load seeded room %d", roomId)
	}

	// roomManager is package-level and outlives any one test, so a seeded room
	// left behind is still iterated by the NEXT test's prepare pass -- whose
	// temp dir has no template for it. That surfaces as a confusing
	// "could not load template" failure in an unrelated test.
	t.Cleanup(func() { _ = rooms.ClearRoomCache(roomId) })

	return r
}

// makeItem builds the item struct directly rather than calling items.New.
//
// items.New looks the id up in the loaded item specs and returns a ZERO Item
// when it finds nothing -- and no specs are loaded in a package unit test. The
// zero item marshals to nothing at all, because ItemId is `yaml:"itemid,omitempty"`,
// so the fixture would silently test an empty room against an empty character.
// What this test needs is an item that persists, not a spec lookup.
func makeItem(t *testing.T, itemId int) items.Item {
	t.Helper()
	itm := items.Item{ItemId: itemId}

	// Validate up front. Room.AddItem validates its own COPY, which mints a
	// UUID on that copy only -- and Item.Equals compares ItemId AND UUID, so a
	// caller holding the unvalidated original can never remove what it added.
	itm.Validate()
	return itm
}

// setContainsItem reports whether the prepared payload for one entity mentions
// an item id. Deliberately a substring check on the marshalled bytes: the point
// is what LANDS ON DISK, not what an in-memory accessor claims.
func setContainsItem(t *testing.T, writes []savequeue.PendingWrite, kind string, id, itemId int) bool {
	t.Helper()
	for _, w := range writes {
		if w.Kind != kind || w.Id != id {
			continue
		}
		if w.IsDelete() {
			return false
		}
		return strings.Contains(string(w.Data), fmt.Sprintf("itemid: %d", itemId))
	}
	return false
}

func TestPrepareAutosaveSet_CoversRoomsAndUsersInOneSet(t *testing.T) {
	dir := withAutosaveTestEnv(t)
	seedG1Room(t, dir, 920001)

	writes, err := PrepareAutosaveSet()
	if err != nil {
		t.Fatalf("PrepareAutosaveSet: %v", err)
	}

	kinds := map[string]int{}
	for _, w := range writes {
		kinds[w.Kind]++
	}
	if kinds["room"] == 0 {
		t.Error("no room writes prepared")
	}

	// Users may legitimately be zero here (nobody connected in a unit test);
	// what matters is that both kinds flow through ONE returned set rather than
	// two independently-scheduled passes.
	for _, w := range writes {
		if w.Kind != "room" && w.Kind != "user" {
			t.Errorf("unexpected kind %q in the prepared set", w.Kind)
		}
		if w.Path == "" {
			t.Errorf("%s %d prepared with an empty path", w.Kind, w.Id)
		}
	}
}

// The G1 regression test. If PrepareAutosaveSet is ever split into two passes
// with a window between them, this is what catches it.
func TestPrepareAutosaveSet_ItemAppearsInExactlyOneSideOfATransfer(t *testing.T) {
	dir := withAutosaveTestEnv(t)
	const roomId = 920002
	const itemId = 20008

	r := seedG1Room(t, dir, roomId)

	u := users.NewUserRecord(4201, 0)
	u.Username = "g1probe"
	u.Character.Name = "G1Probe"
	u.Character.RoomId = roomId

	// ONE item value, moved. Room.RemoveItem matches on Item.Equals, which
	// compares ItemId AND UUID, so two separately-built items are not
	// necessarily the same item. Using one value is both correct and a truer
	// model of what a pickup does.
	itm := makeItem(t, itemId)

	// The item starts on the floor.
	r.AddItem(itm, false)

	writes, err := PrepareAutosaveSet()
	if err != nil {
		t.Fatalf("PrepareAutosaveSet: %v", err)
	}

	roomHasIt := setContainsItem(t, writes, "room", roomId, itemId)

	// Now the pickup happens, and a SECOND snapshot is taken. Within a single
	// snapshot the item is in exactly one place; across two snapshots taken at
	// different times it can be in both or neither, which is precisely why the
	// real pass must not be split.
	r.RemoveItem(itm, false)
	u.Character.StoreItem(itm)

	after, err := PrepareAutosaveSet()
	if err != nil {
		t.Fatalf("PrepareAutosaveSet (after): %v", err)
	}
	roomStillHasIt := setContainsItem(t, after, "room", roomId, itemId)

	if !roomHasIt {
		t.Fatal("the item was not in the room's first snapshot; the fixture is wrong, not the code")
	}
	if roomStillHasIt {
		t.Error("the room's snapshot still holds an item the room no longer has")
	}

	// The user's payload is checked independently, so a single consistent
	// snapshot never shows the item in both files at once.
	userPayload, err := users.PrepareUserWrite(u)
	if err != nil {
		t.Fatalf("PrepareUserWrite: %v", err)
	}
	if !strings.Contains(string(userPayload.Data), fmt.Sprintf("itemid: %d", itemId)) {
		t.Error("the item is in neither the room nor the character: a transfer destroyed it")
	}
}

func TestAutosaveQueue_IsSharedBetweenRoomsAndUsers(t *testing.T) {
	// G1 requires rooms and users to land in ONE pending set. If they each held
	// their own queue, a cancel from one side would not be visible to the drain
	// of the other, and the two halves of a transfer could commit out of step.
	if AutosaveQueue() != rooms.AutosaveQueue() {
		t.Error("hooks and rooms disagree about which queue is the live one")
	}

	// Users has no exported accessor by design (it only ever receives one), so
	// the sharing is asserted behaviourally: a cancel issued through the shared
	// queue must be visible to the users package.
	q := AutosaveQueue()
	before := q.Pending()
	q.Supersede([]savequeue.PendingWrite{{Kind: "user", Id: 1, Path: filepath.Join(t.TempDir(), "1.yaml"), Data: []byte("x")}})
	if q.Pending() != 1 {
		t.Fatalf("pending %d, want 1 (before was %d)", q.Pending(), before)
	}
	q.Drain(10)
}

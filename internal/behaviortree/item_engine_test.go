package behaviortree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

const (
	engineProbeItemId = 9901
	engineProbeRoomId = 9900
	engineProbeMobId  = 9902
	engineProbeInstId = 9903
)

// countingTree counts its item_idle visits in the item's own state.
const countingTree = `
tree:
  type: action
  event: item_idle
  do: increment_state
  key: visits
`

// seedItemEngineWorld seeds one treed item template, one player (user 1)
// and one mob instance, all in one loaded room.
func seedItemEngineWorld(t *testing.T) *users.UserRecord {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		engineProbeItemId: {ItemId: engineProbeItemId, Name: "Probe Lamp", Behavior: "engine_probe"},
	}))
	LoadItemTreeForTest(t, "engine_probe", countingTree)
	room := newProbeRoom(t)
	room.RoomId = engineProbeRoomId
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{engineProbeRoomId: room}, map[string]*rooms.ZoneConfig{}))
	u := users.NewTestUser(1, "probe", "Probe", 0)
	u.Character.RoomId = engineProbeRoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: u}))
	t.Cleanup(seedTestMob(t, engineProbeMobId, engineProbeInstId, engineProbeRoomId, "Probe Keeper"))
	return u
}

// newProbeRoom is rooms.NewRoom with the config isolated: NewRoom persists
// Server.NextRoomId, and without a scratch CONFIG_PATH that write lands in the
// real world's config-overrides.yaml, carrying whatever DataFiles an earlier
// test's override left behind into every other package's test binary.
func newProbeRoom(t *testing.T) *rooms.Room {
	t.Helper()
	configs.SetConfigWithLookupsForTest(t, configs.GetConfig())
	return rooms.NewRoom("probe")
}

func idleEvent() EventContext { return EventContext{EventType: "item_idle"} }

// Rule 3: the tree runs for an item wherever it is, its state is per item
// instance (UUID), and the subject's room is the holder's room.
func TestTryItemBehaviorRunsWornCarriedMobHeldAndFloor(t *testing.T) {
	seedItemEngineWorld(t)
	t.Cleanup(ResetItemBTreeStatesForTest())

	subjects := map[string]ItemSubject{
		"worn":     {UUID: uuid.UUID{1}, ItemId: engineProbeItemId, UserId: 1, Slot: "light"},
		"backpack": {UUID: uuid.UUID{2}, ItemId: engineProbeItemId, UserId: 1},
		"mob-held": {UUID: uuid.UUID{3}, ItemId: engineProbeItemId, MobInstanceId: engineProbeInstId, Slot: "light"},
		"floor":    {UUID: uuid.UUID{4}, ItemId: engineProbeItemId, RoomId: engineProbeRoomId, OnFloor: true},
	}
	for name, s := range subjects {
		if !TryItemBehavior(idleEvent(), s) {
			t.Errorf("%s: TryItemBehavior = false, want true (the tree succeeds)", name)
		}
	}
	TryItemBehavior(idleEvent(), subjects["worn"])
	if got := ItemBTreeStateForTest(uuid.UUID{1}).GetInt("visits"); got != 2 {
		t.Errorf("worn item visits = %d, want 2", got)
	}
	for _, id := range []uuid.UUID{{2}, {3}, {4}} {
		if got := ItemBTreeStateForTest(id).GetInt("visits"); got != 1 {
			t.Errorf("item %v visits = %d, want 1: state is per instance", id, got)
		}
	}
}

// Rule 3 and Rule 14: a holder or room that is gone skips the round.
func TestTryItemBehaviorSkipsAGoneHolder(t *testing.T) {
	seedItemEngineWorld(t)
	t.Cleanup(ResetItemBTreeStatesForTest())
	for name, s := range map[string]ItemSubject{
		"logged-out player": {UUID: uuid.UUID{5}, ItemId: engineProbeItemId, UserId: 77},
		"despawned mob":     {UUID: uuid.UUID{6}, ItemId: engineProbeItemId, MobInstanceId: 7777},
		"unloaded room":     {UUID: uuid.UUID{7}, ItemId: engineProbeItemId, RoomId: 123456, OnFloor: true},
		"no holder at all":  {UUID: uuid.UUID{8}, ItemId: engineProbeItemId},
		"untreed item":      {UUID: uuid.UUID{9}, ItemId: 1, UserId: 1},
	} {
		if TryItemBehavior(idleEvent(), s) {
			t.Errorf("%s: TryItemBehavior = true, want false", name)
		}
	}
}

// Rule 14: a panic inside a node is recovered, logged, and the item does
// nothing that round; the next item still runs.
func TestTryItemBehaviorRecoversAPanickingNode(t *testing.T) {
	seedItemEngineWorld(t)
	t.Cleanup(ResetItemBTreeStatesForTest())
	actionRegistry["probe_panic"] = func(map[string]any, *EvalContext) Result { panic("probe") }
	itemSafeActions["probe_panic"] = true
	t.Cleanup(func() {
		delete(actionRegistry, "probe_panic")
		delete(itemSafeActions, "probe_panic")
	})
	LoadItemTreeForTest(t, "engine_probe", "tree:\n  type: action\n  do: probe_panic\n")
	if TryItemBehavior(idleEvent(), ItemSubject{UUID: uuid.UUID{1}, ItemId: engineProbeItemId, UserId: 1}) {
		t.Error("a panicking tree reported Success")
	}
}

// Rule 8: an item tree may name only the item-safe allowlist; anything else
// refuses at compile time with its path.
func TestItemTreeAllowlistRefusesMobNodes(t *testing.T) {
	for _, bad := range []string{
		"tree:\n  type: action\n  do: attack\n",
		"tree:\n  type: selector\n  children:\n    - type: condition\n      check: mob_in_combat\n",
	} {
		_, err := LoadItemTreeFromBytes([]byte(bad))
		if err == nil || !strings.Contains(err.Error(), "item tree") || !strings.Contains(err.Error(), "item") {
			t.Errorf("compiling %q: err = %v, want an item-tree refusal naming the path", bad, err)
		}
	}
	ok := "tree:\n  type: decorator\n  mod: cooldown\n  rounds: 3\n  child:\n    type: sequence\n    children:\n      - type: condition\n        check: time_of_day\n        period: night\n      - type: action\n        do: set_state\n        key: k\n        value: v\n"
	if _, err := LoadItemTreeFromBytes([]byte(ok)); err != nil {
		t.Errorf("an all-safe item tree refused: %v", err)
	}
}

// X12: a behavior: naming no file, or a file that does not compile, is a
// boot failure that names the item.
func TestValidateItemBehaviorsRefusesAMissingOrBrokenTree(t *testing.T) {
	dir := overrideBTDataFilesDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "behaviors", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "behaviors", "items", "broken_probe.yaml"),
		[]byte("tree:\n  type: action\n  do: attack\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		9911: {ItemId: 9911, Name: "Lost Lamp", Behavior: "no_such_tree"},
		9912: {ItemId: 9912, Name: "Bad Lamp", Behavior: "broken_probe"},
		9913: {ItemId: 9913, Name: "Plain Rock"},
	}))
	t.Cleanup(func() {
		e := GetEngine()
		e.EvictItemTree("no_such_tree")
		e.EvictItemTree("broken_probe")
	})
	err := ValidateItemBehaviors()
	if err == nil {
		t.Fatal("ValidateItemBehaviors passed a missing tree and a broken one")
	}
	for _, want := range []string{"item 9911", "no_such_tree", "item 9912", "broken_probe"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "9913") {
		t.Errorf("error names an item with no behavior: %q", err)
	}
}

// X13: item trees are a fourth kind, and behaviors/items is not a mob zone.
func TestListTreeFiles_ListsItemTreesAsTheirOwnKind(t *testing.T) {
	dir := overrideBTDataFilesDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "behaviors", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"lister_probe.yaml", "424245-numbered_probe.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, "behaviors", "items", name), []byte(countingTree), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	found := false
	for _, r := range ListTreeFiles() {
		if r.Zone == "items" {
			t.Errorf("behaviors/items was listed as a mob zone: %+v", r)
		}
		if r.Kind == "item" && r.Name == "lister_probe" {
			found = true
		}
	}
	if !found {
		t.Error("the item tree lister_probe was not listed as kind item")
	}
}

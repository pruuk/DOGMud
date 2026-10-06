package behaviortree

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The proc node (item behaviour slice 3, Rules 20 to 22).

const (
	procProbeItemId = 90611
	procProbeRoom   = 90612
	procProbeUserId = 90613
	procProbeMobId  = 90614
)

// procProbeTree: a 100% lifesteal on a hit with a 5-round cooldown, and a
// 50% one on a block with none.
const procProbeTree = `
tree:
  type: selector
  children:
    - type: decorator
      event: on_hit
      mod: cooldown
      rounds: 5
      child:
        type: action
        do: proc
        effect: lifesteal
        ratio: 0.5
    - type: decorator
      event: on_block
      mod: random
      percent: 50
      child:
        type: action
        do: proc
        effect: lifesteal
        ratio: 0.5
`

// seedProcWorld: one room, a hurt bearer in it, the probe tree, procs on.
func seedProcWorld(t *testing.T) *users.UserRecord {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.ItemProcsEnabled = true
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		procProbeItemId: {ItemId: procProbeItemId, Name: "Probe Blade", Type: items.Weapon, Hands: 1, Behavior: "proc_probe"},
	}))
	LoadItemTreeForTest(t, "proc_probe", procProbeTree)
	t.Cleanup(ResetItemBTreeStatesForTest())
	room := &rooms.Room{RoomId: procProbeRoom}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{procProbeRoom: room}, map[string]*rooms.ZoneConfig{}))
	u := users.NewTestUser(procProbeUserId, "kesh", "Kesh", 0)
	u.Character.RoomId = procProbeRoom
	u.Character.HealthMax.Value = 1000
	u.Character.Health = 100
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{procProbeUserId: u}))
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(7000)
	return u
}

// fireProbe runs one item instance's tree for a proc event.
func fireProbe(it items.Item, userId int, event string, damage int) bool {
	return TryItemBehavior(EventContext{EventType: event, Proc: &ProcEvent{Damage: damage}},
		ItemSubject{UUID: it.UUID, ItemId: it.ItemId, UserId: userId, Slot: "weapon"})
}

func TestProcLifestealHealsTheHolder(t *testing.T) {
	u := seedProcWorld(t)
	if !fireProbe(items.New(procProbeItemId), u.UserId, "on_hit", 40) {
		t.Fatal("a 100% lifesteal on a 40-damage hit should succeed")
	}
	if u.Character.Health != 120 {
		t.Errorf("health %d, want 120 (half of 40 healed)", u.Character.Health)
	}
}

// A mob holder procs too: today's dispatcher fired a mob's weapon.
func TestProcFiresForAMobHolder(t *testing.T) {
	seedProcWorld(t)
	m := &mobs.Mob{InstanceId: procProbeMobId, Character: characters.Character{
		Name: "Probe Brute", RoomId: procProbeRoom, Conditions: conditions.New(),
	}}
	m.Character.HealthMax.Value = 1000
	m.Character.Health = 100
	mobs.SetInstanceForTest(procProbeMobId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(procProbeMobId, nil) })
	it := items.New(procProbeItemId)
	ok := TryItemBehavior(EventContext{EventType: "on_hit", Proc: &ProcEvent{Damage: 40}},
		ItemSubject{UUID: it.UUID, ItemId: it.ItemId, MobInstanceId: procProbeMobId, Slot: "weapon"})
	if !ok || m.Character.Health <= 100 {
		t.Errorf("mob holder: handled %v, health %d; want a heal", ok, m.Character.Health)
	}
}

// Ruling R1: the cooldown is per template and lives on the holder, so a
// second copy of the item waits it out, and so does a fresh instance after a
// relog (a new UUID, an empty item state, MiscData back from YAML as a
// float).
func TestProcCooldownIsSharedAndSurvivesARelog(t *testing.T) {
	u := seedProcWorld(t)
	first, second := items.New(procProbeItemId), items.New(procProbeItemId)
	if !fireProbe(first, u.UserId, "on_hit", 40) {
		t.Fatal("the first hit should proc")
	}
	key := procCooldownKey(procProbeItemId, "item.selector[0]")
	until, ok := characters.MiscRound(u.Character.GetMiscData(key))
	if !ok || until != 7005 {
		t.Fatalf("holder MiscData %s = %v, want 7005", key, u.Character.GetMiscData(key))
	}
	if fireProbe(second, u.UserId, "on_hit", 40) {
		t.Error("a second copy of the item fired inside the first's cooldown")
	}

	// The relog: item state gone, the round stored as YAML reads it back.
	ResetItemBTreeStatesForTest()
	u.Character.SetMiscData(key, float64(until))
	relogged := items.New(procProbeItemId)
	util.SetRoundCountForTest(7004)
	if fireProbe(relogged, u.UserId, "on_hit", 40) {
		t.Error("a relogged item fired inside the cooldown")
	}
	util.SetRoundCountForTest(7005)
	if !fireProbe(relogged, u.UserId, "on_hit", 40) {
		t.Error("the cooldown should be open at its stored round")
	}
}

// An effect that does nothing (no damage to drain) does not arm the
// cooldown, as markProcCooldown did not.
func TestProcNoOpDoesNotArmTheCooldown(t *testing.T) {
	u := seedProcWorld(t)
	it := items.New(procProbeItemId)
	if fireProbe(it, u.UserId, "on_hit", 0) {
		t.Fatal("a 0-damage lifesteal should not succeed")
	}
	if v := u.Character.GetMiscData(procCooldownKey(procProbeItemId, "item.selector[0]")); v != nil {
		t.Errorf("a no-op armed the cooldown: %v", v)
	}
	if !fireProbe(it, u.UserId, "on_hit", 40) {
		t.Error("the hit after a no-op should proc: the no-op armed a cooldown")
	}
}

// X19: a branch without a random decorator draws no number; one with draws
// exactly one.
func TestProcDrawsOnlyThroughItsRandomDecorator(t *testing.T) {
	u := seedProcWorld(t)
	draws := func(event string) int {
		restore := util.SetRandForTest(99)
		var want []int
		for i := 0; i < 4; i++ {
			want = append(want, util.Rand(1000))
		}
		restore()
		restore = util.SetRandForTest(99)
		defer restore()
		fireProbe(items.New(procProbeItemId), u.UserId, event, 40)
		got := util.Rand(1000)
		for i, w := range want {
			if got == w {
				return i
			}
		}
		t.Fatalf("%s: probe %d matches no early draw", event, got)
		return -1
	}
	if n := draws("on_hit"); n != 0 {
		t.Errorf("a 100%% proc drew %d numbers, want 0", n)
	}
	if n := draws("on_block"); n != 1 {
		t.Errorf("a 50%% proc drew %d numbers, want 1", n)
	}
}

func TestProcOffDoesNothing(t *testing.T) {
	u := seedProcWorld(t)
	cfg := configs.GetConfig()
	cfg.GamePlay.ItemProcsEnabled = false
	configs.SetConfigForTest(t, cfg)
	if fireProbe(items.New(procProbeItemId), u.UserId, "on_hit", 40) || u.Character.Health != 100 {
		t.Errorf("ItemProcsEnabled off: a proc fired (health %d)", u.Character.Health)
	}
}

func TestProcLoadChecks(t *testing.T) {
	branch := func(event, body string) string {
		return "tree:\n  type: selector\n  children:\n    - type: action\n      event: " + event + "\n      do: proc\n" + body
	}
	cases := []struct {
		name, yaml, want string
	}{
		{"idle event", branch("item_idle", "      effect: lifesteal\n"), `proc under event "item_idle"`},
		{"unknown effect", branch("on_hit", "      effect: explode\n"), `proc effect "explode"`},
		{"foreign param", branch("on_hit", "      effect: lifesteal\n      pool: 3\n"), `does not read "pool"`},
		{"non-number", branch("on_hit", "      effect: lifesteal\n      ratio: lots\n"), `want a number`},
		{"random 100", "tree:\n  type: decorator\n  event: on_hit\n  mod: random\n  percent: 100\n  child:\n    type: action\n    do: proc\n    effect: lifesteal\n", `random percent 100`},
		{"cooldown over a mixed selector", "tree:\n  type: decorator\n  event: on_hit\n  mod: cooldown\n  rounds: 3\n  child:\n    type: selector\n    children:\n      - type: action\n        do: set_state\n        key: k\n        value: v\n      - type: action\n        do: proc\n        effect: lifesteal\n", `cooldown over a proc must wrap the proc alone`},
		{"random over a mixed selector", "tree:\n  type: decorator\n  event: on_hit\n  mod: random\n  percent: 50\n  child:\n    type: selector\n    children:\n      - type: action\n        do: set_state\n        key: k\n        value: v\n      - type: action\n        do: proc\n        effect: lifesteal\n", `random over a proc must wrap the proc alone`},
	}
	for _, c := range cases {
		_, _, err := loadItemTreeDef([]byte(c.yaml))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err %v, want %q", c.name, err, c.want)
		}
	}
	if _, _, err := loadItemTreeDef([]byte(branch("on_kill", "      effect: lifesteal\n      ratio: 0.5\n"))); err != nil {
		t.Errorf("a well-formed on_kill proc was refused: %v", err)
	}
	for name, y := range map[string]string{
		"cooldown > random > proc": "tree:\n  type: decorator\n  event: on_hit\n  mod: cooldown\n  rounds: 3\n  child:\n    type: decorator\n    mod: random\n    percent: 50\n    child:\n      type: action\n      do: proc\n      effect: lifesteal\n",
		"cooldown > proc":          "tree:\n  type: decorator\n  event: on_hit\n  mod: cooldown\n  rounds: 3\n  child:\n    type: action\n    do: proc\n    effect: lifesteal\n",
	} {
		if _, _, err := loadItemTreeDef([]byte(y)); err != nil {
			t.Errorf("%s was refused: %v", name, err)
		}
	}
}

func TestProcRefusedOutsideAnItemTree(t *testing.T) {
	_, err := LoadTreeFromBytes([]byte("tree:\n  type: action\n  do: proc\n  effect: lifesteal\n"))
	if err == nil || !strings.Contains(err.Error(), "allowed only in an item tree") {
		t.Errorf("a mob tree named proc: err %v", err)
	}
}

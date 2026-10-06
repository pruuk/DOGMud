package behaviortree

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// The voice nodes (item behaviour slice 2, Rules 16 to 18).

const (
	voiceProbeItemId   = 99640
	voiceProbeItemId2  = 99641
	voiceBearerId      = 71
	voiceListenerId    = 72
	voiceLitRoom       = 9961
	voiceDarkRoom      = 9962
	voiceMobTemplateId = 9963
	voiceMobInstanceId = 9964
)

// voiceProbeTree speaks its idle pool through chatter_ready, its event
// pools straight, and its feeding pool to the holder unpaced.
const voiceProbeTree = `
speech:
  on_idle: ["Hm."]
  on_equip: ["Up we go."]
  on_hunger_feeding: ["A sip."]
tree:
  type: selector
  children:
    - type: sequence
      event: item_idle
      children:
        - type: condition
          check: chatter_ready
        - type: action
          do: speak
          pool: on_idle
    - type: action
      event: on_equip
      do: speak
      pool: on_equip
    - type: action
      event: on_hunger_feeding
      do: speak
      pool: on_hunger_feeding
      to: holder
      paced: false
`

// seedVoiceWorld: a lit and a dark room, the bearer and a listener in the
// lit one, two probe items sharing the probe tree, the Pinnacle switch on
// and the normal chatter level at 100% so no roll can fail.
func seedVoiceWorld(t *testing.T) (bearer, listener *users.UserRecord) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.PinnacleItemsEnabled = true
	cfg.Balance.ItemChatterNormalChancePct = 100
	cfg.Balance.ItemChatterNormalCooldownRounds = 20
	cfg.Balance.ItemChatterListenerCapRounds = 10
	configs.SetConfigForTest(t, cfg)

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		voiceProbeItemId:  {ItemId: voiceProbeItemId, Name: "Probe Shield", Type: items.Offhand, Behavior: "voice_nodes_probe"},
		voiceProbeItemId2: {ItemId: voiceProbeItemId2, Name: "Probe Blade", Type: items.Weapon, Hands: 1, Behavior: "voice_nodes_probe"},
	}))
	LoadItemTreeForTest(t, "voice_nodes_probe", voiceProbeTree)
	t.Cleanup(ResetItemBTreeStatesForTest())
	t.Cleanup(ResetItemListenerCapsForTest())

	lit := &rooms.Room{RoomId: voiceLitRoom, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(60)}
	dark := &rooms.Room{RoomId: voiceDarkRoom, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(0)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{voiceLitRoom: lit, voiceDarkRoom: dark}, map[string]*rooms.ZoneConfig{}))

	bearer = users.NewTestUser(voiceBearerId, "kesh", "Kesh", 0)
	listener = users.NewTestUser(voiceListenerId, "clara", "Clara", 0)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{voiceBearerId: bearer, voiceListenerId: listener}))
	for _, u := range []*users.UserRecord{bearer, listener} {
		u.Character.RoomId = voiceLitRoom
		lit.AddPlayer(u.UserId)
	}
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(5000)
	_ = events.DrainQueuedMessagesForTest(voiceBearerId)
	_ = events.DrainQueuedMessagesForTest(voiceListenerId)
	return bearer, listener
}

func wornProbe(itemId int, id byte) ItemSubject {
	return ItemSubject{UUID: uuid.UUID{id}, ItemId: itemId, UserId: voiceBearerId, Slot: "offhand"}
}

func voiceEvent(name string) EventContext { return EventContext{EventType: name} }

func drained(userId int) string {
	return strings.Join(events.DrainQueuedMessagesForTest(userId), "")
}

// Rule 16: the holder reads "<Item> says", the room "<Name>'s <Item>
// mutters", heard by everyone.
func TestSpeakSendsTheHolderAndTheRoom(t *testing.T) {
	seedVoiceWorld(t)
	if !TryItemBehavior(voiceEvent("item_idle"), wornProbe(voiceProbeItemId, 1)) {
		t.Fatal("an open cooldown, a clear cap and a 100% chance: the idle line must go out")
	}
	if got := drained(voiceBearerId); !strings.Contains(got, `Probe Shield</ansi> says, "<ansi fg="yellow">Hm.`) {
		t.Errorf("the holder reads %q, want the item's says line", got)
	}
	if got := drained(voiceListenerId); !strings.Contains(got, `Kesh</ansi>'s <ansi fg="item">Probe Shield</ansi> mutters, "<ansi fg="yellow">Hm.`) {
		t.Errorf("the listener reads %q, want the bearer's item muttering", got)
	}
}

// Spec X17: the room line is heard, and the bearer's name is hidden by each
// listener's sight. Shown to fail against the Pinnacle tick's sender
// (room.SendTextVisual), which sends a listener in the dark nothing.
func TestSpeakHidesTheBearerFromAListenerInTheDark(t *testing.T) {
	bearer, listener := seedVoiceWorld(t)
	dark := rooms.LoadRoom(voiceDarkRoom)
	rooms.LoadRoom(voiceLitRoom).RemovePlayer(voiceBearerId)
	rooms.LoadRoom(voiceLitRoom).RemovePlayer(voiceListenerId)
	for _, u := range []*users.UserRecord{bearer, listener} {
		u.Character.RoomId = voiceDarkRoom
		dark.AddPlayer(u.UserId)
	}
	if !TryItemBehavior(voiceEvent("item_idle"), wornProbe(voiceProbeItemId, 1)) {
		t.Fatal("the idle line must go out in the dark too")
	}
	got := drained(voiceListenerId)
	if !strings.Contains(got, `Someone</ansi>'s <ansi fg="item">Probe Shield</ansi> mutters`) {
		t.Errorf("a listener in the dark reads %q, want the words with the bearer hidden as someone", got)
	}
	if strings.Contains(got, "Kesh") {
		t.Errorf("a listener in the dark reads the bearer's name: %q", got)
	}
}

// Rule 17: an ambient line needs the item's cooldown, and every listener
// past their cap; hearing one starts the cap. An event line bypasses the
// cap but needs the cooldown.
func TestSpeakPacing(t *testing.T) {
	seedVoiceWorld(t)
	shield, blade := wornProbe(voiceProbeItemId, 1), wornProbe(voiceProbeItemId2, 2)

	if !TryItemBehavior(voiceEvent("item_idle"), shield) {
		t.Fatal("the first idle line must go out")
	}
	drained(voiceBearerId)
	drained(voiceListenerId)

	util.SetRoundCountForTest(5001)
	if TryItemBehavior(voiceEvent("item_idle"), shield) {
		t.Error("the shield's own cooldown is closed: no second idle line")
	}
	if TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Error("the blade's cooldown is open but both listeners are inside their cap: no ambient line")
	}
	if !TryItemBehavior(voiceEvent("on_equip"), blade) {
		t.Error("an event line bypasses the listener cap")
	}
	if TryItemBehavior(voiceEvent("on_equip"), shield) {
		t.Error("an event line still needs the item's own cooldown")
	}
	drained(voiceBearerId)
	drained(voiceListenerId)

	util.SetRoundCountForTest(5010)
	if TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Error("the blade's event line at 5001 armed its cooldown: no idle line at 5010")
	}
	util.SetRoundCountForTest(5021)
	if !TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Error("past both the cap and the blade's cooldown: the idle line goes out")
	}
}

// Ruling S1: the feeding line goes to the holder alone, ignores the
// item's cooldown and does not arm it.
func TestSpeakToTheHolderUnpaced(t *testing.T) {
	seedVoiceWorld(t)
	blade := wornProbe(voiceProbeItemId2, 2)
	if !TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Fatal("the idle line must go out")
	}
	drained(voiceBearerId)
	drained(voiceListenerId)
	if !TryItemBehavior(voiceEvent("on_hunger_feeding"), blade) {
		t.Fatal("an unpaced line goes out with the cooldown closed")
	}
	if got := drained(voiceBearerId); !strings.Contains(got, "A sip.") {
		t.Errorf("the holder reads %q, want the feeding line", got)
	}
	if got := drained(voiceListenerId); got != "" {
		t.Errorf("to: holder sends the room nothing, got %q", got)
	}
}

// Ruling R6: a mob holder's item speaks the room line only, the mob's name
// hidden like a player's.
func TestSpeakForAMobHolder(t *testing.T) {
	seedVoiceWorld(t)
	t.Cleanup(seedTestMob(t, voiceMobTemplateId, voiceMobInstanceId, voiceLitRoom, "ogre"))
	rooms.LoadRoom(voiceLitRoom).AddMob(voiceMobInstanceId)
	held := ItemSubject{UUID: uuid.UUID{3}, ItemId: voiceProbeItemId, MobInstanceId: voiceMobInstanceId, Slot: "offhand"}
	if !TryItemBehavior(voiceEvent("item_idle"), held) {
		t.Fatal("a mob-held voiced item speaks")
	}
	// The renderer capitalises the name at the sentence start.
	if got := drained(voiceListenerId); !strings.Contains(got, `<ansi fg="mobname">Ogre</ansi>'s <ansi fg="item">Probe Shield</ansi> mutters`) {
		t.Errorf("the room reads %q, want the mob's item muttering", got)
	}
}

// The Pinnacle switch silences every voice node.
func TestVoiceNodesHonourThePinnacleSwitch(t *testing.T) {
	seedVoiceWorld(t)
	cfg := configs.GetConfig()
	cfg.GamePlay.PinnacleItemsEnabled = false
	configs.SetConfigForTest(t, cfg)
	if TryItemBehavior(voiceEvent("item_idle"), wornProbe(voiceProbeItemId, 1)) ||
		TryItemBehavior(voiceEvent("on_equip"), wornProbe(voiceProbeItemId, 1)) {
		t.Error("PinnacleItemsEnabled off: no item speaks")
	}
	if got := drained(voiceBearerId) + drained(voiceListenerId); got != "" {
		t.Errorf("PinnacleItemsEnabled off sent %q", got)
	}
}

// Spec X14: hunger_overdue reads the holder's hunger anchor against the
// item's hunger window, as the Pinnacle tick's pickVoiceEvent did.
func TestHungerOverdue(t *testing.T) {
	bearer, _ := seedVoiceWorld(t)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		voiceProbeItemId2: {ItemId: voiceProbeItemId2, Name: "Probe Blade", Type: items.Weapon, Hands: 1,
			HungerRounds: 50, HungerDrainPct: 0.01},
	}))
	LoadItemTreeForTest(t, "hunger_probe", "tree:\n  type: condition\n  check: hunger_overdue\n  fraction: 0.75\n")
	items.GetItemSpec(voiceProbeItemId2).Behavior = "hunger_probe"
	blade := ItemSubject{UUID: uuid.UUID{2}, ItemId: voiceProbeItemId2, UserId: voiceBearerId, Slot: "weapon"}

	if TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Error("no anchor yet: not overdue")
	}
	bearer.Character.SetMiscData("pinnacle_hunger_anchor", uint64(5000-37))
	if TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Error("37 of 50 rounds: not past three quarters")
	}
	bearer.Character.SetMiscData("pinnacle_hunger_anchor", uint64(5000-38))
	if !TryItemBehavior(voiceEvent("item_idle"), blade) {
		t.Error("38 of 50 rounds: past three quarters")
	}
}

// Spec X15, ruling R5: taunt_pull moves the bearer's foe onto the bearer
// and always succeeds, so a failed pull never falls through to another
// line.
func TestTauntPull(t *testing.T) {
	bearer, _ := seedVoiceWorld(t)
	LoadItemTreeForTest(t, "taunt_probe", "tree:\n  type: action\n  do: taunt_pull\n")
	items.GetItemSpec(voiceProbeItemId).Behavior = "taunt_probe"
	t.Cleanup(func() { items.GetItemSpec(voiceProbeItemId).Behavior = "voice_nodes_probe" })
	t.Cleanup(seedTestMob(t, voiceMobTemplateId, voiceMobInstanceId, voiceLitRoom, "ogre"))
	foe := mobs.GetInstance(voiceMobInstanceId)
	shield := wornProbe(voiceProbeItemId, 1)

	if !TryItemBehavior(voiceEvent("item_idle"), shield) {
		t.Error("taunt_pull with nothing to pull still succeeds")
	}
	bearer.Character.SetAggro(0, voiceMobInstanceId, characters.DefaultAttack)
	foe.Character.SetAggro(voiceListenerId, 0, characters.DefaultAttack)

	foe.Character.NonCombatant = true
	if !TryItemBehavior(voiceEvent("item_idle"), shield) {
		t.Error("taunt_pull on a non-combatant still succeeds")
	}
	if got := foe.Character.CurrentCombatTarget().UserId; got != voiceListenerId {
		t.Errorf("a non-combatant is never pulled: it fights user %d, want %d", got, voiceListenerId)
	}

	foe.Character.NonCombatant = false
	if !TryItemBehavior(voiceEvent("item_idle"), shield) {
		t.Error("taunt_pull succeeds")
	}
	if got := foe.Character.CurrentCombatTarget().UserId; got != voiceBearerId {
		t.Errorf("the foe fights user %d, want the bearer %d", got, voiceBearerId)
	}

	// A foe already on the bearer is left as it is.
	already := foe.Character.CurrentCombatTarget()
	TryItemBehavior(voiceEvent("item_idle"), shield)
	if foe.Character.CurrentCombatTarget() != already {
		t.Errorf("a foe already on the bearer was re-forced: %+v", foe.Character.CurrentCombatTarget())
	}
}

// A speak node names a pool its tree has, and an audience it knows.
func TestSpeakRefusesAtLoad(t *testing.T) {
	bad := map[string]string{
		"unknown pool":     "speech:\n  on_idle: [\"Hm.\"]\ntree:\n  type: action\n  do: speak\n  pool: on_kill\n",
		"no speech":        "tree:\n  type: action\n  do: speak\n  pool: on_idle\n",
		"unknown audience": "speech:\n  on_idle: [\"Hm.\"]\ntree:\n  type: action\n  do: speak\n  pool: on_idle\n  to: everyone\n",
	}
	for name, src := range bad {
		if _, _, err := loadItemTreeDef([]byte(src)); err == nil {
			t.Errorf("%s: loaded, want a refusal", name)
		}
	}
	for _, node := range []string{"  type: action\n  do: speak\n  pool: on_idle\n", "  type: condition\n  check: chatter_ready\n"} {
		if _, err := LoadTreeFromBytes([]byte("tree:\n" + node)); err == nil || !strings.Contains(err.Error(), "item") {
			t.Errorf("a mob tree naming an item voice node: err = %v, want a refusal", err)
		}
	}
}

package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The item voice events (item behaviour slice 2, Rule 6, spec X18): equip
// and remove fire on_equip and on_unequip (closes #222), a kill fires
// on_kill into every worn treed item (ruling S3), and the hunger tick
// fires on_hunger_feeding into the weapon's tree (ruling S1).

const (
	eventsShieldId     = 99650
	eventsBladeId      = 99651
	eventsPlainBladeId = 99652
	eventsBearerId     = 81
	eventsListenerId   = 82
	eventsRoomId       = 9981
	eventsMobInstance  = 9982
)

const eventsProbeTree = `
speech:
  on_equip: ["Up we go."]
  on_unequip: ["Down we go."]
  on_kill: ["Another."]
  on_hunger_feeding: ["A sip."]
tree:
  type: selector
  children:
    - type: action
      event: on_equip
      do: speak
      pool: on_equip
    - type: action
      event: on_unequip
      do: speak
      pool: on_unequip
    - type: action
      event: on_kill
      do: speak
      pool: on_kill
    - type: action
      event: on_hunger_feeding
      do: speak
      pool: on_hunger_feeding
      to: holder
      paced: false
`

func seedItemEventsWorld(t *testing.T) (bearer *users.UserRecord, room *rooms.Room) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.PinnacleItemsEnabled = true
	cfg.Balance.ItemChatterNormalCooldownRounds = 20
	cfg.Balance.HungerFeedingLineCooldownRounds = 20
	configs.SetConfigForTest(t, cfg)

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		eventsShieldId: {ItemId: eventsShieldId, Name: "Probe Shield", Type: items.Offhand, Behavior: "item_events_probe"},
		eventsBladeId: {ItemId: eventsBladeId, Name: "Probe Blade", Type: items.Weapon, Hands: 1,
			HungerRounds: 50, HungerDrainPct: 0.01, Behavior: "item_events_probe"},
		eventsPlainBladeId: {ItemId: eventsPlainBladeId, Name: "Plain Blade", Type: items.Weapon, Hands: 1,
			HungerRounds: 50, HungerDrainPct: 0.01},
	}))
	behaviortree.LoadItemTreeForTest(t, "item_events_probe", eventsProbeTree)
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	t.Cleanup(behaviortree.ResetItemListenerCapsForTest())

	room = &rooms.Room{RoomId: eventsRoomId, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(60)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{eventsRoomId: room}, map[string]*rooms.ZoneConfig{}))

	bearer = users.NewTestUser(eventsBearerId, "kesh", "Kesh", 0)
	listener := users.NewTestUser(eventsListenerId, "clara", "Clara", 0)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{eventsBearerId: bearer, eventsListenerId: listener}))
	for _, u := range []*users.UserRecord{bearer, listener} {
		u.Character.RoomId = eventsRoomId
		room.AddPlayer(u.UserId)
	}
	bearer.Character.HealthMax.Value = 1000
	bearer.Character.Health = 1000

	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(7000)
	_ = events.DrainQueuedMessagesForTest(eventsBearerId)
	_ = events.DrainQueuedMessagesForTest(eventsListenerId)
	return bearer, room
}

func eventsText(userId int) string {
	return strings.Join(events.DrainQueuedMessagesForTest(userId), "")
}

// #222: wearing and removing a voiced item fires its equip and unequip
// lines, for a player and for a mob.
func TestEquipmentChangeFiresOnEquipAndOnUnequip(t *testing.T) {
	bearer, room := seedItemEventsWorld(t)
	shield := items.New(eventsShieldId)
	bearer.Character.Equipment.Offhand = shield

	ItemEquipEvents(events.EquipmentChange{UserId: eventsBearerId, ItemsWorn: []items.Item{shield}})
	if got := eventsText(eventsBearerId); !strings.Contains(got, "Up we go.") {
		t.Errorf("on equip the bearer reads %q, want the equip line", got)
	}
	if got := eventsText(eventsListenerId); !strings.Contains(got, "mutters") {
		t.Errorf("on equip the room reads %q, want the item muttering", got)
	}

	util.SetRoundCountForTest(7020)
	bearer.Character.Equipment.Offhand = items.Item{}
	bearer.Character.StoreItem(shield)
	ItemEquipEvents(events.EquipmentChange{UserId: eventsBearerId, ItemsRemoved: []items.Item{shield}})
	if got := eventsText(eventsBearerId); !strings.Contains(got, "Down we go.") {
		t.Errorf("on removal the bearer reads %q, want the unequip line", got)
	}

	ogre := &mobs.Mob{InstanceId: eventsMobInstance, HomeRoomId: eventsRoomId,
		Character: characters.Character{Name: "ogre", RoomId: eventsRoomId, Conditions: conditions.New()}}
	mobs.SetInstanceForTest(eventsMobInstance, ogre)
	t.Cleanup(func() { mobs.SetInstanceForTest(eventsMobInstance, nil) })
	room.AddMob(eventsMobInstance)
	mobShield := items.New(eventsShieldId)
	ogre.Character.Equipment.Offhand = mobShield
	ItemEquipEvents(events.EquipmentChange{MobInstanceId: eventsMobInstance, ItemsWorn: []items.Item{mobShield}})
	if got := eventsText(eventsListenerId); !strings.Contains(got, `Ogre</ansi>'s <ansi fg="item">Probe Shield</ansi> mutters, "<ansi fg="yellow">Up we go.`) {
		t.Errorf("a mob's equip: the room reads %q, want the mob's item muttering", got)
	}
}

// Ruling S3: a kill reaches every worn treed item, not only the weapon,
// and the room hears it (ruling S2).
func TestKillFiresOnKillIntoEveryWornVoicedItem(t *testing.T) {
	bearer, _ := seedItemEventsWorld(t)
	bearer.Character.Equipment.Offhand = items.New(eventsShieldId)

	MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{eventsBearerId: 1}})
	if got := eventsText(eventsBearerId); !strings.Contains(got, "Another.") {
		t.Errorf("the bearer reads %q, want the shield's kill line", got)
	}
	if got := eventsText(eventsListenerId); !strings.Contains(got, "Another.") {
		t.Errorf("the room reads %q, want the kill line muttered", got)
	}
}

// Ruling S1: the feeding line goes to the bearer alone, through the
// weapon's tree, paced only by HungerFeedingLineCooldownRounds; a hungry
// weapon with no tree sends the plain fallback.
func TestHungerFeedingSpeaksThroughTheWeaponsTree(t *testing.T) {
	bearer, _ := seedItemEventsWorld(t)
	c := bearer.Character
	c.Equipment.Weapon = items.New(eventsBladeId)
	c.SetMiscData("pinnacle_hunger_anchor", uint64(7000-60))

	tickHunger(c, bearer, 7000)
	if got := eventsText(eventsBearerId); !strings.Contains(got, "A sip.") {
		t.Errorf("the bearer reads %q, want the tree's feeding line", got)
	}
	if got := eventsText(eventsListenerId); got != "" {
		t.Errorf("the feeding line is the bearer's alone, the room read %q", got)
	}
	health := c.Health
	tickHunger(c, bearer, 7001)
	if got := eventsText(eventsBearerId); got != "" {
		t.Errorf("one round later the feeding line is paced out, got %q", got)
	}
	if c.Health >= health {
		t.Error("the drain itself runs every overdue round")
	}

	c.Equipment.Weapon = items.New(eventsPlainBladeId)
	c.SetMiscData("pinnacle_hunger_msg_next_round", nil)
	tickHunger(c, bearer, 7002)
	if got := eventsText(eventsBearerId); !strings.Contains(got, "The blade feeds on you") {
		t.Errorf("a hungry weapon with no tree: the bearer reads %q, want the fallback line", got)
	}
}

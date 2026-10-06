package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The 5a parity table (sight and gates slice 5a). One player and one mob,
// built alike, stand in one room at a pinned light; every row drives a
// shared body through each actor and asserts the same outcome. The fixture
// asserts both actors' sight before any row runs, so a verdict is exact,
// never rolled.

const (
	gateRoomId    = 7950
	gateUserId    = 7951
	gateMobId     = 97952
	gateInfraCond = 7953
)

type gateLight int

const (
	gateLit gateLight = iota
	gateShapes
	gateDark
	gateBlinded
)

func (l gateLight) String() string { return [...]string{"lit", "shapes", "dark", "blinded"}[l] }

var gateLights = []gateLight{gateLit, gateShapes, gateDark, gateBlinded}

type gateScene struct {
	room *rooms.Room
	user *users.UserRecord
	mob  *mobs.Mob
}

func (s gateScene) actor(who string) Actor {
	if who == "player" {
		return NewUserActorInRoom(s.user, s.room)
	}
	return NewMobActorInRoom(s.mob, s.room)
}

var gateWho = []string{"player", "mob"}

func newGateScene(t *testing.T, light gateLight) gateScene {
	t.Helper()
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "Human", Size: species.Medium},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		gateInfraCond: {ConditionId: gateInfraCond, Name: "Test Heat Sight", RoundInterval: 1, TriggerCount: 10,
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))

	room := &rooms.Room{RoomId: gateRoomId, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(90)}
	if light == gateShapes || light == gateDark {
		room.Lamp = rooms.LampPtr(0)
	}

	u := users.NewTestUser(gateUserId, "gatey", "Gatey", uint64(gateUserId))
	u.Character.RoomId = gateRoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{gateUserId: u}))
	room.AddPlayer(gateUserId)

	mc := characters.New()
	mc.Name, mc.RoomId, mc.Health = "Gatemob", gateRoomId, 100
	mc.HealthMax.Value = 100
	m := &mobs.Mob{InstanceId: gateMobId, Character: *mc}
	m.Character.MobInstanceId = gateMobId
	mobs.SetInstanceForTest(gateMobId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(gateMobId, nil) })
	room.AddMob(gateMobId)

	want := map[gateLight]messaging.SightDecision{
		gateLit: messaging.SightFull, gateShapes: messaging.SightShapes,
		gateDark: messaging.SightNone, gateBlinded: messaging.SightNone,
	}[light]
	for who, c := range map[string]*characters.Character{"player": u.Character, "mob": &m.Character} {
		switch light {
		case gateShapes:
			require.NoError(t, c.AddCondition(gateInfraCond, true))
		case gateBlinded:
			c.Perception = characters.New().Perception
			require.NoError(t, c.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
		}
		require.Equal(t, want, messaging.ParticipantSight(c, room), "fixture: the %s must sit at %s", who, light)
	}
	return gateScene{room: room, user: u, mob: m}
}

// gateItem is a distinct item with its spec inline.
func gateItem(id int, name string, t items.ItemType, cursed bool) items.Item {
	return items.Item{ItemId: id, Spec: &items.ItemSpec{ItemId: id, Name: name, Type: t, Subtype: items.Wearable, Cursed: cursed}}
}

// Equip over a cursed piece: refused with the shared line, the candidate
// stays in the pack, the cursed piece stays on (ruling 8).
func TestGateParity_EquipOverCursedArmour(t *testing.T) {
	for _, who := range gateWho {
		s := newGateScene(t, gateLit)
		a := s.actor(who)
		c := a.GetCharacter()
		c.Equipment.Head = gateItem(39601, "iron helm", items.Head, true)
		require.True(t, c.StoreItem(gateItem(39602, "leather cap", items.Head, false)))
		res := EquipItem(a, "leather cap")
		assert.True(t, res.Found, who)
		assert.False(t, res.Equipped, who)
		assert.Equal(t, `Your Iron Helm is cursed and prevents you from removing it.`, res.FailureReason, who)
		assert.Equal(t, 39601, c.Equipment.Head.ItemId, who)
		_, inPack := c.FindInBackpack("leather cap")
		assert.True(t, inPack, "%s: the candidate stays in the pack", who)
	}
}

func TestGateParity_TakeFloorItem(t *testing.T) {
	want := map[gateLight]error{gateLit: nil, gateShapes: nil, gateDark: ErrTooDark, gateBlinded: ErrTooDark}
	for _, light := range gateLights {
		for _, who := range gateWho {
			s := newGateScene(t, light)
			a := s.actor(who)
			pebble := gateItem(39501, "pebble", items.Junk, false)
			s.room.Items = []items.Item{pebble}
			err := TakeFloorItem(a, pebble, false)
			assert.ErrorIs(t, err, want[light], "%s at %s", who, light)
			if want[light] == nil {
				assert.NoError(t, err, "%s at %s", who, light)
			}
			_, onFloor := s.room.FindOnFloor("pebble", false)
			assert.Equal(t, want[light] != nil, onFloor, "%s at %s: a refusal moves nothing", who, light)
		}
	}
}

func TestGateParity_ExplodingItemIsRefused(t *testing.T) {
	for _, who := range gateWho {
		s := newGateScene(t, gateLit)
		bomb := gateItem(39502, "bomb", items.Junk, false)
		bomb.Adjectives = []string{`exploding`}
		s.room.Items = []items.Item{bomb}
		assert.ErrorIs(t, TakeFloorItem(s.actor(who), bomb, false), ErrExploding, who)
		res := GetItemFromFloor(s.actor(who), "bomb", false)
		assert.True(t, res.Found, who)
		assert.ErrorIs(t, res.Err, ErrExploding, who)
	}
}

func TestGateParity_GetItemFromFloorInTheDarkFindsNothing(t *testing.T) {
	for _, who := range gateWho {
		s := newGateScene(t, gateDark)
		s.room.Items = []items.Item{gateItem(39503, "pebble", items.Junk, false)}
		res := GetItemFromFloor(s.actor(who), "pebble", false)
		assert.False(t, res.Found, "%s: the dark tells the actor nothing about the floor", who)
		assert.ErrorIs(t, res.Err, ErrTooDark, who)
	}
}

func TestGateParity_GoldPickup(t *testing.T) {
	for _, light := range gateLights {
		for _, who := range gateWho {
			s := newGateScene(t, light)
			s.room.Gold = 10
			err := GetGoldFromFloor(s.actor(who), 10)
			if light == gateDark || light == gateBlinded {
				assert.ErrorIs(t, err, ErrTooDark, "%s at %s", who, light)
				assert.Equal(t, 10, s.room.Gold, "%s at %s", who, light)
				continue
			}
			assert.NoError(t, err, "%s at %s", who, light)
			assert.Equal(t, 0, s.room.Gold, "%s at %s", who, light)
		}
	}
}

func TestGateParity_ResolveLook(t *testing.T) {
	// Each actor looks at the other one.
	other := map[string]string{"player": "gatemob", "mob": "gatey"}
	for _, light := range gateLights {
		for _, who := range gateWho {
			s := newGateScene(t, light)
			room := ResolveLook(s.actor(who), "")
			creature := ResolveLook(s.actor(who), other[who])
			switch light {
			case gateLit:
				assert.Equal(t, LookRoom, room.Kind, who)
				assert.Equal(t, LookCreature, creature.Kind, who)
			case gateShapes:
				assert.Equal(t, LookRoom, room.Kind, who)
				assert.Equal(t, LookOther, creature.Kind, "%s: at shapes a creature is not named", who)
				assert.False(t, creature.NamesCreatures, who)
			case gateDark:
				assert.Equal(t, LookTooDark, room.Kind, "%s at %s", who, light)
				assert.Equal(t, LookTooDark, creature.Kind, "%s at %s", who, light)
			case gateBlinded:
				assert.Equal(t, LookBlind, room.Kind, "%s at %s", who, light)
				assert.Equal(t, LookBlind, creature.Kind, "%s at %s", who, light)
			}
		}
	}
}

func TestGateParity_LookCannotNameAHiddenCreature(t *testing.T) {
	other := map[string]string{"player": "gatemob", "mob": "gatey"}
	for _, who := range gateWho {
		s := newGateScene(t, gateLit)
		if who == "player" {
			viewerTestHide(t, &s.mob.Character)
		} else {
			viewerTestHide(t, s.user.Character)
		}
		assert.NotEqual(t, LookCreature, ResolveLook(s.actor(who), other[who]).Kind, who)
	}
}

func TestGateParity_LookThroughAnExitNeedsLight(t *testing.T) {
	for _, light := range []gateLight{gateLit, gateShapes} {
		for _, who := range gateWho {
			s := newGateScene(t, light)
			s.room.Exits = map[string]exit.RoomExit{"north": {RoomId: gateRoomId + 1}}
			res := ResolveLook(s.actor(who), "north")
			if light == gateLit {
				assert.Equal(t, LookExit, res.Kind, who)
				assert.Equal(t, gateRoomId+1, res.ExitRoomId, who)
				continue
			}
			assert.Equal(t, LookExitTooDark, res.Kind, "%s: heat shows shapes here, not in the next room", who)
		}
	}
}

func gateBusy(t *testing.T, c *characters.Character) {
	t.Helper()
	c.Activity = activity.NewMachine()
	require.NoError(t, c.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: "test", RoundsTotal: 3},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin}))
}

func TestGateParity_RemoveWhileBusy(t *testing.T) {
	for _, who := range gateWho {
		s := newGateScene(t, gateLit)
		a := s.actor(who)
		a.GetCharacter().Equipment.Head = gateItem(39701, "cap", items.Head, false)
		gateBusy(t, a.GetCharacter())
		assert.True(t, RemoveEquipment(a, "cap").Busy, who)
		assert.True(t, RemoveAllEquipment(a).Busy, who)
		assert.Equal(t, 39701, a.GetCharacter().Equipment.Head.ItemId, who)
	}
}

func TestGateParity_RemoveCursed(t *testing.T) {
	for _, who := range gateWho {
		s := newGateScene(t, gateLit)
		a := s.actor(who)
		c := a.GetCharacter()
		c.Equipment.Ring = gateItem(39702, "hexed ring", items.Ring, true)
		res := RemoveEquipment(a, "hexed ring")
		assert.True(t, res.Cursed, who)
		assert.False(t, res.Removed, who)
		assert.Equal(t, 39702, c.Equipment.Ring.ItemId, who)

		c.SetSkill(string(skills.Spellcasting), 4)
		res = RemoveEquipment(a, "hexed ring")
		assert.True(t, res.Removed, who)
		assert.True(t, res.CursedOverridden, who)
	}
}

func TestGateParity_RemoveAllSkipsCursed(t *testing.T) {
	for _, who := range gateWho {
		s := newGateScene(t, gateLit)
		a := s.actor(who)
		c := a.GetCharacter()
		c.Equipment.Ring = gateItem(39703, "hexed ring", items.Ring, true)
		c.Equipment.Head = gateItem(39704, "cap", items.Head, false)
		res := RemoveAllEquipment(a)
		require.Len(t, res.Cursed, 1, who)
		assert.Equal(t, 39703, res.Cursed[0].ItemId, who)
		require.Len(t, res.Removed, 1, who)
		assert.Equal(t, 39704, res.Removed[0].ItemId, who)
		assert.Equal(t, 39703, c.Equipment.Ring.ItemId, who)
	}
}

func TestGateParity_CraftNeedsClearSight(t *testing.T) {
	for _, light := range gateLights {
		for _, who := range gateWho {
			s := newGateScene(t, light)
			a := s.actor(who)
			res := InitiateCraft(a, "no-such-recipe")
			if light == gateLit {
				assert.False(t, TooDarkToCraft(a), "%s at %s", who, light)
				assert.False(t, res.CannotSee, "%s at %s", who, light)
				continue
			}
			assert.True(t, TooDarkToCraft(a), "%s at %s: shapes are not enough for fine work", who, light)
			assert.Equal(t, CraftResult{CannotSee: true}, res, "%s at %s", who, light)
		}
	}
}

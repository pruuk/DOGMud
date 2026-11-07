package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wake <name> aims by sight (#454) and narrates by each reader's sight. The
// scene is aimScene's room 1: Aliceia (1) wakes, Bobrick (2) watches, and
// the Skeleton (mob 100) sleeps. At shapes Aliceia makes out Bobrick as
// figure 1 and the Skeleton as figure 2.

const wakeSightSleepId = 9457

// wakeSightScene is aimScene with the sleep condition seeded, keeping
// aimScene's infrared (a reseed replaces the registry).
func wakeSightScene(t *testing.T, band aimBand) (*users.UserRecord, *rooms.Room, *mobs.Mob) {
	t.Helper()
	user, room := aimScene(t, band)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		wakeSightSleepId: {ConditionId: wakeSightSleepId, Name: "Test Sleep", RoundInterval: 1, TriggerCount: 100,
			Flags: []conditions.Flag{conditions.Sleeping}},
		aimInfraredConditionId: {ConditionId: aimInfraredConditionId, Name: "Test Infrared",
			RoundInterval: 1, TriggerCount: 1, Flags: []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))
	return user, room, mobs.GetInstance(100)
}

func putToSleep(t *testing.T, c interface{ AddCondition(int, bool) error }) {
	t.Helper()
	require.NoError(t, c.AddCondition(wakeSightSleepId, true))
}

// A player who makes out shapes wakes a figure by the word the room shows,
// and is never told its name.
func TestWakeSight_ShapesWakesAFigureWithoutNamingIt(t *testing.T) {
	user, room, skel := wakeSightScene(t, aimShapes)
	putToSleep(t, &skel.Character)
	_, _ = Wake(`2.figure`, user, room, events.EventFlag(0))
	require.False(t, skel.Character.HasConditionFlag(conditions.Sleeping), "wake 2.figure left the Skeleton asleep")
	told := aimTold(1)
	assert.Contains(t, told, "You wake a figure.")
	assert.NotContains(t, told, "Skeleton")
}

// At shapes a typed name resolves nothing: it would confirm who is there.
func TestWakeSight_ShapesRefusesATypedName(t *testing.T) {
	user, room, skel := wakeSightScene(t, aimShapes)
	putToSleep(t, &skel.Character)
	_, _ = Wake(`skeleton`, user, room, events.EventFlag(0))
	require.True(t, skel.Character.HasConditionFlag(conditions.Sleeping), "a name typed at shapes woke the Skeleton")
	assert.Contains(t, aimTold(1), "You can only make out shapes here.")
}

// With no sight there is no one to aim at.
func TestWakeSight_NoSightWakesNoOne(t *testing.T) {
	user, room, skel := wakeSightScene(t, aimDark)
	putToSleep(t, &skel.Character)
	_, _ = Wake(`skeleton`, user, room, events.EventFlag(0))
	require.True(t, skel.Character.HasConditionFlag(conditions.Sleeping), "a name typed in the dark woke the Skeleton")
}

// "is not asleep" names no one the player cannot see.
func TestWakeSight_NotAsleepHidesAShapesName(t *testing.T) {
	user, room, _ := wakeSightScene(t, aimShapes)
	_, _ = Wake(`2.figure`, user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "is not asleep.")
	assert.NotContains(t, told, "Skeleton")
}

// The room line follows each observer's sight. Lit, Bobrick reads both
// names; in the dark (Aliceia aims by infrared, Bobrick has none) he reads
// neither.
func TestWakeSight_RoomLineFollowsTheObserversSight(t *testing.T) {
	for _, band := range []aimBand{aimFull, aimShapes} {
		user, room, skel := wakeSightScene(t, band)
		putToSleep(t, &skel.Character)
		aim := `skeleton`
		if band == aimShapes {
			aim = `2.figure`
		}
		_, _ = Wake(aim, user, room, events.EventFlag(0))
		require.False(t, skel.Character.HasConditionFlag(conditions.Sleeping), "band %d: the Skeleton still sleeps", band)
		bob := aimTold(2)
		if band == aimFull {
			assert.Contains(t, bob, "Aliceia wakes Skeleton.")
		} else {
			assert.NotContains(t, bob, "Aliceia", "an observer in the dark read the waker's name")
			assert.NotContains(t, bob, "Skeleton", "an observer in the dark read the sleeper's name")
		}
	}
}

// A sleeping player is told who woke them.
func TestWakeSight_APlayerSleeperIsToldWhoWokeThem(t *testing.T) {
	user, room, _ := wakeSightScene(t, aimFull)
	bob := users.GetByUserId(2)
	putToSleep(t, bob.Character)
	_, _ = Wake(`bobrick`, user, room, events.EventFlag(0))
	require.False(t, bob.Character.HasConditionFlag(conditions.Sleeping))
	assert.Contains(t, aimTold(2), "Aliceia wakes you.")
	assert.Contains(t, aimTold(1), "You wake Bobrick.")
}

// Waking someone is noisy: it reveals a hidden waker, as shout does.
func TestWakeRevealsAHiddenWaker(t *testing.T) {
	user, room, skel := wakeSightScene(t, aimFull)
	putToSleep(t, &skel.Character)
	hideForTest(t, user)
	_, _ = Wake(`skeleton`, user, room, events.EventFlag(0))
	require.False(t, skel.Character.HasConditionFlag(conditions.Sleeping))
	assert.False(t, user.Character.IsHidden(), "a hidden waker stayed hidden")
}

// The asleep refusal's hint names what the player can type at their sight:
// the name lit, the figure at shapes, and no one in the dark (shout wakes
// the room). HideNames is case-sensitive, so a lowercased name in the hint
// would slip past it.
func TestAsleepHintFollowsSight(t *testing.T) {
	for _, band := range []aimBand{aimFull, aimShapes, aimDark} {
		user, _, skel := wakeSightScene(t, band)
		putToSleep(t, &skel.Character)
		require.True(t, actions.RefuseMobIfAsleep(skel, user))
		told := aimTold(1)
		switch band {
		case aimFull:
			assert.Contains(t, told, "Skeleton is fast asleep. Try wake skeleton.")
		case aimShapes:
			assert.Contains(t, told, "A figure is fast asleep. Try wake 2.figure.")
			assert.NotContains(t, strings.ToLower(told), "skeleton", "the shapes hint named the sleeper")
		case aimDark:
			assert.Contains(t, told, "is fast asleep. You could make some noise. Try shout wake up.")
			assert.NotContains(t, strings.ToLower(told), "skeleton", "the dark hint named the sleeper")
		}
	}
}

const (
	wakeLanternId   = 9861
	wakeLanternCond = 9862
)

// wakeLanternScene is shopScene with Marn wearing a lantern whose item_idle
// tree puts it out, so a run of that tree (relightLantern) is observable.
func wakeLanternScene(t *testing.T) (*rooms.Room, *mobs.Mob, *users.UserRecord) {
	t.Helper()
	shop, marn, kesh := shopScene(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		shopWakeSleepId: {ConditionId: shopWakeSleepId, Name: "Test Sleep", Flags: []conditions.Flag{conditions.Sleeping}},
		wakeLanternCond: {ConditionId: wakeLanternCond, Name: "Test Lantern Light", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 52}}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		wakeLanternId: {ItemId: wakeLanternId, Name: "Test Lantern", Type: items.Light, Subtype: items.Wearable,
			WornConditionIds: []int{wakeLanternCond}, Behavior: "wake_lantern_probe"},
	}))
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	t.Cleanup(itemlight.ResetForTest())
	behaviortree.LoadItemTreeForTest(t, "wake_lantern_probe", "tree:\n  type: action\n  do: set_light\n  level: \"off\"\n")
	marn.Character.Equipment.Light = items.Item{ItemId: wakeLanternId, UUID: uuid.UUID{0x61}}
	require.True(t, marn.Character.Conditions.AddCondition(wakeLanternCond, true))
	return shop, marn, kesh
}

func wakeLanternTrim(t *testing.T, m *mobs.Mob) conditions.LightTrim {
	t.Helper()
	recs := m.Character.Conditions.GetConditions(wakeLanternCond)
	require.Len(t, recs, 1)
	return recs[0].LightTrim
}

// wake runs the keeper's lantern tree only for a keeper in their own shop
// (keeperInOwnShop), the rule every other shop wake follows. Control: at home
// the tree runs.
func TestWakeRelightsOnlyAKeeperInTheirOwnShop(t *testing.T) {
	shop, marn, kesh := wakeLanternScene(t)
	_, _ = Wake(`marn`, kesh, shop, 0)
	require.False(t, marn.Character.HasConditionFlag(conditions.Sleeping))
	require.Equal(t, conditions.LightOff, wakeLanternTrim(t, marn), "control: the lantern tree did not run for a keeper at home")

	shop, marn, kesh = wakeLanternScene(t)
	marn.HomeRoomId = 4700 // asleep away from the shop
	_, _ = Wake(`marn`, kesh, shop, 0)
	require.False(t, marn.Character.HasConditionFlag(conditions.Sleeping))
	require.NotEqual(t, conditions.LightOff, wakeLanternTrim(t, marn), "the lantern tree ran for a keeper away from their shop")
}

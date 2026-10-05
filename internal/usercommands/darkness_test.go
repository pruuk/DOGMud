package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	darkTestUmbralCond = 9761 // literal darkness 50, adjustable, secret: condition 132's shape
	darkTestPallCond   = 9762 // magnitude darkness, adjustable, cancellable: condition 131's shape
	darkTestUmbralItem = 999970
)

// darknessFixture seeds the two darkness shapes, an Umbral Lantern-shaped
// light-slot item, a species for Wear to dereference, and returns user 1
// standing in room 2, lit by a lamp of 50, with user 2 watching.
func darknessFixture(t *testing.T) (*users.UserRecord, *users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "human", Size: species.Medium},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		darkTestUmbralCond: {ConditionId: darkTestUmbralCond, Name: "Test Umbral Dark", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {Literal: 50}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
		darkTestPallCond: {ConditionId: darkTestPallCond, Name: "Test Pall", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {UsesMagnitude: true}},
			Flags:   []conditions.Flag{conditions.Adjustable, conditions.Cancellable}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		darkTestUmbralItem: {ItemId: darkTestUmbralItem, Name: "test umbral lantern", NameSimple: "lantern",
			Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{darkTestUmbralCond}},
	}))

	user := users.GetByUserId(1)
	require.NotNil(t, user)
	user.Character.SpeciesId = 0
	user.Character.Stats.Strength.ValueAdj = 100

	room := rooms.LoadRoom(2)
	require.NotNil(t, room)
	room.Biome = "cave"
	room.Lamp = rooms.LampPtr(50)
	user.Character.RoomId = 2
	room.AddPlayer(user.UserId)

	observer := users.GetByUserId(2)
	require.NotNil(t, observer)
	observer.Character.RoomId = 2
	room.AddPlayer(observer.UserId)
	return user, observer, room
}

// The guard the spec names "darkness is never light": a character holding a
// pall and an Umbral Dark sheds no light, is no sneak beacon, and cannot hood
// the lantern. Proven able to fail by making IsLightSource accept
// darkness_strength.
func TestDarknessIsNeverLight(t *testing.T) {
	user, _, room := darknessFixture(t)
	_, ok, why := user.Character.Wear(items.New(darkTestUmbralItem))
	require.True(t, ok, why)
	require.True(t, user.Character.Conditions.AddConditionMagnitude(darkTestPallCond, 4, 50))
	require.Len(t, user.Character.DarknessTerms(), 2, "fixture: both darknesses must be held and on")

	require.Empty(t, user.Character.LightTerms(), "a darkness is not a light term")
	require.False(t, user.Character.EmitsLight(), "a darkness bearer must not shed light")

	// The sneak score takes the no-light branch: exactly what the same
	// character scores holding nothing at all.
	bare := users.NewTestUser(9763, "bare", "Bare", 99763)
	bare.Character.Stats = user.Character.Stats
	bare.Character.Skills = user.Character.Skills
	for _, lit := range []bool{false, true} {
		require.Equal(t, actions.CalcSneakScore(bare.Character, lit), actions.CalcSneakScore(user.Character, lit),
			"lit=%v: a darkness bearer's sneak score moved, so it counted as a light", lit)
	}

	events.DrainQueuedMessagesForTest(user.UserId)
	_, err := Hood("", user, room, 0)
	require.NoError(t, err)
	require.Contains(t, hoodTestText(user.UserId), "has no hood.", "hood must refuse a darkness in the light slot")
}

// Ruling D6 as amended: the equip room line of a darkness item is judged by
// the room before it went on. The lantern takes the room from 50 to 0 as it
// goes on, and the observer, now in the dark, still sees it put on.
func TestEquippingADarknessIsSeenBeforeTheDarkFalls(t *testing.T) {
	user, observer, room := darknessFixture(t)
	user.Character.StoreItem(items.Item{ItemId: darkTestUmbralItem})
	events.DrainQueuedMessagesForTest(observer.UserId)

	handled, err := Equip("test umbral lantern", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, darkTestUmbralItem, user.Character.Equipment.Light.ItemId, "fixture: the lantern must be worn")
	require.Less(t, room.LightLevel(), 25, "fixture: the lantern must have taken the room below normal sight")
	require.Contains(t, hoodTestText(observer.UserId), "puts on their",
		"the observer the lantern just blinded missed it being put on")
}

// Owner rule, 2026-10-05: the equip line is judged against the room as it was
// BEFORE the lantern went on. An observer already blind in a pitch-dark room
// (no sky, no lamp) learns nothing of who put it on; judged as lit, the line
// named the wearer to them.
func TestEquippingADarknessIsNotSeenInARoomAlreadyDark(t *testing.T) {
	user, observer, room := darknessFixture(t)
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = nil
	require.Less(t, room.LightLevel(), 25, "fixture: the room must be dark before the lantern")
	user.Character.StoreItem(items.Item{ItemId: darkTestUmbralItem})
	events.DrainQueuedMessagesForTest(observer.UserId)

	handled, err := Equip("test umbral lantern", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, darkTestUmbralItem, user.Character.Equipment.Light.ItemId, "fixture: the lantern must be worn")
	got := hoodTestText(observer.UserId)
	require.NotContains(t, got, user.Character.Name, "an observer already blind in the dark was told who put on the lantern")
	require.NotContains(t, got, "puts on their", "an observer already blind in the dark saw the lantern put on")
}

package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/lightnotice"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	hoodTestAdjustableCond = 9751
	hoodTestFixedCond      = 9752
	hoodTestLanternItem    = 999960
	hoodTestTorchItem      = 999961
)

// hoodFixture seeds an adjustable lantern and a fixed (non-adjustable) light,
// a species for Wear to dereference, and returns user 1 standing in room 2.
func hoodFixture(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "human", Size: species.Medium},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		hoodTestAdjustableCond: {ConditionId: hoodTestAdjustableCond, Name: "Test Hooded Lantern", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 54}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
		hoodTestFixedCond: {ConditionId: hoodTestFixedCond, Name: "Test Fixed Light", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 56}}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		hoodTestLanternItem: {ItemId: hoodTestLanternItem, Name: "test hooded lantern", Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{hoodTestAdjustableCond}},
		hoodTestTorchItem:   {ItemId: hoodTestTorchItem, Name: "test fixed light", Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{hoodTestFixedCond}},
	}))

	user := users.GetByUserId(1)
	require.NotNil(t, user)
	user.Character.SpeciesId = 0
	user.Character.Stats.Strength.ValueAdj = 100

	room := rooms.LoadRoom(2)
	require.NotNil(t, room)
	user.Character.RoomId = 2
	room.AddPlayer(user.UserId)
	return user, room
}

func hoodTestText(userId int) string {
	var b strings.Builder
	for _, line := range events.DrainQueuedMessagesForTest(userId) {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func TestHoodAndUnhood(t *testing.T) {
	user, room := hoodFixture(t)
	_, ok, why := user.Character.Wear(items.New(hoodTestLanternItem))
	require.True(t, ok, why)
	rec := user.Character.Conditions.LightSources()[0]
	rec.SetLightOutput(30)

	_, err := Hood("", user, room, 0)
	require.NoError(t, err)
	require.True(t, rec.Hooded, "hood did not close the hood")
	require.False(t, user.Character.EmitsLight(), "a hooded lantern still sheds light")

	_, err = Unhood("", user, room, 0)
	require.NoError(t, err)
	require.False(t, rec.Hooded)
	require.Equal(t, conditions.LightFull, rec.LightTrim, "unhood must return the lantern to full strength")
	require.True(t, user.Character.EmitsLight(), "an unhooded lantern sheds no light")

	// A nil room is tolerated.
	_, err = Hood("", user, nil, 0)
	require.NoError(t, err)
	require.True(t, rec.Hooded)
	_, err = Unhood("", user, nil, 0)
	require.NoError(t, err)
	require.False(t, rec.Hooded)
}

func TestHood_NoLightWornRefuses(t *testing.T) {
	user, room := hoodFixture(t)
	require.Less(t, user.Character.Equipment.Light.ItemId, 1, "fixture must start with an empty light slot")
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Hood("", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Contains(t, hoodTestText(user.UserId), "You have no lantern with a hood.")

	handled, err = Unhood("", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Contains(t, hoodTestText(user.UserId), "You have no lantern with a hood.")
}

func TestHood_NonAdjustableLightRefuses(t *testing.T) {
	user, room := hoodFixture(t)
	_, ok, why := user.Character.Wear(items.New(hoodTestTorchItem))
	require.True(t, ok, why)
	recs := user.Character.Conditions.LightSources()
	require.Len(t, recs, 1)
	rec := recs[0]
	require.True(t, user.Character.EmitsLight())
	events.DrainQueuedMessagesForTest(user.UserId)

	_, err := Hood("", user, room, 0)
	require.NoError(t, err)
	out := hoodTestText(user.UserId)
	require.Contains(t, out, "Test Fixed Light")
	require.Contains(t, out, "has no hood.")
	require.False(t, rec.Hooded, "a non-adjustable light was hooded")
	require.True(t, user.Character.EmitsLight(), "refusing to hood must leave the light shining")

	_, err = Unhood("", user, room, 0)
	require.NoError(t, err)
	out = hoodTestText(user.UserId)
	require.Contains(t, out, "Test Fixed Light")
	require.Contains(t, out, "has no hood.")
	require.Equal(t, conditions.LightFull, rec.LightTrim)
}

// The room line for hood is judged against the room just before the hood went
// down (#220): in a dark room the lantern is the only light, so by the time
// the line goes out the room is dark and a plain SendTextVisual would hide it
// from the very people who saw by it.
func TestHood_ObserverSeesBothLinesInADarkRoom(t *testing.T) {
	user, room := hoodFixture(t)
	room.Biome = "cave"

	observer := users.GetByUserId(2)
	require.NotNil(t, observer)
	observer.Character.RoomId = room.RoomId
	room.AddPlayer(observer.UserId)

	_, ok, why := user.Character.Wear(items.New(hoodTestLanternItem))
	require.True(t, ok, why)
	events.DrainQueuedMessagesForTest(observer.UserId)

	_, err := Hood("", user, room, 0)
	require.NoError(t, err)
	require.False(t, user.Character.EmitsLight(), "fixture: the hood must have gone dark")
	require.Contains(t, hoodTestText(observer.UserId), "lowers the hood of their lantern, and its glow goes dark.",
		"an observer who saw by the lantern missed the hood line")

	_, err = Unhood("", user, room, 0)
	require.NoError(t, err)
	require.Contains(t, hoodTestText(observer.UserId), "A light flares to life, revealing",
		"an observer who could not see before the unhood missed the reveal line")
}

// The other side of the same judgement (#220): a watcher who could not see
// even with the lantern lit, here because they hold a darkness deeper than
// the lantern's light, is not told the hood went down. Judged as if lit,
// they were, and read the hooder's name.
func TestHood_ObserverWhoCouldNotSeeBeforeIsNotTold(t *testing.T) {
	const hoodTestGloomCond = 9753
	user, room := hoodFixture(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		hoodTestAdjustableCond: conditions.GetConditionSpec(hoodTestAdjustableCond),
		hoodTestGloomCond: {ConditionId: hoodTestGloomCond, Name: "Test Hood Gloom", Secret: true,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {Literal: 90}}},
	}))
	room.Biome = "cave"

	observer := users.GetByUserId(2)
	require.NotNil(t, observer)
	observer.Character.RoomId = room.RoomId
	room.AddPlayer(observer.UserId)
	require.True(t, observer.Character.Conditions.AddCondition(hoodTestGloomCond, true))

	_, ok, why := user.Character.Wear(items.New(hoodTestLanternItem))
	require.True(t, ok, why)
	require.True(t, user.Character.EmitsLight(), "fixture: the lantern must be lit")
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(observer.Character, room),
		"fixture: the gloom must leave the observer blind even by the lantern")
	events.DrainQueuedMessagesForTest(observer.UserId)

	_, err := Hood("", user, room, 0)
	require.NoError(t, err)
	require.False(t, user.Character.EmitsLight(), "fixture: the hood must have gone dark")
	require.NotContains(t, hoodTestText(observer.UserId), "lowers the hood of their lantern",
		"an observer blind before the hood went down was told of it")
}

func TestHood_TwiceSaysAlreadyHooded(t *testing.T) {
	user, room := hoodFixture(t)
	_, ok, why := user.Character.Wear(items.New(hoodTestLanternItem))
	require.True(t, ok, why)
	rec := user.Character.Conditions.LightSources()[0]

	_, err := Hood("", user, room, 0)
	require.NoError(t, err)
	require.True(t, rec.Hooded)
	events.DrainQueuedMessagesForTest(user.UserId)

	_, err = Hood("", user, room, 0)
	require.NoError(t, err)
	require.Contains(t, hoodTestText(user.UserId), "already hooded")
	require.True(t, rec.Hooded)

	_, err = Unhood("", user, room, 0)
	require.NoError(t, err)
	events.DrainQueuedMessagesForTest(user.UserId)
	_, err = Unhood("", user, room, 0)
	require.NoError(t, err)
	require.Contains(t, hoodTestText(user.UserId), "already open")
}

// laterIndex returns the position of the first line from pool that appears in
// out after position from, or -1.
func laterIndex(out string, from int, pool []string) int {
	for _, l := range pool {
		if i := strings.Index(out[from:], l); i >= 0 {
			return from + i
		}
	}
	return -1
}

// Playtest finding (plan 5a): the band notice for the player's OWN light ran
// only in the NEXT command's pre-check, so "darkness closes in" printed after
// whatever was typed next and read backwards. Hood and unhood now carry their
// own notice, after the action line.
func TestHood_BandNoticeFollowsTheActionLine(t *testing.T) {
	user, room := hoodFixture(t)
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", Name: "Cave", Symbol: ".", SkyLight: rooms.SkyLightPtr(0.0), Indoor: true, MovementCost: 1},
	}))
	room.Biome = "cave"
	require.NoError(t, lightnotice.LoadFrom("../../_datafiles/world/dogmud/narration/light-notices"))
	t.Cleanup(lightnotice.ResetForTest)

	_, ok, why := user.Character.Wear(items.New(hoodTestLanternItem))
	require.True(t, ok, why)
	require.True(t, user.Character.EmitsLight(), "fixture: the lantern must be lit")
	lightnotice.Check(user, lightnotice.TriggerQuiet)
	events.DrainQueuedMessagesForTest(user.UserId)

	var darker, lighter []string
	for _, tr := range []lightnotice.Transition{lightnotice.DarkerDark, lightnotice.DarkerShapes} {
		darker = append(darker, lightnotice.Pool(lightnotice.CauseCarried, tr, true)...)
	}
	for _, tr := range []lightnotice.Transition{lightnotice.LighterFaces, lightnotice.LighterShapes} {
		lighter = append(lighter, lightnotice.Pool(lightnotice.CauseCarried, tr, true)...)
	}
	require.NotEmpty(t, darker)
	require.NotEmpty(t, lighter)

	_, err := Hood("", user, room, 0)
	require.NoError(t, err)
	out := hoodTestText(user.UserId)
	at := strings.Index(out, "You lower the hood over your lantern")
	require.GreaterOrEqual(t, at, 0, out)
	require.GreaterOrEqual(t, laterIndex(out, at, darker), 0,
		"hood must be followed by its darkness notice in the same output:\n%s", out)

	// The next command's pre-check finds the band unchanged and says nothing.
	lightnotice.Check(user, lightnotice.TriggerCommand)
	require.Empty(t, hoodTestText(user.UserId), "the notice was sent twice")

	_, err = Unhood("", user, room, 0)
	require.NoError(t, err)
	out = hoodTestText(user.UserId)
	at = strings.Index(out, "You throw back the hood of your lantern")
	require.GreaterOrEqual(t, at, 0, out)
	require.GreaterOrEqual(t, laterIndex(out, at, lighter), 0,
		"unhood must be followed by its light-returns notice in the same output:\n%s", out)
}

// hoodedDarkRoomWithWatcher wears the lantern, hoods it, and returns the
// bearer, the room and a second watcher standing in it, all queues drained.
// In a cave nothing else lights the room, so the hooded lantern leaves it dark.
func hoodedDarkRoomWithWatcher(t *testing.T, biome string) (*users.UserRecord, *rooms.Room, *users.UserRecord) {
	t.Helper()
	user, room := hoodFixture(t)
	room.Biome = biome
	watcher := users.GetByUserId(2)
	require.NotNil(t, watcher)
	watcher.Character.RoomId = room.RoomId
	room.AddPlayer(watcher.UserId)
	_, ok, why := user.Character.Wear(items.New(hoodTestLanternItem))
	require.True(t, ok, why)
	_, err := Hood("", user, room, 0)
	require.NoError(t, err)
	events.DrainQueuedMessagesForTest(user.UserId)
	events.DrainQueuedMessagesForTest(watcher.UserId)
	return user, room, watcher
}

const (
	hoodThrowsBackLine = "throws back the hood of their lantern, and light floods out."
	hoodRevealLine     = "A light flares to life, revealing"
)

// Owner ruling 2026-10-06: a watcher who could not see a moment before and
// can now is told the bearer appeared, not that a hood was thrown back.
func TestUnhood_WatcherInTheDarkReadsTheReveal(t *testing.T) {
	user, room, watcher := hoodedDarkRoomWithWatcher(t, "cave")
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(watcher.Character, room),
		"fixture: the room must be dark to the watcher before the unhood")

	_, err := Unhood("", user, room, 0)
	require.NoError(t, err)
	out := hoodTestText(watcher.UserId)
	require.Contains(t, out, hoodRevealLine)
	require.Contains(t, out, user.Character.Name)
	require.Contains(t, out, "standing there, holding a lantern.")
	require.NotContains(t, out, hoodThrowsBackLine)
}

func TestUnhood_WatcherInALitRoomReadsThrowsBack(t *testing.T) {
	user, room, watcher := hoodedDarkRoomWithWatcher(t, "city")
	require.NotEqual(t, messaging.SightNone, messaging.ParticipantSight(watcher.Character, room),
		"fixture: the room must be lit to the watcher before the unhood")

	_, err := Unhood("", user, room, 0)
	require.NoError(t, err)
	out := hoodTestText(watcher.UserId)
	require.Contains(t, out, hoodThrowsBackLine)
	require.NotContains(t, out, hoodRevealLine)
}

func TestUnhood_BlindedWatcherInTheDarkReadsNeither(t *testing.T) {
	user, room, watcher := hoodedDarkRoomWithWatcher(t, "cave")
	blindForSpeechTest(t, watcher)
	events.DrainQueuedMessagesForTest(watcher.UserId)

	_, err := Unhood("", user, room, 0)
	require.NoError(t, err)
	out := hoodTestText(watcher.UserId)
	require.NotContains(t, out, hoodRevealLine)
	require.NotContains(t, out, hoodThrowsBackLine)
}

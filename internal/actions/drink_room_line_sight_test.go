package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The drink room lines name the drinker, and since drink path unification a
// mob shares them, so they go out through SendTextVisualHidingNames: an
// observer who makes out shapes reads "a figure", one who sees nothing reads
// nothing, and the player drinker is still left out of their own room line.

const (
	drinkSightDrinkerId  = 7331
	drinkSightInfraId    = 7332
	drinkSightBlindId    = 7333
	drinkSightMobId      = 98331
	drinkSightInfraredId = 7341
)

var drinkSightTag = regexp.MustCompile(`<[^>]*>`)

func drinkSightPlain(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, strings.TrimSpace(drinkSightTag.ReplaceAllString(l, "")))
	}
	return out
}

// drinkSightScene seeds the parity world plus an infrared condition, and
// stands the drinker (7331), an infrared observer (7332) and an observer
// with no special sight (7333) in one room of the given biome.
func drinkSightScene(t *testing.T, biome string) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	seedParityWorld(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		parityHealCond: {ConditionId: parityHealCond, Name: "Test Mending", RoundInterval: 1, TriggerCount: 10,
			TickPool: "health", TickPercent: 0.10},
		75: {ConditionId: 75, Name: "Nausea", RoundInterval: 1, TriggerCount: 5},
		drinkSightInfraredId: {
			ConditionId: drinkSightInfraredId,
			Name:        "Test Heat Eyes",
			Flags:       []conditions.Flag{conditions.InfraredVision},
			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}},
		},
	}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave":    {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
		"city":    {BiomeId: "city", Lamp: rooms.LampPtr(90)},
		"default": {BiomeId: "default"},
	}))
	drinker := users.NewTestUser(drinkSightDrinkerId, "drinker", "Drinker", 97331)
	drinker.Character = newParityChar()
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		drinkSightDrinkerId: drinker,
		drinkSightInfraId:   users.NewTestUser(drinkSightInfraId, "heateye", "Heateye", 97332),
		drinkSightBlindId:   users.NewTestUser(drinkSightBlindId, "plain", "Plain", 97333),
	}))
	if !users.GetByUserId(drinkSightInfraId).Character.Conditions.AddCondition(drinkSightInfraredId, true) {
		t.Fatal("precondition: the observer should now carry infrared")
	}
	room := &rooms.Room{RoomId: 7330, Biome: biome}
	for _, id := range []int{drinkSightDrinkerId, drinkSightInfraId, drinkSightBlindId} {
		room.AddPlayer(id)
		events.DrainQueuedMessagesForTest(id)
	}
	return drinker, room
}

func drinkSightMob(room *rooms.Room) DrinkActor {
	m := &mobs.Mob{InstanceId: drinkSightMobId, Character: *newParityChar()}
	return NewMobActorInRoom(m, room).(DrinkActor)
}

func requireOneLine(t *testing.T, who string, got []string, want string) {
	t.Helper()
	if len(got) != 1 || got[0] != want {
		t.Fatalf("%s read %q, want one line %q", who, got, want)
	}
}

func TestDrinkRoomLine_PlayerDrinkerHiddenFromShapes(t *testing.T) {
	drinker, room := drinkSightScene(t, "cave")
	storeItem(t, drinker.Character, items.New(parityBrewId))

	if res := Quaff(NewUserActorInRoom(drinker, room).(DrinkActor), "test brew"); !res.Drank {
		t.Fatalf("drink did not happen: %+v", res)
	}

	requireOneLine(t, "infrared observer", drinkSightPlain(events.DrainQueuedMessagesForTest(drinkSightInfraId)),
		"A figure drinks Test Brew.")
	if got := events.DrainQueuedMessagesForTest(drinkSightBlindId); len(got) != 0 {
		t.Errorf("an observer who cannot see got %q", got)
	}
	for _, l := range drinkSightPlain(events.DrainQueuedMessagesForTest(drinkSightDrinkerId)) {
		if strings.Contains(l, "drinks") {
			t.Errorf("the drinker read their own room line %q", l)
		}
	}
}

func TestDrinkRoomLine_MobDrinkerHiddenFromShapes(t *testing.T) {
	_, room := drinkSightScene(t, "cave")
	mob := drinkSightMob(room)
	storeItem(t, mob.GetCharacter(), items.New(parityBrewId))

	if res := Quaff(mob, "test brew"); !res.Drank {
		t.Fatalf("drink did not happen: %+v", res)
	}

	requireOneLine(t, "infrared observer", drinkSightPlain(events.DrainQueuedMessagesForTest(drinkSightInfraId)),
		"A figure drinks Test Brew.")
	if got := events.DrainQueuedMessagesForTest(drinkSightBlindId); len(got) != 0 {
		t.Errorf("an observer who cannot see got %q", got)
	}
}

func TestDrinkRoomLine_SpoiledDrinkerHiddenFromShapes(t *testing.T) {
	_, room := drinkSightScene(t, "cave")
	mob := drinkSightMob(room)
	it := items.New(parityAgedId)
	it.CraftedRound = util.GetRoundCount() - 5000
	storeItem(t, mob.GetCharacter(), it)

	if res := Quaff(mob, "aged brew"); !res.Spoiled {
		t.Fatalf("want a spoiled drink: %+v", res)
	}

	requireOneLine(t, "infrared observer", drinkSightPlain(events.DrainQueuedMessagesForTest(drinkSightInfraId)),
		"A figure drinks something and immediately gags.")
	if got := events.DrainQueuedMessagesForTest(drinkSightBlindId); len(got) != 0 {
		t.Errorf("an observer who cannot see got %q", got)
	}
}

// The control: in a lit room both observers read the drinker's name, so the
// hiding above is the sight tier and not a line that never names anyone.
func TestDrinkRoomLine_LitObserversReadTheName(t *testing.T) {
	_, room := drinkSightScene(t, "city")
	mob := drinkSightMob(room)
	storeItem(t, mob.GetCharacter(), items.New(parityBrewId))

	Quaff(mob, "test brew")

	for _, id := range []int{drinkSightInfraId, drinkSightBlindId} {
		requireOneLine(t, "lit observer", drinkSightPlain(events.DrainQueuedMessagesForTest(id)),
			"Drinker drinks Test Brew.")
	}
}

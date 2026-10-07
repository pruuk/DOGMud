package lightnotice

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// withShippedWorld loads the REAL biome files and pins the clock, following
// internal/rooms/city_tier_light_test.go's withShippedBiomesAndClock.
func withShippedWorld(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = `../../_datafiles/world/dogmud`
	cfg.Timing.RoundsPerDay = 900
	cfg.Timing.RoundSeconds = 4
	cfg.Timing.Validate()
	cfg.Balance.Validate()
	configs.SetConfigForTest(t, cfg)

	t.Cleanup(rooms.SeedBiomesForTest(nil))
	rooms.LoadBiomeDataFiles()

	original := util.GetRoundCount()
	t.Cleanup(func() {
		util.SetRoundCountForTest(original)
		gametime.ClearDateCacheForTest()
		gametime.ClearCelestialMemoForTest()
	})

	if err := LoadFrom(shippedDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ResetForTest)
}

// setClock moves the world to a day of year and an hour (900 rounds a day).
func setClock(doy int, hour float64) {
	util.SetRoundCountForTest(uint64(float64(doy-1)*900 + hour*37.5))
	gametime.ClearDateCacheForTest()
	gametime.ClearCelestialMemoForTest()
}

func seedCity(t *testing.T) {
	t.Helper()
	rs := map[int]*rooms.Room{
		1: {RoomId: 1, Zone: "City", Biome: "city_backstreet"},
		2: {RoomId: 2, Zone: "City", Biome: "city_thoroughfare"},
		3: {RoomId: 3, Zone: "City", Biome: "cave"},
	}
	t.Cleanup(rooms.SeedRoomsForTest(rs, map[string]*rooms.ZoneConfig{
		"City": {Name: "City", RoomId: 1, RoomIds: map[int]struct{}{1: {}, 2: {}, 3: {}}},
	}))
}

// logLightAt writes the real light level and the band a sightless-of-any-flag
// (strength 0, reach 0) observer reads at the clock the caller just set, so
// the report carries measured numbers rather than assumed ones.
func logLightAt(t *testing.T, label string, roomId int) {
	t.Helper()
	room := rooms.LoadRoom(roomId)
	if room == nil {
		t.Fatalf("%s: room %d did not load", label, roomId)
	}
	terms := room.LightTerms()
	cfg := configs.GetLightingConfig()
	band := messaging.BandThroughWindow(terms.Level, 0, 0, cfg.BlindBelow, cfg.DimBelow, cfg.DazzleAbove)
	t.Logf("%s: room %d LightTerms().Level=%d band=%s sky=%v lamp=%d hasLamp=%v",
		label, roomId, terms.Level, band, terms.Sky, terms.Lamp, terms.HasLamp)
}

// A backstreet loses faces between noon and midnight on midsummer day; a
// thoroughfare keeps faces from noon to midnight and says nothing.
//
// Before lighting plan 6 the street lamps burned at all hours, so a main
// street at midsummer noon read sky 73 combined with lamp 52, past the 75
// dazzle edge, and its dusk was the glare easing (DarkerFaces). Since plan 6
// (owner ruling O4 as amended) the lamps burn only while it is night or the
// clear sky is too dim to read a face: at noon the street
// reads its daylight alone, 73, faces and not dazzled, and at midnight its
// lamp, 54, still faces. No band moves, so no line.
//
// The backstreet's lamp lights between the two checks while its band drops:
// a lamp that brightened cannot have darkened the room, so the line blames
// the sky (lampAgrees), not "the lamplight fades".
func TestDuskTakesFacesFromTheBackstreetButNotTheThoroughfare(t *testing.T) {
	withShippedWorld(t)
	seedCity(t)
	lane := users.NewTestUser(1, "alice", "Aliceia", 1001)
	lane.Character.RoomId = 1
	street := users.NewTestUser(2, "bob", "Bobrick", 1002)
	street.Character.RoomId = 2
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: lane, 2: street}))

	drainLane := captureFor(t, 1)
	drainStreet := captureFor(t, 2)

	setClock(172, 12)
	logLightAt(t, "backstreet noon", 1)
	logLightAt(t, "thoroughfare noon", 2)
	Check(lane, TriggerQuiet)
	Check(street, TriggerQuiet)

	setClock(172, 0)
	logLightAt(t, "backstreet midnight", 1)
	logLightAt(t, "thoroughfare midnight", 2)
	Check(lane, TriggerCommand)
	Check(street, TriggerCommand)

	got := drainLane()
	if len(got) != 1 || !containsAny(got[0], Pool(CauseSky, DarkerShapes, false)) {
		t.Fatalf("backstreet at midnight: want one sky darker_shapes line, got %q", got)
	}

	if gotStreet := drainStreet(); len(gotStreet) != 0 {
		t.Fatalf("thoroughfare at midnight: faces kept from noon, want no line, got %q", gotStreet)
	}

	Check(lane, TriggerCommand)
	if got := drainLane(); len(got) != 0 {
		t.Fatalf("a second check says nothing, got %q", got)
	}
	Check(street, TriggerCommand)
	if got := drainStreet(); len(got) != 0 {
		t.Fatalf("a second check says nothing, got %q", got)
	}
}

// A night-sight 24 player walking out of a cave into a lamplit thoroughfare is
// dazzled: the shifted dazzle edge is 51 and the street lamp alone is 52.
func TestNightSightIsDazzledByTheThoroughfare(t *testing.T) {
	withShippedWorld(t)
	seedCity(t)
	const catEye = 99024
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		catEye: {
			ConditionId: catEye,
			Name:        "Test Cat Eye",
			Flags:       []conditions.Flag{conditions.NightVision},
			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectNightVisionStrength: {Literal: 24}},
		},
	}))
	u := users.NewTestUser(1, "alice", "Aliceia", 1001)
	u.Character.RoomId = 3
	if err := u.Character.AddCondition(catEye, true); err != nil {
		t.Fatal(err)
	}
	if got := u.Character.NightVisionStrength(); got != 24 {
		t.Fatalf("fixture strength = %d, want 24", got)
	}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: u}))
	drain := captureFor(t, 1)

	setClock(172, 0)
	Check(u, TriggerQuiet)
	if room := rooms.LoadRoom(2); room != nil {
		terms := room.LightTerms()
		band := messaging.BandThroughWindow(terms.Level, u.Character.NightVisionStrength(), u.Character.InfraReach(),
			configs.GetLightingConfig().BlindBelow, configs.GetLightingConfig().DimBelow, configs.GetLightingConfig().DazzleAbove)
		t.Logf("thoroughfare midnight for strength 24: LightTerms().Level=%d band=%s", terms.Level, band)
	}
	u.Character.RoomId = 2
	Check(u, TriggerMove)

	got := drain()
	if len(got) != 1 || !containsAny(got[0], Pool(CauseMovement, IntoDazzle, false)) {
		t.Fatalf("want one movement dazzle line, got %q", got)
	}
}

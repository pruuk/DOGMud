package rooms

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	trimGlowId   = 9721 // adjustable, magnitude
	trimSightId  = 9722 // nightvision 24
	trimGlow2Id  = 9723 // a second adjustable, magnitude
	trimTorchId  = 9724 // literal 56, not adjustable
	trimFromRoom = 7730
	trimToRoom   = 7731
)

func seedTrimFixture(t *testing.T) (a, b *users.UserRecord) {
	t.Helper()
	withShippedBiomesAndClock(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		trimGlowId: {ConditionId: trimGlowId, Name: "Test Glow", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {UsesMagnitude: true}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
		trimSightId: {ConditionId: trimSightId, Name: "Test Sight", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectNightVisionStrength: {Literal: 24}}},
		trimGlow2Id: {ConditionId: trimGlow2Id, Name: "Test Glow Two", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {UsesMagnitude: true}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
		trimTorchId: {ConditionId: trimTorchId, Name: "Test Torch", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 56}}},
	}))
	a = users.NewTestUser(7721, "glowa", "Glowa", 97721)
	b = users.NewTestUser(7722, "glowb", "Glowb", 97722)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7721: a, 7722: b}))
	for _, u := range []*users.UserRecord{a, b} {
		u.Character.Conditions.AddConditionMagnitude(trimGlowId, 4, 90)
	}
	b.Character.Conditions.AddCondition(trimSightId, false)
	return a, b
}

func glowOutput(u *users.UserRecord) (float64, bool) {
	rec := u.Character.Conditions.LightSources()[0]
	return rec.LightNow(conditions.GetConditionSpec(rec.ConditionId))
}

// Entry order sets the level: the first arrival trims to their own eyes, the
// second finds the room already bright enough, and nobody re-trims.
func TestEntryOrderSetsTheTrim(t *testing.T) {
	a, b := seedTrimFixture(t)
	requireBiome(t, "cave")
	room := &Room{RoomId: 7720, Biome: "cave"}

	room.AddPlayer(a.UserId)
	room.TrimLightFor(a.Character)
	if v, ok := glowOutput(a); !ok || math.Abs(v-74) > 1e-9 {
		t.Fatalf("first arrival's glow = (%v, %v), want 74, the top of normal eyes' band", v, ok)
	}
	if got := room.LightLevel(); got != 74 {
		t.Fatalf("cave after the first arrival = %d, want 74", got)
	}

	room.AddPlayer(b.UserId)
	room.TrimLightFor(b.Character)
	if _, ok := glowOutput(b); ok {
		t.Error("the nightvision arrival's glow should trim to nothing: the room is already past their comfort")
	}
	if v, _ := glowOutput(a); math.Abs(v-74) > 1e-9 {
		t.Errorf("the first arrival re-trimmed to %v when someone else entered", v)
	}
}

// In a lamplit room the trim solves the combine: lamp 50 plus the glow lands
// exactly on 74.
func TestTrimSolvesTheCombineInALitRoom(t *testing.T) {
	a, _ := seedTrimFixture(t)
	requireBiome(t, "interior")
	setClock(172, 0) // midnight: the interior's sky adds almost nothing
	room := &Room{RoomId: 7723, Biome: "interior"}
	room.AddPlayer(a.UserId)
	room.TrimLightFor(a.Character)
	if got := room.LightLevel(); got != 74 {
		t.Errorf("a lamplit interior with a trimmed glow reads %d, want 74", got)
	}
}

func TestAHoodedSourceIsNotTrimmed(t *testing.T) {
	a, _ := seedTrimFixture(t)
	requireBiome(t, "cave")
	room := &Room{RoomId: 7724, Biome: "cave"}
	room.AddPlayer(a.UserId)
	rec := a.Character.Conditions.LightSources()[0]
	rec.Hooded = true
	room.TrimLightFor(a.Character)
	if rec.LightTrim != conditions.LightFull {
		t.Errorf("a hooded source was trimmed to %q", rec.LightTrim)
	}
}

// A strengthless adjustable source (a Glow cast at magnitude 0) has nothing
// to trim: lightscale.Trim refuses it, and the record stays at full strength
// rather than being switched off, whether or not the room is already bright.
func TestAStrengthlessSourceStaysAtFullStrength(t *testing.T) {
	_, _ = seedTrimFixture(t)
	requireBiome(t, "cave")
	requireBiome(t, "interior")
	weak := users.NewTestUser(7723, "glowz", "Glowz", 97723)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7723: weak}))
	weak.Character.Conditions.AddConditionMagnitude(trimGlowId, 4, 0)
	for _, biome := range []string{"cave", "interior"} {
		room := &Room{RoomId: 7726, Biome: biome}
		room.AddPlayer(weak.UserId)
		room.TrimLightFor(weak.Character)
		rec := weak.Character.Conditions.LightSources()[0]
		if rec.LightTrim != conditions.LightFull {
			t.Errorf("%s: a strengthless glow landed on %q, want full", biome, rec.LightTrim)
		}
		if _, ok := glowOutput(weak); ok {
			t.Errorf("%s: a strengthless glow adds light", biome)
		}
	}
}

// One bearer's several sources trim in held order, each seeing the room
// without itself but with the earlier trims: the first carries the room to 74
// and the second finds nothing left to do.
func TestABearersSourcesTrimInHeldOrder(t *testing.T) {
	a, _ := seedTrimFixture(t)
	requireBiome(t, "cave")
	a.Character.Conditions.AddConditionMagnitude(trimGlow2Id, 4, 90)
	room := &Room{RoomId: 7725, Biome: "cave"}
	room.AddPlayer(a.UserId)
	room.TrimLightFor(a.Character)

	srcs := a.Character.Conditions.LightSources()
	if len(srcs) != 2 {
		t.Fatalf("bearer holds %d light sources, want 2", len(srcs))
	}
	first, second := srcs[0], srcs[1]
	if first.ConditionId != trimGlowId || second.ConditionId != trimGlow2Id {
		t.Fatalf("held order = %d, %d; want %d, %d", first.ConditionId, second.ConditionId, trimGlowId, trimGlow2Id)
	}
	if v, ok := first.LightNow(conditions.GetConditionSpec(first.ConditionId)); !ok || math.Abs(v-74) > 1e-9 {
		t.Errorf("first source = (%v, %v), want 74", v, ok)
	}
	if second.LightTrim != conditions.LightOff {
		t.Errorf("second source trim = %q, want %q", second.LightTrim, conditions.LightOff)
	}
	if got := room.LightLevel(); got != 74 {
		t.Errorf("cave with both sources trimmed = %d, want 74", got)
	}
}

// A source without the adjustable flag is left at full strength, even when
// something else is trimmed.
func TestANonAdjustableSourceIsLeftAlone(t *testing.T) {
	withShippedBiomesAndClock(t)
	requireBiome(t, "cave")
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		trimTorchId: {ConditionId: trimTorchId, Name: "Test Torch", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 56}}},
	}))
	u := users.NewTestUser(7726, "torcha", "Torcha", 97726)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7726: u}))
	u.Character.Conditions.AddCondition(trimTorchId, false)

	room := &Room{RoomId: 7727, Biome: "cave"}
	room.AddPlayer(u.UserId)
	room.TrimLightFor(u.Character)

	rec := u.Character.Conditions.LightSources()[0]
	if rec.LightTrim != conditions.LightFull {
		t.Errorf("a non-adjustable source was trimmed to %q", rec.LightTrim)
	}
	if got := room.LightLevel(); got != 56 {
		t.Errorf("cave with a full torch = %d, want 56", got)
	}
}

// A source too weak to reach the target runs uncut: it is at full strength,
// and its record says so rather than calling it trimmed.
func TestAnUncutSourceStaysFull(t *testing.T) {
	withShippedBiomesAndClock(t)
	requireBiome(t, "cave")
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		trimGlowId: {ConditionId: trimGlowId, Name: "Test Glow", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {UsesMagnitude: true}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
	}))
	u := users.NewTestUser(7728, "dima", "Dima", 97728)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7728: u}))
	u.Character.Conditions.AddConditionMagnitude(trimGlowId, 4, 54)

	room := &Room{RoomId: 7729, Biome: "cave"}
	room.AddPlayer(u.UserId)
	room.TrimLightFor(u.Character)

	rec := u.Character.Conditions.LightSources()[0]
	if rec.LightTrim != conditions.LightFull {
		t.Errorf("a source below target trimmed to %q, want full", rec.LightTrim)
	}
	if got := room.LightLevel(); got != 54 {
		t.Errorf("cave with an uncut 54 glow = %d, want 54", got)
	}
}

// AddMob is the other seam: a mob walking into a dark cave with a full glow
// arrives with it trimmed to its eyes.
func TestAddMobTrimsTheArrivalsLight(t *testing.T) {
	seedTrimFixture(t)
	requireBiome(t, "cave")

	const instId = 7740
	m := &mobs.Mob{MobId: 7741, InstanceId: instId, Character: characters.Character{Name: "glowmob", RoomId: trimFromRoom, Conditions: conditions.New()}}
	m.Character.Conditions.AddConditionMagnitude(trimGlowId, 4, 90)
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{}, map[int]*mobs.Mob{instId: m}))

	rec := m.Character.Conditions.LightSources()[0]
	if v, ok := rec.LightNow(conditions.GetConditionSpec(rec.ConditionId)); !ok || math.Abs(v-90) > 1e-9 {
		t.Fatalf("mob glow before entry = (%v, %v), want full strength 90", v, ok)
	}
	room := &Room{RoomId: trimToRoom, Biome: "cave"}
	room.AddMob(instId)
	if v, ok := rec.LightNow(conditions.GetConditionSpec(rec.ConditionId)); !ok || math.Abs(v-74) > 1e-9 {
		t.Errorf("mob glow after entering the cave = (%v, %v), want 74", v, ok)
	}
}

// MoveToRoom is the seam: a player walking into a dark cave with a full glow
// arrives with it trimmed to their eyes.
func TestMoveToRoomTrimsTheArrivalsLight(t *testing.T) {
	a, _ := seedTrimFixture(t)
	requireBiome(t, "cave")

	from := &Room{RoomId: trimFromRoom, Zone: "TrimZone", Title: "Mouth", Biome: "cave", players: []int{}}
	to := &Room{RoomId: trimToRoom, Zone: "TrimZone", Title: "Depths", Biome: "cave", players: []int{}}
	t.Cleanup(SeedRoomsForTest(
		map[int]*Room{trimFromRoom: from, trimToRoom: to},
		map[string]*ZoneConfig{
			"TrimZone": {Name: "TrimZone", RoomId: trimFromRoom, RoomIds: map[int]struct{}{trimFromRoom: {}, trimToRoom: {}}},
		},
	))

	a.Character.RoomId = trimFromRoom
	from.AddPlayer(a.UserId)
	MarkRoomOccupancy(trimFromRoom, 1, 0)

	if v, ok := glowOutput(a); !ok || math.Abs(v-90) > 1e-9 {
		t.Fatalf("glow before the move = (%v, %v), want full strength 90", v, ok)
	}
	if err := MoveToRoom(a.UserId, trimToRoom); err != nil {
		t.Fatalf("MoveToRoom: %v", err)
	}
	if v, ok := glowOutput(a); !ok || math.Abs(v-74) > 1e-9 {
		t.Errorf("glow after walking into the cave = (%v, %v), want 74", v, ok)
	}
}

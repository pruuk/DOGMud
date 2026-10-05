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
	darkUmbralId    = 9751 // literal darkness 50, adjustable: the Umbral Dark's shape
	darkFixedId     = 9752 // magnitude darkness, NOT adjustable: someone else's darkness
	darkSight24Id   = 9753 // nightvision 24
	darkInfra30Id   = 9754 // infra reach 30
	darkInfra50Id   = 9755 // infra reach 50
	darkHoodedId    = 9756 // literal light 54, adjustable: the hooded lantern's shape
	darkGlowId      = 9757 // magnitude light, adjustable: glow's shape
	darkFirstUser   = 7750
	darkFirstRoom   = 7770
	darkMobInst     = 7790
	darkMobId       = 7791
	darkTrimEpsilon = 1e-9
)

// seedDarknessTrim seeds the conditions every darkness trim test uses, on the
// shipped biomes and clock (the cave biome has no sky, so a cave room's light
// is its lamp alone).
func seedDarknessTrim(t *testing.T) *darkUsers {
	t.Helper()
	withShippedBiomesAndClock(t)
	requireBiome(t, "cave")
	lit := func(id int, name string, v conditions.EffectValue, kind conditions.EffectKind, adjustable bool) *conditions.ConditionSpec {
		s := &conditions.ConditionSpec{ConditionId: id, Name: name, TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{kind: v}}
		if adjustable {
			s.Flags = []conditions.Flag{conditions.Adjustable}
		}
		return s
	}
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		darkUmbralId:  lit(darkUmbralId, "Test Umbral", conditions.EffectValue{Literal: 50}, conditions.EffectDarknessStrength, true),
		darkFixedId:   lit(darkFixedId, "Test Fixed Dark", conditions.EffectValue{UsesMagnitude: true}, conditions.EffectDarknessStrength, false),
		darkSight24Id: lit(darkSight24Id, "Test Sight", conditions.EffectValue{Literal: 24}, conditions.EffectNightVisionStrength, false),
		darkInfra30Id: lit(darkInfra30Id, "Test Infra 30", conditions.EffectValue{Literal: 30}, conditions.EffectInfraReach, false),
		darkInfra50Id: lit(darkInfra50Id, "Test Infra 50", conditions.EffectValue{Literal: 50}, conditions.EffectInfraReach, false),
		darkHoodedId:  lit(darkHoodedId, "Test Hooded", conditions.EffectValue{Literal: 54}, conditions.EffectLightStrength, true),
		darkGlowId:    lit(darkGlowId, "Test Glow", conditions.EffectValue{UsesMagnitude: true}, conditions.EffectLightStrength, true),
	}))
	return &darkUsers{t: t, all: map[int]*users.UserRecord{}}
}

// darkUsers is one test's users. SeedUsersForTest replaces the whole
// registry, so every add reseeds all of the test's users together; the
// stacked cleanups restore the registry in reverse.
type darkUsers struct {
	t   *testing.T
	all map[int]*users.UserRecord
}

// add makes a test user holding the given literal conditions and seeds it
// beside every user this test already made.
func (d *darkUsers) add(id int, conds ...int) *users.UserRecord {
	d.t.Helper()
	u := users.NewTestUser(id, "darku", "Darku", uint64(90000+id))
	for _, c := range conds {
		u.Character.Conditions.AddCondition(c, false)
	}
	d.all[id] = u
	seed := make(map[int]*users.UserRecord, len(d.all))
	for k, v := range d.all {
		seed[k] = v
	}
	d.t.Cleanup(users.SeedUsersForTest(seed))
	return u
}

// sourceNow is the record's current output, and false when it is off.
func sourceNow(c *characters.Character, conditionId int) (float64, bool) {
	for _, rec := range c.Conditions.LightAndDarknessSources() {
		if rec.ConditionId == conditionId {
			return rec.LightNow(conditions.GetConditionSpec(rec.ConditionId))
		}
	}
	return 0, false
}

// The Rule 3 trim table of the spec, through a real room: an Umbral Lantern
// (full 50) entering alone, or beside someone else's darkness.
func TestUmbralLanternTrimsToItsBearersEyes(t *testing.T) {
	us := seedDarknessTrim(t)
	cases := []struct {
		name      string
		lamp      int // 0 means no lamp: an unlit cave
		eyes      []int
		otherDark float64 // 0 means none
		wantOff   bool
		wantOut   float64
		wantRoom  int
	}{
		{"normal eyes, cave: off", 0, nil, 0, true, 0, 0},
		{"normal eyes, tavern 50", 50, nil, 0, false, 25, 25},
		{"normal eyes, light 70", 70, nil, 0, false, 45, 25},
		{"normal eyes, light 90: full", 90, nil, 0, false, 50, 40},
		{"nightvision 24, light 50", 50, []int{darkSight24Id}, 0, false, 49, 1},
		{"infravision 30, cave", 0, []int{darkInfra30Id}, 0, false, 30, -30},
		{"infravision 50, cave: full", 0, []int{darkInfra50Id}, 0, false, 50, -50},
		{"infravision 50, light 50: full", 50, []int{darkInfra50Id}, 0, false, 50, 0},
		{"normal eyes, light 50, other darkness 20", 50, nil, 20, false, 12.93, 25},
		{"normal eyes, light 50, other darkness 30: off", 50, nil, 30, true, 0, 20},
	}
	for i, c := range cases {
		room := &Room{RoomId: darkFirstRoom + i, Biome: "cave"}
		if c.lamp > 0 {
			room.Lamp = LampPtr(c.lamp)
		}
		if c.otherDark > 0 {
			other := us.add(darkFirstUser + 100 + i)
			other.Character.Conditions.AddConditionMagnitude(darkFixedId, 4, c.otherDark)
			room.AddPlayer(other.UserId)
		}
		bearer := us.add(darkFirstUser+i, append(append([]int{}, c.eyes...), darkUmbralId)...)
		room.AddPlayer(bearer.UserId)
		room.TrimLightFor(bearer.Character)

		out, on := sourceNow(bearer.Character, darkUmbralId)
		switch {
		case c.wantOff && on:
			t.Errorf("%s: output %v, want off", c.name, out)
		case !c.wantOff && (!on || math.Abs(out-c.wantOut) > 0.01):
			t.Errorf("%s: output (%v, %v), want %v", c.name, out, on, c.wantOut)
		}
		if got := room.LightLevel(); got != c.wantRoom {
			t.Errorf("%s: room after = %d, want %d", c.name, got, c.wantRoom)
		}
	}
}

// A full-strength result is recorded as full, not as trimmed at 50.
func TestAnUncutDarknessStaysFull(t *testing.T) {
	us := seedDarknessTrim(t)
	room := &Room{RoomId: darkFirstRoom + 20, Biome: "cave"}
	bearer := us.add(darkFirstUser+20, darkInfra50Id, darkUmbralId)
	room.AddPlayer(bearer.UserId)
	room.TrimLightFor(bearer.Character)
	if rec := bearer.Character.Conditions.DarknessSources()[0]; rec.LightTrim != conditions.LightFull {
		t.Errorf("an uncut darkness trim state = %q, want full", rec.LightTrim)
	}
}

// The light row of Rule 3: a light in a darkened room may run brighter before
// it dazzles, because the light trim solves against its target plus the
// room's darkness (ruling D3).
func TestALightTrimsAgainstTheRoomsDarkness(t *testing.T) {
	us := seedDarknessTrim(t)
	cases := []struct {
		name     string
		lamp     int
		darkness bool
		wantFull bool
		wantRoom int
	}{
		{"light 70, darkness 50", 70, true, true, 23},
		{"light 74, darkness 50", 74, true, true, 26},
		{"light 74, no darkness: off", 74, false, false, 74},
	}
	for i, c := range cases {
		room := &Room{RoomId: darkFirstRoom + 30 + i, Biome: "cave", Lamp: LampPtr(c.lamp)}
		if c.darkness {
			other := us.add(darkFirstUser + 130 + i)
			other.Character.Conditions.AddConditionMagnitude(darkFixedId, 4, 50)
			room.AddPlayer(other.UserId)
		}
		bearer := us.add(darkFirstUser+30+i, darkHoodedId)
		room.AddPlayer(bearer.UserId)
		room.TrimLightFor(bearer.Character)

		rec := bearer.Character.Conditions.LightSources()[0]
		if c.wantFull && rec.LightTrim != conditions.LightFull {
			t.Errorf("%s: the lantern trimmed to %q, want full", c.name, rec.LightTrim)
		}
		if !c.wantFull && rec.LightTrim != conditions.LightOff {
			t.Errorf("%s: the lantern trim = %q, want off", c.name, rec.LightTrim)
		}
		if got := room.LightLevel(); got != c.wantRoom {
			t.Errorf("%s: room after = %d, want %d", c.name, got, c.wantRoom)
		}
	}
}

// A bearer's light and darkness trim in held order, each seeing the room as
// the earlier ones left it. Glow first: the glow lights the cave to 74, then
// the darkness cuts it to the bearer's floor. Darkness first: the cave is
// already below the floor, so the darkness goes off, then the glow runs to 74.
func TestALightAndADarknessTrimInHeldOrder(t *testing.T) {
	us := seedDarknessTrim(t)

	glowFirst := us.add(darkFirstUser + 40)
	glowFirst.Character.Conditions.AddConditionMagnitude(darkGlowId, 4, 90)
	glowFirst.Character.Conditions.AddCondition(darkUmbralId, false)
	room := &Room{RoomId: darkFirstRoom + 40, Biome: "cave"}
	room.AddPlayer(glowFirst.UserId)
	room.TrimLightFor(glowFirst.Character)
	if v, ok := sourceNow(glowFirst.Character, darkGlowId); !ok || math.Abs(v-74) > darkTrimEpsilon {
		t.Errorf("glow first: glow = (%v, %v), want 74", v, ok)
	}
	if v, ok := sourceNow(glowFirst.Character, darkUmbralId); !ok || math.Abs(v-49) > darkTrimEpsilon {
		t.Errorf("glow first: darkness = (%v, %v), want 49", v, ok)
	}
	if got := room.LightLevel(); got != 25 {
		t.Errorf("glow first: room = %d, want 25", got)
	}

	darkFirst := us.add(darkFirstUser+41, darkUmbralId)
	darkFirst.Character.Conditions.AddConditionMagnitude(darkGlowId, 4, 90)
	room2 := &Room{RoomId: darkFirstRoom + 41, Biome: "cave"}
	room2.AddPlayer(darkFirst.UserId)
	room2.TrimLightFor(darkFirst.Character)
	if _, ok := sourceNow(darkFirst.Character, darkUmbralId); ok {
		t.Error("darkness first: the darkness should go off in an unlit cave")
	}
	if got := room2.LightLevel(); got != 74 {
		t.Errorf("darkness first: room = %d, want 74", got)
	}
}

// Arrivals trim in entry order and nobody already here re-trims: darkness
// arriving is never countered by what is there, and a later arrival does
// not move an earlier one (arc ruling 3).
func TestDarknessEntryOrderAcrossThreeBearers(t *testing.T) {
	us := seedDarknessTrim(t)
	room := &Room{RoomId: darkFirstRoom + 50, Biome: "cave", Lamp: LampPtr(50)}

	a := us.add(darkFirstUser+50, darkUmbralId)
	room.AddPlayer(a.UserId)
	room.TrimLightFor(a.Character)
	if got := room.LightLevel(); got != 25 {
		t.Fatalf("after the normal-eyed bearer: room %d, want 25", got)
	}

	b := us.add(darkFirstUser+51, darkInfra50Id, darkUmbralId)
	room.AddPlayer(b.UserId)
	room.TrimLightFor(b.Character)
	if rec := b.Character.Conditions.DarknessSources()[0]; rec.LightTrim != conditions.LightFull {
		t.Errorf("the infravision bearer's darkness = %q, want full", rec.LightTrim)
	}
	if got := room.LightLevel(); got != -1 {
		t.Errorf("after the infravision bearer: room %d, want -1", got)
	}

	c := us.add(darkFirstUser + 52)
	c.Character.Conditions.AddConditionMagnitude(darkGlowId, 4, 90)
	room.AddPlayer(c.UserId)
	room.TrimLightFor(c.Character)
	if rec := c.Character.Conditions.LightSources()[0]; rec.LightTrim != conditions.LightFull {
		t.Errorf("the glow arriving in the darkened room = %q, want full", rec.LightTrim)
	}
	if got := room.LightLevel(); got != 39 {
		t.Errorf("after the glow: room %d, want 39", got)
	}

	if v, ok := sourceNow(a.Character, darkUmbralId); !ok || math.Abs(v-25) > darkTrimEpsilon {
		t.Errorf("the first bearer re-trimmed to (%v, %v) when others entered, want 25", v, ok)
	}
}

// A spawn lists its mob with listMobInRoom, not AddMob, so it does not trim:
// the Phantom's lantern is at full strength in its lair from the start.
func TestASpawnedMobsDarknessDoesNotTrim(t *testing.T) {
	seedDarknessTrim(t)
	m := &mobs.Mob{MobId: darkMobId, InstanceId: darkMobInst,
		Character: characters.Character{Name: "darkmob", RoomId: darkFirstRoom + 60, Conditions: conditions.New()}}
	m.Character.Conditions.AddCondition(darkUmbralId, false)
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{}, map[int]*mobs.Mob{darkMobInst: m}))

	room := &Room{RoomId: darkFirstRoom + 60, Biome: "cave"}
	listMobInRoom(room, darkMobInst)
	if rec := m.Character.Conditions.DarknessSources()[0]; rec.LightTrim != conditions.LightFull {
		t.Errorf("a spawned mob's darkness = %q, want full", rec.LightTrim)
	}
	if got := room.LightLevel(); got != -50 {
		t.Errorf("the lair after a spawn = %d, want -50", got)
	}
}

// A mob walking in (AddMob) does trim: a normal-eyed mob's darkness goes off
// in an unlit cave.
func TestAMobWalkingInTrimsItsDarkness(t *testing.T) {
	seedDarknessTrim(t)
	m := &mobs.Mob{MobId: darkMobId, InstanceId: darkMobInst + 1,
		Character: characters.Character{Name: "darkmob", RoomId: darkFirstRoom + 61, Conditions: conditions.New()}}
	m.Character.Conditions.AddCondition(darkUmbralId, false)
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{}, map[int]*mobs.Mob{darkMobInst + 1: m}))

	room := &Room{RoomId: darkFirstRoom + 62, Biome: "cave"}
	room.AddMob(darkMobInst + 1)
	if _, ok := sourceNow(&m.Character, darkUmbralId); ok {
		t.Error("a normal-eyed mob walking into an unlit cave kept its darkness on")
	}
}

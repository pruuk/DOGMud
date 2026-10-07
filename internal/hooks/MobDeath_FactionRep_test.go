package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crimes"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// writeFactionDef writes a faction definition YAML to the override dir.
func writeFactionDef(t *testing.T, slug, body string) {
	t.Helper()
	dir := os.Getenv("DOGMUD_FACTIONS_DIR_OVERRIDE")
	if err := os.WriteFile(filepath.Join(dir, slug+".yaml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

// setupFactionsForHookTest wires temp dirs + loads thornwall_citizens.
func setupFactionsForHookTest(t *testing.T) {
	t.Helper()
	t.Setenv("DOGMUD_FACTIONS_DIR_OVERRIDE", t.TempDir())
	t.Setenv("DOGMUD_FACTIONS_REP_DIR_OVERRIDE", t.TempDir())
	t.Setenv("DOGMUD_FACTIONS_CRIMES_DIR_OVERRIDE", t.TempDir())
	writeFactionDef(t, "thornwall_citizens", `
faction_id: thornwall_citizens
display_name: "Thornwall Citizenry"
description: "x"
default_rep: 0
allies: []
enemies: []
`)
	if err := factions.LoadAllDefinitions(); err != nil {
		t.Fatal(err)
	}
	factions.ClearCache()
	crimes.ClearCache()
}

// seedHookRoom registers a minimal room via SeedRoomsForTest and
// returns a cleanup func.
func seedHookRoom(t *testing.T, roomId int) (*rooms.Room, func()) {
	t.Helper()
	room := &rooms.Room{
		RoomId:      roomId,
		Zone:        "Thornwall City",
		Title:       "Test Room",
		Description: "A test room.",
		Exits:       map[string]exit.RoomExit{},
		Biome:       "city",
	}
	cleanupRooms := rooms.SeedRoomsForTest(
		map[int]*rooms.Room{roomId: room},
		map[string]*rooms.ZoneConfig{
			"Thornwall City": {
				Name:    "Thornwall City",
				RoomId:  roomId,
				RoomIds: map[int]struct{}{roomId: {}},
			},
		},
	)
	cleanupBiomes := rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"city": {
			BiomeId:      "city",
			Name:         "City",
			Symbol:       "#",
			MovementCost: 1.0,
			// Lamp pins this fixture fully lit regardless of the ambient
			// test round. Since graded lighting plan 3a Task 8,
			// Room.LightLevel() reads the real celestial term at whatever
			// round util.GetRoundCount() holds, which a bare unpinned round
			// reads as shapes tier, not full. None of this helper's callers
			// mutate the room's biome to simulate darkness, so this is safe.
			Lamp: rooms.LampPtr(90),
		},
	})
	return room, func() {
		cleanupBiomes()
		cleanupRooms()
	}
}

// makeCitizenMob builds a citizen mob spec + instance pair.
func makeCitizenMob(mobId, instId, roomId int, name string) (*mobs.Mob, *mobs.Mob) {
	spec := &mobs.Mob{
		MobId:  mobs.MobId(mobId),
		Groups: []string{"thornwall_citizens"},
		Zone:   "Thornwall City",
		Character: characters.Character{
			Name:       name,
			Conditions: conditions.New(),
		},
	}
	inst := &mobs.Mob{
		MobId:      mobs.MobId(mobId),
		InstanceId: instId,
		Groups:     []string{"thornwall_citizens"},
		Zone:       "Thornwall City",
		Character: characters.Character{
			Name:       name,
			RoomId:     roomId,
			Conditions: conditions.New(),
		},
	}
	return spec, inst
}

// TestMobDeathFactionRep_MurderWithWitness verifies that when a
// citizen is killed with a witness present the crime is recorded
// with PerpPlayer and the rep delta is applied.
func TestMobDeathFactionRep_MurderWithWitness(t *testing.T) {
	setupFactionsForHookTest(t)

	const roomId = 467

	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()

	// Victim: citizen mob 100, instance 200 (the one being killed).
	victimSpec, victimInst := makeCitizenMob(100, 200, roomId, "innocent citizen")
	// Witness: citizen mob 101, instance 201 (alive in the room).
	witnessSpec, witnessInst := makeCitizenMob(101, 201, roomId, "city guard")

	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{100: victimSpec, 101: witnessSpec},
		map[int]*mobs.Mob{200: victimInst, 201: witnessInst},
	)
	defer cleanupMobs()

	// Only the witness is in the room (victim is already dead /
	// excluded by instance id).
	room.AddMob(201)

	evt := events.MobDeath{
		MobId:        100,
		InstanceId:   200,
		RoomId:       roomId,
		PlayerDamage: map[int]int{17: 50},
	}
	MobDeathFactionRep(evt)

	// Witness present: should record PerpPlayer crime.
	got := crimes.AllForFaction("thornwall_citizens", true)
	if len(got) != 1 {
		t.Fatalf("expected 1 crime, got %d: %+v", len(got), got)
	}
	c := got[0]
	if c.Kind != crimes.KindMurder {
		t.Errorf("kind = %s, want murder", c.Kind)
	}
	if c.Perpetrator.Type != crimes.PerpPlayer || c.Perpetrator.Id != 17 {
		t.Errorf("perpetrator = %+v, want {player, 17}", c.Perpetrator)
	}

	// Rep should have been bumped.
	want := int(configs.GetBalanceConfig().CrimeRepDeltaMurder)
	if got := factions.GetRep("thornwall_citizens", 17); got != want {
		t.Errorf("rep = %d, want %d", got, want)
	}
}

// TestMobDeathFactionRep_LoneMurderUnknownPerp verifies that when
// there are no witnesses the crime records PerpUnknown and NO rep
// bump is applied.
func TestMobDeathFactionRep_LoneMurderUnknownPerp(t *testing.T) {
	setupFactionsForHookTest(t)

	const roomId = 467

	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()
	_ = room // no mobs added — room is empty

	victimSpec, victimInst := makeCitizenMob(100, 200, roomId, "innocent citizen")
	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{100: victimSpec},
		map[int]*mobs.Mob{200: victimInst},
	)
	defer cleanupMobs()

	// Victim instance excluded; room has no other mobs → no witnesses.
	evt := events.MobDeath{
		MobId:        100,
		InstanceId:   200,
		RoomId:       roomId,
		PlayerDamage: map[int]int{17: 50},
	}
	MobDeathFactionRep(evt)

	got := crimes.AllForFaction("thornwall_citizens", true)
	if len(got) != 1 {
		t.Fatalf("expected 1 crime, got %d", len(got))
	}
	c := got[0]
	if c.Kind != crimes.KindMurder {
		t.Errorf("kind = %s, want murder", c.Kind)
	}
	if c.Perpetrator.Type != crimes.PerpUnknown {
		t.Errorf("perpetrator type = %s, want unknown", c.Perpetrator.Type)
	}

	// No rep bump for lone/unwitnessed murder.
	if rep := factions.GetRep("thornwall_citizens", 17); rep != 0 {
		t.Errorf("rep = %d, want 0 (lone murder, no witness)", rep)
	}
}

// TestMobDeathFactionRep_UpgradesAssaultToMurder verifies that when a
// recent assault row exists it is upgraded in-place and only the
// incremental delta is applied.
func TestMobDeathFactionRep_UpgradesAssaultToMurder(t *testing.T) {
	setupFactionsForHookTest(t)

	const roomId = 467

	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()

	// Witness present so perp is identified.
	witnessSpec, witnessInst := makeCitizenMob(101, 201, roomId, "witness")
	victimSpec, victimInst := makeCitizenMob(100, 200, roomId, "victim")
	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{100: victimSpec, 101: witnessSpec},
		map[int]*mobs.Mob{200: victimInst, 201: witnessInst},
	)
	defer cleanupMobs()
	room.AddMob(201)

	// Pre-seed an assault row with hadExternalWitness=true (witness
	// was present at first aggression).
	assaultVictim := &mobs.Mob{MobId: 100}
	crimes.Record([]string{"thornwall_citizens"}, crimes.KindAssault,
		crimes.Perpetrator{Type: crimes.PerpPlayer, Id: 17},
		assaultVictim, 200, roomId, "Thornwall City", true)
	// Apply the assault delta so we can track the incremental.
	cfg := configs.GetBalanceConfig()
	factions.BumpRep("thornwall_citizens", 17, int(cfg.CrimeRepDeltaAssault))

	repAfterAssault := factions.GetRep("thornwall_citizens", 17)

	evt := events.MobDeath{
		MobId:        100,
		InstanceId:   200,
		RoomId:       roomId,
		PlayerDamage: map[int]int{17: 50},
	}
	MobDeathFactionRep(evt)

	// Still ONE crime row (upgraded, not a new row).
	got := crimes.AllForFaction("thornwall_citizens", true)
	if len(got) != 1 {
		t.Fatalf("expected 1 crime after upgrade, got %d", len(got))
	}
	if got[0].Kind != crimes.KindMurder {
		t.Errorf("kind after upgrade = %s, want murder", got[0].Kind)
	}

	// Rep should have moved by the incremental difference only.
	wantDelta := int(cfg.CrimeRepDeltaMurder) - int(cfg.CrimeRepDeltaAssault)
	wantRep := repAfterAssault + wantDelta
	if rep := factions.GetRep("thornwall_citizens", 17); rep != wantRep {
		t.Errorf("rep = %d, want %d (repAfterAssault=%d, increment=%d)",
			rep, wantRep, repAfterAssault, wantDelta)
	}
}

// TestMobDeathFactionRep_NoFactionsNoChange verifies the early-exit
// path when the victim mob has no defined factions.
func TestMobDeathFactionRep_NoFactionsNoChange(t *testing.T) {
	setupFactionsForHookTest(t)

	mob := &mobs.Mob{
		MobId:      999,
		InstanceId: 201,
		Groups:     []string{"humanoid"}, // no defined faction
		Character:  characters.Character{Name: "x", RoomId: 467, Conditions: conditions.New()},
	}
	cleanup := mobs.SeedMobsForTest(map[int]*mobs.Mob{999: mob}, map[int]*mobs.Mob{201: mob})
	defer cleanup()

	evt := events.MobDeath{
		MobId:        999,
		InstanceId:   201,
		RoomId:       467,
		PlayerDamage: map[int]int{17: 50},
	}
	MobDeathFactionRep(evt)

	if got := crimes.AllForFaction("thornwall_citizens", true); len(got) != 0 {
		t.Errorf("non-faction kill recorded a crime: %+v", got)
	}
	if rep := factions.GetRep("thornwall_citizens", 17); rep != 0 {
		t.Errorf("rep changed for non-faction kill: %d", rep)
	}
}

// TestMobDeathFactionRep_NoPlayersNoChange verifies the early-exit
// path when PlayerDamage is empty (environmental / mob death).
func TestMobDeathFactionRep_NoPlayersNoChange(t *testing.T) {
	setupFactionsForHookTest(t)

	mob := &mobs.Mob{
		MobId:      100,
		InstanceId: 202,
		Groups:     []string{"thornwall_citizens"},
		Character:  characters.Character{Name: "x", RoomId: 467, Conditions: conditions.New()},
	}
	cleanup := mobs.SeedMobsForTest(map[int]*mobs.Mob{100: mob}, map[int]*mobs.Mob{202: mob})
	defer cleanup()

	evt := events.MobDeath{
		MobId:        100,
		InstanceId:   202,
		RoomId:       467,
		PlayerDamage: map[int]int{}, // no damager
	}
	MobDeathFactionRep(evt)

	if got := crimes.AllForFaction("thornwall_citizens", true); len(got) != 0 {
		t.Errorf("crime recorded with no damager: %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Four-case upgrade tests (chunk 1.3 lone-murder fix)
// ---------------------------------------------------------------------------

// TestMobDeathFactionRep_CaseA_ExternalWitnessAtBoth verifies that when
// witnesses are present at both assault-time (HadExternalWitness=true)
// and murder-time (witnesses in room), the perp is identified and the
// incremental delta is applied.
func TestMobDeathFactionRep_CaseA_ExternalWitnessAtBoth(t *testing.T) {
	setupFactionsForHookTest(t)
	cfg := configs.GetBalanceConfig()

	const roomId = 467
	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()

	// Witness present at murder time.
	witnessSpec, witnessInst := makeCitizenMob(101, 201, roomId, "city guard")
	victimSpec, victimInst := makeCitizenMob(100, 200, roomId, "victim")
	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{100: victimSpec, 101: witnessSpec},
		map[int]*mobs.Mob{200: victimInst, 201: witnessInst},
	)
	defer cleanupMobs()
	room.AddMob(201)

	// Pre-seed assault with HadExternalWitness=true.
	assaultVictim := &mobs.Mob{MobId: 100}
	crimes.Record([]string{"thornwall_citizens"}, crimes.KindAssault,
		crimes.Perpetrator{Type: crimes.PerpPlayer, Id: 17},
		assaultVictim, 200, roomId, "Thornwall City", true)
	factions.BumpRep("thornwall_citizens", 17, int(cfg.CrimeRepDeltaAssault))
	repAfterAssault := factions.GetRep("thornwall_citizens", 17)

	evt := events.MobDeath{
		MobId: 100, InstanceId: 200, RoomId: roomId,
		PlayerDamage: map[int]int{17: 50},
	}
	MobDeathFactionRep(evt)

	got := crimes.AllForFaction("thornwall_citizens", true)
	if len(got) != 1 {
		t.Fatalf("CaseA: expected 1 crime, got %d", len(got))
	}
	c := got[0]
	if c.Kind != crimes.KindMurder {
		t.Errorf("CaseA: kind = %s, want murder", c.Kind)
	}
	if c.Perpetrator.Type != crimes.PerpPlayer || c.Perpetrator.Id != 17 {
		t.Errorf("CaseA: perpetrator = %+v, want {player, 17}", c.Perpetrator)
	}
	wantDelta := int(cfg.CrimeRepDeltaMurder) - int(cfg.CrimeRepDeltaAssault)
	wantRep := repAfterAssault + wantDelta
	if rep := factions.GetRep("thornwall_citizens", 17); rep != wantRep {
		t.Errorf("CaseA: rep = %d, want %d", rep, wantRep)
	}
}

// TestMobDeathFactionRep_CaseB_ExternalAssaultButLoneKill verifies that
// when the assault was externally witnessed but no witnesses are present
// at the murder, the existing identified perp is preserved with no
// additional rep delta.
func TestMobDeathFactionRep_CaseB_ExternalAssaultButLoneKill(t *testing.T) {
	setupFactionsForHookTest(t)
	cfg := configs.GetBalanceConfig()

	const roomId = 467
	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()
	_ = room // no witnesses at murder time

	victimSpec, victimInst := makeCitizenMob(100, 200, roomId, "victim")
	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{100: victimSpec},
		map[int]*mobs.Mob{200: victimInst},
	)
	defer cleanupMobs()

	// Pre-seed assault with HadExternalWitness=true (was externally seen).
	assaultVictim := &mobs.Mob{MobId: 100}
	crimes.Record([]string{"thornwall_citizens"}, crimes.KindAssault,
		crimes.Perpetrator{Type: crimes.PerpPlayer, Id: 17},
		assaultVictim, 200, roomId, "Thornwall City", true)
	factions.BumpRep("thornwall_citizens", 17, int(cfg.CrimeRepDeltaAssault))
	repAfterAssault := factions.GetRep("thornwall_citizens", 17)

	evt := events.MobDeath{
		MobId: 100, InstanceId: 200, RoomId: roomId,
		PlayerDamage: map[int]int{17: 50},
	}
	MobDeathFactionRep(evt)

	got := crimes.AllForFaction("thornwall_citizens", true)
	if len(got) != 1 {
		t.Fatalf("CaseB: expected 1 crime, got %d", len(got))
	}
	c := got[0]
	if c.Kind != crimes.KindMurder {
		t.Errorf("CaseB: kind = %s, want murder", c.Kind)
	}
	// Perp must remain the player from the assault record.
	if c.Perpetrator.Type != crimes.PerpPlayer || c.Perpetrator.Id != 17 {
		t.Errorf("CaseB: perpetrator = %+v, want {player, 17}", c.Perpetrator)
	}
	// No additional rep delta — rep stays at assault level.
	if rep := factions.GetRep("thornwall_citizens", 17); rep != repAfterAssault {
		t.Errorf("CaseB: rep = %d, want %d (no increment)", rep, repAfterAssault)
	}
}

// TestMobDeathFactionRep_CaseC_LoneAssaultLoneMurder verifies the bug fix:
// lone assault (victim self-witness only) → lone murder → perp becomes
// unknown and the assault rep delta is refunded.
func TestMobDeathFactionRep_CaseC_LoneAssaultLoneMurder(t *testing.T) {
	setupFactionsForHookTest(t)
	cfg := configs.GetBalanceConfig()

	const roomId = 467
	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()
	_ = room // no witnesses at murder time

	victimSpec, victimInst := makeCitizenMob(100, 200, roomId, "victim")
	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{100: victimSpec},
		map[int]*mobs.Mob{200: victimInst},
	)
	defer cleanupMobs()

	// Pre-seed assault with HadExternalWitness=false (victim self-witness only).
	assaultVictim := &mobs.Mob{MobId: 100}
	crimes.Record([]string{"thornwall_citizens"}, crimes.KindAssault,
		crimes.Perpetrator{Type: crimes.PerpPlayer, Id: 17},
		assaultVictim, 200, roomId, "Thornwall City", false)
	factions.BumpRep("thornwall_citizens", 17, int(cfg.CrimeRepDeltaAssault))
	// Rep after lone assault: should be deltaAssault (negative).
	repAfterAssault := factions.GetRep("thornwall_citizens", 17)

	evt := events.MobDeath{
		MobId: 100, InstanceId: 200, RoomId: roomId,
		PlayerDamage: map[int]int{17: 50},
	}
	MobDeathFactionRep(evt)

	got := crimes.AllForFaction("thornwall_citizens", true)
	if len(got) != 1 {
		t.Fatalf("CaseC: expected 1 crime (upgraded), got %d", len(got))
	}
	c := got[0]
	if c.Kind != crimes.KindMurder {
		t.Errorf("CaseC: kind = %s, want murder", c.Kind)
	}
	// Perp must now be unknown — victim is dead, no survivor to identify killer.
	if c.Perpetrator.Type != crimes.PerpUnknown {
		t.Errorf("CaseC: perpetrator = %+v, want {unknown}", c.Perpetrator)
	}
	// Rep must be refunded back to 0 (assault delta cancelled out).
	wantRep := repAfterAssault - int(cfg.CrimeRepDeltaAssault) // refund = negate delta
	if rep := factions.GetRep("thornwall_citizens", 17); rep != wantRep {
		t.Errorf("CaseC: rep = %d, want %d (refunded to zero)", rep, wantRep)
	}
	if wantRep != 0 {
		t.Errorf("CaseC: expected wantRep=0 for clean-start test, got %d", wantRep)
	}
}

// configsAlive avoids unused-import warning if configs isn't
// referenced in an active test body.
var _ = configs.GetBalanceConfig

// #431: an assault by an unknown perpetrator upgrades in place on the kill,
// like an identified one, instead of the kill writing a second murder row.
// Nobody identified the assault, so it charged no rep and nothing is
// refunded; the murder row stays unknown when the kill is unseen too.
func TestMobDeathFactionRep_UnknownAssaultUpgradesInPlace_LoneKill(t *testing.T) {
	setupFactionsForHookTest(t)
	const roomId = 467
	_, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()

	victimSpec, victimInst := makeCitizenMob(100, 200, roomId, "victim")
	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{100: victimSpec},
		map[int]*mobs.Mob{200: victimInst},
	)
	defer cleanupMobs()

	crimes.Record([]string{"thornwall_citizens"}, crimes.KindAssault,
		crimes.Perpetrator{Type: crimes.PerpUnknown},
		&mobs.Mob{MobId: 100}, 200, roomId, "Thornwall City", false)

	MobDeathFactionRep(events.MobDeath{
		MobId: 100, InstanceId: 200, RoomId: roomId,
		PlayerDamage: map[int]int{17: 50},
	})

	got := crimes.AllForFaction("thornwall_citizens", true)
	if len(got) != 1 {
		t.Fatalf("expected 1 crime (upgraded in place), got %d: %+v", len(got), got)
	}
	if got[0].Kind != crimes.KindMurder || got[0].Perpetrator.Type != crimes.PerpUnknown {
		t.Errorf("row = %+v, want an unknown-perpetrator murder", got[0])
	}
	if got[0].Perpetrator.Id != 0 {
		t.Errorf("unknown row must not carry an id: %+v", got[0].Perpetrator)
	}
	if rep := factions.GetRep("thornwall_citizens", 17); rep != 0 {
		t.Errorf("rep = %d, want 0 (no assault charge, nothing to refund)", rep)
	}
}

// An unknown assault followed by a witnessed kill: one row, now identified,
// and the killer pays the full murder delta because the assault charged
// nothing.
func TestMobDeathFactionRep_UnknownAssaultUpgradesInPlace_WitnessedKill(t *testing.T) {
	setupFactionsForHookTest(t)
	const roomId = 467
	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()

	witnessSpec, witnessInst := makeCitizenMob(101, 201, roomId, "witness")
	victimSpec, victimInst := makeCitizenMob(100, 200, roomId, "victim")
	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{100: victimSpec, 101: witnessSpec},
		map[int]*mobs.Mob{200: victimInst, 201: witnessInst},
	)
	defer cleanupMobs()
	room.AddMob(201)

	crimes.Record([]string{"thornwall_citizens"}, crimes.KindAssault,
		crimes.Perpetrator{Type: crimes.PerpUnknown},
		&mobs.Mob{MobId: 100}, 200, roomId, "Thornwall City", false)

	MobDeathFactionRep(events.MobDeath{
		MobId: 100, InstanceId: 200, RoomId: roomId,
		PlayerDamage: map[int]int{17: 50},
	})

	got := crimes.AllForFaction("thornwall_citizens", true)
	if len(got) != 1 {
		t.Fatalf("expected 1 crime (upgraded in place), got %d: %+v", len(got), got)
	}
	if got[0].Kind != crimes.KindMurder ||
		got[0].Perpetrator.Type != crimes.PerpPlayer || got[0].Perpetrator.Id != 17 {
		t.Errorf("row = %+v, want murder by player 17", got[0])
	}
	want := int(configs.GetBalanceConfig().CrimeRepDeltaMurder)
	if rep := factions.GetRep("thornwall_citizens", 17); rep != want {
		t.Errorf("rep = %d, want the full murder delta %d", rep, want)
	}
}

// Two unknown assaults on two different victims stay separate: killing one
// victim upgrades only that victim's row.
func TestMobDeathFactionRep_UnknownAssaultsOnTwoVictimsStaySeparate(t *testing.T) {
	setupFactionsForHookTest(t)
	const roomId = 467
	_, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()

	victimSpec, victimInst := makeCitizenMob(100, 200, roomId, "victim")
	otherSpec, otherInst := makeCitizenMob(102, 202, roomId, "other")
	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{100: victimSpec, 102: otherSpec},
		map[int]*mobs.Mob{200: victimInst, 202: otherInst},
	)
	defer cleanupMobs()

	unknown := crimes.Perpetrator{Type: crimes.PerpUnknown}
	crimes.Record([]string{"thornwall_citizens"}, crimes.KindAssault, unknown,
		&mobs.Mob{MobId: 100}, 200, roomId, "Thornwall City", false)
	// The other victim's assault is the most recent row.
	crimes.Record([]string{"thornwall_citizens"}, crimes.KindAssault, unknown,
		&mobs.Mob{MobId: 102}, 202, roomId, "Thornwall City", false)

	MobDeathFactionRep(events.MobDeath{
		MobId: 100, InstanceId: 200, RoomId: roomId,
		PlayerDamage: map[int]int{17: 50},
	})

	got := crimes.AllForFaction("thornwall_citizens", true)
	if len(got) != 2 {
		t.Fatalf("expected 2 crimes, got %d: %+v", len(got), got)
	}
	for _, c := range got {
		switch c.VictimInstanceId {
		case 200:
			if c.Kind != crimes.KindMurder {
				t.Errorf("killed victim's row = %+v, want murder", c)
			}
		case 202:
			if c.Kind != crimes.KindAssault {
				t.Errorf("other victim's row = %+v, want it left as assault", c)
			}
		default:
			t.Errorf("unexpected row %+v", c)
		}
	}
}

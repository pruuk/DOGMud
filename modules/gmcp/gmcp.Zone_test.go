package gmcp

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// zoneDiffFixture seeds a lit zone of three rooms: the hall 9760 with the
// gallery 9761 to its north and the pantry 9763 to its east. The returned
// user stands in the hall, has already mapped the pantry, has never been in
// the gallery, and has a GMCP-negotiated connection.
func zoneDiffFixture(t *testing.T) *users.UserRecord {
	t.Helper()
	mudlog.SetupLogger(nil, ``, ``, false) // the mapper logs when it builds a zone
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default"},
	}))
	zero := 0.0
	lit := func(id int, title string, exits map[string]exit.RoomExit) *rooms.Room {
		return &rooms.Room{RoomId: id, Zone: "DiffZone", Biome: "default", SkyLight: &zero,
			Lamp: rooms.LampPtr(60), Title: title, Exits: exits}
	}
	hall := lit(9760, "Hall", map[string]exit.RoomExit{"north": {RoomId: 9761}, "east": {RoomId: 9763}})
	gallery := lit(9761, "Gallery", map[string]exit.RoomExit{"south": {RoomId: 9760}})
	pantry := lit(9763, "Pantry", map[string]exit.RoomExit{"west": {RoomId: 9760}})
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9760: hall, 9761: gallery, 9763: pantry},
		map[string]*rooms.ZoneConfig{"DiffZone": {Name: "DiffZone", RoomId: 9760,
			RoomIds: map[int]struct{}{9760: {}, 9761: {}, 9763: {}}}},
	))
	require.NotNil(t, mapper.GetMapper(9760, true), "the fixture zone must map")

	u := users.NewTestUser(9764, "mapper", "Mapper", 97764)
	u.Character.RoomId = 9760
	u.Character.MarkRoomVisited("DiffZone", 9760)
	u.Character.MarkRoomVisited("DiffZone", 9763)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9764: u}))
	gmcpModule.cache.Add(97764, GMCPSettings{GMCPAccepted: true, HelloReceived: true})
	t.Cleanup(func() { gmcpModule.cache.Remove(97764) })
	hall.AddPlayer(9764)
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`GMCPOut`) })
	return u
}

// drainZoneMaps returns the Zone.Map payloads queued for userId.
func drainZoneMaps(t *testing.T, userId int) []GMCPZoneModule_Payload {
	t.Helper()
	var out []GMCPZoneModule_Payload
	for _, e := range events.DrainQueuedByTypeForTest(`GMCPOut`) {
		o := e.(GMCPOut)
		if o.UserId != userId || o.Module != `Zone.Map` {
			continue
		}
		p, ok := o.Payload.(GMCPZoneModule_Payload)
		require.True(t, ok, "payload type %T", o.Payload)
		out = append(out, p)
	}
	return out
}

func zoneMapRoomIds(p GMCPZoneModule_Payload) []int {
	ids := []int{}
	for _, r := range p.Rooms {
		ids = append(ids, r.RoomId)
	}
	return ids
}

// #650: a fully explored character was sent about 271 KB of Zone.Map on
// every step. The map now goes out whole once, then only as what changed.
func TestZoneMap_FullOnceThenNothingForARepeat(t *testing.T) {
	u := zoneDiffFixture(t)
	g := &GMCPZoneModule{zoneSent: map[int]*zoneSentState{}}

	g.buildAndSend(GMCPZoneUpdate{UserId: u.UserId})
	maps := drainZoneMaps(t, u.UserId)
	require.Len(t, maps, 1)
	require.True(t, maps[0].Full)
	require.ElementsMatch(t, []int{9760, 9763}, zoneMapRoomIds(maps[0]))
	require.NotNil(t, maps[0].Party, "party is always a list")

	g.buildAndSend(GMCPZoneUpdate{UserId: u.UserId})
	require.Empty(t, drainZoneMaps(t, u.UserId), "nothing changed, so nothing is sent")
}

// Stepping into the gallery sends the gallery, plus the hall, whose north exit
// stops being a fog stub and becomes a full edge now both ends are mapped. The
// pantry is unchanged and stays off the wire.
func TestZoneMap_NewRoomIsAPartialSend(t *testing.T) {
	u := zoneDiffFixture(t)
	g := &GMCPZoneModule{zoneSent: map[int]*zoneSentState{}}
	g.buildAndSend(GMCPZoneUpdate{UserId: u.UserId})
	drainZoneMaps(t, u.UserId)

	u.Character.MarkRoomVisited("DiffZone", 9760)
	u.Character.RoomId = 9761
	g.buildAndSend(GMCPZoneUpdate{UserId: u.UserId})
	maps := drainZoneMaps(t, u.UserId)
	require.Len(t, maps, 1)
	require.False(t, maps[0].Full)
	require.Equal(t, "DiffZone", maps[0].Zone)
	require.ElementsMatch(t, []int{9760, 9761}, zoneMapRoomIds(maps[0]))
}

func TestZoneMap_DespawnForgetsWhatWasSent(t *testing.T) {
	u := zoneDiffFixture(t)
	g := &GMCPZoneModule{zoneSent: map[int]*zoneSentState{}}
	g.buildAndSend(GMCPZoneUpdate{UserId: u.UserId})
	drainZoneMaps(t, u.UserId)

	g.zoneDespawnHandler(events.PlayerDespawn{UserId: u.UserId})
	g.buildAndSend(GMCPZoneUpdate{UserId: u.UserId})
	maps := drainZoneMaps(t, u.UserId)
	require.Len(t, maps, 1)
	require.True(t, maps[0].Full, "after a logout the next map is whole")
	require.ElementsMatch(t, []int{9760, 9763}, zoneMapRoomIds(maps[0]))
}

type fakeZoneWorld struct {
	cfgs     map[string]*rooms.ZoneConfig
	blockers map[string][]rooms.ZoneBlocker
	deleted  []string
	renamed  []string
	saved    []rooms.ZoneConfig
}

func newFakeZoneWorld() *fakeZoneWorld {
	return &fakeZoneWorld{
		// RoomIds mirrors the live engine, which populates this per-zone lookup
		// as rooms load. buildZoneList reads RoomCount from it rather than
		// rescanning every room in the world.
		cfgs: map[string]*rooms.ZoneConfig{
			"Testzone": {Name: "Testzone", RoomId: 100, RoomIds: map[int]struct{}{100: {}}},
		},
		blockers: map[string][]rooms.ZoneBlocker{},
	}
}

func (w *fakeZoneWorld) deps() zoneDeps {
	return zoneDeps{
		load: func(z string) *rooms.ZoneConfig { return w.cfgs[z] },
		save: func(c rooms.ZoneConfig) error {
			w.saved = append(w.saved, c)
			cp := c
			w.cfgs[c.Name] = &cp
			return nil
		},
		del: func(z string) error {
			w.deleted = append(w.deleted, z)
			delete(w.cfgs, z)
			return nil
		},
		blockers:  func(z string) []rooms.ZoneBlocker { return w.blockers[z] },
		zoneNames: func() []string { return []string{"Testzone"} },
		roomIds:   func(z string) []int { return []int{100} },
		rename: func(oldName, newName string) error {
			w.renamed = append(w.renamed, oldName+"->"+newName)
			return nil
		},
		renameBlockers: func(z string) []rooms.ZoneBlocker { return nil },
	}
}

func TestBuildZoneDelete_BlocksWhenNotEmpty(t *testing.T) {
	w := newFakeZoneWorld()
	w.blockers["Testzone"] = []rooms.ZoneBlocker{
		{Kind: "room", Id: "room 101"},
		{Kind: "inbound-exit", Id: "room 900 (Other) east"},
	}

	res := buildZoneDelete(w.deps(), "Testzone")

	if res.Ok {
		t.Fatal("delete must be refused when the zone is not empty")
	}
	if len(res.ZoneRefs) != 2 {
		t.Errorf("expected 2 blockers surfaced, got %d", len(res.ZoneRefs))
	}
	if len(w.deleted) != 0 {
		t.Error("d.del must not be called when blocked")
	}
}

func TestBuildZoneDelete_DeletesCleanZone(t *testing.T) {
	w := newFakeZoneWorld()
	res := buildZoneDelete(w.deps(), "Testzone")
	if !res.Ok {
		t.Fatalf("clean zone should delete, got %+v", res)
	}
	if len(w.deleted) != 1 || w.deleted[0] != "Testzone" {
		t.Errorf("expected Testzone deleted, got %v", w.deleted)
	}
}

func TestBuildZoneDelete_UnknownZone(t *testing.T) {
	w := newFakeZoneWorld()
	if res := buildZoneDelete(w.deps(), "Nowhere"); res.Ok {
		t.Error("unknown zone must not report success")
	}
}

func TestBuildZoneList_ReportsRoomCounts(t *testing.T) {
	w := newFakeZoneWorld()
	rowsAny := buildZoneList(w.deps())
	if len(rowsAny.Zones) != 1 {
		t.Fatalf("expected 1 zone, got %d", len(rowsAny.Zones))
	}
	row := rowsAny.Zones[0]
	if row.Zone != "Testzone" || row.RoomCount != 1 {
		t.Errorf("unexpected row: %+v", row)
	}
}

func TestBuildZoneGet_MapsFieldsAndEnums(t *testing.T) {
	w := newFakeZoneWorld()
	w.cfgs["Testzone"] = &rooms.ZoneConfig{
		Name: "Testzone", RoomId: 100, DefaultBiome: "land",
		Region: "Windward Marches", Instanced: true, DeathPolicy: "ejected",
		NonCartesian: true, DefaultPlane: 7,
	}
	d, ok := buildZoneGet(w.deps(), "Testzone")
	if !ok {
		t.Fatal("expected zone detail")
	}
	if d.Name != "Testzone" || d.DefaultBiome != "land" || !d.Instanced ||
		d.DeathPolicy != "ejected" || !d.NonCartesian || d.DefaultPlane != 7 {
		t.Errorf("fields not mapped: %+v", d)
	}
	if len(d.Enums.DeathPolicies) == 0 {
		t.Error("death policy enum must be server-supplied")
	}
}

func TestBuildZoneGet_UnknownZone(t *testing.T) {
	w := newFakeZoneWorld()
	if _, ok := buildZoneGet(w.deps(), "Nowhere"); ok {
		t.Error("unknown zone must not return detail")
	}
}

func baseZoneReq() zoneUpdateReq {
	return zoneUpdateReq{
		Name: "Testzone", RoomId: 100, DefaultBiome: "land",
		DeathPolicy: "rejoin", DefaultPlane: 7,
	}
}

// A non_cartesian zone on plane 0 marks plane 0 (the overworld) non-Euclidean
// via the sticky OR in PlaneRegistry.Mark, disabling collision and reciprocity
// enforcement for the ENTIRE world. Refuse to save that combination.
func TestBuildZoneUpdate_RejectsNonCartesianOnOverworldPlane(t *testing.T) {
	w := newFakeZoneWorld()
	req := baseZoneReq()
	req.NonCartesian = true
	req.DefaultPlane = 0

	if res := buildZoneUpdate(w.deps(), req); res.Ok {
		t.Error("non_cartesian with plane 0 must be rejected")
	}
	if len(w.saved) != 0 {
		t.Errorf("nothing may be saved on validation failure, got %d", len(w.saved))
	}
}

func TestBuildZoneUpdate_AllowsNonCartesianOnOwnPlane(t *testing.T) {
	w := newFakeZoneWorld()
	req := baseZoneReq()
	req.NonCartesian = true
	req.DefaultPlane = 7

	if res := buildZoneUpdate(w.deps(), req); !res.Ok {
		t.Errorf("non_cartesian on its own plane should save: %+v", res)
	}
}

func TestBuildZoneUpdate_RejectsUnknownDeathPolicy(t *testing.T) {
	w := newFakeZoneWorld()
	req := baseZoneReq()
	req.DeathPolicy = "vaporize"
	if res := buildZoneUpdate(w.deps(), req); res.Ok {
		t.Error("unknown death policy must be rejected")
	}
}

func TestBuildZoneUpdate_RoundTripsFields(t *testing.T) {
	w := newFakeZoneWorld()
	req := baseZoneReq()
	req.Region = "Windward Marches"
	req.MusicFile = "theme.mp3"
	req.Instanced = true
	req.EntryRoom = 100

	if res := buildZoneUpdate(w.deps(), req); !res.Ok {
		t.Fatalf("update should succeed: %+v", res)
	}
	got := w.saved[0]
	if got.Region != "Windward Marches" || got.MusicFile != "theme.mp3" ||
		!got.Instanced || got.EntryRoom != 100 {
		t.Errorf("fields not round-tripped: %+v", got)
	}
}

func TestBuildZoneUpdate_RejectsEntryRoomOutsideZone(t *testing.T) {
	w := newFakeZoneWorld() // roomIds returns only [100]
	req := baseZoneReq()
	req.Instanced = true
	req.EntryRoom = 999
	if res := buildZoneUpdate(w.deps(), req); res.Ok {
		t.Error("entry room outside the zone must be rejected")
	}
}

// buildZoneList used to derive RoomCount from the roomIds dep, whose real
// wiring rescans EVERY room in the world with a disk read per room — once per
// zone. Measured on the live world: 49 zones x 1384 rooms = 67,816
// LoadRoomTemplate calls, ~10s, for a list the author expects instantly.
//
// ZoneConfig.RoomIds is the engine's own per-zone room lookup, kept current as
// rooms load and only pruned by a genuine delete (idle-room unloading does not
// touch it). The count must come from there, and the expensive scan must not
// run at all.
func TestBuildZoneList_DoesNotRescanEveryRoom(t *testing.T) {
	w := newFakeZoneWorld()
	w.cfgs["Testzone"] = &rooms.ZoneConfig{
		Name:   "Testzone",
		RoomId: 100,
		RoomIds: map[int]struct{}{
			100: {}, 101: {}, 102: {}, 103: {},
		},
	}

	d := w.deps()
	roomIdsCalls := 0
	d.roomIds = func(z string) []int {
		roomIdsCalls++
		return []int{100} // deliberately wrong count; must not be used
	}

	got := buildZoneList(d)

	if len(got.Zones) != 1 {
		t.Fatalf("expected 1 zone, got %d", len(got.Zones))
	}
	if got.Zones[0].RoomCount != 4 {
		t.Errorf("RoomCount = %d, want 4 (from ZoneConfig.RoomIds)", got.Zones[0].RoomCount)
	}
	if roomIdsCalls != 0 {
		t.Errorf("buildZoneList called the full-room-scan dep %d times; it must not call it at all", roomIdsCalls)
	}
}

func TestBuildZoneRename_RefusesWhenBlocked(t *testing.T) {
	w := newFakeZoneWorld()
	d := w.deps()
	d.renameBlockers = func(z string) []rooms.ZoneBlocker {
		return []rooms.ZoneBlocker{{Kind: "player", Id: "1 player(s) in room 100"}}
	}
	renameCalls := 0
	d.rename = func(oldName, newName string) error { renameCalls++; return nil }

	res := buildZoneRename(d, zoneRenameReq{Zone: "Testzone", NewName: "Quiet Water"})
	if res.Ok {
		t.Error("rename must be refused while a player is in the zone")
	}
	if len(res.ZoneRefs) != 1 {
		t.Errorf("expected the blocker surfaced, got %d", len(res.ZoneRefs))
	}
	if renameCalls != 0 {
		t.Error("d.rename must not be called when blocked")
	}
}

func TestBuildZoneRename_RenamesQuietZone(t *testing.T) {
	w := newFakeZoneWorld()
	d := w.deps()
	var gotOld, gotNew string
	d.rename = func(oldName, newName string) error { gotOld, gotNew = oldName, newName; return nil }

	res := buildZoneRename(d, zoneRenameReq{Zone: "Testzone", NewName: "Quiet Water"})
	if !res.Ok {
		t.Fatalf("quiet zone should rename, got %+v", res)
	}
	if gotOld != "Testzone" || gotNew != "Quiet Water" {
		t.Errorf("rename called with (%q,%q)", gotOld, gotNew)
	}
}

func TestBuildZoneRename_UnknownZone(t *testing.T) {
	w := newFakeZoneWorld()
	if res := buildZoneRename(w.deps(), zoneRenameReq{Zone: "Nowhere", NewName: "X Y"}); res.Ok {
		t.Error("unknown zone must not report success")
	}
}

func TestBuildZoneRename_SurfacesRenameError(t *testing.T) {
	w := newFakeZoneWorld()
	d := w.deps()
	d.rename = func(oldName, newName string) error { return fmt.Errorf("target path already exists") }
	res := buildZoneRename(d, zoneRenameReq{Zone: "Testzone", NewName: "Quiet Water"})
	if res.Ok {
		t.Error("a rename error must not report success")
	}
	if res.Error == "" {
		t.Error("expected the error surfaced to the author")
	}
}

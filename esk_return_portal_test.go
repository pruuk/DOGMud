package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	yaml "gopkg.in/yaml.v2"
)

// Room ids for the Pothole Coulee arch and its way back (owner ruling
// 2026-10-17). Warden Esk (mob 9107) keeps the arch in the Threshold House
// (5215); asking him for passage, or walking through the arch, moves an
// Opened player to the Thornwall Temple Interior (468). Before this ruling
// nothing led back, so a new player who crossed early was cut off from the
// coulee quests they still held.
const (
	eskRoomId        = 5215
	eskMobId         = 9107
	thornwallTemple  = 468
	templePortalExit = "portal"
)

// TestThornwallTemplePortalLeadsBackToEsk pins the return portal: the temple
// the arch delivers to carries a `portal` exit back to Esk's room, so the
// arch is no longer a one-way door out of the newbie area. It also reads
// both arch transits (Esk's behavior tree and the arch room's own script)
// and fails if either stops landing in the temple that holds the portal.
func TestThornwallTemplePortalLeadsBackToEsk(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()

	esk := rooms.LoadRoom(eskRoomId)
	if esk == nil {
		t.Fatalf("room %d did not load: the test is not seeing the world", eskRoomId)
	}
	spawnsEsk := false
	for _, s := range esk.SpawnInfo {
		if s.MobId == eskMobId {
			spawnsEsk = true
		}
	}
	if !spawnsEsk {
		t.Fatalf("room %d (%s) no longer spawns Warden Esk (mob %d)", eskRoomId, esk.Title, eskMobId)
	}

	temple := rooms.LoadRoom(thornwallTemple)
	if temple == nil {
		t.Fatalf("room %d did not load", thornwallTemple)
	}
	ex, ok := temple.Exits[templePortalExit]
	if !ok {
		t.Fatalf("room %d (%s) has no %q exit: a player who crossed the arch has no way back to Esk",
			thornwallTemple, temple.Title, templePortalExit)
	}
	if ex.RoomId != eskRoomId {
		t.Errorf("room %d %q exit leads to %d, want Esk's room %d", thornwallTemple, templePortalExit, ex.RoomId, eskRoomId)
	}
	if ex.Secret {
		t.Errorf("room %d %q exit is secret: a newcomer must be able to see the way back", thornwallTemple, templePortalExit)
	}
	if ex.HasLock() {
		t.Errorf("room %d %q exit is locked: the way back must stay open", thornwallTemple, templePortalExit)
	}
	if _, ok := temple.Nouns[templePortalExit]; !ok {
		t.Errorf("room %d has no %q noun: `look portal` should say where it goes", thornwallTemple, templePortalExit)
	}

	// The portal is a named exit with no map delta, so the mapper skips it.
	// The consistency check (which the boot runs in panic mode) must find no
	// error in either zone it joins.
	for _, zr := range []struct {
		roomId int
		zone   string
	}{
		{thornwallTemple, temple.Zone},
		{eskRoomId, esk.Zone},
	} {
		m := mapper.GetMapper(zr.roomId)
		if m == nil {
			t.Fatalf("no mapper for room %d", zr.roomId)
		}
		for _, f := range mapper.FilterFindingsToZone(m.CheckConsistency(zr.zone, rooms.IsZoneNonCartesian(zr.zone)), zr.zone) {
			if f.Severity == "error" {
				t.Errorf("map consistency: %s", f)
			}
		}
	}

	// Both arch transits must land in the temple that holds the portal.
	root := filepath.Join("_datafiles", "world", "dogmud", "behaviors")
	for _, rel := range []string{
		filepath.Join("pothole_coulee", "9107-warden_esk.yaml"),
		filepath.Join("rooms", "pothole_coulee", "5215.yaml"),
	} {
		dests := movePlayerDestinations(t, filepath.Join(root, rel))
		if len(dests) == 0 {
			t.Errorf("%s: no move_player action found; the arch transit moved and this test lost it", rel)
		}
		for _, d := range dests {
			if d != thornwallTemple {
				t.Errorf("%s: move_player lands in room %d, but the return portal is in %d", rel, d, thornwallTemple)
			}
		}
	}
}

// movePlayerDestinations returns the room_id of every move_player action in a
// behavior tree file.
func movePlayerDestinations(t *testing.T, path string) []int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []int
	var walk func(v interface{})
	walk = func(v interface{}) {
		switch n := v.(type) {
		case map[interface{}]interface{}:
			if n["do"] == "move_player" {
				if id, ok := n["room_id"].(int); ok {
					out = append(out, id)
				}
			}
			for _, c := range n {
				walk(c)
			}
		case []interface{}:
			for _, c := range n {
				walk(c)
			}
		}
	}
	walk(doc)
	return out
}

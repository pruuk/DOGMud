package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
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

// TestEskPassageClosesTwoRoads: quest 29 "Two Roads" closes when the player
// takes Esk's road. Its own room_enter trigger on 468 never fires for the
// arch, because a move_player teleport does not raise room_enter (only a
// walk, a login and a start do), so both arch transits grant 29-end
// themselves, under the same gate as that trigger: holds 29-start, lacks
// 29-end. A player who already chose Toke's road, or never held quest 29,
// crosses with no second "completed" banner.
func TestEskPassageClosesTwoRoads(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	quests.LoadDataFiles()
	if q := quests.GetQuest("29-start"); q == nil || q.Name != "Two Roads" {
		t.Fatalf("quest 29 is %+v, want the shipped Two Roads", q)
	}

	behaviors := filepath.Join("_datafiles", "world", "dogmud", "behaviors")
	transits := []struct {
		name  string
		file  string
		event behaviortree.EventContext
	}{
		{"ask esk passage", filepath.Join(behaviors, "pothole_coulee", "9107-warden_esk.yaml"),
			behaviortree.EventContext{EventType: "player_ask", Text: "passage"}},
		{"enter arch", filepath.Join(behaviors, "rooms", "pothole_coulee", "5215.yaml"),
			behaviortree.EventContext{EventType: "room_command", Command: "enter", Rest: "arch"}},
	}
	players := []struct {
		name      string
		progress  map[int]string
		wantStep  string // quest 29 progress after the crossing ("" = none)
		wantGrant bool   // a 29-end quest event reaches the reward pipeline
	}{
		{"on the Two Roads step", map[int]string{30: "end", 29: "start"}, "end", true},
		{"already chose Toke's road", map[int]string{30: "end", 29: "end"}, "end", false},
		{"never held quest 29", map[int]string{30: "end"}, "", false},
	}

	for _, tr := range transits {
		tree, err := behaviortree.LoadTreeFromFile(tr.file)
		if err != nil {
			t.Fatalf("load %s: %v", tr.file, err)
		}
		for _, p := range players {
			t.Run(tr.name+"/"+p.name, func(t *testing.T) {
				t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
					eskRoomId:       {RoomId: eskRoomId, Zone: "Pothole Coulee", Title: "The Threshold House"},
					thornwallTemple: {RoomId: thornwallTemple, Zone: "Thornwall City", Title: "Temple Interior"},
				}, map[string]*rooms.ZoneConfig{}))
				u := users.NewTestUser(1, "esktest", "Esktest", 1)
				u.Character.RoomId = eskRoomId
				u.Character.QuestProgress = map[int]string{}
				for id, step := range p.progress {
					u.Character.QuestProgress[id] = step
				}
				t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: u}))

				events.ProcessEvents() // discard leftovers from earlier tests
				var granted []string
				lid := events.RegisterListener(events.Quest{}, func(e events.Event) events.ListenerReturn {
					if q, ok := e.(events.Quest); ok && q.UserId == 1 {
						granted = append(granted, q.QuestToken)
					}
					return events.Continue
				})
				t.Cleanup(func() { events.UnregisterListener(events.Quest{}, lid) })

				ev := tr.event
				ev.UserId, ev.RoomId = 1, eskRoomId
				if res := tree.Evaluate(&behaviortree.EvalContext{
					Event: ev, MobState: behaviortree.NewBehaviorState(), RoomId: eskRoomId,
				}); res != behaviortree.Success {
					t.Fatalf("the Opened player's crossing returned %v, want Success (a failure falls through to dialogue)", res)
				}
				behaviortree.DrainAllDelayedActionsForTest(t)
				// move_player queues a `look`; this binary has no command
				// world to run it, so drop it before the queue is processed.
				events.DrainQueuedUserInputsForTest(1)
				events.ProcessEvents()

				if u.Character.RoomId != thornwallTemple {
					t.Errorf("player is in room %d after the crossing, want %d", u.Character.RoomId, thornwallTemple)
				}
				if got := u.Character.QuestProgress[29]; got != p.wantStep {
					t.Errorf("quest 29 progress after the crossing is %q, want %q", got, p.wantStep)
				}
				n := 0
				for _, tok := range granted {
					if tok == "29-end" {
						n++
					}
				}
				if p.wantGrant && n != 1 {
					t.Errorf("29-end quest events queued: %d, want exactly 1 (granted: %v)", n, granted)
				}
				if !p.wantGrant && n != 0 {
					t.Errorf("29-end quest events queued: %d, want 0: the completion banner would repeat (granted: %v)", n, granted)
				}
			})
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

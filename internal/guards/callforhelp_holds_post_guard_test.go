package guards

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/hooks"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The real mobs and rooms of the smugglers' tunnels, from shipped data. Torvan
// Cresk's table is one exit south of the Tunnel Lookout's anteroom, and the
// Smuggler Enforcer's tunnel is one exit north of it.
const (
	tunnelEnforcerRoom = 496
	tunnelLookoutRoom  = 497
	tunnelTorvanRoom   = 498

	tunnelRunnerMob   = 245 // maxwander 1: free to answer a warning
	tunnelEnforcerMob = 246 // maxwander 0, and the lookout's routine
	tunnelLookoutMob  = 247
	tunnelTorvanMob   = 249 // maxwander 0, no routine
)

type tunnelScene struct {
	lookout, torvan, enforcer, runner *mobs.Mob
	player                            *users.UserRecord
}

// tunnelMob spawns a shipped mob into a shipped room and returns it.
func tunnelMob(t *testing.T, mobId mobs.MobId, roomId int) *mobs.Mob {
	t.Helper()
	m := mobs.NewMobByIdFresh(mobId, roomId)
	if m == nil {
		t.Fatalf("mob %d did not spawn", mobId)
	}
	rooms.LoadRoom(roomId).AddMob(m.InstanceId)
	return m
}

// tunnelWorld loads the shipped world and config and spawns the tunnel crew,
// with a player standing in Torvan's room. It asserts the fields this guard
// reads, so a content change fails here by name.
func tunnelWorld(t *testing.T) *tunnelScene {
	t.Helper()
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	gametime.ClearDateCacheForTest()
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(species.SeedSpeciesForTest(nil))
	t.Cleanup(keywords.SeedKeywordsForTest())
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	species.LoadDataFiles()
	mutators.LoadDataFiles()
	mobs.LoadPatrols()
	mobs.LoadSchedules()

	dataPath := configs.GetFilePathsConfig().DataFiles.String() + `/mobs`
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataPath)
	if err != nil {
		t.Fatalf("loading mob templates from %s: %v", dataPath, err)
	}
	t.Cleanup(mobs.SeedMobsForTest(templates, map[int]*mobs.Mob{}))

	originalRound := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCount(originalRound) })

	s := &tunnelScene{
		lookout:  tunnelMob(t, tunnelLookoutMob, tunnelLookoutRoom),
		torvan:   tunnelMob(t, tunnelTorvanMob, tunnelTorvanRoom),
		enforcer: tunnelMob(t, tunnelEnforcerMob, tunnelEnforcerRoom),
		runner:   tunnelMob(t, tunnelRunnerMob, tunnelEnforcerRoom),
	}
	if s.torvan.MaxWander != 0 || s.enforcer.MaxWander != 0 || s.runner.MaxWander == 0 {
		t.Fatalf("fixture: maxwander torvan=%d enforcer=%d runner=%d, want 0, 0 and nonzero",
			s.torvan.MaxWander, s.enforcer.MaxWander, s.runner.MaxWander)
	}
	if s.enforcer.Routine == "" || s.enforcer.Routine != s.lookout.Routine {
		t.Fatalf("fixture: enforcer routine %q, lookout routine %q: the routine path is not under test",
			s.enforcer.Routine, s.lookout.Routine)
	}
	for _, helper := range []*mobs.Mob{s.torvan, s.enforcer, s.runner} {
		if !helper.ConsidersAnAlly(s.lookout) {
			t.Fatalf("fixture: %s is no ally of the lookout", helper.Character.Name)
		}
	}

	s.player = users.NewTestUser(9301, "tunnelplayer", "Tunnel Player", 0)
	s.player.Character.RoomId = tunnelTorvanRoom
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{s.player.UserId: s.player}))
	rooms.LoadRoom(tunnelTorvanRoom).AddPlayer(s.player.UserId)

	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	return s
}

// moveCommands keeps the queued commands that would take a mob out of its room.
func moveCommands(cmds []string) []string {
	var moves []string
	for _, c := range cmds {
		verb := strings.ToLower(strings.Fields(c + " x")[0])
		switch verb {
		case "go", "pathto", "wander", "north", "south", "east", "west":
			moves = append(moves, c)
		}
	}
	return moves
}

// The live defect (#375 phase 7 tier 1 playtest): Torvan Cresk was dragged out
// of his room several times a minute by a player merely standing beside him.
// The Tunnel Lookout next door scans on every idle round, sees that player, and
// calls for help with no fight on. The call moved every ally in range, Torvan
// included, away from the player it had just spotted, and his home recovery
// walked him back into the next call. A mob with maxwander 0 holds its post
// against a warning like that; a mob free to wander still musters to it.
func TestACallWithNoFightLeavesAMaxWanderZeroMobAtItsPost(t *testing.T) {
	s := tunnelWorld(t)

	// The trigger is live: the real lookout's real idle tick sees the player
	// next door and asks for help, or the rest of this test proves nothing.
	hooks.HandleIdleMobs(events.MobIdle{MobInstanceId: s.lookout.InstanceId})
	if got := events.DrainQueuedInputsForTest(s.lookout.InstanceId); len(got) == 0 || got[0] != "callforhelp" {
		t.Fatalf("the lookout's idle tick queued %q, want callforhelp", got)
	}

	// The room is calm, so this call is a warning.
	lookoutRoom := rooms.LoadRoom(tunnelLookoutRoom)
	if !lookoutRoom.IsCalm() {
		t.Fatal("fixture: the lookout's room should be calm")
	}
	if _, err := mobcommands.CallForHelp("", s.lookout, lookoutRoom); err != nil {
		t.Fatalf("callforhelp: %v", err)
	}

	if got := moveCommands(events.DrainQueuedInputsForTest(s.torvan.InstanceId)); len(got) != 0 {
		t.Errorf("Torvan (maxwander 0) was told to leave his table on a warning: %q", got)
	}
	if got := moveCommands(events.DrainQueuedInputsForTest(s.enforcer.InstanceId)); len(got) != 0 {
		t.Errorf("the Enforcer (maxwander 0, routine match) was told to leave its post on a warning: %q", got)
	}
	// The warning still reaches a mob free to wander. Without this the two
	// checks above would pass if the call moved nobody at all.
	if got := moveCommands(events.DrainQueuedInputsForTest(s.runner.InstanceId)); len(got) == 0 {
		t.Error("the Runner (maxwander 1) did not answer the warning")
	}
}

// A real fight still brings everyone: a mob with maxwander 0 answers a caller
// who is fighting, and its home recovery brings it back afterwards.
func TestACallInAFightStillBringsAMaxWanderZeroMob(t *testing.T) {
	s := tunnelWorld(t)

	// The player has walked next door and is fighting the lookout.
	rooms.LoadRoom(tunnelTorvanRoom).RemovePlayer(s.player.UserId)
	s.player.Character.RoomId = tunnelLookoutRoom
	lookoutRoom := rooms.LoadRoom(tunnelLookoutRoom)
	lookoutRoom.AddPlayer(s.player.UserId)
	s.lookout.Character.SetAggro(s.player.UserId, 0, characters.DefaultAttack)
	if lookoutRoom.IsCalm() {
		t.Fatal("fixture: the lookout's room should not be calm")
	}

	if _, err := mobcommands.CallForHelp("", s.lookout, lookoutRoom); err != nil {
		t.Fatalf("callforhelp: %v", err)
	}

	for _, helper := range []*mobs.Mob{s.torvan, s.enforcer, s.runner} {
		if got := moveCommands(events.DrainQueuedInputsForTest(helper.InstanceId)); len(got) == 0 {
			t.Errorf("%s did not answer a call from a real fight", helper.Character.Name)
		}
	}
}

// Torvan on his own never walks out: his archetype, goals, idle commands and
// home logic issue no move, with a player beside him and no lookout's call.
// The live trace put every move on the lookout's call; this holds that.
func TestTorvanAloneNeverLeavesOnHisOwnIdle(t *testing.T) {
	s := tunnelWorld(t)
	round := util.GetRoundCount()
	for i := 0; i < 500; i++ {
		util.SetRoundCount(round + uint64(i) + 1)
		hooks.HandleIdleMobs(events.MobIdle{MobInstanceId: s.torvan.InstanceId})
		if got := moveCommands(events.DrainQueuedInputsForTest(s.torvan.InstanceId)); len(got) != 0 {
			t.Fatalf("idle tick %d: Torvan issued %q", i, got)
		}
	}
	if s.torvan.Character.RoomId != tunnelTorvanRoom {
		t.Fatalf("Torvan is in room %d, want %d", s.torvan.Character.RoomId, tunnelTorvanRoom)
	}
}

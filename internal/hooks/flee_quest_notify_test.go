package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The 2026-10-17 pre-deploy playtest: the tutorial's flee step listened on
// command_issued, so it finished (and Dewey spoke) the moment `flee` was
// typed, before "You flee to the north exit!" and even when the flee was
// later refused. The step now listens on the `flee` command event, which the
// round hook sends only once the flee has worked and the player has arrived.

const fleeProbeToken = "999301-start"

func listenForFlee(t *testing.T, u *users.UserRecord) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: 999301,
		Name:    "flee success probe",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:   "command",
			Command: "flee",
			Actions: []questengine.ActionDef{{Grant: fleeProbeToken}},
		}},
	})
	u.Character.ClearQuestToken(fleeProbeToken)
	t.Cleanup(func() { u.Character.ClearQuestToken(fleeProbeToken) })
}

func startFlee(t *testing.T, u *users.UserRecord, room *rooms.Room) {
	t.Helper()
	if err := u.Character.Validate(); err != nil {
		t.Fatalf("validate fleer: %v", err)
	}
	u.Character.Stamina = 0
	u.Character.SetAggro(0, 100, characters.DefaultAttack)
	u.Character.CombatPhase.OnRoundTick()
	if _, err := usercommands.Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}
	if !u.Character.IsDisengaging() {
		t.Fatalf("fixture did not enter Disengaging")
	}
}

func TestHandlePlayerFlee_QuestFleeEventOnlyOnArrival(t *testing.T) {
	t.Run("no exit sends nothing", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		u := users.GetByUserId(1)
		room := rooms.LoadRoom(1)
		room.Exits = nil
		listenForFlee(t, u)
		startFlee(t, u, room)
		if !handlePlayerFlee(u, room, u.UserId) {
			t.Fatal("no-exit failure did not handle flee")
		}
		if u.Character.HasQuest(fleeProbeToken) {
			t.Fatal("a flee that found no exit sent the flee command event")
		}
	})

	t.Run("a flee that works sends it after the move", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		// The arrival runs Look, which reads the keyword legend.
		t.Cleanup(keywords.SeedKeywordsForTest())
		u := users.GetByUserId(1)
		room := rooms.LoadRoom(1)
		listenForFlee(t, u)
		startFlee(t, u, room)
		if u.Character.HasQuest(fleeProbeToken) {
			t.Fatal("typing flee alone sent the flee command event")
		}
		if !handlePlayerFlee(u, room, u.UserId) {
			t.Fatal("flee was not resolved")
		}
		if u.Character.RoomId != 2 {
			t.Fatalf("fixture: fleer should have reached room 2, is in %d", u.Character.RoomId)
		}
		if !u.Character.HasQuest(fleeProbeToken) {
			t.Fatal("a flee that worked did not send the flee command event")
		}
	})
}

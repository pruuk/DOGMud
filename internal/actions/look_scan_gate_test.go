package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A room_gate guards a room as a locked door does: a step is refused, so a look
// or a scan through the exit does not show what is behind it. Quest 64's runaway
// hides behind one, and `look up` from the stair must not name her.

const lookGateQuestId = 999640

func registerLookGate(t *testing.T) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: lookGateQuestId,
		Name:    "Look Gate Quest",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:      "room_gate",
			Room:       scanLockThereId,
			Conditions: questengine.Conditions{Missing: []string{"999640-start"}},
			Actions:    []questengine.ActionDef{{SendText: "You have no reason to go there."}},
		}},
	})
}

// gatedPlayer is scanLockScene's scanner as a real player, holding the gate's
// token or not.
func gatedPlayer(t *testing.T, hereLamp int, infra, holdsToken bool) Actor {
	t.Helper()
	fake := scanLockScene(t, hereLamp, false, infra, true)
	u := &users.UserRecord{UserId: scanLockUserId, Character: fake.char}
	if holdsToken {
		u.Character.QuestProgress = map[int]string{lookGateQuestId: "start"}
	}
	return NewUserActorInRoom(u, fake.room)
}

func TestResolveLook_AGatedExitReadsAsLocked(t *testing.T) {
	registerLookGate(t)
	for _, c := range []struct {
		name       string
		lamp       int
		infra      bool
		holdsToken bool
		want       LookKind
		wantGated  bool
	}{
		{"light sees out, gate refuses", 90, false, false, LookExitLocked, true},
		{"heat reaches, gate refuses: no figure", 30, true, false, LookExitLocked, true},
		{"neither reaches: too dark, as a locked exit reads", 30, false, false, LookExitTooDark, false},
		{"light sees out, gate allows", 90, false, true, LookExit, false},
		{"heat reaches, gate allows", 30, true, true, LookExitShapes, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := ResolveLook(gatedPlayer(t, c.lamp, c.infra, c.holdsToken), "north")
			require.Equal(t, c.want, res.Kind)
			require.Equal(t, c.wantGated, res.ExitGated)
		})
	}
}

func TestScan_AGatedExitHidesWhoIsBehindIt(t *testing.T) {
	registerLookGate(t)

	refused := Scan(gatedPlayer(t, 90, false, false), ScanOptions{})
	require.Len(t, refused.Sightings, 1)
	require.True(t, refused.Sightings[0].Locked, "reads as a locked exit")
	require.Empty(t, refused.Sightings[0].Mobs, "the scout behind the gate is not named")

	heat := Scan(gatedPlayer(t, 30, true, false), ScanOptions{})
	require.Len(t, heat.Sightings, 1)
	require.True(t, heat.Sightings[0].Locked)
	require.Empty(t, heat.Sightings[0].Mobs, "no figure by heat either")

	allowed := Scan(gatedPlayer(t, 90, false, true), ScanOptions{})
	require.Len(t, allowed.Sightings, 1)
	require.False(t, allowed.Sightings[0].Locked, "a player the gate allows sees normally")
	require.Len(t, allowed.Sightings[0].Mobs, 1)
}

// A mob walks through a room_gate (fleeExit, mobcommands), so it sees through
// one too: a scout's sightings do not change.
func TestLookAndScan_AGateDoesNotBlindAMob(t *testing.T) {
	registerLookGate(t)
	mob := scanLockScene(t, 90, false, false, false)
	require.Equal(t, LookExit, ResolveLook(mob, "north").Kind)
	result := Scan(mob, ScanOptions{HostileOnly: true})
	require.Len(t, result.Sightings, 1)
	require.False(t, result.Sightings[0].Locked)
	require.Len(t, result.Sightings[0].Mobs, 1)
}

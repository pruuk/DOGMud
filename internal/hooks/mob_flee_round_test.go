package hooks

import (
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Slice 4a: a mob's flee used to resolve inside the command, free, ignoring
// standing. It now enters Disengaging and escapes on the next round through
// the same resolver as a player.

// fleeingMob readies mob 100 (seedAllRegistries) to fight player 1 in room 1.
func fleeingMob(t *testing.T) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	require.NoError(t, m.Character.Validate())
	// seedAllRegistries builds the fixture as a bare Character literal, which
	// production never does: every real spawn path sets MobInstanceId (see
	// mobs.go's AdoptCharacter and instance-creation comments). Without it,
	// combat.ResolveFleeBlockers's fleerMid is 0 and a blocker targeting this
	// mob by MobInstanceId is silently invisible to the contest.
	m.Character.MobInstanceId = m.InstanceId
	m.Character.StaminaMax.Value = 1000
	m.Character.Stamina = 1000
	m.Character.SetAggro(1, 0, characters.DefaultAttack)
	m.Character.CombatPhase.OnRoundTick()
	require.True(t, m.Character.IsInCombat(), "fixture: mob must be fighting")
	return m, rooms.LoadRoom(1)
}

func mobInRoom(room *rooms.Room, instId int) bool {
	return slices.Contains(room.GetMobs(rooms.FindAll), instId)
}

func TestMobFlee_EscapesOnTheNextRoundNotInTheCommand(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	m, room := fleeingMob(t)

	begin := actions.BeginFlee(actions.NewMobActorInRoom(m, room), "")
	require.True(t, begin.Accepted, "begin = %+v", begin)
	require.True(t, m.Character.IsDisengaging())
	require.True(t, mobInRoom(room, m.InstanceId), "the command itself must not move the mob")
	require.Less(t, m.Character.Stamina, 1000, "a mob's flee costs stamina")

	require.True(t, handleMobFlee(m, room))
	require.False(t, mobInRoom(room, m.InstanceId), "the round did not move the mob out")
	require.True(t, mobInRoom(rooms.LoadRoom(2), m.InstanceId), "the mob is not in the room north")
	require.False(t, m.Character.IsInCombat())
}

func TestMobFlee_BlockedMobReturnsToTheFight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	cfg := configs.GetConfig()
	cfg.Balance.ContestFloor = 0
	configs.SetConfigForTest(t, cfg)

	m, room := fleeingMob(t)
	m.Character.Stats.Dexterity.ValueAdj = 1
	u := users.GetByUserId(1)
	require.NoError(t, u.Character.Validate())
	u.Character.Stats.Dexterity.ValueAdj = 100
	u.Character.SetAggro(0, m.InstanceId, characters.DefaultAttack)

	require.True(t, actions.BeginFlee(actions.NewMobActorInRoom(m, room), "").Accepted)
	events.DrainQueuedMessagesForTest(1)
	require.True(t, handleMobFlee(m, room))
	require.True(t, mobInRoom(room, m.InstanceId), "a blocked mob left the room")
	require.Equal(t, combatphase.Engaged, m.Character.CombatPhase.State())
}

func TestMobFlee_KnockedDownMobCannotFlee(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	m, room := fleeingMob(t)
	setCombatPositionParallel(&m.Character, position.Prone)

	begin := actions.BeginFlee(actions.NewMobActorInRoom(m, room), "")
	require.Equal(t, actions.FleeRefuseProne, begin.Refusal)
	require.False(t, m.Character.IsDisengaging())
}

func TestMobFlee_CorneredMobStaysInTheFight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	m, room := fleeingMob(t)
	room.Exits = nil

	require.True(t, actions.BeginFlee(actions.NewMobActorInRoom(m, room), "").Accepted)
	require.True(t, handleMobFlee(m, room))
	require.True(t, m.Character.IsInCombat(), "a cornered mob dropped its fight")
	require.Equal(t, combatphase.Engaged, m.Character.CombatPhase.State())
}

func TestMobFlee_CombatEndingRetractsTheAdmission(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	m, room := fleeingMob(t)

	require.True(t, actions.BeginFlee(actions.NewMobActorInRoom(m, room), "").Accepted)
	targeting.Release(&m.Character, targeting.ReasonDisengage)
	require.False(t, m.Character.IsInCombat())
	_, admitted := m.Character.TakeFleeAdmission()
	require.False(t, admitted, "combat ended but the mob's flee admission survived")
}

func TestHandleMobCombat_DisengagingMobEscapes(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	m, room := fleeingMob(t)

	require.True(t, actions.BeginFlee(actions.NewMobActorInRoom(m, room), "").Accepted)
	handleMobCombat(events.NewRound{RoundNumber: 7})
	require.False(t, mobInRoom(room, m.InstanceId), "the mob round pass did not resolve the flee")
}

package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Finding 3 / chunk 5.2 — harmful spells must honour the same target
// authorization policy as melee.
//
// The bug: melee refused non-combatants and player_attack_immune mobs, but
// InitiateCast checked only charm and non-combatant on HarmSingle, and nothing
// at all on HarmMulti or HarmArea. A protected quest or tutorial NPC could be
// killed with a spell.
// ---------------------------------------------------------------------------

// playerStub is a stubActor that reports itself as a player, so the
// player-only authorization branches in InitiateCast are exercised.
type playerStub struct {
	*stubActor
	userId int
	sent   []string
}

func (p *playerStub) IsPlayer() bool { return true }
func (p *playerStub) GetUserId() int { return p.userId }
func (p *playerStub) SendText(_ messaging.Category, s string) {
	p.sent = append(p.sent, s)
}

func newPlayerActor() (*playerStub, *characters.Character, *rooms.Room) {
	char := newTestChar()
	room := newTestRoom()
	return &playerStub{stubActor: newStubActor(char, room), userId: 1}, char, room
}

// seedRoomMob registers a mob instance and puts it in the room, returning a
// cleanup func. mutate customizes the mob before registration.
func seedRoomMob(t *testing.T, room *rooms.Room, instanceId int, name string, mutate func(*mobs.Mob)) func() {
	t.Helper()

	m := &mobs.Mob{
		MobId:      1,
		InstanceId: instanceId,
		Character: characters.Character{
			Name:   name,
			RoomId: room.RoomId,
			Health: 100,
			Buffs:  buffs.New(),
		},
	}
	m.Character.HealthMax.Value = 100
	if mutate != nil {
		mutate(m)
	}

	mobs.SetInstanceForTest(instanceId, m)
	room.AddMob(instanceId)

	return func() {
		room.RemoveMob(instanceId)
		mobs.SetInstanceForTest(instanceId, nil)
	}
}

// --- HarmSingle, named target ----------------------------------------------

func TestInitiateCast_HarmSingle_RefusesAttackImmuneMob(t *testing.T) {
	_, cleanupSpell := seedTestSpell("harm-immune-single", spells.HarmSingle, 4)
	defer cleanupSpell()

	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9001, "Caravan Guard", func(m *mobs.Mob) {
		m.PlayerAttackImmune = true
	})()

	result := InitiateCast(actor, "harm-immune-single", "caravan guard")

	assert.True(t, result.NoTarget, "attack-immune mob must not be a valid harm target")
	assert.False(t, result.Initiated)
	assert.Empty(t, result.TargetMobInstanceIds)
	assert.NotEmpty(t, actor.sent, "player should be told why the target was refused")
}

func TestInitiateCast_HarmSingle_RefusesNonCombatant(t *testing.T) {
	_, cleanupSpell := seedTestSpell("harm-noncombat-single", spells.HarmSingle, 4)
	defer cleanupSpell()

	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9002, "Barkeep", func(m *mobs.Mob) {
		m.NonCombatant = true
	})()

	result := InitiateCast(actor, "harm-noncombat-single", "barkeep")

	assert.True(t, result.NoTarget)
	assert.False(t, result.Initiated)
}

func TestInitiateCast_HarmSingle_AllowsOrdinaryMob(t *testing.T) {
	_, cleanupSpell := seedTestSpell("harm-ok-single", spells.HarmSingle, 4)
	defer cleanupSpell()

	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9003, "Bandit", nil)()

	result := InitiateCast(actor, "harm-ok-single", "bandit")

	require.True(t, result.Initiated, "an ordinary mob must remain targetable")
	assert.Equal(t, []int{9003}, result.TargetMobInstanceIds)
}

// --- HarmSingle, aggro fallback (no target named) ---------------------------

// A player_attack_immune mob can still fight, so it can put itself in the
// player's aggro slot. The no-target fallback must not turn that into a
// licence to nuke it.
func TestInitiateCast_HarmSingle_RefusesAttackImmuneAggroFallback(t *testing.T) {
	_, cleanupSpell := seedTestSpell("harm-immune-fallback", spells.HarmSingle, 4)
	defer cleanupSpell()

	actor, char, room := newPlayerActor()
	defer seedRoomMob(t, room, 9004, "Caravan Guard", func(m *mobs.Mob) {
		m.PlayerAttackImmune = true
	})()
	char.SetAggro(0, 9004, characters.DefaultAttack)

	result := InitiateCast(actor, "harm-immune-fallback", "")

	assert.True(t, result.NoTarget, "aggro fallback must apply the same policy")
	assert.False(t, result.Initiated)
	assert.Empty(t, result.TargetMobInstanceIds)
}

// --- HarmMulti --------------------------------------------------------------

func TestInitiateCast_HarmMulti_RefusesAttackImmuneMob(t *testing.T) {
	_, cleanupSpell := seedTestSpell("harm-immune-multi", spells.HarmMulti, 4)
	defer cleanupSpell()

	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9005, "Caravan Guard", func(m *mobs.Mob) {
		m.PlayerAttackImmune = true
	})()

	result := InitiateCast(actor, "harm-immune-multi", "caravan guard")

	assert.True(t, result.NoTarget, "HarmMulti enforced no policy at all before finding 3")
	assert.False(t, result.Initiated)
}

func TestInitiateCast_HarmMulti_RefusesNonCombatant(t *testing.T) {
	_, cleanupSpell := seedTestSpell("harm-noncombat-multi", spells.HarmMulti, 4)
	defer cleanupSpell()

	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9006, "Barkeep", func(m *mobs.Mob) {
		m.NonCombatant = true
	})()

	result := InitiateCast(actor, "harm-noncombat-multi", "barkeep")

	assert.True(t, result.NoTarget)
	assert.False(t, result.Initiated)
}

func TestInitiateCast_HarmMulti_RefusesCompanion(t *testing.T) {
	_, cleanupSpell := seedTestSpell("harm-companion-multi", spells.HarmMulti, 4)
	defer cleanupSpell()

	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9007, "Wolf", func(m *mobs.Mob) {
		m.Character.Charm(1, 100, "")
	})()

	result := InitiateCast(actor, "harm-companion-multi", "wolf")

	assert.True(t, result.NoTarget)
	assert.False(t, result.Initiated)
}

func TestInitiateCast_HarmMulti_AllowsOrdinaryMob(t *testing.T) {
	_, cleanupSpell := seedTestSpell("harm-ok-multi", spells.HarmMulti, 4)
	defer cleanupSpell()

	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9008, "Bandit", nil)()

	result := InitiateCast(actor, "harm-ok-multi", "bandit")

	require.True(t, result.Initiated)
	assert.Equal(t, []int{9008}, result.TargetMobInstanceIds)
}

// --- HarmArea ---------------------------------------------------------------

func TestInitiateCast_HarmArea_ExcludesProtectedMobs(t *testing.T) {
	_, cleanupSpell := seedTestSpell("harm-area-policy", spells.HarmArea, 4)
	defer cleanupSpell()

	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9009, "Bandit", nil)()
	defer seedRoomMob(t, room, 9010, "Caravan Guard", func(m *mobs.Mob) {
		m.PlayerAttackImmune = true
	})()
	defer seedRoomMob(t, room, 9011, "Barkeep", func(m *mobs.Mob) {
		m.NonCombatant = true
	})()

	result := InitiateCast(actor, "harm-area-policy", "")

	require.True(t, result.Initiated)
	assert.Contains(t, result.TargetMobInstanceIds, 9009, "ordinary mobs are still hit")
	assert.NotContains(t, result.TargetMobInstanceIds, 9010, "attack-immune mobs must be spared")
	assert.NotContains(t, result.TargetMobInstanceIds, 9011, "non-combatants must be spared")
}

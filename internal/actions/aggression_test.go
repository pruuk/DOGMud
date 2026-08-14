package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

func newAggroTestMob(instanceId int) *mobs.Mob {
	m := &mobs.Mob{
		MobId:      1,
		InstanceId: instanceId,
		HomeRoomId: 1,
		Character: characters.Character{
			Name:      "Target-Dummy",
			RoomId:    1,
			Health:    100,
			Buffs:     buffs.New(),
			Cooldowns: map[string]int{},
		},
	}
	m.Character.HealthMax.Value = 100
	return m
}

func newAggroTestUser() *users.UserRecord {
	return &users.UserRecord{
		UserId: 7,
		Character: &characters.Character{
			Name:      "Aggressor",
			RoomId:    1,
			Buffs:     buffs.New(),
			Cooldowns: map[string]int{},
		},
	}
}

// SeedAggression sits on the hot path of every melee special move, so a nil
// anywhere must be inert rather than fatal. A panic here would take down the
// connection goroutine on an ordinary kick.
func TestSeedAggression_NilArgumentsAreInert(t *testing.T) {
	user := newAggroTestUser()
	mob := newAggroTestMob(101)
	room := &rooms.Room{RoomId: 1}

	assert.NotPanics(t, func() { SeedAggression(nil, mob, room, true) })
	assert.NotPanics(t, func() { SeedAggression(user, nil, room, true) })
	assert.NotPanics(t, func() { SeedAggression(user, mob, nil, true) })
}

// A mob belonging to no faction has no crime to record against it, on either
// freshness. It must still be safe to call, because most of the world's
// wildlife is factionless.
func TestSeedAggression_FactionlessMobIsSafe(t *testing.T) {
	user := newAggroTestUser()
	room := &rooms.Room{RoomId: 1}

	assert.NotPanics(t, func() {
		SeedAggression(user, newAggroTestMob(101), room, true)
	})
	assert.NotPanics(t, func() {
		SeedAggression(user, newAggroTestMob(102), room, false)
	})
}

// RecordAssaultCrime moved here from internal/usercommands so the melee
// specials could reach it through AcquireMeleeTarget. Guard the same nil
// safety its callers rely on.
func TestRecordAssaultCrime_FactionlessMobReturnsEarly(t *testing.T) {
	assert.NotPanics(t, func() {
		RecordAssaultCrime(newAggroTestUser(), newAggroTestMob(101), &rooms.Room{RoomId: 1})
	})
}

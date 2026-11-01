package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A locked container surrenders nothing to a thief until it is unlocked or
// picked, the gate get, put and look all apply. Rank 2 used to be the only
// thing between a fresh character and every locked chest in the world
// (#375 phase 7 lowered it to 1). The refusal spends no cooldown.
func TestSteal_LockedContainerRefuses(t *testing.T) {
	pinStealCooldown(t)
	actor := newStealPlayerActor(200, 8)
	actor.room.Containers = map[string]rooms.Container{
		"chest": {Gold: 100, Lock: gamelock.Lock{Difficulty: 20}},
	}

	res := Steal(actor, StealOptions{ContainerNoun: "chest"})

	require.False(t, res.Succeeded, "outcome: %+v", res)
	assert.Equal(t, "locked", res.Reason)
	assert.Equal(t, 100, actor.room.Containers["chest"].Gold, "the gold stays in the chest")
	assert.Zero(t, actor.char.GetCooldown(stealKey()), "a refusal spends no cooldown")
}

// The same chest, once picked, can be stolen from: the gate reads the lock's
// state, not whether the container has a lock at all.
func TestSteal_UnlockedContainerIsOpen(t *testing.T) {
	pinStealCooldown(t)
	actor := newStealPlayerActor(200, 8)
	// Test binaries load Go defaults, where a game hour spans no rounds and a
	// lock relocks at once; pin the shipped timing, as gamelock's tests do.
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
	defer util.ResetRoundCountForTest()
	util.SetRoundCountForTest(uint64(util.RoundCountMinimum) + 1000)
	lock := gamelock.Lock{Difficulty: 20}
	lock.SetUnlocked()
	require.False(t, lock.IsLocked(), "the fixture must really be unlocked")
	actor.room.Containers = map[string]rooms.Container{"chest": {Gold: 100, Lock: lock}}

	res := Steal(actor, StealOptions{ContainerNoun: "chest"})

	require.True(t, res.Succeeded, "outcome: %+v", res)
}

// plant is the other half of the same rule: nothing goes into a locked
// container, and the planter keeps the item.
func TestPlant_LockedContainerRefuses(t *testing.T) {
	pinStealCooldown(t)
	actor := newPlantPlayerActor(200, 8)
	actor.room.Containers = map[string]rooms.Container{
		"chest": {Lock: gamelock.Lock{Difficulty: 20}},
	}
	seedPlantItem(actor)
	carried := len(actor.char.Items)

	res := Plant(actor, PlantOptions{ContainerNoun: "chest", ItemNoun: "!1"})

	require.False(t, res.Succeeded, "outcome: %+v", res)
	assert.Equal(t, "locked", res.Reason)
	assert.Empty(t, actor.room.Containers["chest"].Items, "nothing went into the chest")
	assert.Len(t, actor.char.Items, carried, "the planter keeps the item")
	assert.Zero(t, actor.char.GetCooldown(stealKey()), "a refusal spends no cooldown")
}

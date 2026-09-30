package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The skullduggery cooldown is spent by an attempt, never by a refusal. It
// used to be armed in Steal and Plant before any target was resolved, so a
// fresh character (Skullduggery rank 1 passes the command's visibility gate
// but fails every theft's rank-2 gate) was refused on the first try and then
// locked out of trying again for the whole cooldown.

const stealCooldownTestMobId = 9911

// pinStealCooldown ships the balance and timing a player actually gets:
// StealCooldown 60 real seconds on a four-second round, 15 rounds. The test
// binary's own default would be a zero cooldown.
func pinStealCooldown(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.StealCooldown = 60
	c.Balance.ContestFloor = 0
	c.Balance.SkillWeight = 5.0
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
}

func stealKey() string { return skills.Skullduggery.String(`steal`) }

func requireNoStealCooldown(t *testing.T, char *characters.Character, why string) {
	t.Helper()
	require.True(t, char.CooldownReady(stealKey()),
		"%s: a refusal must leave the steal cooldown unarmed, got %d rounds", why, char.GetCooldown(stealKey()))
}

func TestStealCooldown_RankOneRefusalLeavesItUnarmed(t *testing.T) {
	pinStealCooldown(t)

	t.Run("mob", func(t *testing.T) {
		mobs.SetInstanceForTest(stealCooldownTestMobId, newStealTestMob(stealCooldownTestMobId, 50, 1))
		defer mobs.SetInstanceForTest(stealCooldownTestMobId, nil)
		actor := newStealPlayerActor(100, 1)
		res := Steal(actor, StealOptions{TargetMobInstanceId: stealCooldownTestMobId})
		require.Equal(t, "not advanced enough", res.Reason)
		requireNoStealCooldown(t, actor.char, "rank 1 against a mob")
	})

	t.Run("container", func(t *testing.T) {
		actor := newStealPlayerActor(100, 1)
		actor.room.Containers = map[string]rooms.Container{"chest": {Gold: 100}}
		res := Steal(actor, StealOptions{ContainerNoun: "chest"})
		require.Equal(t, "not advanced enough", res.Reason)
		requireNoStealCooldown(t, actor.char, "rank 1 against a container")
	})

	t.Run("player", func(t *testing.T) {
		mark := users.NewTestUser(3011, "mark", "Mark", 0)
		mark.Character.Gold = 50
		defer users.SeedUsersForTest(map[int]*users.UserRecord{3011: mark})()
		actor := newStealMobActor(100, 1)
		res := Steal(actor, StealOptions{TargetUserId: 3011})
		require.Equal(t, "not advanced enough", res.Reason)
		requireNoStealCooldown(t, actor.char, "rank 1 against a player")
	})

	t.Run("household", func(t *testing.T) {
		h := setupHousehold(t, 99301, 1, nil, false)
		pinStealCooldown(t)
		res := Steal(h.thief, StealOptions{HouseholdItem: h.itm})
		require.Equal(t, "not advanced enough", res.Reason)
		requireNoStealCooldown(t, h.thief.char, "rank 1 against a household bauble")
	})
}

func TestStealCooldown_OtherRefusalsLeaveItUnarmed(t *testing.T) {
	pinStealCooldown(t)

	t.Run("mob not found", func(t *testing.T) {
		actor := newStealPlayerActor(100, 5)
		res := Steal(actor, StealOptions{TargetMobInstanceId: 424242})
		require.Equal(t, "target not found", res.Reason)
		requireNoStealCooldown(t, actor.char, "a vanished mob")
	})

	t.Run("container not found", func(t *testing.T) {
		actor := newStealPlayerActor(100, 5)
		res := Steal(actor, StealOptions{ContainerNoun: "wardrobe"})
		require.Equal(t, "not found", res.Reason)
		requireNoStealCooldown(t, actor.char, "a missing container")
	})

	t.Run("player not found", func(t *testing.T) {
		actor := newStealMobActor(100, 5)
		res := Steal(actor, StealOptions{TargetUserId: 424242})
		require.Equal(t, "target not found", res.Reason)
		requireNoStealCooldown(t, actor.char, "a vanished player")
	})

	t.Run("companion", func(t *testing.T) {
		m := newStealTestMob(stealCooldownTestMobId, 50, 1)
		m.Character.Charmed = characters.NewCharm(77, characters.CharmPermanent, "")
		mobs.SetInstanceForTest(stealCooldownTestMobId, m)
		defer mobs.SetInstanceForTest(stealCooldownTestMobId, nil)
		actor := newStealPlayerActor(100, 5)
		res := Steal(actor, StealOptions{TargetMobInstanceId: stealCooldownTestMobId})
		require.Equal(t, "companion", res.Reason)
		requireNoStealCooldown(t, actor.char, "a companion")
	})

	t.Run("empty container", func(t *testing.T) {
		actor := newStealPlayerActor(100, 5)
		actor.room.Containers = map[string]rooms.Container{"chest": {}}
		res := Steal(actor, StealOptions{ContainerNoun: "chest"})
		require.Equal(t, "empty", res.Reason)
		requireNoStealCooldown(t, actor.char, "an empty container")
	})

	t.Run("plant target not found", func(t *testing.T) {
		actor := newPlantPlayerActor(100, 5)
		seedPlantItem(actor)
		res := Plant(actor, PlantOptions{TargetMobInstanceId: 424242, ItemNoun: "!1"})
		require.Equal(t, "target not found", res.Reason)
		requireNoStealCooldown(t, actor.char, "planting on a vanished mob")
	})

	t.Run("plant container not found", func(t *testing.T) {
		actor := newPlantPlayerActor(100, 5)
		seedPlantItem(actor)
		res := Plant(actor, PlantOptions{ContainerNoun: "wardrobe", ItemNoun: "!1"})
		require.Equal(t, "not found", res.Reason)
		requireNoStealCooldown(t, actor.char, "planting in a missing container")
	})
}

// A real attempt, won or lost, spends the cooldown, and it lasts the
// configured 60 seconds: 15 four-second rounds.
func TestStealCooldown_AnAttemptArmsItForTheConfiguredSeconds(t *testing.T) {
	pinStealCooldown(t)

	for _, tc := range []struct {
		name       string
		perception int
		want       func(StealResult) bool
	}{
		{"won", 1, func(r StealResult) bool { return r.Succeeded }},
		{"lost", 1000000, func(r StealResult) bool { return r.Detected && !r.Succeeded }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mobs.SetInstanceForTest(stealCooldownTestMobId, newStealTestMob(stealCooldownTestMobId, 50, tc.perception))
			defer mobs.SetInstanceForTest(stealCooldownTestMobId, nil)
			// A mob thief: its outcome is immediate, with no pickpocket pause.
			actor := newStealMobActor(200, 8)
			if tc.name == "lost" {
				actor.char.Stats.Dexterity.ValueAdj = 1
				actor.char.Skills[string(skills.Skullduggery)] = 2
			}
			res := Steal(actor, StealOptions{TargetMobInstanceId: stealCooldownTestMobId})
			require.True(t, tc.want(res), "outcome: %+v", res)
			assert.Equal(t, 15, actor.char.GetCooldown(stealKey()),
				"60 real seconds at 4-second rounds")

			again := Steal(actor, StealOptions{TargetMobInstanceId: stealCooldownTestMobId})
			assert.True(t, again.OnCooldown, "a second try inside the cooldown is refused")
		})
	}

	t.Run("container", func(t *testing.T) {
		actor := newStealPlayerActor(200, 8)
		actor.room.Containers = map[string]rooms.Container{"chest": {Gold: 100}}
		res := Steal(actor, StealOptions{ContainerNoun: "chest"})
		require.True(t, res.Succeeded, "outcome: %+v", res)
		assert.Equal(t, 15, actor.char.GetCooldown(stealKey()))
	})

	t.Run("plant", func(t *testing.T) {
		mobs.SetInstanceForTest(stealCooldownTestMobId, newPlantTestMob(stealCooldownTestMobId, 1))
		defer mobs.SetInstanceForTest(stealCooldownTestMobId, nil)
		actor := newPlantPlayerActor(200, 8)
		seedPlantItem(actor)
		res := Plant(actor, PlantOptions{TargetMobInstanceId: stealCooldownTestMobId, ItemNoun: "!1"})
		require.True(t, res.Succeeded || res.Detected, "outcome: %+v", res)
		assert.Equal(t, 15, actor.char.GetCooldown(stealKey()))
	})
}

// An unwatched container plant is an uncontested attempt, not a refusal, so
// it spends the cooldown too. The score must be read (and the cooldown
// armed) even when no observer is there to contest it.
func TestStealCooldown_AnUnwatchedContainerPlantArmsIt(t *testing.T) {
	pinStealCooldown(t)
	actor := newPlantPlayerActor(200, 8)
	actor.room.Containers = map[string]rooms.Container{"chest": {}}
	seedPlantItem(actor)
	res := Plant(actor, PlantOptions{ContainerNoun: "chest", ItemNoun: "!1"})
	require.True(t, res.Succeeded, "an unwatched plant succeeds: %+v", res)
	assert.Equal(t, 15, actor.char.GetCooldown(stealKey()),
		"an unwatched container plant arms the 60-second cooldown")
}

// Steal and plant share one skullduggery cooldown: a steal spends it, and a
// plant straight after is refused without trying.
func TestStealCooldown_StealThenPlantIsRefused(t *testing.T) {
	pinStealCooldown(t)
	actor := newStealPlayerActor(200, 8)
	actor.room.Containers = map[string]rooms.Container{"chest": {Gold: 100}}
	res := Steal(actor, StealOptions{ContainerNoun: "chest"})
	require.True(t, res.Succeeded, "outcome: %+v", res)

	seedPlantItem(actor)
	plant := Plant(actor, PlantOptions{ContainerNoun: "chest", ItemNoun: "!1"})
	assert.True(t, plant.OnCooldown, "a plant inside the steal's cooldown is refused: %+v", plant)
	assert.False(t, plant.Succeeded)
	_, stillCarried := actor.char.FindInBackpack("!1")
	assert.True(t, stillCarried, "the refused plant keeps its item")
}

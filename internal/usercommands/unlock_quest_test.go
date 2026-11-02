package usercommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 tier 1: unlocking is a quest step (quest 14's strongbox), so
// unlock raises a `command` event aimed at the container, as picklock does.
const (
	unlockProbeQuest = 99704
	unlockTestKey    = 999963
	unlockProbeToken = "99704-start"
)

func unlockQuestFixture(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
	prevRound := util.GetRoundCount()
	util.SetRoundCountForTest(uint64(util.RoundCountMinimum) + 1000)
	t.Cleanup(func() { util.SetRoundCountForTest(prevRound) })
	t.Cleanup(seedAllRegistries())

	room := rooms.LoadRoom(2)
	require.NotNil(t, room)
	before := room.Containers
	room.Containers = map[string]rooms.Container{"lockbox": {Lock: gamelock.Lock{Difficulty: 3}}}
	t.Cleanup(func() { room.Containers = before })

	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		unlockTestKey: {ItemId: unlockTestKey, Name: "test key", Type: items.Key,
			KeyLockId: fmt.Sprintf("%d-lockbox", room.RoomId), Weight: 0.1},
	}))
	user := users.GetByUserId(1)
	require.NotNil(t, user)
	user.Character.Stats.Strength.ValueAdj = 100

	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: unlockProbeQuest,
		Name:    "Unlock Probe",
		Triggers: []quests.TriggerDef{{
			Event:   "command",
			Command: "unlock",
			Target:  quests.TargetContainer,
			Noun:    "lockbox",
			Actions: []quests.ActionDef{{Grant: unlockProbeToken}},
		}},
	})
	return user, room
}

func TestUnlock_BackpackKeyRaisesTheQuestEvent(t *testing.T) {
	user, room := unlockQuestFixture(t)
	require.True(t, acquire.AcquireItem(user, items.New(unlockTestKey), acquire.AcquireGive))

	_, err := Unlock("lockbox", user, room, 0)
	require.NoError(t, err)

	require.False(t, room.Containers["lockbox"].Lock.IsLocked(), "the key opened it")
	require.True(t, user.Character.HasQuest(unlockProbeToken))
}

func TestUnlock_KeyRingRaisesTheQuestEvent(t *testing.T) {
	user, room := unlockQuestFixture(t)
	user.Character.SetKey(fmt.Sprintf("key-%d-lockbox", room.RoomId), fmt.Sprintf("%d", unlockTestKey))

	_, err := Unlock("lockbox", user, room, 0)
	require.NoError(t, err)

	require.True(t, user.Character.HasQuest(unlockProbeToken))
}

func TestUnlock_NoKeyRaisesNothing(t *testing.T) {
	user, room := unlockQuestFixture(t)

	_, err := Unlock("lockbox", user, room, 0)
	require.NoError(t, err)

	require.True(t, room.Containers["lockbox"].Lock.IsLocked())
	require.False(t, user.Character.HasQuest(unlockProbeToken), "a refused unlock must raise nothing")
}

// The player typed the step's command and the box stands open, perhaps because
// another player opened it. Without this the step waits for the relock.
func TestUnlock_AlreadyOpenContainerRaisesTheEvent(t *testing.T) {
	user, room := unlockQuestFixture(t)
	box := room.Containers["lockbox"]
	box.Lock.SetUnlocked()
	room.Containers["lockbox"] = box

	_, err := Unlock("lockbox", user, room, 0)
	require.NoError(t, err)

	require.True(t, user.Character.HasQuest(unlockProbeToken))
}

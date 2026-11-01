package usercommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 (quest 99, Quiet Hands): picking a lock is a quest step, so
// picklock raises a `command` event when a lock opens, aimed at the room
// container by name. Each success path is driven on its own: the keyring and
// the pin game are separate branches with separate calls to the helper.
const (
	picklockProbeQuest = 99703
	picklockTestPicks  = 999962
	picklockProbeToken = "99703-start"
)

func picklockQuestFixture(t *testing.T) (*users.UserRecord, *rooms.Room, string) {
	t.Helper()
	// Test binaries load Go defaults, where a game hour spans no rounds and a
	// lock relocks at once, and a lock opened in round 0 reads as never opened.
	// Pin the shipped timing and a real round, as gamelock's tests do.
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
	prevRound := util.GetRoundCount()
	util.SetRoundCountForTest(uint64(util.RoundCountMinimum) + 1000)
	t.Cleanup(func() { util.SetRoundCountForTest(prevRound) })
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		picklockTestPicks: {ItemId: picklockTestPicks, Name: "test lockpicks", Type: items.Lockpicks, Uses: 3, Weight: 0.1},
	}))
	user := users.GetByUserId(1)
	require.NotNil(t, user)
	user.Character.Stats.Strength.ValueAdj = 100
	_ = user.Character.GetSkillLevel(skills.Skullduggery) // allocates Skills
	user.Character.Skills[string(skills.Skullduggery)] = 1
	require.True(t, acquire.AcquireItem(user, items.New(picklockTestPicks), acquire.AcquireGive))

	room := rooms.LoadRoom(2)
	require.NotNil(t, room)
	before := room.Containers
	room.Containers = map[string]rooms.Container{"lockbox": {Lock: gamelock.Lock{Difficulty: 3}}}
	t.Cleanup(func() { room.Containers = before })

	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: picklockProbeQuest,
		Name:    "Picklock Probe",
		Triggers: []quests.TriggerDef{{
			Event:   "command",
			Command: "picklock",
			Target:  quests.TargetContainer,
			Noun:    "lockbox",
			Actions: []quests.ActionDef{{Grant: picklockProbeToken}},
		}},
	})
	return user, room, fmt.Sprintf("%d-%s", room.RoomId, "lockbox")
}

func TestPicklock_OpenedLockRaisesTheQuestEvent(t *testing.T) {
	user, room, lockId := picklockQuestFixture(t)
	user.Character.SetKey(lockId, util.GetLockSequence(lockId, 3, string(configs.GetServerConfig().Seed), 0))

	_, err := Picklock("lockbox", user, room, 0)
	require.NoError(t, err)

	require.True(t, user.Character.HasQuest(picklockProbeToken),
		"an opened container lock must raise command picklock aimed at the container")
}

func TestPicklock_UnfinishedPickRaisesNoQuestEvent(t *testing.T) {
	user, room, _ := picklockQuestFixture(t)

	_, err := Picklock("lockbox", user, room, 0) // no keyring: the pin game only starts
	require.NoError(t, err)
	user.ClearPrompt()

	require.False(t, user.Character.HasQuest(picklockProbeToken),
		"a pick that has not opened the lock must raise nothing")
}

// The pin game is the path a first-time student takes (no keyring yet), and
// its success branch has its own call. The event must wait for the last pin.
func TestPicklock_PinGameRaisesTheQuestEventOnTheLastPinOnly(t *testing.T) {
	user, room, lockId := picklockQuestFixture(t)
	t.Cleanup(user.ClearPrompt)
	seq := util.GetLockSequence(lockId, 3, string(configs.GetServerConfig().Seed), 0)
	require.Len(t, seq, 3)
	run := func() {
		_, err := Picklock("lockbox", user, room, 0)
		require.NoError(t, err)
	}

	drive(t, user, run, seq[:1], seq[1:2])
	require.Equal(t, seq[:2], user.Character.GetKey(lockId), "the first two pins are on the keyring")
	require.False(t, user.Character.HasQuest(picklockProbeToken),
		"a pin game short of its last pin must raise nothing")

	user.RoutePromptInput(seq[2:], true)
	run()
	require.True(t, user.Character.HasQuest(picklockProbeToken),
		"the last pin must raise command picklock aimed at the container")
	require.False(t, room.Containers["lockbox"].Lock.IsLocked(), "the lock is open")
	require.Nil(t, user.GetPrompt(), "the pin game is over")
}

// A wrong pin breaks the picks and opens nothing.
func TestPicklock_WrongPinRaisesNoQuestEvent(t *testing.T) {
	user, room, lockId := picklockQuestFixture(t)
	t.Cleanup(user.ClearPrompt)
	seq := util.GetLockSequence(lockId, 3, string(configs.GetServerConfig().Seed), 0)
	wrong := "U"
	if seq[:1] == "U" {
		wrong = "D"
	}

	drive(t, user, func() {
		_, err := Picklock("lockbox", user, room, 0)
		require.NoError(t, err)
	}, wrong)

	require.True(t, room.Containers["lockbox"].Lock.IsLocked(), "a wrong pin opens nothing")
	require.False(t, user.Character.HasQuest(picklockProbeToken), "a wrong pin must raise nothing")
}

// Once open, the lock refuses a second pick, and the refusal raises nothing:
// the event fires once for the opening.
func TestPicklock_AlreadyOpenLockRaisesNoSecondEvent(t *testing.T) {
	user, room, lockId := picklockQuestFixture(t)
	user.Character.SetKey(lockId, util.GetLockSequence(lockId, 3, string(configs.GetServerConfig().Seed), 0))

	_, err := Picklock("lockbox", user, room, 0)
	require.NoError(t, err)
	require.True(t, user.Character.HasQuest(picklockProbeToken), "the first pick opens the lock")
	user.Character.ClearQuestToken(picklockProbeToken)

	_, err = Picklock("lockbox", user, room, 0)
	require.NoError(t, err)

	require.False(t, user.Character.HasQuest(picklockProbeToken),
		"a lock already open must raise nothing")
}

// An exit lock is aimed at nothing and carries the exit's name as its noun, on
// the keyring path and with no container in the room to confuse it with.
func TestPicklock_OpenedExitLockCarriesTheExitNameAsItsNoun(t *testing.T) {
	user, room, _ := picklockQuestFixture(t)
	const gate = "testgate"
	room.Exits[gate] = exit.RoomExit{RoomId: 1, Lock: gamelock.Lock{Difficulty: 3}}
	t.Cleanup(func() { delete(room.Exits, gate) })
	lockId := fmt.Sprintf("%d-%s", room.RoomId, gate)
	user.Character.SetKey(lockId, util.GetLockSequence(lockId, 3, string(configs.GetServerConfig().Seed), 0))

	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: picklockProbeQuest,
		Name:    "Picklock Exit Probe",
		Triggers: []quests.TriggerDef{{
			Event:   "command",
			Command: "picklock",
			Target:  quests.TargetNone,
			Noun:    gate,
			Actions: []quests.ActionDef{{Grant: picklockProbeToken}},
		}},
	})

	_, err := Picklock(gate, user, room, 0)
	require.NoError(t, err)

	require.True(t, user.Character.HasQuest(picklockProbeToken),
		"an opened exit lock must raise command picklock with the exit's name")
}

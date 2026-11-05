package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// For Bloom only time online counts (owner, 2026-11-04): a player must not be able
// to log out and wait off a craving or a withdrawal. BloomAbstinentRounds is
// ticked by BloomTick, whose loop covers online users only, and is saved, so a
// relog neither stops the count nor moves it.

const (
	bloomTestUser      = 7401
	bloomTestStart     = uint64(2_000_000)
	bloomTestOnset     = 60
	bloomTestDecay     = 300
	bloomCrashId       = 91
	bloomWithdrawalId  = 92
	bloomCommunionId   = 90
	bloomWaferItemId   = 40108
	bloomCleanFragment = "finally loosened"
)

func seedBloomTick(t *testing.T) {
	t.Helper()
	drainEventQueueOnCleanup(t)
	cfg := configs.GetConfig()
	cfg.Balance.BloomWithdrawalOnsetRounds = bloomTestOnset
	cfg.Balance.BloomAddictionDecayRounds = bloomTestDecay
	cfg.Balance.BloomCommunionRounds = 30
	cfg.Balance.BloomAddictionPerDose = 1
	cfg.Balance.BloomCrashRoundsMult = 2.5
	cfg.Balance.BloomMutationAdvanceChance = 0.000001
	cfg.Balance.BloomNewMutationChance = 0.000001
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(bloomTestStart)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		bloomCommunionId:  {ConditionId: bloomCommunionId, Name: "Communion", TriggerCount: 30, RoundInterval: 1},
		bloomCrashId:      {ConditionId: bloomCrashId, Name: "Crash", TriggerCount: 75, RoundInterval: 1},
		bloomWithdrawalId: {ConditionId: bloomWithdrawalId, Name: "Withdrawal", TriggerCount: 200, RoundInterval: 1},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		bloomWaferItemId: {ItemId: bloomWaferItemId, Name: "Bloom Wafer", Type: items.Potion, Subtype: items.Drinkable, Uses: 1, Toxicity: 20},
	}))
}

// logIn makes the character the one online user.
func logIn(t *testing.T, c *characters.Character) *users.UserRecord {
	t.Helper()
	u := users.NewTestUser(bloomTestUser, "bloomuser", "BloomUser", 7401)
	u.Character = c
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{bloomTestUser: u}))
	events.DrainQueuedConditionsForTest(bloomTestUser)
	events.DrainQueuedMessagesForTest(bloomTestUser)
	return u
}

// logOut leaves nobody online, which is what a user record gone from the manager
// means to users.GetOnlineUserIds. The world clock goes on.
func logOut(t *testing.T) {
	t.Helper()
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{}))
}

// saveAndLoad is the character through its own YAML, then Validate, as
// users.LoadUser does.
func saveAndLoad(t *testing.T, c *characters.Character) *characters.Character {
	t.Helper()
	out, err := yaml.Marshal(c)
	require.NoError(t, err)
	loaded := characters.New()
	require.NoError(t, yaml.Unmarshal(out, loaded))
	require.NoError(t, loaded.Validate())
	return loaded
}

// tick runs n rounds, moving the world clock too, and returns the conditions they
// queued and what the player read.
func tick(n int) (map[int]bool, string) {
	for i := 0; i < n; i++ {
		round := util.GetRoundCount() + 1
		util.SetRoundCountForTest(round)
		BloomTick(events.NewRound{RoundNumber: round})
	}
	queued := map[int]bool{}
	for _, q := range events.DrainQueuedConditionsForTest(bloomTestUser) {
		queued[q.ConditionId] = true
	}
	return queued, strings.Join(events.DrainQueuedMessagesForTest(bloomTestUser), " ")
}

// dosed takes a wafer through the real drink path and leaves the player online.
func dosed(t *testing.T) *users.UserRecord {
	t.Helper()
	seedBloomTick(t)
	c := characters.New()
	c.Name = "BloomUser"
	c.Stats.Vitality.Base = 300
	c.Stats.Vitality.Recalculate()
	c.BloomAbstinentRounds = 40 // what a dose resets
	require.True(t, c.StoreItem(items.New(bloomWaferItemId)))
	u := logIn(t, c)
	res := actions.Quaff(actions.NewUserActorInRoom(u, &rooms.Room{RoomId: 99999}).(actions.DrinkActor), "bloom wafer")
	require.True(t, res.Drank)
	require.Zero(t, u.Character.BloomAbstinentRounds, "a dose restarts the count")
	events.DrainQueuedConditionsForTest(bloomTestUser)
	events.DrainQueuedMessagesForTest(bloomTestUser)
	return u
}

// Nothing happens while logged out, however long, and coming back is no shortcut.
func TestBloom_RoundsOfflineChangeNothing(t *testing.T) {
	u := dosed(t)
	c := u.Character
	c.BloomAddiction = 3
	logOut(t)

	queued, _ := tick(10 * bloomTestDecay)
	assert.Empty(t, queued)
	assert.Zero(t, c.BloomAbstinentRounds, "no count while away")
	assert.Equal(t, 3, c.BloomAddiction, "no decay while away")

	// The world clock has run a long way, and the first round back is still round one.
	logIn(t, c)
	queued, out := tick(1)
	assert.False(t, queued[bloomWithdrawalId], "no withdrawal for time spent away")
	assert.Equal(t, 3, c.BloomAddiction)
	assert.EqualValues(t, 1, c.BloomAbstinentRounds)
	assert.NotContains(t, out, bloomCleanFragment)
}

func TestBloom_RoundsOnlineReachOnset(t *testing.T) {
	dosed(t)
	queued, _ := tick(bloomTestOnset - 1)
	assert.False(t, queued[bloomWithdrawalId], "not before the onset")
	queued, _ = tick(1)
	assert.True(t, queued[bloomWithdrawalId], "at the onset")
}

// A logout partway keeps the count, through a save and a load: it neither restarts
// the wait nor counts the time away.
func TestBloom_ALogoutPartwayKeepsTheCount(t *testing.T) {
	u := dosed(t)
	const before = bloomTestOnset - 10
	tick(before)
	require.EqualValues(t, before, u.Character.BloomAbstinentRounds)

	loaded := saveAndLoad(t, u.Character)
	require.EqualValues(t, before, loaded.BloomAbstinentRounds, "the count is saved")

	logOut(t)
	tick(5000)
	require.EqualValues(t, before, loaded.BloomAbstinentRounds, "time away adds nothing")

	logIn(t, loaded)
	queued, _ := tick(9)
	assert.False(t, queued[bloomWithdrawalId], "one short of the onset")
	queued, _ = tick(1)
	assert.True(t, queued[bloomWithdrawalId], "the relog did not stop the withdrawal")
}

// Decay counts online rounds and slides one period per point, so the next point
// is one whole period away.
func TestBloom_DecayCountsOnlineRoundsOnly(t *testing.T) {
	u := dosed(t)
	c := u.Character
	c.BloomAddiction = 3

	tick(bloomTestDecay - 1)
	assert.Equal(t, 3, c.BloomAddiction)
	tick(1)
	assert.Equal(t, 2, c.BloomAddiction)
	assert.Zero(t, c.BloomAbstinentRounds, "one period comes off the count")

	logOut(t)
	tick(10 * bloomTestDecay)
	assert.Equal(t, 2, c.BloomAddiction, "a period away is no point lost")

	logIn(t, c)
	tick(bloomTestDecay - 1)
	assert.Equal(t, 2, c.BloomAddiction)
	tick(1)
	assert.Equal(t, 1, c.BloomAddiction)

	tick(bloomTestDecay - 1)
	_, out := tick(1)
	assert.Equal(t, 0, c.BloomAddiction)
	assert.Contains(t, out, bloomCleanFragment, "the slow way still ends clean")
	assert.Zero(t, c.BloomAbstinentRounds)
}

// A save from before the count existed has addiction and reads 0. It takes no
// false withdrawal on login, and cannot sit outside the clock for ever.
func TestBloom_AnOldSaveStartsAtZero(t *testing.T) {
	seedBloomTick(t)
	var c characters.Character
	require.NoError(t, yaml.Unmarshal([]byte("name: Old\nbloom_addiction: 3\n"), &c))
	require.NoError(t, c.Validate())
	require.Zero(t, c.BloomAbstinentRounds)
	logIn(t, &c)

	queued, out := tick(1)
	assert.False(t, queued[bloomWithdrawalId], "no false withdrawal on login")
	assert.Equal(t, 3, c.BloomAddiction, "no decay yet")
	assert.NotContains(t, out, bloomCleanFragment)

	queued, _ = tick(bloomTestOnset - 1)
	assert.True(t, queued[bloomWithdrawalId], "not stuck: the withdrawal arrives")

	tick(bloomTestDecay - bloomTestOnset)
	assert.Equal(t, 2, c.BloomAddiction, "not stuck: the craving fades")
}

// A clean character keeps no count, so the first dose starts from nothing.
func TestBloom_ACleanCharacterCountsNothing(t *testing.T) {
	seedBloomTick(t)
	c := characters.New()
	logIn(t, c)
	tick(bloomTestDecay)
	assert.Zero(t, c.BloomAbstinentRounds)
}

// The flag was runtime only, so a Communion whose last round fell on the first
// round back never crashed.
func TestBloomCrash_SurvivesARelog(t *testing.T) {
	seedBloomTick(t)
	c := characters.New()
	require.NoError(t, c.AddConditionScaled(bloomCommunionId, 1.0))
	u := logIn(t, c)
	tick(1)
	require.True(t, u.Character.BloomHadCommunion)

	u = logIn(t, saveAndLoad(t, u.Character))
	require.True(t, u.Character.BloomHadCommunion, "the flag is saved")
	require.True(t, u.Character.HasCondition(bloomCommunionId), "the high is saved too")
	// The round tick ran before this listener and took the last of the high.
	u.Character.RemoveCondition(bloomCommunionId)
	u.Character.Conditions.Prune()
	require.False(t, u.Character.HasCondition(bloomCommunionId))

	queued, _ := tick(1)
	assert.True(t, queued[bloomCrashId], "the high ended, so the crash comes")
}

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

// A relog must not stop withdrawal (owner, 2026-11-04). The dose round and the
// Communion flag are saved, the round count persists across a restart, so a saved
// round still means something on the next login.

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

func bloomUser(t *testing.T, c *characters.Character) *users.UserRecord {
	t.Helper()
	u := users.NewTestUser(bloomTestUser, "bloomuser", "BloomUser", 7401)
	u.Character = c
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{bloomTestUser: u}))
	events.DrainQueuedConditionsForTest(bloomTestUser)
	events.DrainQueuedMessagesForTest(bloomTestUser)
	return u
}

// relog is a save and a load: the character through its own YAML.
func relog(t *testing.T, c *characters.Character) *characters.Character {
	t.Helper()
	out, err := yaml.Marshal(c)
	require.NoError(t, err)
	loaded := characters.New()
	require.NoError(t, yaml.Unmarshal(out, loaded))
	require.NoError(t, loaded.Validate(), "as users.LoadUser does")
	return loaded
}

// tickAt runs one Bloom tick at a round and returns the conditions it queued and
// what the player read.
func tickAt(round uint64) (map[int]bool, string) {
	util.SetRoundCountForTest(round)
	BloomTick(events.NewRound{RoundNumber: round})
	queued := map[int]bool{}
	for _, q := range events.DrainQueuedConditionsForTest(bloomTestUser) {
		queued[q.ConditionId] = true
	}
	return queued, strings.Join(events.DrainQueuedMessagesForTest(bloomTestUser), " ")
}

// A dose taken through the real drink path, then a save and a load.
func doseThenRelog(t *testing.T) *users.UserRecord {
	t.Helper()
	seedBloomTick(t)
	c := characters.New()
	c.Name = "BloomUser"
	c.Stats.Vitality.Base = 300
	c.Stats.Vitality.Recalculate()
	require.True(t, c.StoreItem(items.New(bloomWaferItemId)))
	u := bloomUser(t, c)
	res := actions.Quaff(actions.NewUserActorInRoom(u, &rooms.Room{RoomId: 99999}).(actions.DrinkActor), "bloom wafer")
	require.True(t, res.Drank)
	require.Equal(t, bloomTestStart, u.Character.BloomLastDoseRound, "the dose stamps the round")
	return bloomUser(t, relog(t, u.Character))
}

func TestBloomDose_WithdrawalSurvivesARelog(t *testing.T) {
	u := doseThenRelog(t)
	require.Equal(t, bloomTestStart, u.Character.BloomLastDoseRound, "the stamp is saved")

	queued, _ := tickAt(bloomTestStart + bloomTestOnset - 1)
	assert.False(t, queued[bloomWithdrawalId], "not before the onset")

	queued, _ = tickAt(bloomTestStart + bloomTestOnset)
	assert.True(t, queued[bloomWithdrawalId], "the relog did not stop the withdrawal")
}

func TestBloomDose_DecaySurvivesARelog(t *testing.T) {
	u := doseThenRelog(t)
	require.Equal(t, 1, u.Character.BloomAddiction)
	u.Character.BloomAddiction = 3

	tickAt(bloomTestStart + bloomTestDecay - 1)
	assert.Equal(t, 3, u.Character.BloomAddiction)

	tickAt(bloomTestStart + bloomTestDecay)
	assert.Equal(t, 2, u.Character.BloomAddiction, "the craving still fades")
	assert.Equal(t, bloomTestStart+bloomTestDecay, u.Character.BloomLastDoseRound, "the clock slides one period")
}

// A save from before the round was stored, addicted with no round. It must not
// read as a dose at round zero (a false withdrawal on login), and must not sit
// outside the clock for ever. The clock starts at the first round online.
func TestBloomOldSave_StartsTheClockAtLogin(t *testing.T) {
	seedBloomTick(t)
	c := characters.New()
	c.BloomAddiction = 3
	u := bloomUser(t, c)

	queued, out := tickAt(bloomTestStart)
	assert.False(t, queued[bloomWithdrawalId], "no false withdrawal on login")
	assert.Equal(t, 3, u.Character.BloomAddiction, "no decay yet")
	assert.NotContains(t, out, bloomCleanFragment)
	assert.Equal(t, bloomTestStart, u.Character.BloomLastDoseRound)

	queued, _ = tickAt(bloomTestStart + bloomTestOnset)
	assert.True(t, queued[bloomWithdrawalId], "not stuck: the withdrawal arrives")

	tickAt(bloomTestStart + bloomTestDecay)
	assert.Equal(t, 2, u.Character.BloomAddiction, "not stuck: the craving fades")
}

// A round ahead of the clock (a lost round file) must not wrap around into a huge
// abstinence and empty the addiction in a few ticks.
func TestBloomStamp_AheadOfTheClockRestarts(t *testing.T) {
	seedBloomTick(t)
	c := characters.New()
	c.BloomAddiction = 3
	c.BloomLastDoseRound = bloomTestStart + 5000
	u := bloomUser(t, c)

	queued, _ := tickAt(bloomTestStart)
	assert.False(t, queued[bloomWithdrawalId])
	assert.Equal(t, 3, u.Character.BloomAddiction)
	assert.Equal(t, bloomTestStart, u.Character.BloomLastDoseRound)
}

// Rounds run while a player is away, so time away is abstinence. A long absence
// lands in one tick: clean with no withdrawal if it was long enough, and the
// addict who is still addicted comes back to the craving.
func TestBloomAbsence_IsAbstinence(t *testing.T) {
	seedBloomTick(t)
	c := characters.New()
	c.BloomAddiction = 3
	c.BloomLastDoseRound = bloomTestStart
	u := bloomUser(t, c)

	queued, out := tickAt(bloomTestStart + 5*bloomTestDecay)
	assert.Equal(t, 0, u.Character.BloomAddiction)
	assert.False(t, queued[bloomWithdrawalId], "away long enough to be clean, so no withdrawal")
	assert.Contains(t, out, bloomCleanFragment)
	assert.Zero(t, u.Character.BloomLastDoseRound)

	c = characters.New()
	c.BloomAddiction = 9
	c.BloomLastDoseRound = bloomTestStart
	u = bloomUser(t, c)
	queued, out = tickAt(bloomTestStart + 2*bloomTestDecay + bloomTestOnset)
	assert.Equal(t, 7, u.Character.BloomAddiction, "two periods away, two points")
	assert.True(t, queued[bloomWithdrawalId], "still addicted, past the onset")
	assert.NotContains(t, out, bloomCleanFragment)
}

// The crash has the same hole in a narrower window: the flag was runtime only, so
// a Communion whose last round fell on the first round back never crashed.
func TestBloomCrash_SurvivesARelog(t *testing.T) {
	seedBloomTick(t)
	c := characters.New()
	require.NoError(t, c.AddConditionScaled(bloomCommunionId, 1.0))
	u := bloomUser(t, c)
	tickAt(bloomTestStart)
	require.True(t, u.Character.BloomHadCommunion)

	u = bloomUser(t, relog(t, u.Character))
	require.True(t, u.Character.BloomHadCommunion, "the flag is saved")
	require.True(t, u.Character.HasCondition(bloomCommunionId), "the high is saved too")
	// The round tick ran before this listener and took the last of the high.
	u.Character.RemoveCondition(bloomCommunionId)
	u.Character.Conditions.Prune()
	require.False(t, u.Character.HasCondition(bloomCommunionId))

	queued, _ := tickAt(bloomTestStart + 1)
	assert.True(t, queued[bloomCrashId], "the high ended, so the crash comes")
}

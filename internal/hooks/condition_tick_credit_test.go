package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #240: a damage-over-time tick harms in its caster's name, so a tick that
// kills queues the death with the caster as killer, and a tick on a mob
// credits the caster in the damage map every mob-death consumer reads
// (Death_MobKillCredit, MobDeath_FactionRep, MobDeath_QuestNotify, the
// bounty claim, item procs).

// poisonMobWithCaster gives the fixture's Skeleton (mob 100, 50 health) the
// spell dot at amount per round, cast by caster.
func poisonMobWithCaster(t *testing.T, amount int, caster state.ActorRef) *mobs.Mob {
	t.Helper()
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	require.NoError(t, mob.Character.AddConditionMagnitudeBy(conditions.ConditionIdPoisoned, 3, -float64(amount), "spell", caster))
	events.DrainQueuedCharacterDiedForTest()
	return mob
}

func TestDotTick_OnAMobCreditsThePlayerCaster(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()
	mob := poisonMobWithCaster(t, 7, state.ActorRef{UserId: 1})

	tickMobConditions(mob, 100)

	assert.Equal(t, map[int]int{1: 7}, mob.Character.PlayerDamage, "the caster is credited with the tick")
}

func TestDotTick_ALethalTickOnAMobNamesTheCasterAsKiller(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()
	mob := poisonMobWithCaster(t, 80, state.ActorRef{UserId: 1})

	tickMobConditions(mob, 100)

	died := events.DrainQueuedCharacterDiedForTest()
	require.Len(t, died, 1)
	assert.Equal(t, 1, died[0].KillerUserId)
	assert.Equal(t, map[int]int{1: 80}, mob.Character.PlayerDamage)
}

// R10: a caster who has logged out is still credited; the damage map and the
// killer ref are keyed by user id and need no live record.
func TestDotTick_AnOfflineCasterIsStillCredited(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()
	require.Nil(t, users.GetByUserId(77), "user 77 must be offline for this test")
	mob := poisonMobWithCaster(t, 80, state.ActorRef{UserId: 77})

	tickMobConditions(mob, 100)

	died := events.DrainQueuedCharacterDiedForTest()
	require.Len(t, died, 1)
	assert.Equal(t, 77, died[0].KillerUserId)
	assert.Equal(t, map[int]int{77: 80}, mob.Character.PlayerDamage)
}

// A charmed mob's dot credits the player who charmed it, as its spell damage
// does (creditSpellDamage); a free mob's credits no player but is still the
// killer.
func TestDotTick_AMobCasterCreditsItsCharmerOrNobody(t *testing.T) {
	for _, charmed := range []bool{true, false} {
		cleanup := seedAllRegistries()
		restore := conditions.SeedConditionRecordsForTest()
		casterMob := &mobs.Mob{MobId: 2, InstanceId: 300, Character: characters.Character{Name: "Merchant", RoomId: 1}}
		if charmed {
			casterMob.Character.Charmed = characters.NewCharm(2, -1, "")
		}
		restoreMobs := mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{100: mobs.GetInstance(100), 300: casterMob})
		mob := poisonMobWithCaster(t, 80, state.ActorRef{MobInstanceId: 300})

		tickMobConditions(mob, 100)

		died := events.DrainQueuedCharacterDiedForTest()
		require.Len(t, died, 1)
		assert.Equal(t, 300, died[0].KillerMobInstanceId)
		if charmed {
			assert.Equal(t, map[int]int{2: 80}, mob.Character.PlayerDamage, "the charmer is credited")
		} else {
			assert.Empty(t, mob.Character.PlayerDamage, "a free mob credits no player")
		}
		restoreMobs()
		restore()
		cleanup()
	}
}

// A record with no caster (a hazard, a potion, a mob caster lost to a
// restart, which the load strips) harms anonymously, as before.
func TestDotTick_NoCasterHarmsAnonymously(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()
	mob := poisonMobWithCaster(t, 80, state.ActorRef{})

	tickMobConditions(mob, 100)

	died := events.DrainQueuedCharacterDiedForTest()
	require.Len(t, died, 1)
	assert.Zero(t, died[0].KillerUserId)
	assert.Zero(t, died[0].KillerMobInstanceId)
	assert.Empty(t, mob.Character.PlayerDamage)
}

// The player tick passes the caster too: a dot kill on a player names its
// caster, which the bounty resolver reads (PlayerDeath_BountyResolve).
func TestDotTick_ALethalTickOnAPlayerNamesTheCaster(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()
	victim := users.GetByUserId(2)
	victim.Character.HealthMax.Value = 20
	victim.Character.Health = 20
	require.NoError(t, victim.Character.AddConditionMagnitudeBy(conditions.ConditionIdPoisoned, 3, -50, "spell", state.ActorRef{UserId: 1}))
	events.DrainQueuedCharacterDiedForTest()

	UserRoundTick(events.NewRound{RoundNumber: 1})

	var killers []int
	for _, d := range events.DrainQueuedCharacterDiedForTest() {
		if d.UserId == 2 {
			killers = append(killers, d.KillerUserId)
		}
	}
	assert.Equal(t, []int{1}, killers)
}

package actions

// U6b Task 15 — steal and plant leave the sqrt regime.
//
// The attacker was (Dex + sqrt-curve x25) times a now-deleted steal-specific
// global balance knob — a sqrt regime TIMES a tuning knob — and every
// defender was RAW Perception with the skill term at x0 (including the fourth
// steal contest, the container observer pass). These tests discriminate the
// new linear shapes from the old ones: each scenario would resolve the OTHER
// way under the dead regime.
//
// There is deliberately no crit-tier coverage: steal/plant outcomes are
// caught/unseen, not damage (see stealVictimScore).

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/stretchr/testify/require"
)

// pinStealLinearKnobs pins the two knobs the linear-shape arithmetic below
// depends on: ContestFloor 0 (no ±1-in-8 sentinel outcomes muddying the
// statistics) and SkillWeight 5 (the shipped value, explicit so the score
// arithmetic in each test's comment stays true).
func pinStealLinearKnobs(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.ContestFloor = 0
	c.Balance.SkillWeight = 5.0
	configs.SetConfigForTest(t, c)
}

// TestSteal_AttackerScoreIsLinearRankTimesSkillWeight replaces the sqrt-regime
// pinning: Dex 5 + rank 100 scores 5 + 100x5 = 505 against a Perception-200
// mob. Under the dead shape the same attacker scored (5 + 3.0x25) x 1.0 = 80
// and lost this matchup nearly every time; linear, they win it nearly every
// time.
func TestSteal_AttackerScoreIsLinearRankTimesSkillWeight(t *testing.T) {
	pinStealLinearKnobs(t)
	const instId = 9931
	const trials = 20

	target := newStealTestMob(instId, 50, 200) // Per 200, no skullduggery
	mobs.SetInstanceForTest(instId, target)
	defer mobs.SetInstanceForTest(instId, nil)

	actor := newStealPlayerActor(5, 100) // 5 + 100x5 = 505

	succeeded := 0
	for i := 0; i < trials; i++ {
		target.Character.Gold = 50
		mobs.SetInstanceForTest(instId, target)
		delete(actor.char.Cooldowns, skills.Skullduggery.String("steal"))
		if Steal(actor, StealOptions{TargetMobInstanceId: instId}).Succeeded {
			succeeded++
		}
	}

	require.Greater(t, succeeded, trials*3/4,
		"a rank-100 thief (505 linear) must beat Perception 200; under the dead "+
			"sqrt-x25-times-knob regime this attacker scored ~80 and lost")
}

// TestSteal_DefenderSkullduggeryIsTheCounterCraft: the victim's skullduggery
// now enters the contest at xSkillWeight (it was x0). A Perception-5 mob with
// skullduggery 200 defends at 5 + 200x5 = 1005 against a 110+2x5 = 120 thief
// and catches them nearly every time; before Task 15 the same mob defended at
// raw 5 and was robbed blind.
func TestSteal_DefenderSkullduggeryIsTheCounterCraft(t *testing.T) {
	pinStealLinearKnobs(t)
	const instId = 9932
	const trials = 20

	target := newStealTestMob(instId, 50, 5)
	target.Character.Skills = map[string]int{string(skills.Skullduggery): 200}
	mobs.SetInstanceForTest(instId, target)
	defer mobs.SetInstanceForTest(instId, nil)

	actor := newStealPlayerActor(110, 2)

	detected := 0
	for i := 0; i < trials; i++ {
		target.Character.Gold = 50
		mobs.SetInstanceForTest(instId, target)
		delete(actor.char.Cooldowns, skills.Skullduggery.String("steal"))
		if Steal(actor, StealOptions{TargetMobInstanceId: instId}).Detected {
			detected++
		}
	}

	require.Greater(t, detected, trials*3/4,
		"a skullduggery-200 victim (1005) must catch a 120-score thief; the "+
			"defender's skill entered the contest at x0 before Task 15")
}

// TestSteal_ContainerObserverPassScoresLinear converts the FOURTH steal
// contest (steal.go's container observer pass): the best observer used to be
// picked and scored on RAW Perception, so a Perception-1 observer could never
// catch anyone regardless of skill. It now scores stealVictimScore, so a
// Perception-1, skullduggery-200 observer (1005) catches a 120-score thief
// nearly every time.
func TestSteal_ContainerObserverPassScoresLinear(t *testing.T) {
	pinStealLinearKnobs(t)
	const instId = 9933
	const trials = 20

	actor := newStealPlayerActor(110, 2)
	room := actor.room
	room.Containers = map[string]rooms.Container{
		"chest": {Gold: 100},
	}

	observer := newStealTestMob(instId, 0, 1) // Per 1 — blind under the old shape
	observer.Character.Skills = map[string]int{string(skills.Skullduggery): 200}
	mobs.SetInstanceForTest(instId, observer)
	defer mobs.SetInstanceForTest(instId, nil)
	room.AddMob(instId)
	defer room.RemoveMob(instId)

	detected := 0
	for i := 0; i < trials; i++ {
		room.Containers["chest"] = rooms.Container{Gold: 100}
		delete(actor.char.Cooldowns, skills.Skullduggery.String("steal"))
		if Steal(actor, StealOptions{ContainerNoun: "chest"}).Detected {
			detected++
		}
	}

	require.Greater(t, detected, trials*3/4,
		"the container observer pass must score Perception + skullduggery x "+
			"SkillWeight; on raw Perception this observer (Per 1) could never spot anyone")
}

// TestPlant_AttackerScoreIsLinearRankTimesSkillWeight: plant shares steal's
// engine and converts with it (deleting the knob without converting plant
// would not even build). Same discrimination as the steal variant.
func TestPlant_AttackerScoreIsLinearRankTimesSkillWeight(t *testing.T) {
	pinStealLinearKnobs(t)
	const instId = 9934
	const trials = 20

	target := newPlantTestMob(instId, 200) // Per 200, no skullduggery
	mobs.SetInstanceForTest(instId, target)
	defer mobs.SetInstanceForTest(instId, nil)

	actor := newPlantPlayerActor(5, 100) // 505 linear; ~80 under the dead regime

	planted := 0
	for i := 0; i < trials; i++ {
		actor.char.Items = nil
		seedPlantItem(actor)
		delete(actor.char.Cooldowns, skills.Skullduggery.String("steal"))
		if Plant(actor, PlantOptions{TargetMobInstanceId: instId, ItemNoun: "!1"}).Succeeded {
			planted++
		}
	}

	require.Greater(t, planted, trials*3/4,
		"a rank-100 planter (505 linear) must beat Perception 200; the sqrt "+
			"regime scored this attacker ~80")
}

// TestPlant_DefenderSkullduggeryIsTheCounterCraft: plant victims gain the same
// skullduggery x SkillWeight defence as steal victims (it was x0).
func TestPlant_DefenderSkullduggeryIsTheCounterCraft(t *testing.T) {
	pinStealLinearKnobs(t)
	const instId = 9935
	const trials = 20

	target := newPlantTestMob(instId, 5)
	target.Character.Skills = map[string]int{string(skills.Skullduggery): 200}
	mobs.SetInstanceForTest(instId, target)
	defer mobs.SetInstanceForTest(instId, nil)

	actor := newPlantPlayerActor(110, 2)

	detected := 0
	for i := 0; i < trials; i++ {
		actor.char.Items = nil
		seedPlantItem(actor)
		delete(actor.char.Cooldowns, skills.Skullduggery.String("steal"))
		if Plant(actor, PlantOptions{TargetMobInstanceId: instId, ItemNoun: "!1"}).Detected {
			detected++
		}
	}

	require.Greater(t, detected, trials*3/4,
		"a skullduggery-200 mark (1005) must catch a 120-score planter; the "+
			"defender's skill entered the contest at x0 before Task 15")
}

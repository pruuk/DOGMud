package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Empathic Shroud is a real hide (#444, owner 2026-10-09): while it hides a
// character, the spell's score replaces Dexterity plus Skullduggery in every
// opposed roll, and only one hide holds, the stronger.

func seedShroudRecords(t *testing.T) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {ConditionId: 9, Name: "Hidden",
			Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
		conditions.ConditionIdEmpathicShroud: {ConditionId: conditions.ConditionIdEmpathicShroud, Name: "Empathic Shroud",
			TriggerCount: 16, RoundInterval: 1,
			Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
	}))
}

// shroudedChar is a character the shroud hides at score, with Dexterity dex
// on Base (the add door validates, which recalculates ValueAdj from Base) and
// stamina to pay for a sneak.
func shroudedChar(t *testing.T, dex int, score float64) *characters.Character {
	t.Helper()
	c := newTestChar()
	c.Stats.Dexterity.Base = dex
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, score, "spell"))
	require.True(t, c.HiddenByShroud(), "fixture: the shroud must hide the character")
	// After the add: its Validate recalculates the pool maxima.
	c.StaminaMax.Value = 100
	c.Stamina = 100
	return c
}

func TestCalcSneakScore_ShroudScoreReplacesDexAndSkullduggery(t *testing.T) {
	seedShroudRecords(t)
	c := shroudedChar(t, 40, 250)
	require.Less(t, c.SneakBaseScore(), 250.0, "fixture: the shroud must differ from the sneak base")
	assert.Equal(t, 250.0, CalcSneakScore(c, false), "dark room, no light carried: the bare base")

	plain := newTestChar()
	plain.Stats.Dexterity.Base = 40
	require.NoError(t, plain.Validate())
	assert.Equal(t, plain.SneakBaseScore(), CalcSneakScore(plain, false), "an unshrouded hider keeps Dex plus Skullduggery")
}

func TestSneak_WhileShroudedStrongerShroudIsAlreadyHidden(t *testing.T) {
	seedShroudRecords(t)
	c := shroudedChar(t, 40, 250)
	stamina := c.Stamina

	result := Sneak(newStubActor(c, newTestRoom()))

	assert.True(t, result.AlreadyHidden)
	assert.False(t, result.ReplacedShroud)
	assert.True(t, c.HiddenByShroud(), "the stronger shroud stays")
	assert.Equal(t, stamina, c.Stamina, "a refusal costs nothing")
}

func TestSneak_WhileShroudedStrongerSneakTakesOver(t *testing.T) {
	seedShroudRecords(t)
	c := shroudedChar(t, 300, 100)

	result := Sneak(newStubActor(c, newTestRoom()))

	assert.True(t, result.Success)
	assert.True(t, result.ReplacedShroud)
	assert.False(t, result.AlreadyHidden)
	assert.False(t, result.RollHappened, "already hidden: no roll, so no practice")
	assert.Equal(t, characters.CostPaid, result.Cost.Status, "the normal sneak cost applies")
	assert.Positive(t, result.Cost.Charged)
	assert.True(t, c.IsHidden(), "no reveal")
	assert.False(t, c.HiddenByShroud())
	assert.False(t, c.Conditions.HasCondition(conditions.ConditionIdEmpathicShroud), "31 is discarded, not cancelled")
	assert.Equal(t, true, c.GetMiscData("sneaking"))
}

// Steal and plant roll the shroud's score while the shroud hides the thief
// (owner, 2026-10-09), in the theft contest and in keeping a stolen bauble
// out of sight; a visible thief keeps Dexterity plus Skullduggery.
func TestTheftScores_ShroudScoreWhileShroudHidden(t *testing.T) {
	seedShroudRecords(t)
	full := configs.GetConfig()
	full.Balance.StealHiddenBonus = 25
	configs.SetConfigForTest(t, full)
	cfg := configs.GetBalanceConfig()
	room := newSearchTestRoom(9702)

	plain := newTestChar()
	plain.Stats.Dexterity.Base = 40
	plain.SetSkill("skullduggery", 4)
	require.NoError(t, plain.Validate())
	sneakBase := float64(plain.Stats.Dexterity.ValueAdj) + 4*float64(cfg.SkillWeight)
	assert.Equal(t, sneakBase*messaging.SightMult(plain, room), newTheftAttempt(plain, room, cfg).points,
		"a visible thief is unchanged")
	assert.Equal(t, sneakBase, carrierScore(plain), "a visible carrier is unchanged")

	shrouded := shroudedChar(t, 40, 250)
	shrouded.SetSkill("skullduggery", 4)
	require.NotEqual(t, 250.0, shrouded.SneakBaseScore(), "fixture: the shroud must differ from the sneak base")
	want := 250 + float64(cfg.StealHiddenBonus)
	assert.Equal(t, want*messaging.SightMult(shrouded, room), newTheftAttempt(shrouded, room, cfg).points,
		"the shroud's score, plus the hidden bonus")
	assert.Equal(t, want, carrierScore(shrouded))
}

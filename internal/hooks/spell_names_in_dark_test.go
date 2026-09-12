package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Aliceia (1) casts on Bobrick (2); both stand in room 1.

func castHealOnBobrick(t *testing.T) (caster, target []string) {
	t.Helper()
	spell := &spells.SpellData{SpellId: "heal", Name: "Heal", EffectType: "heal", EffectMagnitude: 3}
	applyPlayerEffect(users.GetByUserId(1), users.GetByUserId(2), rooms.LoadRoom(1), spell, 3, spellContestAttackWin())
	return drainPlain(1), drainPlain(2)
}

func TestCrossCastHeal_InTheDarkNobodyIsNamed(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	drainPlain(1)
	drainPlain(2)

	caster, target := castHealOnBobrick(t)
	assert.Equal(t, 1, countContaining(target, "Something's Heal envelops you in healing energy."))
	assert.Equal(t, 0, countContaining(target, "Aliceia"))
	assert.Equal(t, 1, countContaining(caster, "You weave restorative magic around something."))
	assert.Equal(t, 0, countContaining(caster, "Bobrick"))
}

func TestCrossCastHeal_InfraredTargetReadsAFigure(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	require.True(t, users.GetByUserId(2).Character.Buffs.AddBuff(heatEyesBuffId, true))
	drainPlain(1)
	drainPlain(2)

	_, target := castHealOnBobrick(t)
	assert.Equal(t, 1, countContaining(target, "A figure's Heal envelops you in healing energy."))
}

func TestCrossCastHeal_LitRoomIsUnchanged(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	drainPlain(1)
	drainPlain(2)

	caster, target := castHealOnBobrick(t)
	assert.Equal(t, 1, countContaining(target, "Aliceia's Heal envelops you in healing energy."))
	assert.Equal(t, 1, countContaining(caster, "You weave restorative magic around Bobrick."))
}

func TestCrossCastDamage_TargetInTheDarkReadsSomething(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "sparks", Name: "Sparks", EffectType: "damage", DamageMultiplier: 0.8, BaseFolds: 4}
	applyPlayerEffect(users.GetByUserId(1), users.GetByUserId(2), rooms.LoadRoom(1), spell, 10, spellContestAttackWin())
	target := drainPlain(2)
	assert.Equal(t, 1, countContaining(target, "Something's Sparks strikes you!"))
	assert.Equal(t, 0, countContaining(target, "Aliceia"))
}

func TestSpellDefence_DefenderInTheDarkIsToldWithTheAttackerHidden(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	drainPlain(1)
	drainPlain(2)

	out := combat.ChannelDefenceResult{Defended: true, DefenceType: "dodge"}
	sendSpellChannelDefenceMessages(rooms.LoadRoom(1), messaging.CategorySpellVital, out,
		"Aliceia", "Bobrick", "Hex", users.GetByUserId(1), users.GetByUserId(2))

	defender := drainPlain(2)
	assert.Equal(t, 1, countContaining(defender, "You withstand something's Hex."),
		"a defender who cannot see used to be told nothing at all")
	assert.Equal(t, 0, countContaining(defender, "Aliceia"))
	assert.Equal(t, 1, countContaining(drainPlain(1), "Something withstands your Hex."))
}

func TestMobCastOnPlayer_TargetInTheDarkReadsSomething(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	drainPlain(2)
	original := runSpellChannelAttack
	runSpellChannelAttack = func(combat.AttackChannel, combat.AttackSide, *characters.Character, *characters.Character) combat.ChannelDefenceResult {
		return spellContestAttackWin()
	}
	t.Cleanup(func() { runSpellChannelAttack = original })

	spell := &spells.SpellData{SpellId: "test-hex", Name: "Hex", Type: spells.HarmSingle}
	resolveMobSpellAgainstPlayer(mobs.GetInstance(100), users.GetByUserId(2), rooms.LoadRoom(1), spell, combat.AttackSide{}, 10)

	target := drainPlain(2)
	assert.Equal(t, 1, countContaining(target, "Something's Hex takes effect on you."))
	assert.Equal(t, 0, countContaining(target, "Skeleton"))
}

func TestPurgeAffliction_CrossCastInTheDarkNamesNobody(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	drainPlain(1)
	drainPlain(2)

	bob := users.GetByUserId(2)
	resolvePurgeAffliction(users.GetByUserId(1), rooms.LoadRoom(1),
		purgeTarget{char: bob.Character, user: bob, name: bob.Character.Name})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Something purges the afflictions from your body."))
	assert.Equal(t, 1, countContaining(drainPlain(1), "You direct purging energy towards something."))
}

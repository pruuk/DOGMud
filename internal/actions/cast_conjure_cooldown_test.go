package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GoMudEngine/GoMud/internal/spells"
)

// Corpse-free summons take their own `conjure` cooldown key instead of the
// shared `special-move` one.
//
// Measured in U10b-0 Phase D: conjure-water costs 30 CP, has difficulty 15 and
// waitrounds 2, and needs NO corpse and NO target. On the shared 4-round key it
// therefore ran at the special-move ceiling of 225 casts/hour, standing still,
// outside a sanctuary -- and `dismiss` has no cooldown and costs no slot, so the
// summon/dismiss loop was free. That made manifestation the easiest track in the
// game to grind, not the hardest.
//
// The discriminator is SummonMobId > 0 && !SummonRequiresCorpse. That is exactly
// the five conjures plus summon-hive-swarm and summon-steppe-spirit. Every
// raise-* spell carries SummonMobId too but is excluded by the corpse flag, and
// charm has no SummonMobId at all, so both stay ordinary combat actions on the
// shared budget.

func seedSummonSpell(spellId string, mobId int, requiresCorpse bool) (*spells.SpellData, func()) {
	sd := &spells.SpellData{
		SpellId:              spellId,
		Name:                 "Test " + spellId,
		Type:                 spells.Neutral,
		BaseFolds:            4,
		Cost:                 5,
		Schools:              []string{spells.SchoolManifestation},
		SummonMobId:          mobId,
		SummonRequiresCorpse: requiresCorpse,
	}
	cleanup := spells.SeedSpellsForTest(map[string]*spells.SpellData{spellId: sd})
	return sd, cleanup
}

func TestInitiateCast_CorpseFreeSummon_UsesConjureCooldown(t *testing.T) {
	sd, cleanup := seedSummonSpell("test-conjure", 310, false)
	defer cleanup()

	actor, char, _ := newCastActor()
	result := InitiateCast(actor, sd.SpellId, "")
	require.False(t, result.OnCooldown, "a fresh cast must not be blocked")

	assert.Greater(t, char.Cooldowns["conjure"], 0,
		"a corpse-free summon must consume the dedicated conjure cooldown")
	assert.Zero(t, char.Cooldowns["special-move"],
		"a corpse-free summon must NOT consume a combat special-move slot")
}

func TestInitiateCast_CorpseBoundRaise_StaysOnSpecialMove(t *testing.T) {
	sd, cleanup := seedSummonSpell("test-raise", 301, true)
	defer cleanup()

	actor, char, _ := newCastActor()
	result := InitiateCast(actor, sd.SpellId, "")
	require.False(t, result.OnCooldown)

	assert.Greater(t, char.Cooldowns["special-move"], 0,
		"raise spells are already corpse-bound and stay on the shared budget")
	assert.Zero(t, char.Cooldowns["conjure"],
		"raise must not take the conjure key")
}

func TestInitiateCast_NonSummonManifestation_StaysOnSpecialMove(t *testing.T) {
	// charm: manifestation school, needs no corpse, but has NO SummonMobId.
	sd := &spells.SpellData{
		SpellId:   "test-charm",
		Name:      "Test charm",
		Type:      spells.HarmSingle,
		BaseFolds: 4,
		Cost:      5,
		Schools:   []string{spells.SchoolManifestation},
	}
	cleanup := spells.SeedSpellsForTest(map[string]*spells.SpellData{sd.SpellId: sd})
	defer cleanup()

	actor, char, _ := newCastActor()
	_ = InitiateCast(actor, sd.SpellId, "")

	assert.Zero(t, char.Cooldowns["conjure"],
		"charm is not a summon and must not take the conjure key -- gating it "+
			"on a 36-round cooldown would gut a combat spell")
}

// A conjure already on cooldown must be refused, and must not silently fall
// back to the shared key.
func TestInitiateCast_ConjureOnCooldown_Refused(t *testing.T) {
	sd, cleanup := seedSummonSpell("test-conjure-cd", 313, false)
	defer cleanup()

	actor, char, _ := newCastActor()
	char.Cooldowns["conjure"] = 5

	result := InitiateCast(actor, sd.SpellId, "")

	assert.True(t, result.OnCooldown, "a conjure on cooldown must be refused")
	assert.False(t, result.Initiated)
	assert.Zero(t, char.Cooldowns["special-move"],
		"a refused conjure must not consume a special-move slot as a fallback")
}

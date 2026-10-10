package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// boilingBloodId is Blood Boil's own condition (#249), the id its spell YAML
// names in condition_ids. The spec below mirrors
// _datafiles/world/dogmud/conditions/143-boiling_blood.yaml.
const boilingBloodId = 143

// seedBoilingBlood replaces the condition registry with 143 and adds the
// shipped records (121 Poisoned, 122 Bleeding) on top. Call it after
// t.Cleanup(seedAllRegistries()) so the restores run in reverse.
func seedBoilingBlood(t *testing.T) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		boilingBloodId: {ConditionId: boilingBloodId, Name: "Boiling Blood",
			TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10,
			Flags:    []conditions.Flag{conditions.Bleeding},
			TickPool: "health", TickFromMagnitude: true,
			StartActorText:  "You set the blood in {actee}'s veins boiling.",
			StartUserText:   "{actor}'s spell sets your blood boiling in your veins!",
			StartRoomText:   "{actor}'s spell sets {actee}'s blood boiling.",
			TriggerUserText: `<ansi fg="red">Your blood boils and seeps from your skin!</ansi>`,
			EndUserText:     "Your blood cools at last and stops seeping."},
	}))
	t.Cleanup(conditions.SeedConditionRecordsForTest())
}

func bloodBoilTestSpell() *spells.SpellData {
	return &spells.SpellData{SpellId: "blood-boil", Name: "Blood Boil",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetSingle,
		EffectType: "dot", EffectMagnitude: 40, BaseFolds: 6, ConditionIds: []int{boilingBloodId}}
}

// Neural Toxin names no condition, so it keeps 121 Poisoned and its trio.
func neuralToxinTestSpell() *spells.SpellData {
	return &spells.SpellData{SpellId: "neural-toxin", Name: "Neural Toxin",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle,
		EffectType: "dot", EffectMagnitude: 30, BaseFolds: 5}
}

// castOnBobrick has Aliceia (1) cast spell on Bobrick (2) in lit room 1, with
// Orin (3) watching, and returns what each of the three read.
func castOnBobrick(t *testing.T, spell *spells.SpellData) (caster, holder, room []string) {
	t.Helper()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90) // pin fully lit
	drainCastParties()
	u1, u2, r := users.GetByUserId(1), users.GetByUserId(2), rooms.LoadRoom(1)
	applySpellEffect(newSpellEffectCtx(u1.Character, actions.NewUserActorInRoom(u1, r),
		actions.NewUserActorInRoom(u2, r), r, spell, 10, spellContestAttackWin()))
	return drainPlain(1), drainPlain(2), drainPlain(3)
}

// #249: Blood Boil landed 121 Poisoned, so it read as poison everywhere. It
// lands its own 143, and each audience reads that record's start line once,
// in place of the spell's generic "afflicts" trio.
func TestBloodBoil_LandsBoilingBloodAndTellsEachAudienceOnce(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	seedBoilingBlood(t)

	caster, holder, room := castOnBobrick(t, bloodBoilTestSpell())

	target := users.GetByUserId(2).Character
	recs := target.GetConditions(boilingBloodId)
	require.Len(t, recs, 1, "Blood Boil lands Boiling Blood")
	assert.Less(t, recs[0].TickAmount, 0, "and it harms")
	assert.Empty(t, target.GetConditions(conditions.ConditionIdPoisoned), "not Poisoned")

	assert.Equal(t, 1, countContaining(caster, "You set the blood in Bobrick's veins boiling."), "%v", caster)
	assert.Equal(t, 1, countContaining(holder, "Aliceia's spell sets your blood boiling in your veins!"), "%v", holder)
	assert.Equal(t, 1, countContaining(room, "Aliceia's spell sets Bobrick's blood boiling."), "%v", room)
	for _, lines := range [][]string{caster, holder, room} {
		assert.Zero(t, countContaining(lines, "afflicts"), "the generic trio is not sent too: %v", lines)
	}
}

// Neural Toxin names no condition: it lands 121 and reads exactly as before.
func TestNeuralToxin_StillLandsPoisonedWithItsOwnLines(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	seedBoilingBlood(t)

	caster, holder, room := castOnBobrick(t, neuralToxinTestSpell())

	target := users.GetByUserId(2).Character
	require.Len(t, target.GetConditions(conditions.ConditionIdPoisoned), 1)
	assert.Empty(t, target.GetConditions(boilingBloodId))
	assert.Equal(t, 1, countContaining(caster, "Your Neural Toxin afflicts Bobrick!"), "%v", caster)
	assert.Equal(t, 1, countContaining(holder, "Aliceia's Neural Toxin afflicts you!"), "%v", holder)
	assert.Equal(t, 1, countContaining(room, "Aliceia's Neural Toxin afflicts Bobrick!"), "%v", room)
}

// A hidden mob's Blood Boil still reads "Something's spell ..." to its
// victim, as every other condition spell's result line does (owner ruling
// 2026-10-10: a hidden caster's result lines keep "Something's").
func TestBloodBoil_AHiddenMobCasterReadsSomething(t *testing.T) {
	m := litRoomOneWithHiddenSkeleton(t)
	seedBoilingBlood(t)
	pinSpellContest(t)
	drainPlain(2)

	resolveMobSpellAgainstPlayer(m, users.GetByUserId(2), rooms.LoadRoom(1), bloodBoilTestSpell(), combat.AttackSide{}, 10)

	requireUnnamed(t, drainPlain(2), "Something's spell sets your blood boiling in your veins!")
}

// A dot kill on Blood Boil credits its caster (#240 still holds on 143).
func TestBloodBoil_ALethalTickCreditsTheCaster(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	seedBoilingBlood(t)
	u1, r := users.GetByUserId(1), rooms.LoadRoom(1)
	mob := mobs.GetInstance(100)

	applySpellEffect(newSpellEffectCtx(u1.Character, actions.NewUserActorInRoom(u1, r),
		actions.NewMobActorInRoom(mob, r), r, bloodBoilTestSpell(), 80, spellContestAttackWin()))
	require.Len(t, mob.Character.GetConditions(boilingBloodId), 1)
	events.DrainQueuedCharacterDiedForTest()

	tickMobConditions(mob, 100)

	died := events.DrainQueuedCharacterDiedForTest()
	require.Len(t, died, 1)
	assert.Equal(t, 1, died[0].KillerUserId, "the caster is the killer")
	assert.Positive(t, mob.Character.PlayerDamage[1], "and is credited with the harm")
}

// The target of Blood Boil shows as bleeding, not poisoned (#249).
func TestBloodBoil_TheTargetReadsBleedingNotPoisoned(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	seedBoilingBlood(t)

	castOnBobrick(t, bloodBoilTestSpell())

	adj := users.GetByUserId(2).Character.GetAdjectives()
	assert.Contains(t, adj, "bleeding")
	assert.NotContains(t, adj, "poisoned")
}

// Purge Affliction and Cleansing Wave cured Blood Boil through 121's poison
// flag. Blood Boil's 143 is a bleed, so both purges cure it by name of the
// spells' own records, and a combat bleed (122) stays (owner call
// 2026-10-10).
func TestPurge_CuresBoilingBloodButNotACombatBleed(t *testing.T) {
	cases := map[string]func(caster, target *users.UserRecord, room *rooms.Room){
		"purge affliction": func(caster, target *users.UserRecord, room *rooms.Room) {
			resolvePurgeAffliction(caster, room, purgeTarget{char: target.Character, user: target, name: target.Character.Name})
		},
		"cleansing wave": func(caster, target *users.UserRecord, room *rooms.Room) {
			wave := &spells.SpellData{SpellId: "cleansing-wave", Name: "Cleansing Wave", EffectType: "purge"}
			applySpellEffect(newSpellEffectCtx(caster.Character, actions.NewUserActorInRoom(caster, room),
				actions.NewUserActorInRoom(target, room), room, wave, 10, spellContestAttackWin()))
		},
	}
	for name, purge := range cases {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(seedAllRegistries())
			seedBoilingBlood(t)
			t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{
				"blood-boil": bloodBoilTestSpell(), "neural-toxin": neuralToxinTestSpell(),
			}))
			caster, target, room := users.GetByUserId(1), users.GetByUserId(2), rooms.LoadRoom(1)
			require.NoError(t, target.Character.AddConditionMagnitude(boilingBloodId, 5, -4, "spell"))
			require.NoError(t, target.Character.AddConditionMagnitude(conditions.ConditionIdPoisoned, 5, -4, "spell"))
			require.NoError(t, target.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 5, -4, "rake"))

			purge(caster, target, room)

			// GetConditions, not HasCondition: a cure only expires a record
			// and leaves it for the round's prune.
			assert.Empty(t, target.Character.GetConditions(boilingBloodId), "Blood Boil is cured")
			assert.Empty(t, target.Character.GetConditions(conditions.ConditionIdPoisoned), "poison is cured, as before")
			assert.Len(t, target.Character.GetConditions(conditions.ConditionIdBleeding), 1, "a combat bleed is not")
		})
	}
}

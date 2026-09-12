package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const purgeTestPoisonBuffId = 7201

func seedPurgeTestPoison() func() {
	return buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		purgeTestPoisonBuffId: {BuffId: purgeTestPoisonBuffId, Name: "Test Venom", RoundInterval: 1, TriggerCount: 5,
			Flags: []buffs.Flag{buffs.Poison}, StartUserText: "venom", EndUserText: "gone"},
	})
}

// pinSpellContest makes the mob-target loop's ONE channel contest a win for the
// duration of a test. resolveSpell contests every mob target before reaching the
// Go hook switch, and a fumble (about 2.3% of rolls) aborts the hook entirely,
// so without this pin these lanes would go red a couple of runs in a hundred
// for a reason that has nothing to do with the dispatch under test.
func pinSpellContest(t *testing.T) {
	t.Helper()
	original := runSpellChannelAttack
	runSpellChannelAttack = func(combat.AttackChannel, combat.AttackSide, *characters.Character, *characters.Character) combat.ChannelDefenceResult {
		return spellContestAttackWin()
	}
	t.Cleanup(func() { runSpellChannelAttack = original })
}

// The 5a playtest cast Purge Affliction at a charmed companion and the CASTER
// was purged: the dispatch only knew player targets. A named mob target is
// purged and narrated, and the caster is left alone.
func TestPurgeAffliction_NamedMobTargetIsPurgedNotTheCaster(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedPurgeTestPoison()
	defer restore()
	pinSpellContest(t)
	room := rooms.LoadRoom(1)
	caster := users.GetByUserId(1)
	mob := mobs.GetInstance(100)
	require.True(t, caster.Character.Buffs.AddBuff(purgeTestPoisonBuffId, false))
	require.True(t, mob.Character.Buffs.AddBuff(purgeTestPoisonBuffId, false))
	mob.Character.AddCondition(characters.ConditionPoisoned, 5, 1, "test")
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "purge-affliction", Name: "Purge Affliction", Type: spells.HelpSingle}
	resolveSpell(caster, activity.CastingData{SpellId: "purge-affliction", TargetMobInstanceIds: []int{100}}, spell, room)

	// HasBuff is NOT the probe: a purge marks the buff expired
	// (Buffs.HasFlag(flag, true) sets TriggersLeftExpired) and leaves it in the
	// list for the round sweep to collect, so HasBuff stays true for a purged
	// buff. HasFlag(_, false) skips expired buffs, which is what "no longer
	// poisoned" means.
	assert.False(t, mob.Character.Buffs.HasFlag(buffs.Poison, false), "the named mob is purged")
	assert.False(t, mob.Character.HasCondition(characters.ConditionPoisoned))
	assert.True(t, caster.Character.Buffs.HasFlag(buffs.Poison, false), "the caster keeps their own poison")
	casterLines := drainPlain(1)
	// The mob name is rendered by mobDisplayName, which appends the adjectives
	// the mob carries at the moment of the cast, so the line reads
	// "Skeleton (poisoned)" here. The name itself is what this asserts.
	assert.Equal(t, 1, countContaining(casterLines, "You direct purging energy towards Skeleton"))
	assert.Equal(t, 0, countContaining(casterLines, "from your body"))
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia directs purging energy towards Skeleton"))
}

func TestPurgeAffliction_NamedMobTargetInTheDarkIsHiddenFromTheRoom(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedPurgeTestPoison()
	defer restore()
	pinSpellContest(t)
	darken(t, 1)
	room := rooms.LoadRoom(1)
	caster := users.GetByUserId(1)
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "purge-affliction", Name: "Purge Affliction", Type: spells.HelpSingle}
	resolveSpell(caster, activity.CastingData{SpellId: "purge-affliction", TargetMobInstanceIds: []int{100}}, spell, room)

	casterLines := drainPlain(1)
	assert.Equal(t, 1, countContaining(casterLines, "You direct purging energy towards something."))
	assert.Equal(t, 0, countContaining(casterLines, "Skeleton"), "a caster who cannot see does not name the mob")
	assert.Equal(t, 0, countContaining(drainPlain(2), "purging"), "an unsighted observer reads nothing")
}

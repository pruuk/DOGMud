package hooks

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func shieldSpellForParityTest() *spells.SpellData {
	return &spells.SpellData{
		SpellId: "test-ward", Name: "Ward", AttackType: combatvocab.AttackNone,
		DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle,
		EffectType: "shield", EffectMagnitude: 75, BaseFolds: 4, PrimaryStat: "willpower",
		Schools: []string{spells.SchoolEnhancement},
	}
}

// parityShieldBonus is the shield's strength on the fixture's equalised
// caster, from the fixture's numbers: willpower 100 plus spellcasting 3
// times SkillWeight, a third of that, scaled by the spell's magnitude 75.
// Test binaries read the Go default SkillWeight, not the shipped one.
func parityShieldBonus() float64 {
	weighted := int(math.Round(3 * float64(configs.GetBalanceConfig().SkillWeight)))
	return float64(int(math.Round(float64((100+weighted)/3) * 75 / 100.0)))
}

// landQueuedConditions applies every queued condition event as the event
// loop would. A ward or a heal lands through events.Condition (messaging M6
// slice 1), so it is held only once its event is applied.
func landQueuedConditions() {
	for _, evt := range events.DrainQueuedMobConditionsForTest(0) { // zero drains every holder
		ApplyConditions(evt)
	}
}

// shieldRecord lands the queued ward and returns c's one Conviction Ward
// record (the ward a shield spell that names none lands), or fails the test.
func shieldRecord(t *testing.T, c *characters.Character) *conditions.Condition {
	t.Helper()
	landQueuedConditions()
	recs := c.GetConditions(conditions.ConditionIdConvictionWard)
	require.Len(t, recs, 1, "the shield must leave one Conviction Ward record")
	return recs[0]
}

// Slice 3b (audit row 15): a shield on a charmed pet applied nothing, since
// a mob target had no shield arm.
func TestSpellShield_APetCanBeShielded(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	f.targetMob.Character.Charm(1, -1, "")
	spell := shieldSpellForParityTest()

	resolveAgainstMob(f.casterUser, f.targetMob, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)

	rec := shieldRecord(t, &f.targetMob.Character)
	assert.Equal(t, parityShieldBonus(), rec.Magnitude)
	assert.Equal(t, calcSpellDuration(4, 3, 100), rec.TriggersLeft)
	assert.Equal(t, state.ActorRef{UserId: 1}, rec.Caster, "the ward remembers its caster")
	assert.Equal(t, []string{"A faint ward of conviction shimmers around Ghoul."}, drainPlain(3))
	assert.Equal(t, []string{"Your conviction hardens into a ward around Ghoul."}, drainPlain(1),
		"the ward's caster line is the caster's only line")
}

// Slice 3b (audit row 3): a creature's shield on a player applied nothing.
func TestSpellShield_ACreatureShieldsAPlayer(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	spell := shieldSpellForParityTest()

	resolveMobSpellAgainstPlayer(f.casterMob, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, &f.casterMob.Character, nil), spell.EffectMagnitude)

	assert.Equal(t, parityShieldBonus(), shieldRecord(t, f.targetUser.Character).Magnitude)
	assert.Equal(t, []string{"A ward of hardened conviction settles around you."}, drainPlain(2))
}

// Owner ruling 3: a shield does not crit. The player-to-player arm multiplied
// it by 1.5 on a crit no cast could reach.
func TestSpellShield_ACritChangesNothing(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	caster := actions.NewUserActorInRoom(f.casterUser, f.room)
	target := actions.NewUserActorInRoom(f.targetUser, f.room)

	applySpellEffect(newSpellEffectCtx(f.casterUser.Character, caster, target, f.room,
		shieldSpellForParityTest(), 75, spellContestAttackCrit()))

	assert.Equal(t, parityShieldBonus(), shieldRecord(t, f.targetUser.Character).Magnitude,
		"a crit must not strengthen a shield")
}

const testBulwarkId = 7221 // a second ward, physical and spell

// seedTestBulwark adds a second ward to the fixture's registry, keeping the
// fixture's condition 100 and the condition records (119 among them).
func seedTestBulwark(t *testing.T) {
	t.Helper()
	mag := conditions.EffectValue{UsesMagnitude: true}
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		100: conditions.GetConditionSpec(100),
		testBulwarkId: {ConditionId: testBulwarkId, Name: "Test Bulwark", Family: conditions.FamilyWard,
			RoundInterval: 1, TriggerCount: 10,
			Effects: map[conditions.EffectKind]conditions.EffectValue{
				conditions.EffectMitigationFlat: mag, conditions.EffectMitigationMagical: mag},
			StartActorText: "A bulwark rises around {actee}.", StartUserText: "A bulwark rises around you.",
			StartRoomText: "A bulwark rises around {actee_plain}.", EndUserText: "Your Test Bulwark fades."},
	}))
	t.Cleanup(conditions.SeedConditionRecordsForTest())
}

// R3 and R7: a shield spell lands its own ward, named by its condition_ids,
// and that ward's kinds all read the one shield strength.
func TestSpellShield_LandsTheSpellsOwnWard(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedTestBulwark(t)
	spell := shieldSpellForParityTest()
	spell.ConditionIds = []int{testBulwarkId}

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)
	landQueuedConditions()

	c := f.targetUser.Character
	require.True(t, c.HasCondition(testBulwarkId))
	assert.False(t, c.HasCondition(conditions.ConditionIdConvictionWard))
	assert.Equal(t, parityShieldBonus(), c.Conditions.Effect(conditions.EffectMitigationFlat))
	assert.Equal(t, parityShieldBonus(), c.Conditions.Effect(conditions.EffectMitigationMagical))
	assert.Zero(t, c.Conditions.Effect(conditions.EffectMitigationConviction), "a ward blocks only what it lists")
}

// R4 and R6: a second ward replaces the first, announced to the holder.
func TestSpellShield_ASecondWardReplacesTheFirst(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedTestBulwark(t)
	ward := shieldSpellForParityTest()
	bulwark := shieldSpellForParityTest()
	bulwark.ConditionIds = []int{testBulwarkId}

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, ward,
		spellAttackSideFor(ward, f.casterUser.Character, nil), ward.EffectMagnitude)
	landQueuedConditions()
	drainPlain(2)
	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, bulwark,
		spellAttackSideFor(bulwark, f.casterUser.Character, nil), bulwark.EffectMagnitude)
	landQueuedConditions()

	c := f.targetUser.Character
	assert.False(t, c.HasCondition(conditions.ConditionIdConvictionWard))
	assert.True(t, c.HasCondition(testBulwarkId))
	assert.Equal(t, []string{"Your Conviction Ward fades as Test Bulwark takes hold.", "A bulwark rises around you."}, drainPlain(2))
}

// A re-cast of a ward already held is a refresh: the ward's start lines stay
// quiet, so the spell's own lines tell each audience it was renewed.
func TestSpellShield_ARecastOfTheSameWardKeepsTheSpellsLines(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	spell := shieldSpellForParityTest()
	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)
	landQueuedConditions()
	drainPlain(1)
	drainPlain(2)

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)
	landQueuedConditions()

	assert.Equal(t, []string{"A shimmering magical barrier forms around you, bolstering your defenses."}, drainPlain(2))
}

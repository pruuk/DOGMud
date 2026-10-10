package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func massMendSpellForAreaTest() *spells.SpellData {
	return &spells.SpellData{
		SpellId: "test-mass-mend", Name: "Mass Mend", AttackType: combatvocab.AttackNone,
		DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetArea,
		EffectType: "heal", EffectMagnitude: 3, BaseFolds: 4, PrimaryStat: "willpower",
		Schools: []string{spells.SchoolVital},
	}
}

// holdsRegen lands the queued heals (a heal travels the condition event,
// messaging M6 slice 1) and reports whether c holds the Regenerating record.
func holdsRegen(c *characters.Character) bool {
	landQueuedConditions()
	return len(c.GetConditions(conditions.ConditionIdRegenerating)) > 0
}

// addRatForAreaTest puts a third mob, Rat (instance 102), in the fixture room.
func addRatForAreaTest(t *testing.T, f *spellParityFixture) *mobs.Mob {
	t.Helper()
	rat := &mobs.Mob{MobId: 3, InstanceId: 102, HomeRoomId: 1, Character: characters.Character{
		Name: "Rat", RoomId: 1, MobInstanceId: 102,
		Conditions: conditions.New(), Cooldowns: map[string]int{},
		Position: position.NewMachine(), CombatPhase: combatphase.NewMachine(),
	}}
	equaliseSpellCombatant(&rat.Character)
	mobs.SetInstanceForTest(102, rat)
	t.Cleanup(func() { mobs.SetInstanceForTest(102, nil) })
	f.room.AddMob(102)
	return rat
}

// partyForAreaTest makes Aliceia (1) lead a party Bobrick (2) has joined;
// Carys (3) stays outside it.
func partyForAreaTest(t *testing.T) {
	t.Helper()
	p := parties.New(1)
	require.NotNil(t, p)
	require.True(t, p.InvitePlayer(2))
	require.True(t, p.AcceptInvite(2))
	t.Cleanup(p.Disband)
}

// Slice 3b (audit row 13, owner ruling 2026-09-28): a player's area heal
// lands on every player in the room and on mobs charmed by the caster or a
// party member, not on a stranger's pet. It used to take any charmed mob.
func TestHelpArea_PlayerHealsThePartysCompanionsNotAStrangersPet(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	partyForAreaTest(t)
	rat := addRatForAreaTest(t, f)
	rat.Character.Charm(1, -1, "")         // Aliceia's own pet
	f.targetMob.Character.Charm(2, -1, "") // Bobrick's companion, a party member's
	f.casterMob.Character.Charm(3, -1, "") // Carys's pet; Carys is not in the party
	spell := massMendSpellForAreaTest()

	resolveSpell(f.casterUser, activity.CastingData{SpellId: spell.SpellId}, spell, f.room)

	assert.True(t, holdsRegen(&rat.Character), "the caster's own pet is healed")
	assert.True(t, holdsRegen(&f.targetMob.Character), "a party member's companion is healed")
	assert.False(t, holdsRegen(&f.casterMob.Character), "a stranger's pet is not")
	for _, u := range []*users.UserRecord{f.casterUser, f.targetUser, f.watcher} {
		assert.True(t, holdsRegen(u.Character), "%s is a player in the room", u.Character.Name)
	}
}

// Messaging M6 slice 1 (#338): an area heal that names a heal with authored
// start lines lands it on every player in the room, caster included, and each
// audience reads exactly one line per target: the caster the start_actor line
// for each other target and its own holder line for itself, each target its
// holder line, everyone else the observer line. No generic trio line is sent.
// A help area heal reaches every player in the room, so the third player is
// both a target and the observer of the other two.
func TestHelpArea_AnAreaHealTellsEachAudienceOncePerTarget(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedTestHeals(t)
	spell := massMendSpellForAreaTest()
	spell.ConditionIds = []int{testGentleHealId}

	resolveSpell(f.casterUser, activity.CastingData{SpellId: spell.SpellId}, spell, f.room)
	landQueuedConditions()

	for _, u := range []*users.UserRecord{f.casterUser, f.targetUser, f.watcher} {
		assert.Len(t, u.Character.GetConditions(testGentleHealId), 1, "%s holds the heal once", u.Character.Name)
	}
	caster, bobrick, carys := drainPlain(1), drainPlain(2), drainPlain(3)
	assert.ElementsMatch(t, []string{
		"A gentle heal settles on you.",
		"A gentle heal settles on Bobrick.",
		"A gentle heal settles on Carys.",
	}, caster, "the caster: its own holder line, an actor line per other target")
	assert.ElementsMatch(t, []string{
		"A gentle heal settles on you.",
		"A gentle heal settles on Aliceia.",
		"A gentle heal settles on Carys.",
	}, bobrick, "a target: its holder line, an observer line per other target")
	assert.ElementsMatch(t, []string{
		"A gentle heal settles on you.",
		"A gentle heal settles on Aliceia.",
		"A gentle heal settles on Bobrick.",
	}, carys, "a target: its holder line, an observer line per other target")
	for _, lines := range [][]string{caster, bobrick, carys} {
		for _, line := range lines {
			assert.NotContains(t, line, "Mass Mend", "the spell's own trio is not sent")
			assert.NotContains(t, line, "warm glow")
			assert.NotContains(t, line, "restorative")
			assert.NotContains(t, line, "envelops")
		}
	}
}

// An uncharmed mob's area heal lands on itself and its packmates, the mobs
// its own AI would heal (mobs.FindPackmatesInRoom), and never on a player or
// a mob outside the pack.
func TestHelpArea_AMobHealsItsPackAndNoPlayer(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	rat := addRatForAreaTest(t, f)
	f.casterMob.Routine = "crypt"
	f.targetMob.Routine = "crypt"
	spell := massMendSpellForAreaTest()

	// InitiateCast fills a mob's area help with every player and mob in the
	// room (actions/cast.go); resolution must narrow it.
	resolveMobSpell(f.casterMob, activity.CastingData{SpellId: spell.SpellId,
		TargetUserIds: []int{1, 2, 3}, TargetMobInstanceIds: []int{100, 101, 102}}, spell, f.room)

	assert.True(t, holdsRegen(&f.casterMob.Character), "the caster heals itself")
	assert.True(t, holdsRegen(&f.targetMob.Character), "a packmate is healed")
	assert.False(t, holdsRegen(&rat.Character), "a mob outside the pack is not")
	for _, u := range []*users.UserRecord{f.casterUser, f.targetUser, f.watcher} {
		assert.False(t, holdsRegen(u.Character), "%s is not the mob's ally", u.Character.Name)
	}
}

// A charmed mob stands on its owner's side: its area heal lands as its
// owner's would, and on itself; a stranger's pet is left out. This is the
// companion case: the AI companion is charmed to its owner permanently, so
// its own area heal reaches its owner.
func TestHelpArea_ACharmedMobHealsItsOwnersSide(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	partyForAreaTest(t)
	rat := addRatForAreaTest(t, f)
	f.casterMob.Character.Charm(1, -1, "") // Aliceia's pet casts
	f.targetMob.Character.Charm(2, -1, "") // Bobrick's companion
	rat.Character.Charm(3, -1, "")         // Carys's pet
	spell := massMendSpellForAreaTest()

	resolveMobSpell(f.casterMob, activity.CastingData{SpellId: spell.SpellId,
		TargetUserIds: []int{1, 2, 3}, TargetMobInstanceIds: []int{100, 101, 102}}, spell, f.room)

	assert.True(t, holdsRegen(&f.casterMob.Character), "the caster heals itself")
	assert.True(t, holdsRegen(&f.targetMob.Character), "the party's companion is healed")
	assert.False(t, holdsRegen(&rat.Character), "a stranger's pet is not on the owner's side")
	assert.True(t, holdsRegen(f.casterUser.Character), "the owner is healed")
}

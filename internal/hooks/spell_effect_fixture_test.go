package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// spellRecordCall is one captured recordSpell call.
type spellRecordCall struct {
	src, tgt                    combat.SourceTarget
	hit, crit, backfire, fizzle bool
	dmg                         int
}

// spellParityFixture puts two players, a watcher and two mobs in lit room 1,
// every combatant with the same stats and spell skill, so a spell cast
// through any pairing must land the same amount (parity slice 3a).
type spellParityFixture struct {
	room       *rooms.Room
	casterUser *users.UserRecord // Aliceia, user 1
	targetUser *users.UserRecord // Bobrick, user 2
	watcher    *users.UserRecord // Carys, user 3: reads only room lines
	casterMob  *mobs.Mob         // Skeleton, instance 100
	targetMob  *mobs.Mob         // Ghoul, instance 101
	records    []spellRecordCall
}

// newSpellParityFixture seeds the registries, pins the ONE spell contest to
// out, and captures every recordSpell call. Every restore is a t.Cleanup,
// so they run last-in first-out after the test body.
func newSpellParityFixture(t *testing.T, out combat.ChannelDefenceResult) *spellParityFixture {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	room := roomForCollapseTest(t)

	u1, u2 := users.GetByUserId(1), users.GetByUserId(2)
	u3 := users.NewTestUser(3, "cara", "Carys", 1003)
	u3.Character.RoomId = 1
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: u1, 2: u2, 3: u3}))
	room.AddPlayer(3)

	ghoul := &mobs.Mob{MobId: 2, InstanceId: 101, HomeRoomId: 1, Character: characters.Character{
		Name: "Ghoul", RoomId: 1, MobInstanceId: 101,
		Conditions: conditions.New(), Cooldowns: map[string]int{},
		Position: position.NewMachine(), CombatPhase: combatphase.NewMachine(),
	}}
	mobs.SetInstanceForTest(101, ghoul)
	room.AddMob(101)

	skeleton := mobs.GetInstance(100)
	skeleton.Character.MobInstanceId = 100

	f := &spellParityFixture{room: room, casterUser: u1, targetUser: u2, watcher: u3,
		casterMob: skeleton, targetMob: ghoul}
	for _, c := range []*characters.Character{u1.Character, u2.Character, &skeleton.Character, &ghoul.Character} {
		equaliseSpellCombatant(c)
	}

	originalContest := runSpellChannelAttack
	runSpellChannelAttack = func(messaging.RoomVisibility, combatvocab.Attack, combat.AttackSide,
		*characters.Character, *characters.Character) combat.ChannelDefenceResult {
		return out
	}
	t.Cleanup(func() { runSpellChannelAttack = originalContest })

	originalRecord := recordSpell
	recordSpell = func(src, tgt combat.SourceTarget, hit, crit, backfire, fizzle bool, dmg int,
		_ float64, _, _ *characters.Character, _ uint64) {
		f.records = append(f.records, spellRecordCall{src: src, tgt: tgt, hit: hit, crit: crit,
			backfire: backfire, fizzle: fizzle, dmg: dmg})
	}
	t.Cleanup(func() { recordSpell = originalRecord })

	for _, id := range []int{1, 2, 3} {
		drainPlain(id)
	}
	events.DrainQueuedPlayerAttackedMobsForTest(0)
	// A ward or a heal lands through the condition queue (messaging M6 slice
	// 1); an earlier test's leftover event must not land in this one.
	events.DrainQueuedMobConditionsForTest(0) // zero drains every holder
	return f
}

// equaliseSpellCombatant gives a player or a mob the same six stats, the same
// spellcasting rank, full conviction and a deep health pool.
func equaliseSpellCombatant(c *characters.Character) {
	c.Stats.Strength.ValueAdj = 100
	c.Stats.Dexterity.ValueAdj = 100
	c.Stats.Perception.ValueAdj = 100
	c.Stats.Vitality.ValueAdj = 100
	c.Stats.Willpower.ValueAdj = 100
	c.Stats.Charisma.ValueAdj = 100
	c.SetSkill(string(skills.Spellcasting), 3)
	c.Health = 1000
	c.HealthMax.Value = 1000
	c.Conviction = 50
	c.ConvictionMax.Value = 50
}

func dotSpellForParityTest() *spells.SpellData {
	return &spells.SpellData{
		SpellId: "test-blight", Name: "Blight", AttackType: combatvocab.AttackSpell,
		DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle,
		EffectType: "dot", EffectMagnitude: 10, BaseFolds: 6, PrimaryStat: "willpower",
		Schools: []string{spells.SchoolVital},
	}
}

func knockdownSpellForParityTest() *spells.SpellData {
	return &spells.SpellData{
		SpellId: "test-shove", Name: "Shove", AttackType: combatvocab.AttackSpell,
		DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetSingle,
		EffectType: "knockdown", DamageMultiplier: 0.5, EffectMagnitude: 20, PrimaryStat: "willpower",
		Schools: []string{spells.SchoolElemental},
	}
}

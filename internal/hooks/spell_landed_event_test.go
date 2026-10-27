package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #375 phase 3b slice 3: resolveSpell raises spell_landed once per target a
// player's spell landed on, from its own per-target verdicts, under one
// guard, so a trigger fires at most once per cast.

const (
	spellLandedProbeId   = 999252
	spellLandedProbeGold = 7
	spellLandedStoneId   = 999253
)

// listenForSpellLanded registers a one-trigger quest on the global engine
// (restored at cleanup) that grants the probe's start token and pays
// spellLandedProbeGold when trig fires. It returns how many times the trigger
// has fired, read from the gold paid, so a double fire shows as 2.
func listenForSpellLanded(t *testing.T, user *users.UserRecord, trig questengine.TriggerDef) func() int {
	t.Helper()
	restore := questengine.ResetEngineForTest()
	t.Cleanup(restore)
	const token = "999252-start"
	trig.Event = "spell_landed"
	trig.Actions = []questengine.ActionDef{{Grant: token}, {GiveGold: spellLandedProbeGold}}
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId:  spellLandedProbeId,
		Name:     "spell landed probe",
		Steps:    []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{trig},
	})
	user.Character.ClearQuestToken(token)
	t.Cleanup(func() { user.Character.ClearQuestToken(token) })
	startGold := user.Character.Gold
	return func() int {
		fires := (user.Character.Gold - startGold) / spellLandedProbeGold
		if fires > 0 {
			require.True(t, user.Character.HasQuest(token), "the gold was paid but the grant did not land")
		}
		return fires
	}
}

// forceSpellContest pins the one channel contest to out for the test.
func forceSpellContest(t *testing.T, out combat.ChannelDefenceResult) {
	t.Helper()
	original := runSpellChannelAttack
	runSpellChannelAttack = func(messaging.RoomVisibility, combatvocab.Attack, combat.AttackSide, *characters.Character, *characters.Character) combat.ChannelDefenceResult {
		return out
	}
	t.Cleanup(func() { runSpellChannelAttack = original })
}

// spellLandedScene seeds the shared registries with room 1 pinned lit and
// returns the caster (user 1), the room and Skeleton instance 100 (template 1).
func spellLandedScene(t *testing.T) (*users.UserRecord, *rooms.Room, *mobs.Mob) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(90) // pin fully lit; see combat_blind_warning_test.go
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Character.MobInstanceId = mob.InstanceId
	return users.GetByUserId(1), room, mob
}

func testHealSpell() *spells.SpellData {
	return &spells.SpellData{
		SpellId: "test-mend", Name: "Test Mend",
		AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle,
		EffectType: "heal", EffectMagnitude: 10,
		Schools: []string{spells.SchoolVital},
	}
}

func testBoltSpell() *spells.SpellData {
	return &spells.SpellData{
		SpellId: "test-bolt", Name: "Test Bolt",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetMulti,
		EffectType: "damage", EffectMagnitude: 5, DamageMultiplier: 1.0,
		Schools: []string{spells.SchoolMental},
	}
}

// Case 1: a heal that reaches a mob of template 1 satisfies `effect: heal, mob: 1`.
func TestResolveSpell_HealOnAMobRaisesSpellLanded(t *testing.T) {
	caster, room, mob := spellLandedScene(t)
	fired := listenForSpellLanded(t, caster, questengine.TriggerDef{Effect: "heal", Mob: int(mob.MobId)})

	resolveSpell(caster, activity.CastingData{SpellId: "test-mend", TargetMobInstanceIds: []int{mob.InstanceId}}, testHealSpell(), room)

	require.Equal(t, 1, fired(), "a heal that reached mob template %d must raise spell_landed once", mob.MobId)
}

// Case 2: a harm spell the mob defended did not land, so `noun: <spell>` stays
// unfired; the same trigger fires once the contest is a hit (the control that
// proves the probe can see a fire at all).
func TestResolveSpell_DefendedHarmRaisesNoSpellLanded(t *testing.T) {
	caster, room, mob := spellLandedScene(t)
	mob.Character.Health = 1000
	mob.Character.HealthMax.Value = 1000
	fired := listenForSpellLanded(t, caster, questengine.TriggerDef{Noun: "test-bolt"})
	cast := activity.CastingData{SpellId: "test-bolt", TargetMobInstanceIds: []int{mob.InstanceId}}

	forceSpellContest(t, combat.ChannelDefenceResult{Defence: combatvocab.DefenceQuell, Defended: true, DefensiveCrit: true})
	resolveSpell(caster, cast, testBoltSpell(), room)
	require.Equal(t, 0, fired(), "a defended cast landed on nothing and must raise no spell_landed")

	forceSpellContest(t, spellContestAttackWin())
	resolveSpell(caster, cast, testBoltSpell(), room)
	require.Equal(t, 1, fired(), "control: the same trigger fires when the bolt hits")
}

// Case 3: one cast that hits two mobs of the same template fires the trigger
// once, not once per target.
func TestResolveSpell_HarmOnTwoMobsOfOneTemplateFiresOnce(t *testing.T) {
	caster, room, first := spellLandedScene(t)
	first.Character.Health = 1000
	first.Character.HealthMax.Value = 1000
	secondValue := *first
	second := &secondValue
	second.InstanceId = 101
	second.Character.MobInstanceId = second.InstanceId
	specValue := *first
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{int(first.MobId): &specValue}, map[int]*mobs.Mob{
		first.InstanceId:  first,
		second.InstanceId: second,
	}))
	room.AddMob(second.InstanceId)
	forceSpellContest(t, spellContestAttackWin())
	fired := listenForSpellLanded(t, caster, questengine.TriggerDef{Noun: "test-bolt", Mob: int(first.MobId)})

	resolveSpell(caster, activity.CastingData{SpellId: "test-bolt", TargetMobInstanceIds: []int{first.InstanceId, second.InstanceId}}, testBoltSpell(), room)

	require.Less(t, first.Character.Health, 1000, "fixture: the bolt must hit the first mob")
	require.Less(t, second.Character.Health, 1000, "fixture: the bolt must hit the second mob")
	require.Equal(t, 1, fired(), "two landings of one cast fire the trigger once")
}

// Case 4: a self-cast heal arrives as a player target that is the caster, so
// `target: self, effect: heal` fires.
func TestResolveSpell_SelfCastHealRaisesSpellLandedOnSelf(t *testing.T) {
	caster, room, _ := spellLandedScene(t)
	caster.Character.HealthMax.Value = 100
	caster.Character.Health = 10
	fired := listenForSpellLanded(t, caster, questengine.TriggerDef{Target: quests.TargetSelf, Effect: "heal"})

	resolveSpell(caster, activity.CastingData{SpellId: "test-mend", TargetUserIds: []int{caster.UserId}}, testHealSpell(), room)

	require.Equal(t, 1, fired(), "a self-cast heal must raise spell_landed aimed at self")
}

// Case 5: identify has no creature target; a cast that identified an item
// raises one event aimed at nothing, which `noun: identify` matches. A cast
// that named nothing the caster carries identified nothing and raises none.
func TestResolveSpell_IdentifyRaisesSpellLandedWhenItIdentifies(t *testing.T) {
	caster, room, _ := spellLandedScene(t)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		spellLandedStoneId: {ItemId: spellLandedStoneId, Name: "dull stone"},
	}))
	fired := listenForSpellLanded(t, caster, questengine.TriggerDef{Noun: "identify"})
	identify := &spells.SpellData{
		SpellId: "identify", Name: "Identify",
		AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSelf,
		EffectType: "identify",
	}

	resolveSpell(caster, activity.CastingData{SpellId: "identify", SpellRest: "stone"}, identify, room)
	require.Equal(t, 0, fired(), "identify on an item the caster does not carry identified nothing")

	require.True(t, caster.Character.StoreItem(items.New(spellLandedStoneId)), "fixture: store the stone")
	resolveSpell(caster, activity.CastingData{SpellId: "identify", SpellRest: "stone"}, identify, room)
	require.Equal(t, 1, fired(), "identify that identified an item must raise spell_landed")
}

// captureSpellLanded replaces the spell_landed seam for the test and returns
// the targets of every event resolveSpell raised, one slice per cast.
func captureSpellLanded(t *testing.T) *[][]questengine.Target {
	t.Helper()
	var casts [][]questengine.Target
	original := notifySpellLanded
	notifySpellLanded = func(_ *users.UserRecord, _ int, _, _ string, targets ...questengine.Target) questengine.NotifyResult {
		casts = append(casts, append([]questengine.Target(nil), targets...))
		return questengine.NotifyResult{}
	}
	t.Cleanup(func() { notifySpellLanded = original })
	return &casts
}

func goHookSpell(id string) *spells.SpellData {
	return &spells.SpellData{SpellId: id, Name: id, AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle}
}

// fold-recall is a single-target help spell: cast with no argument it carries
// the caster in TargetUserIds, and the player loop resolves that uncontested.
// A recall with no anchor moved nobody, so it landed on nothing.
func TestResolveSpell_FoldRecallWithNoAnchorRaisesNoSpellLanded(t *testing.T) {
	caster, room, _ := spellLandedScene(t)
	pinSpellContest(t)
	casts := captureSpellLanded(t)
	caster.Character.SetMiscData("fold-anchor-room", nil)

	resolveSpell(caster, activity.CastingData{SpellId: "fold-recall", TargetUserIds: []int{caster.UserId}}, goHookSpell("fold-recall"), room)

	require.Len(t, *casts, 1, "fixture: one cast, one notify call")
	require.Empty(t, (*casts)[0], "a recall that found no anchor landed on nothing")
}

func TestResolveSpell_FoldRecallWithAnAnchorRaisesOneSelfEvent(t *testing.T) {
	caster, room, _ := spellLandedScene(t)
	pinSpellContest(t)
	casts := captureSpellLanded(t)
	caster.Character.SetMiscData("fold-anchor-room", 2)
	t.Cleanup(func() { caster.Character.SetMiscData("fold-anchor-room", nil) })

	resolveSpell(caster, activity.CastingData{SpellId: "fold-recall", TargetUserIds: []int{caster.UserId}}, goHookSpell("fold-recall"), room)

	require.Equal(t, 2, caster.Character.RoomId, "fixture: the recall must move the caster")
	require.Equal(t, [][]questengine.Target{{questengine.TargetSelf()}}, *casts, "a recall that moved the caster lands once, on self")
}

// Purge on a named mob lands once, on that mob, from the hook's own branch.
func TestResolveSpell_PurgeOnANamedMobRaisesOneMobEvent(t *testing.T) {
	caster, room, mob := spellLandedScene(t)
	pinSpellContest(t)
	casts := captureSpellLanded(t)

	resolveSpell(caster, activity.CastingData{SpellId: "purge-affliction", TargetMobInstanceIds: []int{mob.InstanceId}}, goHookSpell("purge-affliction"), room)

	require.Equal(t, [][]questengine.Target{{questengine.TargetMob(mob)}}, *casts, "purge on a mob lands once, on the mob")
}

// Charm resolves inside the mob loop, where `landed` is the contest verdict.
// A won contest on a charm-immune mob charms nothing, so it must not count.
func TestResolveSpell_CharmThatDidNotTakeRaisesNoSpellLanded(t *testing.T) {
	caster, room, mob := spellLandedScene(t)
	mob.CharmImmune = true
	forceSpellContest(t, spellContestAttackWin())
	fired := listenForSpellLanded(t, caster, questengine.TriggerDef{Effect: "charm"})
	charm := &spells.SpellData{
		SpellId: "test-charm", Name: "Test Charm",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamageSocial, Targeting: combatvocab.TargetSingle,
		EffectType: "charm",
		Schools:    []string{spells.SchoolMental},
	}

	resolveSpell(caster, activity.CastingData{SpellId: "test-charm", TargetMobInstanceIds: []int{mob.InstanceId}}, charm, room)
	require.False(t, mob.Character.IsCharmed(caster.UserId), "fixture: an immune mob is not charmed")
	require.Equal(t, 0, fired(), "a charm that did not take landed on nothing")

	// The control needs a caster whose pool can carry the charm reserve; see
	// charmedRounds and charmDurationTestConfig for why a default one is refused.
	charmDurationTestConfig(t)
	caster.Character.Stats.Charisma.Base = 150
	caster.Character.Stats.Charisma.Recalculate()
	caster.Character.Stats.Willpower.Base = 150
	caster.Character.Stats.Willpower.Recalculate()
	caster.Character.RecalculateStats()
	mob.CharmImmune = false
	resolveSpell(caster, activity.CastingData{SpellId: "test-charm", TargetMobInstanceIds: []int{mob.InstanceId}}, charm, room)
	require.True(t, mob.Character.IsCharmed(caster.UserId), "fixture: control charm must take")
	require.Equal(t, 1, fired(), "control: a charm that took raises spell_landed")
}

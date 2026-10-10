package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// A heal cast at your own companion used to run a quell contest: the
// companion could "defend" the heal, a fumble backfired on you, and a
// defensive crit handed the companion a counter-swing at its owner
// (spell_resolution.go:406 on master 612b85d54 ran the seam unconditionally,
// and an absent target_defense_type routed to spell-mental). Non-harm casts
// take the uncontested path now, for mob targets as they always did for
// player targets.
func TestNonHarmCastAtAMobRunsNoContest(t *testing.T) {
	contests := 0
	restore := combat.SetChannelAttackContestRunnerForTest(func(atk float64, entries []contest.Entry) contest.Result {
		contests++
		return contest.Result{}
	})
	t.Cleanup(restore)

	cleanupConditions := conditions.SeedConditionRecordsForTest()
	defer cleanupConditions()

	const roomId = 8830
	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()
	user := users.NewTestUser(1, "caster", "Cala", 1001)
	user.Character.RoomId = roomId
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: user})
	defer restoreUsers()
	room.AddPlayer(user.UserId)
	mob := charmTestMob(t, 8831, roomId)
	mobs.SetInstanceForTest(mob.InstanceId, mob)
	defer mobs.SetInstanceForTest(mob.InstanceId, nil)
	room.AddMob(mob.InstanceId)
	heal := &spells.SpellData{
		SpellId: "test-heal", Name: "Test Heal", PrimaryStat: "willpower",
		AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle,
		EffectType: "heal", EffectMagnitude: 10,
	}
	fumbled, landed := resolveAgainstMob(user, mob, room, heal, spellAttackSideFor(heal, user.Character, nil), heal.EffectMagnitude)

	if contests != 0 {
		t.Fatalf("a non-harm cast ran %d contest(s) against a mob", contests)
	}
	if fumbled || !landed {
		t.Errorf("fumbled=%v landed=%v; an uncontested cast lands and cannot fumble", fumbled, landed)
	}
	// applySpellHeal applies a regenerating condition rather than an
	// instant heal (spell_help_effects.go); confirm the effect actually
	// applied rather than asserting on Health, which the applier never
	// touches directly. It travels the condition event, so it is landed
	// first (messaging M6 slice 1).
	landQueuedConditions()
	if !mob.Character.HasCondition(conditions.ConditionIdRegenerating) {
		t.Error("the heal did not apply: no regenerating condition on the mob")
	}
}

// The harm path must still contest, or the test above proves nothing.
func TestHarmCastAtAMobStillRunsOneContest(t *testing.T) {
	contests := 0
	restore := combat.SetChannelAttackContestRunnerForTest(func(atk float64, entries []contest.Entry) contest.Result {
		contests++
		return contest.Result{Success: true}
	})
	t.Cleanup(restore)

	const roomId = 8832
	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()
	user := users.NewTestUser(1, "caster", "Cala", 1001)
	user.Character.RoomId = roomId
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: user})
	defer restoreUsers()
	room.AddPlayer(user.UserId)
	mob := charmTestMob(t, 8833, roomId)
	mobs.SetInstanceForTest(mob.InstanceId, mob)
	defer mobs.SetInstanceForTest(mob.InstanceId, nil)
	room.AddMob(mob.InstanceId)
	bolt := &spells.SpellData{
		SpellId: "test-bolt", Name: "Test Bolt", PrimaryStat: "willpower",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle,
		EffectType: "damage", DamageMultiplier: 1,
	}
	resolveAgainstMob(user, mob, room, bolt, spellAttackSideFor(bolt, user.Character, nil), 0)
	if contests != 1 {
		t.Fatalf("a harm cast ran %d contest(s), want exactly 1", contests)
	}
}

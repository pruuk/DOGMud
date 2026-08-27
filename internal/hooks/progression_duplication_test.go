package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func countUses(c *characters.Character) (strength, dexterity, weapon int) {
	return c.GetStatUseCount("strength"),
		c.GetStatUseCount("dexterity"),
		c.GetSkillUseCount("weapon-combat")
}

// TestMeleeProgressionFiresOncePerRound pins the intended contract: ONE
// strength track and ONE weapon-combat track per attacker per round.
//
// U9: melee progression was confirmed (via this test, RED before the fix) to
// fire twice per round against the same actors — once from combat.Attack*vs*
// / trackMobAttackProgression (phase 2) and once from applyCombatProgression
// (phase 5, the unified orchestrator's dedicated progression phase). Phase 2
// was deleted; phase 5 is now the single melee progression path. This test
// stays as a permanent regression guard so the duplication cannot come back.
func TestMeleeProgressionFiresOncePerRound(t *testing.T) {
	atk, def := newCombatPairForTest(t)

	beforeStr, beforeDex, beforeWeapon := countUses(atk.GetCharacter())

	runOneCombatRoundForTest(t, atk, def)

	afterStr, afterDex, afterWeapon := countUses(atk.GetCharacter())

	if got := afterWeapon - beforeWeapon; got != 1 {
		t.Errorf("weapon-combat tracked %d times in one round, want 1", got)
	}

	// U10b-1 Task 22 INVERTED the two assertions below, and this test now
	// guards the opposite of what it used to.
	//
	// It previously required strength == 1 and dexterity == EXACTLY 2, and its
	// own comment said dexterity was "tracked directly AND as weapon-combat's
	// primary stat, so the intended count is exactly 2". That double-roll came
	// from emitAttackerStatGain, which called OnStatUse directly for strength
	// and dexterity every round, at full weight, win or lose, on top of the
	// round's award.
	//
	// Owner ruling: a stat roll IS progression, so a full-weight one that
	// ignores the outcome is a firing-rule violation exactly like a skill roll
	// would be. Both bare rolls are deleted.
	//
	// What remains is ONE dexterity track, as weapon-combat's primary inside
	// the award, and NO strength track at all -- strength is not any melee
	// skill's primary, so an attacker no longer trains it merely by swinging.
	if got := afterDex - beforeDex; got != 1 {
		t.Errorf("dexterity tracked %d times in one round, want exactly 1 (the award's primary-stat roll); a second means emitAttackerStatGain's bare roll is back", got)
	}
	if got := afterStr - beforeStr; got != 0 {
		t.Errorf("strength tracked %d times in one round, want 0; swinging no longer trains strength directly", got)
	}
}

// A defender who dodges four swings in a round takes ONE dodge progression
// event, not five. combat_helpers.go's sendDefenseMessages rolled per defended
// swing while processDefenderProgression rolled once per round.
func TestMeleeDefenceProgressionFiresOncePerRound(t *testing.T) {
	atk, def := newDualWieldingCombatPairForTest(t)
	before := def.GetCharacter().GetSkillUseCount("unarmed-combat")

	runOneCombatRoundAllDodgedForTest(t, atk, def)

	if got := def.GetCharacter().GetSkillUseCount("unarmed-combat") - before; got != 1 {
		t.Errorf("defender dodge tracked %d times in one round, want 1", got)
	}
}

// TestMeleeCritReceived_TracksDefenderVitality is Task 15b's end-to-end proof
// that routing melee's crit-received event through characters.ApplyProgression
// did not silently drop its use-count tracking.
//
// applyCombatProgression has built a progression.Outcome{ToughenStat:
// characters.ToughenStatFor("physical"), ...} and handed it to the seam since
// U9 -- that part was never in question. What Task 15b's applyBonusProgression
// change fixes is that the seam's bonus tier used to apply a BLANKET
// "bonus events never track" rule to every Class, including ClassObserved (the
// defender's crit-RECEIVED event). That silently regressed pre-seam
// OnCritReceived, which always tracked before rolling (see the comment on
// applyBonusProgression). Without tracking, vitality's virtual rank never
// moves regardless of how many crits a defender has eaten -- the exact fyttyn
// exploit shape U9 was supposed to have closed.
//
// This drives the FULL production path (handleCombatRound -> ... ->
// applyCombatProgression -> characters.ApplyProgression), not the applier in
// isolation (that half is pinned directly in
// internal/characters/progression_apply_test.go), so a regression in the
// wiring between melee and the applier would also be caught here.
func TestMeleeCritReceived_TracksDefenderVitality(t *testing.T) {
	cleanupCfg := enableMobProgression(t)
	defer cleanupCfg()

	atk, def := newCombatPairForTest(t)
	before := def.GetCharacter().GetStatUseCount("vitality")

	runOneCombatRoundForTest(t, atk, def) // forceCrit=true: guarantees res.Crit

	if got := def.GetCharacter().GetStatUseCount("vitality") - before; got != 1 {
		t.Errorf("defender vitality use count changed by %d in one forced-crit round, want 1 -- "+
			"crit-received progression must track through the seam", got)
	}
}

// newDualWieldingCombatPairForTest builds a player attacker vs. mob defender
// (PvM quadrant), like newCombatPairForTest, except the attacker wields two
// separate one-handed weapons (main hand + offhand) instead of a single
// two-handed one. That gives collectAttackWeapons two entries instead of one,
// so calculateCombat resolves two independent swing contests this round
// instead of one -- the minimum needed to distinguish "fires once per
// defended swing" from "fires once per round".
//
// The defender (mob 100) is left unarmed (no Equipment.Weapon), so
// combat.DefenceEntriesFor returns dodge only -- IsUnarmedStyle() is true and
// parry/block never enter the contest. Its Dexterity is set absurdly high so
// dodge overwhelms the attacker's score regardless of the RNG: RunContest's
// per-swing floor is pinned to 0 by runOneCombatRoundAllDodgedForTest, so
// nothing but the ~2.3% self-relative fumble chance (see dice.RollResult.ZScore
// docs -- fixed regardless of stat magnitude) can keep either swing from being
// defended.
func newDualWieldingCombatPairForTest(t *testing.T) (atk actions.Actor, def actions.Actor) {
	t.Helper()

	cleanupRegistries := seedAllRegistries()
	t.Cleanup(cleanupRegistries)

	cleanupSpecies := species.SeedSpeciesForTest(map[int]*species.Species{
		1: {SpeciesId: 1, Name: "Human", UnarmedName: "fist"},
	})
	t.Cleanup(cleanupSpecies)

	cleanupAtkMsgs := items.SeedAttackMessagesForTest(items.MinimalCombatMessageFixture())
	t.Cleanup(cleanupAtkMsgs)

	u1 := users.GetByUserId(1)
	require.NotNil(t, u1)
	u1.Character.SpeciesId = 1
	// Two one-handed weapons: collectAttackWeapons produces one entry per
	// slot (unlike a 2H weapon, which suppresses the offhand entry via
	// IsBlockedBy2H), so this round resolves two independent swings.
	u1.Character.Equipment.Weapon = items.Item{ItemId: 30, Spec: &items.ItemSpec{Type: items.Weapon, Hands: 1, Subtype: items.Stabbing}}
	u1.Character.Equipment.Offhand = items.Item{ItemId: 31, Spec: &items.ItemSpec{Type: items.Weapon, Hands: 1, Subtype: items.Stabbing}}

	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.SpeciesId = 1
	m.Character.HealthMax.Value = 100000
	m.Character.Health = 100000
	// Overwhelming dodge advantage. Equipment.Weapon stays zero-value, so
	// IsUnarmedStyle() is true and dodge is the only defence candidate.
	m.Character.Stats.Dexterity.ValueAdj = 1000000
	m.Character.Skills = map[string]int{"unarmed-combat": 100}

	room1 := rooms.LoadRoom(1)
	require.NotNil(t, room1)

	atk = actions.NewUserActorInRoom(u1, room1)
	def = actions.NewMobActorInRoom(m, room1)

	atk.GetCharacter().SetAggro(0, m.InstanceId, characters.DefaultAttack)

	return atk, def
}

// runOneCombatRoundAllDodgedForTest drives exactly one round through the
// unified combat orchestrator with forceCrit=false (unlike
// runOneCombatRoundForTest's forced hit) and ContestFloor pinned to 0 so the
// defender's overwhelming dodge score decides every swing deterministically
// instead of a 12.5% floor rescuing the attacker on about one run in eight.
// See pinContestFloorOff in internal/combat/run_contest_test.go for the same
// pattern; that helper is unexported to package combat, so it is duplicated
// here rather than imported.
func runOneCombatRoundAllDodgedForTest(t *testing.T, atk, def actions.Actor) {
	t.Helper()

	cfg := configs.GetConfig()
	cfg.Balance.ContestFloor = 0
	configs.SetConfigForTest(t, cfg)

	var affPlayers, affMobs []int
	handleCombatRound(
		atk, def,
		events.NewRound{RoundNumber: 1},
		0, // moonMod
		&cfg,
		&affPlayers,
		&affMobs,
		false, // forceCrit: let the defender's overwhelming dodge score win every contest
	)
	events.ProcessEvents()
}

// newCombatPairForTest builds a player attacker vs. mob defender (PvM
// quadrant) ready to drive one full round through handleCombatRound.
//
// The attacker is equipped with a real two-handed weapon rather than left
// unarmed: collectAttackWeapons only produces ONE weapon entry for a 2H
// weapon (IsBlockedBy2H suppresses the usual offhand-fist entry), so
// res.WeaponHits has exactly one element and the attacker's combat skill
// tag resolves to weapon-combat instead of unarmed-combat. That keeps the
// per-round progression counts attributable to a single weapon/skill
// instead of being muddied by a second (offhand fist / unarmed-combat)
// attack entry.
func newCombatPairForTest(t *testing.T) (atk actions.Actor, def actions.Actor) {
	t.Helper()

	cleanupRegistries := seedAllRegistries()
	t.Cleanup(cleanupRegistries)

	// Combat lookups dereference species.GetSpecies(SpeciesId) without a nil
	// guard (see NewRound_DoCombat_routing_test.go for the same seeding).
	cleanupSpecies := species.SeedSpeciesForTest(map[int]*species.Species{
		1: {SpeciesId: 1, Name: "Human", UnarmedName: "fist"},
	})
	t.Cleanup(cleanupSpecies)

	// items.GetAttackMessage infinite-recurses when attackMessages is empty
	// (the production map is loaded by items.LoadDataFiles at startup, but
	// unit tests don't trigger that).
	cleanupAtkMsgs := items.SeedAttackMessagesForTest(items.MinimalCombatMessageFixture())
	t.Cleanup(cleanupAtkMsgs)

	u1 := users.GetByUserId(1)
	require.NotNil(t, u1)
	u1.Character.SpeciesId = 1
	// Real two-handed weapon: exactly one WeaponHits entry (no offhand
	// fist), and GetCombatSkillTag() resolves to weapon-combat.
	u1.Character.Equipment.Weapon = items.Item{ItemId: 30, Spec: &items.ItemSpec{Type: items.Weapon, Hands: 2}}
	u1.Character.Equipment.Offhand = items.Item{ItemId: 0}

	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.SpeciesId = 1
	// Plenty of health so the defender survives the forced-crit hit and
	// end-of-round death/retarget handling stays on the "both alive" path.
	m.Character.HealthMax.Value = 100000
	m.Character.Health = 100000

	room1 := rooms.LoadRoom(1)
	require.NotNil(t, room1)

	atk = actions.NewUserActorInRoom(u1, room1)
	def = actions.NewMobActorInRoom(m, room1)

	atk.GetCharacter().SetAggro(0, m.InstanceId, characters.DefaultAttack)

	return atk, def
}

// runOneCombatRoundForTest drives exactly one round through the unified
// combat orchestrator (handleCombatRound) with forceCrit=true. forceCrit
// guarantees every swing this round is a clean hit, which makes the
// progression call counts deterministic instead of depending on
// dice.RollStat's unseedable RNG.
func runOneCombatRoundForTest(t *testing.T, atk, def actions.Actor) {
	t.Helper()

	cfg := configs.GetConfig()
	var affPlayers, affMobs []int
	handleCombatRound(
		atk, def,
		events.NewRound{RoundNumber: 1},
		0, // moonMod
		&cfg,
		&affPlayers,
		&affMobs,
		true, // forceCrit: guarantees a clean hit every swing this round
	)
	events.ProcessEvents()
}

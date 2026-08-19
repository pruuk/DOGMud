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

	if got := afterStr - beforeStr; got != 1 {
		t.Errorf("strength tracked %d times in one round, want 1", got)
	}
	if got := afterWeapon - beforeWeapon; got != 1 {
		t.Errorf("weapon-combat tracked %d times in one round, want 1", got)
	}
	// Dexterity is tracked directly AND as weapon-combat's primary stat, so the
	// intended count is exactly 2. Asserting only "at most 2" would not catch
	// under-counting (e.g. a regression dropping it to 0 or 1), so pin it exactly.
	if got := afterDex - beforeDex; got != 2 {
		t.Errorf("dexterity tracked %d times in one round, want exactly 2", got)
	}
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

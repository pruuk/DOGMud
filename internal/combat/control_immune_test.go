package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

func TestExecuteSkillMove_ControlImmuneNeverKnockedDown(t *testing.T) {
	cleanup := mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"immovable": {MutationId: "immovable", Name: "Immovable", Rarity: 9,
			Pros: []mutations.MutationEffect{{Type: "flag", Target: "control-immune"}}},
	})
	defer cleanup()

	pinDefenceAdmissionConfig(t)
	pinContestFloorOff(t)

	atk := characters.New()

	// U6b Task 7: these params previously pinned the legacy scalar-defence
	// shape (AttackStat/DefenseStat at x1 skill weights, no defence set) —
	// a defect kept alive only for unconverted callers. The move now
	// resolves through the channel seam; the overwhelming AttackSide keeps
	// hits (and thus knockdown rolls) near-certain, which is all this test
	// needs to distinguish control immunity.
	params := func(def *characters.Character) SkillMoveParams {
		return SkillMoveParams{
			Attacker: atk, Defender: def,
			Channel: ChannelMelee,
			Attack: AttackSide{ // overwhelming, so attacks land
				Stat: 300, StatName: "strength",
				Skill: skills.WeaponCombat, SkillRank: 50,
				Mult: 1.0,
			},
			// KnockdownFactor is irrelevant for the control-immune assertion --
			// IsControlImmune short-circuits BEFORE the knockdown contest ever
			// runs, so no factor value can move that branch. It also has to be
			// large enough that the NORMAL defender's knockdown contest (a real
			// opposed roll under U10) lands at least once across 30 attempts.
			DamagePercent: 0.5, KnockdownFactor: 1000.0,
			DamageStat: 100,
		}
	}

	// Control-immune defender: NEVER knocked down, across many attempts.
	immune := characters.New()
	immune.HealthMax.Value = 100000
	immune.Health = 100000
	immune.Mutations = map[string]int{"immovable": 1}
	for i := 0; i < 30; i++ {
		if ExecuteSkillMove(params(immune)).KnockedDown {
			t.Fatal("control-immune defender was knocked down")
		}
	}

	// Normal defender: gets knocked down at least once (proves the mechanic
	// works and immunity is the difference).
	normal := characters.New()
	normal.HealthMax.Value = 100000
	normal.Health = 100000
	knockedAtLeastOnce := false
	for i := 0; i < 30 && !knockedAtLeastOnce; i++ {
		if ExecuteSkillMove(params(normal)).KnockedDown {
			knockedAtLeastOnce = true
		}
	}
	if !knockedAtLeastOnce {
		t.Fatal("normal defender was never knocked down — test setup can't distinguish immunity")
	}
}

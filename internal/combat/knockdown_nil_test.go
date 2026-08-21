package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These pin the nil-Position guards on the knockdown paths, the siblings of
// HandleGrappleCritFailure surfaced by the 2026-07-21 guard sweep. A
// pre-Validate() Character has no Position FSM; prod combatants are always
// Validated, so the guards only shield under-initialized test fixtures — but
// without them the (dice-gated) knockdown branches nil-panic the same way the
// grapple crit path used to.

func TestHandleDoubleFumble_NilPosition(t *testing.T) {
	src := &characters.Character{Name: "Src"} // Position intentionally nil
	tgt := &characters.Character{Name: "Tgt"} // Position intentionally nil
	require.NotPanics(t, func() {
		handleDoubleFumble(&AttackResult{}, src, tgt)
	})
}

func TestHandleDoubleFumble_BothGoProne(t *testing.T) {
	src := &characters.Character{Name: "Src", Position: position.NewMachine()}
	tgt := &characters.Character{Name: "Tgt", Position: position.NewMachine()}

	handleDoubleFumble(&AttackResult{}, src, tgt)

	assert.True(t, src.IsProne(), "source should be knocked prone by the double fumble")
	assert.True(t, tgt.IsProne(), "target should be knocked prone by the double fumble")
}

func TestExecuteSkillMove_NilDefenderPosition(t *testing.T) {
	// The knockdown branch derefs Defender.Position and is gated behind two
	// dice rolls. Loop enough to hit it essentially every run; with the guard
	// in place the call never panics regardless of whether knockdown triggers.
	// U6b Task 7: converted off the legacy scalar-defence shape (a pinned
	// defect) onto the seam. The characters are built with New() so the seam's
	// defence-set/costing/progression plumbing has real pools and maps to work
	// on; the Position under test is then EXPLICITLY nilled — the guard's
	// trigger is a nil FSM, however the character got that way.
	require.NotPanics(t, func() {
		for i := 0; i < 200; i++ {
			def := characters.New()
			def.Name = "Def"
			def.Position = nil // intentionally nil — the guard under test
			def.HealthMax.Value = 100
			def.Health = 100
			atk := characters.New()
			atk.Name = "Atk"
			ExecuteSkillMove(SkillMoveParams{
				Attacker: atk,
				Defender: def,
				Channel:  ChannelMelee,
				Attack: AttackSide{ // overwhelming attacker → near-certain hit
					Stat: 500, StatName: "strength",
					Skill: skills.WeaponCombat, SkillRank: 50,
					Mult: 1.0,
				},
				DamagePercent:   0.8,
				KnockdownFactor: 2.0, // guarantee the knockdown branch on a hit
				DamageStat:      100,
			})
		}
	})
}

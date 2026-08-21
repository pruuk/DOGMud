package actionspec

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Every registered action's governing skill must resolve to a real stat.
//
// This does not test the registry directly -- it tests that
// skills.SkillPrimaryStats covers every skill the registry names. A registry
// row whose skill has no primary stat would award a skill roll and silently no
// stat roll, which is the exact half-wiring the U9 spec warns about: it
// compiles, it plays, and it quietly denies players half their progression.
//
// SkillEquippedCombat rows are skipped because their skill is resolved from the
// actor's current weapon at call time rather than from the Spec.
func TestEveryRegisteredActionSkillHasAPrimaryStat(t *testing.T) {
	for action, spec := range registry {
		if spec.SkillSource == SkillEquippedCombat {
			continue
		}
		if spec.Skill == "" {
			t.Errorf("action %q has SkillSource %v but no Skill", action, spec.SkillSource)
			continue
		}
		if skills.GetSkillPrimaryStat(string(spec.Skill)) == "" {
			t.Errorf("action %q names skill %q, which has no entry in skills.SkillPrimaryStats",
				action, spec.Skill)
		}
	}
}

// The registry is the single table both the cost calculator and anything else
// reads. An unregistered action returns the zero Spec rather than panicking --
// deliberately, so a missing row is a flat cost that never varies with skill or
// load (findable) instead of a crash mid-combat-round (not acceptable).
func TestSpecForUnregisteredActionIsZero(t *testing.T) {
	if got := SpecFor(Action("no-such-action")); got != (Spec{}) {
		t.Errorf("SpecFor(unregistered) = %+v, want the zero Spec", got)
	}
}

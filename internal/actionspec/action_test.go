package actionspec

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/skills"
)

func TestStatFor_DefaultsToSkillPrimaryStat(t *testing.T) {
	// Dodge is registered with UnarmedCombat and no Stat override, and
	// unarmed-combat's primary stat is dexterity.
	if got := StatFor(SpecFor(ActionDodge)); got != "dexterity" {
		t.Errorf("StatFor(dodge) = %q, want %q", got, "dexterity")
	}
}

func TestStatFor_OverrideWins(t *testing.T) {
	s := Spec{Skill: skills.Spellcasting, SkillSource: SkillFixed, Stat: "charisma"}
	if got := StatFor(s); got != "charisma" {
		t.Errorf("StatFor(override) = %q, want %q", got, "charisma")
	}
}

func TestStatFor_NoSkillNoStat(t *testing.T) {
	if got := StatFor(Spec{}); got != "" {
		t.Errorf("StatFor(zero) = %q, want empty", got)
	}
}

// Every registered action must resolve to a real stat. A registry row whose
// skill has no primary stat would award a skill roll and silently no stat roll,
// which is the exact half-wiring the U9 spec warns about.
func TestEveryRegisteredActionResolvesAStat(t *testing.T) {
	for action, spec := range registry {
		if spec.SkillSource == SkillEquippedCombat {
			continue // skill is resolved from the actor's weapon at call time
		}
		if StatFor(spec) == "" {
			t.Errorf("action %q resolves no stat (skill %q)", action, spec.Skill)
		}
	}
}

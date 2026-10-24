package guards

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoFile reads a file relative to the repo root, independent of the test
// binary's CWD.
func repoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// Drift guard 1 (#301): PowerScore, consider, the AI companion and live combat
// share one set of helpers. A second copy of a swing, hit, damage or defence
// formula in any of them fails here. Consider also claims the special-move
// cooldown and trains through the shared resolved-contest award, never the
// old per-use hook.
func TestFightReadUsesTheSharedHelpers(t *testing.T) {
	type rule struct {
		file          string
		must, mustNot []string
	}
	rules := []rule{
		{"internal/combat/calculations.go",
			[]string{"meleeOffenseRaw(", "meleeAvoidance("},
			[]string{"swings :=", "addWeaponDPS", "GetDefenseScore(", "CalcRawDamage(char.Stats.Strength"}},
		{"internal/combat/fight_expect.go",
			[]string{"buildAttackPlan(", "buildDamageParams(", "calcAttackScore(", "applyPositionHitModifiers(",
				"calcCritThreshold(", "defenceEntryScore(", "SpellRawDamage(", "SpellMitigation("},
			[]string{"swings :=", "GetDefenseScoreFor("}},
		{"internal/combat/combat_helpers.go",
			[]string{"defenceEntryScore(targetChar, defenseType, isThirdParty, ctx)"}, nil},
		{"internal/hooks/combat_shared_helpers.go",
			[]string{"combat.SpellRawDamage(", "combat.SpellMitigation("},
			[]string{"spellData.CasterStatValue("}},
		{"internal/actions/consider.go",
			[]string{"ReadFight(", "ClaimSpecialMove(", "AwardResolved("},
			[]string{"PowerScore", "predictionFor", "OnSkillUse("}},
		{"modules/aicompanion/actions.go", []string{"FightReadPlain("}, []string{"considerWords", ".Ratio"}},
		{"modules/aicompanion/combat.go", []string{"worstRead("}, []string{"considerWords", ".Ratio", "worstOdds"}},
	}
	for _, r := range rules {
		src := repoFile(t, r.file)
		for _, s := range r.must {
			if !strings.Contains(src, s) {
				t.Errorf("%s no longer calls %s: the fight read and combat must share it", r.file, s)
			}
		}
		for _, s := range r.mustNot {
			if strings.Contains(src, s) {
				t.Errorf("%s contains %q: a second copy of a shared formula (#301)", r.file, s)
			}
		}
	}
}

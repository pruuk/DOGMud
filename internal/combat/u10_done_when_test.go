package combat

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// U10's "Done when" list as a test (the U6b lesson: prose criteria fail
// silently). Yaml assertions read the HEAD blob via git show — the working
// copy is skip-worktree and deliberately stale on dev machines. The
// single-reader scan is restricted to Go files, matched on the field-access
// form (a leading dot) so a comment merely NAMING the knob — such as
// combat_throttle.go's explanatory note about routing through the seam —
// cannot trip it, and documentation (context.md) is excluded outright by the
// Go-only filter.
func TestU10DoneWhen_DeadPathsStayDead(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))

	readFile := func(rel string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(b)
	}
	mustNotContain := func(content, name, needle, why string) {
		t.Helper()
		if strings.Contains(content, needle) {
			t.Errorf("%s still contains %q — %s", name, needle, why)
		}
	}

	mustNotContain(readFile("internal/characters/cast_helpers.go"),
		"cast_helpers.go", "CalcConcentrationChance",
		"the flat concentration curve was deleted by U10")
	mustNotContain(readFile("internal/combat/skill_moves.go"),
		"skill_moves.go", "RollStat(50",
		"knockdown is an opposed contest, not a threshold roll")
	mustNotContain(readFile("internal/characters/skills.go"),
		"skills.go", "RollStat(50",
		"free recovery is contested or automatic, never a solo roll")
	mustNotContain(readFile("internal/actions/combat_throttle.go"),
		"combat_throttle.go", "ThrottleInterruptChance",
		"the throttle interrupt goes through the concentration seam")

	// ConcentrationFloor single-reader rule (Done-when 2), Go files only.
	// Matched on the field-access form (".ConcentrationFloor") rather than
	// the bare word, so an explanatory comment naming the knob (e.g.
	// combat_throttle.go's note that the interrupt routes through the seam)
	// does not register as a second reader.
	out, err := exec.Command("git", "-C", repoRoot, "grep", "-l", "-E",
		`\.ConcentrationFloor`, "--", "internal", "modules").Output()
	if err != nil && len(out) == 0 {
		t.Fatalf("git grep: %v", err)
	}
	for _, f := range strings.Fields(string(out)) {
		if !strings.HasSuffix(f, ".go") {
			continue // docs may name the knob; only code is scanned
		}
		if strings.Contains(f, "configs/") || strings.HasSuffix(f, "_test.go") {
			continue
		}
		if f != "internal/combat/run_concentration_contest.go" {
			t.Errorf("unexpected ConcentrationFloor reader: %s (the seam must stay the only one)", f)
		}
	}

	// Yaml criteria against the HEAD blob (Done-when 4).
	blob, err := exec.Command("git", "-C", repoRoot, "show",
		"HEAD:_datafiles/config.yaml").Output()
	if err != nil {
		t.Fatalf("git show config.yaml: %v", err)
	}
	yaml := string(blob)
	mustNotContain(yaml, "HEAD:config.yaml", "SpellInitiationWillpowerDivisor",
		"the knob died with the curve")
	mustNotContain(yaml, "HEAD:config.yaml", "KnockdownChance",
		"threshold knobs became intended-rate factors")
	for _, key := range []string{"ConcentrationFloor:", "ConcentrationDamageThresholdPct:",
		"BashKnockdownFactor:", "TripKnockdownFactor:", "KickKnockdownFactor:"} {
		if !strings.Contains(yaml, key) {
			t.Errorf("HEAD config.yaml missing shipped knob %s", key)
		}
	}
}

package actions

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/progression"
)

// repoRootForTest resolves the repository root from this file's own location.
//
// Test binaries do NOT reliably start in the package directory: all tests share
// one binary, so a relative path passes or fails depending on which package ran
// first. Anchor on runtime.Caller instead.
//
// Duplicated per package because Go test helpers are not visible across
// packages.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

// pinConfigForTest enables progression for a test.
//
// A test binary does NOT see an all-zero config: ensureConfigValidated applies
// every <=0-idiom default on the first read. What stays false are the
// ConfigBools, and those are the two that gate progression entirely, so a test
// that forgets them asserts against a path that can never advance anything.
func pinConfigForTest(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.UseSkillProgression = true
	cfg.Balance.MobProgressionEnabled = true
	// Pinned explicitly: a Config built from scratch skips the sentinel. See
	// newUnloadedConfig in internal/configs/configs.go.
	cfg.Balance.ProgressionFailureFraction = 0.35
	configs.SetConfigForTest(t, cfg)
}

// ---------------------------------------------------------------------------
// Actor.AwardResolved recording
// ---------------------------------------------------------------------------

// recordedAward is one observed Actor.AwardResolved call.
type recordedAward struct {
	won   bool
	cands []progression.Candidate
}

// awardRecorder is the shared AwardResolved implementation for this package's
// Actor fakes. Embed it; the promoted method satisfies the interface and the
// awards slice is what tests assert on.
//
// It RECORDS rather than no-opping on purpose. U10b-1's call-site conversions
// (Tasks 8+) each need to prove which skills a site offered as candidates and
// whether it reported a win, and a bare no-op stub would force every one of
// those tasks to re-edit every fake in the package.
//
// The candidate slice is COPIED. AwardResolved takes a variadic, so a caller
// that builds candidates in a reused backing array would otherwise have its
// recorded history rewritten underneath it by the next call.
type awardRecorder struct {
	awards []recordedAward
}

func (r *awardRecorder) AwardResolved(won bool, cands ...progression.Candidate) {
	r.awards = append(r.awards, recordedAward{
		won:   won,
		cands: append([]progression.Candidate(nil), cands...),
	})
}

// awardedCandidate returns the recorded candidate for skillName across every
// award, plus how many awards offered it. Tests normally want exactly one.
func (r *awardRecorder) awardedCandidate(skillName string) (progression.Candidate, int) {
	var found progression.Candidate
	n := 0
	for _, a := range r.awards {
		for _, c := range a.cands {
			if c.Skill == skillName {
				found = c
				n++
			}
		}
	}
	return found, n
}

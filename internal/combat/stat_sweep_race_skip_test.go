package combat

import "testing"

// skipStatSweepUnderRace skips a long single-goroutine statistical sweep in
// a -race build (#536). Under -race the two melee sweeps took about 330s of
// combat's 390 to 405s in a local Docker run and pushed the package to the
// CI 900s timeout, while the race detector has nothing to find in a test that
// never starts a goroutine. They still run on every PR: the "Statistical
// sweeps" step in .github/actions/codegen-and-test/action.yml runs them
// without -race. A sweep that calls this must be named in that step's -run
// pattern, or it runs nowhere in CI.
func skipStatSweepUnderRace(t *testing.T) {
	t.Helper()
	if raceDetectorOn {
		t.Skip("single-goroutine statistical sweep; runs without -race in the CI \"Statistical sweeps\" step (#536)")
	}
}

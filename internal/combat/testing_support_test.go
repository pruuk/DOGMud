package combat

import "testing"

// setContestGapSaturationForTest overrides the configured saturation for one
// test and restores it afterwards.
//
// It lives in a _test.go file deliberately. The previous version sat beside
// RunContest and pulled "testing" into a production hot path, which the
// codebase avoids everywhere else (internal/behaviortree/test_export.go,
// internal/configs/testing_support.go, internal/species/testing_support.go).
//
// ⚠️ Tests cannot reach this knob through config.yaml: a Go test binary never
// loads it, and GetBalanceConfig runs the validators, so the field reads its
// validated default (0, identity) rather than the shipped value.
func setContestGapSaturationForTest(t *testing.T, k float64) {
	t.Helper()
	prev := contestGapSaturationOverride
	contestGapSaturationOverride = &k
	t.Cleanup(func() { contestGapSaturationOverride = prev })
}

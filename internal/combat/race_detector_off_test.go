//go:build !race

package combat

// raceDetectorOn reports whether this test binary was built with -race.
// See skipStatSweepUnderRace.
const raceDetectorOn = false

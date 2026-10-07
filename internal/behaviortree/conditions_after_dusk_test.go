package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// pinAfterDuskClock pins the shipped day (900 rounds, latitude 46.5) and
// returns the round midwinter's dusk falls in (lighting 5e, X2).
func pinAfterDuskClock(t *testing.T) uint64 {
	t.Helper()
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.NightHours = 8
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	c.Balance.WorldLatitude = 46.5
	c.Balance.Validate()
	configs.SetConfigForTest(t, c)
	gametime.ClearDateCacheForTest()
	// The celestial memo is keyed on the round alone, like the date cache;
	// period: lamplit reads it (lighting plan 6).
	gametime.ClearCelestialMemoForTest()
	prev := util.GetRoundCount()
	t.Cleanup(func() {
		util.SetRoundCountForTest(prev)
		gametime.ClearDateCacheForTest()
		gametime.ClearCelestialMemoForTest()
	})
	for r := uint64(355*900 + 450); r < 356*900; r++ {
		if gametime.GetDate(r).Night {
			return r
		}
	}
	t.Fatal("midwinter has no dusk after noon")
	return 0
}

// period: after_dusk with hours: N is true from the night boundary until N
// game hours later, then false; it is false all afternoon.
func TestCondTimeOfDay_AfterDusk(t *testing.T) {
	dusk := pinAfterDuskClock(t)
	params := map[string]any{"period": "after_dusk", "hours": 1}
	for _, c := range []struct {
		name  string
		round uint64
		want  Result
	}{
		{"the dusk round", dusk, Success},
		{"half an hour after dusk", dusk + 19, Success},
		{"just over an hour after dusk", dusk + 38, Failure},
		{"the round before dusk", dusk - 1, Failure},
		{"noon", 355*900 + 450, Failure},
		{"midnight", 356 * 900, Failure},
	} {
		util.SetRoundCountForTest(c.round)
		if got := condTimeOfDay(params, nil); got != c.want {
			t.Errorf("%s (round %d): got %v, want %v", c.name, c.round, got, c.want)
		}
	}
}

// A missing or non-positive hours never matches.
func TestCondTimeOfDay_AfterDuskNeedsHours(t *testing.T) {
	dusk := pinAfterDuskClock(t)
	util.SetRoundCountForTest(dusk)
	for _, p := range []map[string]any{
		{"period": "after_dusk"},
		{"period": "after_dusk", "hours": 0},
		{"period": "after_dusk", "hours": -2},
	} {
		if got := condTimeOfDay(p, nil); got != Failure {
			t.Errorf("%v at dusk: got %v, want Failure", p, got)
		}
	}
}

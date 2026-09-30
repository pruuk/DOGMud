package gametime

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Benchmark_GetDate_Uncached(b *testing.B) {

	util.IncrementRoundCount()
	for n := 0; n < b.N; n++ {
		getDate(uint64(n))
	}
}

func Benchmark_GetDate_Cached(b *testing.B) {

	for n := 0; n < b.N; n++ {
		GetDate()
	}
}

// pinRoundSeconds pins the two timing knobs AddPeriod's second and minute
// branches read, and clears the date cache on both sides so no earlier test's
// RoundsPerDay leaks in.
func pinRoundSeconds(t *testing.T, roundSeconds int) {
	t.Helper()
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = configs.ConfigInt(roundSeconds)
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
	ClearDateCacheForTest()
	t.Cleanup(ClearDateCacheForTest)
}

// Real seconds convert to rounds by RoundSeconds, rounded up, so a cooldown
// never ends before the time its config comment promises. Before this branch
// existed "60 real seconds" fell through to the rounds failover: 60 rounds,
// four minutes at a four-second round.
func TestAddPeriod_RealSecondsConvertByRoundSeconds(t *testing.T) {
	pinRoundSeconds(t, 4)
	gd := GetDate(1000)
	for _, tc := range []struct {
		period string
		rounds uint64
	}{
		{"60 real seconds", 15},
		{"60 seconds real", 15},
		{"60 irl seconds", 15},
		{"60 real secs", 15},
		{"1 real second", 1}, // a quarter of a round rounds up to one
		{"5 real seconds", 2},
		{"0 real seconds", 1}, // a quantity below one is one, as for every unit
	} {
		if got := gd.AddPeriod(tc.period) - gd.RoundNumber; got != tc.rounds {
			t.Errorf("AddPeriod(%q) = %d rounds, want %d", tc.period, got, tc.rounds)
		}
	}
}

// Seconds always mean real seconds, with or without "real" or "irl", and even
// under a "game" modifier. On the game clock (900 rounds a day at RoundSeconds
// 4) "30 seconds" used to be ceil(30*900/86400) = 1 round, so a respawn written
// "30 seconds" came back in about four real seconds.
func TestAddPeriod_SecondsAreAlwaysRealSeconds(t *testing.T) {
	pinRoundSeconds(t, 4)
	gd := GetDate(1000)
	for _, tc := range []struct {
		period string
		rounds uint64
	}{
		{"30 seconds", 8}, // 7.5 rounds, rounded up
		{"60 seconds", 15},
		{"60 secs", 15},
		{"30 game seconds", 8},
		{"30 seconds gametime", 8},
		{"1 second", 1},
		{"0 seconds", 1}, // a quantity below one is one, as for every unit
	} {
		if got := gd.AddPeriod(tc.period) - gd.RoundNumber; got != tc.rounds {
			t.Errorf("AddPeriod(%q) = %d rounds, want %d", tc.period, got, tc.rounds)
		}
	}
}

// The existing units are untouched by the seconds branch.
func TestAddPeriod_ExistingUnitsUnchanged(t *testing.T) {
	pinRoundSeconds(t, 4)
	gd := GetDate(1000)
	for _, tc := range []struct {
		period string
		rounds uint64
	}{
		{"10 rounds", 10},
		{"7 flurbles", 7}, // the failover
		{"2 real minutes", 30},
		{"1 real hour", 900},
		{"1 real day", 21600},
		{"1 game day", 900},
		{"2 hours", 75},
		{"48 minutes", 30},
	} {
		if got := gd.AddPeriod(tc.period) - gd.RoundNumber; got != tc.rounds {
			t.Errorf("AddPeriod(%q) = %d rounds, want %d", tc.period, got, tc.rounds)
		}
	}
}

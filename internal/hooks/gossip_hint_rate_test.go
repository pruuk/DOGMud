package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/facts"
	"github.com/GoMudEngine/GoMud/internal/gossip"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/worldevents"
	"github.com/stretchr/testify/require"
)

// TestBuildGossipLine_DiscLineHintRate measures how often a gossiper that knows
// two ordinary facts and one disc-line hint says the hint (#667 Task 15).
//
// The plan's rate for k other facts and one hint is 1/(k+1) of the gossip
// when the zone has no recent events, and 0.3/(k+1) when it does (an event is
// drawn 70% of the time, a known fact 30%, then one fact uniformly). For k = 2
// that is 1/3 and 0.1. The bounds are wide so the draw cannot flake at 20000
// rounds (the standard error is under 0.4 points).
func TestBuildGossipLine_DiscLineHintRate(t *testing.T) {
	tmp := t.TempDir()
	prev := configs.GetFilePathsConfig()
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": tmp}))
	defer func() {
		if err := configs.AddOverlayOverrides(map[string]any{
			"FilePaths.DataFiles": prev.DataFiles.String(),
		}); err != nil {
			t.Logf("failed to restore DataFiles override: %v", err)
		}
	}()

	facts.ResetForTest()
	defer facts.ResetForTest()
	worldevents.ResetForTest()
	defer worldevents.ResetForTest()

	defer gossip.SeedForTest(map[string][]string{
		"fact-disc-line": {"HINT {description}"},
		"fact-default":   {"OTHER {description}"},
		"fallback":       {"Quiet day."},
		// Every event type resolves through this one key, so an event line is
		// recognisable whichever type the mob draws.
		"MobKilledByPlayer-Local": {"EVENT {desc}"},
	})()

	const mobTemplateId = 9902
	for _, f := range []struct {
		id   string
		tags []string
	}{
		{"rate-test-other-a", nil},
		{"rate-test-other-b", nil},
		{"rate-test-disc", []string{"disc-line"}},
	} {
		require.NoError(t, facts.Declare(f.id, facts.DeclareOpts{
			Description:  "something happened.",
			Significance: worldevents.Local,
			Zone:         "RateTestZone",
			Tags:         f.tags,
		}))
		facts.RecordKnowsFact(mobTemplateId, f.id, facts.SourceWitnessed)
	}

	mob := &mobs.Mob{
		MobId:     mobTemplateId,
		Character: characters.Character{Zone: "RateTestZone"},
	}

	const rounds = 20000
	share := func() float64 {
		hints := 0
		for i := 0; i < rounds; i++ {
			if strings.HasPrefix(buildGossipLine(mob), "HINT ") {
				hints++
			}
		}
		return float64(hints) / rounds
	}

	// No recent events: every gossip is a known fact, so a hint is 1/(k+1).
	quiet := share()

	// Recent events: 70% event, 30% fact, so a hint is 0.3/(k+1).
	worldevents.EmitWorldEvent(worldevents.WorldEvent{
		Type:         worldevents.MobKilledByPlayer,
		Significance: worldevents.Local,
		ZoneName:     "RateTestZone",
		Description:  "a wolf was killed.",
	})
	sawEvent := false
	for i := 0; i < 200 && !sawEvent; i++ {
		sawEvent = strings.HasPrefix(buildGossipLine(mob), "EVENT ")
	}
	require.True(t, sawEvent, "the event fixture must reach the mob")
	busy := share()

	t.Logf("hint share of gossip, mob knows 2 other facts + 1 hint, %d rounds each", rounds)
	t.Logf("  no recent events: %.4f (plan 1/3 = 0.3333)", quiet)
	t.Logf("  recent events:    %.4f (plan 0.3/3 = 0.1000)", busy)
	t.Logf("  one hint per %.1f gossips quiet, per %.1f busy; at 75 rounds of 4 s that is %.0f and %.0f minutes",
		1/quiet, 1/busy, 1/quiet*5, 1/busy*5)

	require.InDelta(t, 1.0/3.0, quiet, 0.03, "quiet-zone hint share")
	require.InDelta(t, 0.1, busy, 0.03, "busy-zone hint share")
}

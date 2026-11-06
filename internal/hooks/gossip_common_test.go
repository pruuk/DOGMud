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

// Common talk (#375, Veyra's rumors): a fact tagged facts.CommonTag reaches
// every gossiper's ordinary gossip with no knows_facts entry, as one slot
// among the mob's own facts, and never goes on the hint timer.
func TestBuildGossipLine_CommonTalk(t *testing.T) {
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
		"fact-veyra-line": {"COMMON {description}"},
		"fact-default":    {"OWN {description}"},
		"fallback":        {"Quiet day."},
	})()

	for _, id := range []string{"common-a", "common-b", "common-c"} {
		require.NoError(t, facts.Declare(id, facts.DeclareOpts{
			Description:  "a crafter sells secrets.",
			Significance: worldevents.Local,
			Tags:         []string{"veyra-line", facts.CommonTag},
		}))
	}

	// A gossiper that knows nothing tells common talk, not the fallback.
	blank := &mobs.Mob{MobId: 9911, Character: characters.Character{Zone: "CommonTestZone"}}
	for i := 0; i < 50; i++ {
		require.True(t, strings.HasPrefix(buildGossipLine(blank), "COMMON "), "a mob with no facts tells common talk")
	}

	// A gossiper that knows one fact: common talk is one slot of two, however
	// many common facts there are.
	require.NoError(t, facts.Declare("own-fact", facts.DeclareOpts{
		Description: "the lake is low.", Significance: worldevents.Local, Zone: "CommonTestZone",
	}))
	const knower = 9912
	facts.RecordKnowsFact(knower, "own-fact", facts.SourceWitnessed)
	mob := &mobs.Mob{MobId: knower, Character: characters.Character{Zone: "CommonTestZone"}}
	const rounds = 20000
	common := 0
	for i := 0; i < rounds; i++ {
		if strings.HasPrefix(buildGossipLine(mob), "COMMON ") {
			common++
		}
	}
	share := float64(common) / rounds
	t.Logf("common talk share for a mob that knows 1 fact, quiet zone: %.4f (want 1/2)", share)
	require.InDelta(t, 0.5, share, 0.03)

	// Common talk is not a hint: the hint timer has nothing to say.
	require.Empty(t, dueGossipHint(mob, 1_000_000))
}

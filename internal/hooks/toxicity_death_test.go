package hooks_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	_ "github.com/GoMudEngine/GoMud/internal/hooks" // wire init() observers
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
)

// TestDeathClearsToxicity pins the rule that toxicity is the PRICE OF AN EFFECT,
// so with no effect there is no price. Death already strips every buff, which
// takes away the potion effects the toxicity was paid for; leaving the toxicity
// behind charges the player a second time for a benefit they no longer hold.
// They have also already lost the potions, the materials and the brewing time.
//
// It also closes a death spiral. ToxicitySicknessDamage deals acute HP damage
// above 90% that scales past the cap and can kill, and decay HALVES above 75% --
// slowest exactly where it is most dangerous. Without this clear a player could
// die of toxicity, respawn at 5% health still at critical toxicity, and die
// again with no way out. The alchemy formula sharpened that: a low-alchemy
// character's ceiling is near 33, so the danger band is far easier to reach than
// it was under the old flat 120.
func TestDeathClearsToxicity(t *testing.T) {
	c := characters.New()
	c.Stats.Vitality.Base = 300
	c.Stats.Vitality.Recalculate()
	c.Toxicity = 95

	_ = c.Life.TransitionToDead(
		life.DeadData{},
		state.TransitionReason{Trigger: life.TriggerSuicide},
	)

	if c.Toxicity != 0 {
		t.Errorf("Toxicity = %v after death, want 0", c.Toxicity)
	}
}

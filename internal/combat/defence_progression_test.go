package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

// newDefenceTestCharacter builds a bare, safely-initialised character for
// resolveChannelAttackWithRunner tests. characters.New() is already the
// pattern this package's other defence tests use (see defenceFixture in
// defense_affordability_test.go) -- it fully initialises the maps
// (SkillUseCount, StatUseCount, Cooldowns, MiscData, ...) that
// GetSkillUseCount, QuoteDefenseCost and ApplyProgression touch, so no
// additional setup is required for a runner that never reads the character's
// stats.
func newDefenceTestCharacter(t *testing.T) *characters.Character {
	t.Helper()
	return characters.New()
}

// A floored outcome must award the ordinary defence event but never a bonus.
func TestChannelDefence_FlooredAwardsNoBonus(t *testing.T) {
	defender := newDefenceTestCharacter(t)
	attacker := newDefenceTestCharacter(t)

	runner := func(atk float64, entries []contest.Entry) contest.Result {
		return contest.Result{
			Contested: true,
			Winner:    entries[0].Name,
			Floored:   true,
			Success:   false,
			Margin:    -1,
		}
	}

	before := defender.GetSkillUseCount("unarmed-combat")
	resolveChannelAttackWithRunner(ChannelMelee, channelSideForSignTest(ChannelMelee, attacker), attacker, defender, runner)

	if got := defender.GetSkillUseCount("unarmed-combat") - before; got != 1 {
		t.Errorf("floored defence awarded %d ordinary events, want 1", got)
	}
	// A floored save is the system overriding the dice; it is not a defensive
	// crit and must not pay the bonus. Assert via the dedupe map, which only a
	// bonus event touches.
	if defender.ClaimedBonusThisRound("unarmed-combat") {
		t.Error("a floored outcome fired a bonus progression event")
	}
}

package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Finding 3 / chunk 5.2 — the resolution-time half.
//
// Spells fold over several rounds, so the target set chosen at cast time is
// stale by the time the spell lands. A mob can become a companion, or a
// builder can flag it non_combatant, between initiation and resolution. The
// authorization policy therefore has to be re-checked when the spell lands,
// not only when it is aimed.
// ---------------------------------------------------------------------------

func TestPlayerHarmTargetPermitted_HarmfulSpellsRespectPolicy(t *testing.T) {
	harmful := []spells.SpellType{spells.HarmSingle, spells.HarmMulti, spells.HarmArea}

	for _, st := range harmful {
		ordinary := &mobs.Mob{Character: characters.Character{Name: "Bandit"}}
		assert.True(t, playerHarmTargetPermitted(st, ordinary),
			"%s must still land on an ordinary mob", st)

		immune := &mobs.Mob{
			PlayerAttackImmune: true,
			Character:          characters.Character{Name: "Caravan Guard"},
		}
		assert.False(t, playerHarmTargetPermitted(st, immune),
			"%s must not land on an attack-immune mob", st)

		nonCombatant := &mobs.Mob{
			NonCombatant: true,
			Character:    characters.Character{Name: "Barkeep"},
		}
		assert.False(t, playerHarmTargetPermitted(st, nonCombatant),
			"%s must not land on a non-combatant", st)

		companion := &mobs.Mob{Character: characters.Character{Name: "Wolf"}}
		companion.Character.Charm(7, 100, "")
		assert.False(t, playerHarmTargetPermitted(st, companion),
			"%s must not land on a companion", st)
	}
}

// Help spells legitimately target companions, so the harm policy must not be
// applied to them.
func TestPlayerHarmTargetPermitted_HelpSpellsAreUnaffected(t *testing.T) {
	companion := &mobs.Mob{Character: characters.Character{Name: "Wolf"}}
	companion.Character.Charm(7, 100, "")

	for _, st := range []spells.SpellType{spells.HelpSingle, spells.HelpMulti, spells.HelpArea, spells.Neutral} {
		assert.True(t, playerHarmTargetPermitted(st, companion),
			"%s must still reach a companion", st)
	}
}

// A mob that gained protection while the spell was folding must be dropped at
// resolution even though it passed the check at cast time.
func TestPlayerHarmTargetPermitted_ProtectionGainedMidCast(t *testing.T) {
	m := &mobs.Mob{Character: characters.Character{Name: "Stray Dog"}}

	assert.True(t, playerHarmTargetPermitted(spells.HarmSingle, m),
		"target was legal when the cast started")

	// The player charms it with a second spell before the first one lands.
	m.Character.Charm(7, 100, "")

	assert.False(t, playerHarmTargetPermitted(spells.HarmSingle, m),
		"target became a companion mid-cast and must be spared")
}

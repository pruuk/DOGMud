package mobs

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// CheckPlayerHarm — the single authorization policy for player-initiated
// harmful actions (finding 3 / chunk 5.2).
// ---------------------------------------------------------------------------

func TestCheckPlayerHarm_OrdinaryMobIsAllowed(t *testing.T) {
	m := &Mob{Character: characters.Character{Name: "Bandit"}}

	assert.Equal(t, HarmAllowed, CheckPlayerHarm(m))
}

func TestCheckPlayerHarm_NonCombatantIsBlocked(t *testing.T) {
	m := &Mob{NonCombatant: true, Character: characters.Character{Name: "Barkeep"}}

	assert.Equal(t, HarmBlockedNonCombatant, CheckPlayerHarm(m))
}

// The canonical field lives on Character; the legacy Mob field is checked too.
func TestCheckPlayerHarm_CharacterNonCombatantIsBlocked(t *testing.T) {
	m := &Mob{Character: characters.Character{Name: "Barkeep", NonCombatant: true}}

	assert.Equal(t, HarmBlockedNonCombatant, CheckPlayerHarm(m))
}

// This is the finding-3 case: a combat-capable mob that players may not attack.
func TestCheckPlayerHarm_AttackImmuneIsBlocked(t *testing.T) {
	m := &Mob{
		PlayerAttackImmune: true,
		Character:          characters.Character{Name: "Caravan Guard"},
	}

	assert.Equal(t, HarmBlockedAttackImmune, CheckPlayerHarm(m))
}

func TestCheckPlayerHarm_CompanionIsBlocked(t *testing.T) {
	m := &Mob{Character: characters.Character{Name: "Wolf"}}
	m.Character.Charm(7, 100, "")

	assert.Equal(t, HarmBlockedCompanion, CheckPlayerHarm(m))
}

// Anyone's companion is off-limits, not just the caster's own. Matches the
// pre-existing behavior in attack.go and the HarmArea resolution filter.
func TestCheckPlayerHarm_AnotherPlayersCompanionIsBlocked(t *testing.T) {
	m := &Mob{Character: characters.Character{Name: "Wolf"}}
	m.Character.Charm(99, 100, "")

	assert.Equal(t, HarmBlockedCompanion, CheckPlayerHarm(m))
}

// Companion takes precedence so the player gets the most informative message.
func TestCheckPlayerHarm_CompanionOutranksOtherBlocks(t *testing.T) {
	m := &Mob{
		NonCombatant:       true,
		PlayerAttackImmune: true,
		Character:          characters.Character{Name: "Wolf"},
	}
	m.Character.Charm(7, 100, "")

	assert.Equal(t, HarmBlockedCompanion, CheckPlayerHarm(m))
}

// Callers nil-check their own lookups; a nil mob must not manufacture a
// player-facing rejection message for a target that does not exist.
func TestCheckPlayerHarm_NilMobIsAllowed(t *testing.T) {
	assert.Equal(t, HarmAllowed, CheckPlayerHarm(nil))
}

func TestHarmBlock_Blocked(t *testing.T) {
	assert.False(t, HarmAllowed.Blocked())
	assert.True(t, HarmBlockedCompanion.Blocked())
	assert.True(t, HarmBlockedNonCombatant.Blocked())
	assert.True(t, HarmBlockedAttackImmune.Blocked())
}

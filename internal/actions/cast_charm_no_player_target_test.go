package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Charm must refuse a player target, which is what the helpfile has always
// claimed and nothing enforced.
//
// Before U10c this was a SILENT no-op: charm declared no target_defense_type,
// so a player-targeted cast took the uncontested shortcut in resolveSpell into
// applyPlayerEffect, which has no charm arm. The caster lost 120 conviction and
// nothing happened. Declaring target_defense_type: social would have been worse
// -- it routes to a real ChannelSocial contest that charges the victim
// conviction for a defy and trains their rhetoric, still for no effect.
//
// Making charm work on players is a PvP feature with its own design and consent
// questions. It is not U10c's to invent by side effect. See spec section 14.
func TestInitiateCast_CharmRefusesAPlayerTarget(t *testing.T) {
	const targetUserId = 7311

	restoreSpells := spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"charm": {
			SpellId:           "charm",
			Name:              "Charm",
			Type:              spells.HarmSingle,
			EffectType:        "charm",
			Cost:              120,
			Difficulty:        60,
			BaseFolds:         36,
			PrimaryStat:       "charisma",
			TargetDefenseType: "social",
			Schools:           []string{"manifestation"},
		},
	})
	defer restoreSpells()

	target := seedDrainAreaPlayer(targetUserId, "charmbait", "Charmbait")
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{targetUserId: target})
	defer restoreUsers()

	actor, _, room := newPlayerActor()
	room.AddPlayer(targetUserId)
	defer room.RemovePlayer(targetUserId)

	result := InitiateCast(actor, "charm", "Charmbait")

	require.True(t, result.NoTarget,
		"charm at a player must be refused as an invalid target")

	joined := strings.Join(actor.sent, "\n")
	require.NotEmpty(t, actor.sent,
		"the refusal must SAY something -- a silent refusal is the bug this replaces")
	if !strings.Contains(strings.ToLower(joined), "mind") &&
		!strings.Contains(strings.ToLower(joined), "player") {
		t.Errorf("refusal text does not explain itself: %q", joined)
	}
}

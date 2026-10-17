package species

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/require"
)

// Slice 1 of the combat reachability spec (#541): the bird and horse species
// ported from PR #208, speechless beasts like every other animal species.
func TestShippedBirdAndHorseSpecies(t *testing.T) {
	all := dogmudSpecies(t)
	cases := []struct {
		id      int
		name    string
		attack  items.ItemSubType
		unarmed string
		parts   []string
	}{
		{25, "bird", items.Claws, "beak and claws", []string{"legs", "eyes", "mouth", "skin"}},
		{26, "horse", items.Slam, "hooves", []string{"legs", "eyes", "mouth", "skin", "tail"}},
	}
	for _, c := range cases {
		sp := all[c.id]
		require.NotNil(t, sp, "species %d missing", c.id)
		require.Equal(t, c.name, sp.Name)
		require.Equal(t, c.attack, sp.NaturalAttack)
		require.Equal(t, c.unarmed, sp.UnarmedName)
		require.ElementsMatch(t, c.parts, sp.BodyParts)
		require.False(t, sp.CanSpeak(), "%s must set speech: false like every beast", c.name)
		require.False(t, sp.Selectable)
		require.True(t, sp.Tameable)
	}
}

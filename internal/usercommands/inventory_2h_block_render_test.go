package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #681: the equipment screens hide the offhand line only when the main-hand
// weapon fills both of THIS wearer's hands. A Large wearer holds a two-hander
// in one hand, so its offhand weapon must be listed; a Medium wearer's
// two-hander still hides the offhand line.
func TestInventoryTemplates_TwoHanderBlocksOffhandByWearerSize(t *testing.T) {
	useDogmudTemplates(t)
	const largeId, mediumId = 9681, 9682
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		largeId:  {SpeciesId: largeId, Name: "golem", Size: species.Large},
		mediumId: {SpeciesId: mediumId, Name: "human", Size: species.Medium},
	}))

	lance := items.Item{ItemId: 99681, Spec: &items.ItemSpec{ItemId: 99681,
		Name: "probe lance", NameSimple: "probe lance",
		Type: items.Weapon, Subtype: items.Slashing, Hands: 2}}
	dagger := items.Item{ItemId: 99682, Spec: &items.ItemSpec{ItemId: 99682,
		Name: "probe dagger", NameSimple: "probe dagger",
		Type: items.Weapon, Subtype: items.Slashing, Hands: 1}}

	render := func(t *testing.T, tpl string, speciesId int) string {
		t.Helper()
		c := characters.New()
		c.SpeciesId = speciesId
		c.Equipment.Weapon = lance
		c.Equipment.Offhand = dagger
		data := map[string]any{
			`Character`:          c,
			`Equipment`:          &c.Equipment,
			`ItemNames`:          []string{},
			`ItemNamesFormatted`: []string{},
			`EncumbranceLabel`:   `light`,
			`EncumbranceColor`:   `white`,
		}
		out, err := templates.Process(tpl, data, 0)
		require.NoError(t, err)
		// NameComplex capitalises the name; compare in lower case.
		out = strings.ToLower(out)
		require.Contains(t, out, "probe lance", "the render must reach the hand lines")
		return out
	}

	for _, tpl := range []string{"character/inventory", "character/inventory-look"} {
		t.Run(tpl, func(t *testing.T) {
			large := render(t, tpl, largeId)
			assert.Contains(t, large, "offhand:", "Large wearer: the offhand line must show")
			assert.Contains(t, large, "probe dagger", "Large wearer: the offhand weapon must be listed")

			medium := render(t, tpl, mediumId)
			assert.NotContains(t, medium, "offhand:", "Medium wearer: the two-hander hides the offhand line")
		})
	}
}

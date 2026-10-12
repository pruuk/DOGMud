package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/require"
)

// A mob that starts a timed craft says what it is making.
func TestCraftBeginLineNamesTheItem(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		8301: {ItemId: 8301, Name: "iron dagger"},
	}))

	line := craftBeginLine("Brann", &crafting.RecipeSpec{Output: crafting.RecipeOutput{ItemId: 8301}})
	require.Contains(t, line, `<ansi fg="mobname">Brann</ansi> begins working on `)
	require.Contains(t, line, `<ansi fg="itemname">Iron Dagger</ansi>.`)

	require.Contains(t, craftBeginLine("Brann", &crafting.RecipeSpec{}), "begins working on something.")
	require.Contains(t, craftBeginLine("Brann", nil), "begins working on something.")
}

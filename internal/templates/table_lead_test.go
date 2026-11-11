package templates

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A table names the column that leads each item of its two-line form
// (tables/*.w30.template; plan 3b-3, ruling 6). The detail line under the
// lead lists every other column in order, the first included.

var tableLeadTagRE = regexp.MustCompile(`</?ansi[^>]*>`)

func tableLeadLines(out string) []string {
	plain := tableLeadTagRE.ReplaceAllString(strings.ReplaceAll(out, "\r\n", "\n"), "")
	return strings.Split(strings.TrimRight(plain, "\n"), "\n")
}

func leadTable() TemplateTable {
	return GetTable("History",
		[]string{"Type", "Time", "Log"},
		[][]string{
			{"shop", "Nov 11 '26 3:04PM", "Purchased a lantern"},
			{"combat", "Nov 11 '26 3:05PM", ""},
		})
}

func TestTableLeadDefaultsToTheFirstColumn(t *testing.T) {
	tbl := leadTable()
	assert.Equal(t, 0, tbl.Lead())
	assert.Equal(t, "Time: Nov 11 '26 3:04PM · Log: Purchased a lantern", tbl.RowDetail(0))
}

func TestTableLeadNamesAColumn(t *testing.T) {
	tbl := leadTable().WithLead(2)
	assert.Equal(t, 2, tbl.Lead())
	assert.Equal(t, "Type: shop · Time: Nov 11 '26 3:04PM", tbl.RowDetail(0))
	// A blank cell is left out, as before.
	assert.Equal(t, "Type: combat · Time: Nov 11 '26 3:05PM", tbl.RowDetail(1))
}

func TestTableLeadOutOfRangeKeepsTheFirstColumn(t *testing.T) {
	assert.Equal(t, 0, leadTable().WithLead(3).Lead())
	assert.Equal(t, 0, leadTable().WithLead(-1).Lead())
	assert.Equal(t, 1, leadTable().WithLead(1).WithLead(7).Lead(), "a bad column leaves the lead as it was")
}

// The two-line form heads each item with the lead column's cell; the
// bordered and plain-column forms are the same whatever the lead.
func TestTableFormsLeadWithTheNamedColumn(t *testing.T) {
	out, err := ProcessAtWidth("tables/generic", leadTable().WithLead(2), 30)
	require.NoError(t, err)
	assert.Equal(t, []string{
		".: History",
		"Purchased a lantern",
		"  Type: shop ·",
		"  Time: Nov 11 '26 3:04PM",
		"",
		"  Type: combat ·",
		"  Time: Nov 11 '26 3:05PM",
	}, tableLeadLines(out))

	for _, width := range []int{80, 60} {
		plain, err := ProcessAtWidth("tables/generic", leadTable(), width)
		require.NoError(t, err)
		led, err := ProcessAtWidth("tables/generic", leadTable().WithLead(2), width)
		require.NoError(t, err)
		assert.Equal(t, plain, led, "at %d the lead changed the form", width)
	}
}

// The shop list's two-line form leads with its lead column too.
func TestShopListFormLeadsWithTheNamedColumn(t *testing.T) {
	tbl := GetTable("Items available", []string{"Qty", "Name", "Type", "Price"},
		[][]string{{"3", "a flask of clear river water", "drinkable", "12"}}).WithLead(1)
	out, err := ProcessAtWidth("tables/shoplist", tbl, 30)
	require.NoError(t, err)
	assert.Equal(t, []string{
		".: Items available",
		"a flask of clear river water",
		"  Qty: 3 · Type: drinkable ·",
		"  Price: 12",
	}, tableLeadLines(out))
}

// The detail line breaks only between "Header: value" pairs, keeping each
// whole where it fits the line at its indent (ruling 3), and wraps a pair
// wider than that between its words at the same indent. A pair that fits
// its line only without the " ·" after it takes the line alone, the break
// standing for the dot, so no line starts with a stray "·". A run of "? "
// (a lock sequence not yet learned) wraps like any other words.
func TestTableDetailKeepsPairsWhole(t *testing.T) {
	tbl := GetTable("Keyring", []string{"Where", "Location", "Seller", "Sequence"},
		[][]string{{"north", "#60557 Town Square", "Mistress Odell", "U D " + strings.Repeat("? ", 28)}})
	out, err := ProcessAtWidth("tables/generic", tbl, 30)
	require.NoError(t, err)
	lines := tableLeadLines(out)
	assert.Equal(t, []string{
		".: Keyring",
		"north",
		"  Location: #60557 Town Square",
		"  Seller: Mistress Odell ·",
		"  Sequence: U D ? ? ? ? ? ? ?",
		"  ? ? ? ? ? ? ? ? ? ? ? ? ? ?",
		"  ? ? ? ? ? ? ?",
	}, lines)
	for _, line := range lines {
		assert.LessOrEqual(t, WidestLine(line), 30, "%q", line)
	}
}

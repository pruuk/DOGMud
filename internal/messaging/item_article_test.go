package messaging

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// TestDropDoubledItemArticles: a line that puts its own word before an item's
// tagged name must not read "the A Folded Page" or "the Soren's Iron Pin"
// (#667). A name takes no "the" or possessive when its item is proper_name
// (a person's possessive); a trade possessive keeps them (#799).
func TestDropDoubledItemArticles(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		91801: {ItemId: 91801, Name: "Soren's Iron Pin", ProperName: true},
		91802: {ItemId: 91802, Name: "Warden's Lash"},
		91803: {ItemId: 91803, Name: "Foreman's Tally-Book"},
	})()
	cases := []struct{ name, in, want string }{
		{"the before A",
			`You show the <ansi fg="item">A Folded Page</ansi> to <ansi fg="mobname">Malk</ansi>.`,
			`You show the <ansi fg="item">Folded Page</ansi> to <ansi fg="mobname">Malk</ansi>.`},
		{"the before The, itemname tag",
			`You pick up the <ansi fg="itemname">The Warden's Lash</ansi>.`,
			`You pick up the <ansi fg="itemname">Warden's Lash</ansi>.`},
		{"your before An",
			`You wear your <ansi fg="itemname">An Old Cloak</ansi>.`,
			`You wear your <ansi fg="itemname">Old Cloak</ansi>.`},
		{"this before A, a shopkeeper's offer",
			`Any interest in this <ansi fg="itemname">A Page About a Carved Hill</ansi>?`,
			`Any interest in this <ansi fg="itemname">Page About a Carved Hill</ansi>?`},
		{"their before Some",
			`Bo gave their <ansi fg="item">Some Rope</ansi> to Mal.`,
			`Bo gave their <ansi fg="item">Rope</ansi> to Mal.`},
		{"the before a possessive name loses the the",
			`You look at the <ansi fg="item">Soren's Iron Pin</ansi> in your backpack:`,
			`You look at <ansi fg="item">Soren's Iron Pin</ansi> in your backpack:`},
		{"The opening a sentence before a possessive keeps a capital",
			`The <ansi fg="item">Soren's Iron Pin</ansi> falls.`,
			`<ansi fg="item">Soren's Iron Pin</ansi> falls.`},
		{"your before a person's name is dropped",
			`You wear your <ansi fg="item">Soren's Iron Pin</ansi>.`,
			`You wear <ansi fg="item">Soren's Iron Pin</ansi>.`},
		{"their before a person's name is dropped",
			`<ansi fg="username">Ann</ansi> gives you their <ansi fg="item">Soren's Iron Pin</ansi>.`,
			`<ansi fg="username">Ann</ansi> gives you <ansi fg="item">Soren's Iron Pin</ansi>.`},
		{"his, her and its before a person's name are dropped",
			`He holds his <ansi fg="item">Soren's Iron Pin</ansi>, she her <ansi fg="item">Soren's Iron Pin</ansi>, it its <ansi fg="item">Soren's Iron Pin</ansi>.`,
			`He holds <ansi fg="item">Soren's Iron Pin</ansi>, she <ansi fg="item">Soren's Iron Pin</ansi>, it <ansi fg="item">Soren's Iron Pin</ansi>.`},
		{"a person's name with an adjective suffix",
			`You see the <ansi fg="item">Soren's Iron Pin <ansi fg="black-bold">(worn)</ansi></ansi>.`,
			`You see <ansi fg="item">Soren's Iron Pin <ansi fg="black-bold">(worn)</ansi></ansi>.`},
		{"the before a trade possessive stays",
			`You look at the <ansi fg="item">Warden's Lash</ansi> in your backpack:`,
			`You look at the <ansi fg="item">Warden's Lash</ansi> in your backpack:`},
		{"a trade possessive keeps the, your and their",
			`the <ansi fg="item">Foreman's Tally-Book</ansi>, your <ansi fg="item">Foreman's Tally-Book</ansi>, their <ansi fg="item">Foreman's Tally-Book</ansi>`,
			`the <ansi fg="item">Foreman's Tally-Book</ansi>, your <ansi fg="item">Foreman's Tally-Book</ansi>, their <ansi fg="item">Foreman's Tally-Book</ansi>`},
		{"plain name stays",
			`You pick up the <ansi fg="itemname">Iron Sword</ansi>.`,
			`You pick up the <ansi fg="itemname">Iron Sword</ansi>.`},
		{"an item with no word before it keeps its article",
			`You see <ansi fg="item">A Folded Page</ansi> here.`,
			`You see <ansi fg="item">A Folded Page</ansi> here.`},
		{"one-word name is not an article",
			`You drop the <ansi fg="item">A</ansi>.`,
			`You drop the <ansi fg="item">A</ansi>.`},
		{"a mob name is not an item",
			`You hit the <ansi fg="mobname">A Slime</ansi>.`,
			`You hit the <ansi fg="mobname">A Slime</ansi>.`},
		{"two items",
			`You swap the <ansi fg="item">A Pin</ansi> for the <ansi fg="item">The Ring</ansi>.`,
			`You swap the <ansi fg="item">Pin</ansi> for the <ansi fg="item">Ring</ansi>.`},
	}
	for _, c := range cases {
		if got := DropDoubledItemArticles(c.in); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

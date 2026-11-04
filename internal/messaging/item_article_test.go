package messaging

import "testing"

// TestDropDoubledItemArticles: a line that puts its own word before an item's
// tagged name must not read "the A Folded Page" or "the Soren's Iron Pin" (#667).
func TestDropDoubledItemArticles(t *testing.T) {
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
		{"their before Some",
			`Bo gave their <ansi fg="item">Some Rope</ansi> to Mal.`,
			`Bo gave their <ansi fg="item">Rope</ansi> to Mal.`},
		{"the before a possessive name loses the the",
			`You look at the <ansi fg="item">Soren's Iron Pin</ansi> in your backpack:`,
			`You look at <ansi fg="item">Soren's Iron Pin</ansi> in your backpack:`},
		{"The opening a sentence before a possessive keeps a capital",
			`The <ansi fg="item">Soren's Iron Pin</ansi> falls.`,
			`<ansi fg="item">Soren's Iron Pin</ansi> falls.`},
		{"your before a possessive stays",
			`You wear your <ansi fg="item">Soren's Iron Pin</ansi>.`,
			`You wear your <ansi fg="item">Soren's Iron Pin</ansi>.`},
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

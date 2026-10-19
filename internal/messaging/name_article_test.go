package messaging

import "testing"

// TestLowerMidSentenceNameArticles: a creature's own article reads lowercase
// mid-sentence and keeps its capital where the name begins a sentence (#572).
func TestLowerMidSentenceNameArticles(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"mid-sentence mob",
			`You barrel into <ansi fg="mobname">A Culvert Slime</ansi> and land a sloppy hit!`,
			`You barrel into <ansi fg="mobname">a Culvert Slime</ansi> and land a sloppy hit!`},
		{"An and a suffixed tag",
			`You prepare to fight <ansi fg="mobname-aggro">An Ox</ansi>.`,
			`You prepare to fight <ansi fg="mobname-aggro">an Ox</ansi>.`},
		{"The, mob named through the username tag",
			`Fog curls around <ansi fg="username">The Pass-Apex</ansi>.`,
			`Fog curls around <ansi fg="username">the Pass-Apex</ansi>.`},
		{"roster entry, nested tag and duplicate index",
			`Also here: <ansi fg="mobname"><ansi fg="mobname-dup2">A Deep Slime #2</ansi></ansi>`,
			`Also here: <ansi fg="mobname"><ansi fg="mobname-dup2">a Deep Slime #2</ansi></ansi>`},
		{"start of text keeps the capital",
			`<ansi fg="mobname">A Culvert Slime</ansi> bites you.`,
			`<ansi fg="mobname">A Culvert Slime</ansi> bites you.`},
		{"after a sentence end keeps the capital",
			`You stagger. <ansi fg="mobname">The Old White</ansi> lunges!`,
			`You stagger. <ansi fg="mobname">The Old White</ansi> lunges!`},
		{"start of a line keeps the capital",
			"You look around.\n<ansi fg=\"mobname\">A Culvert Slime</ansi> is here",
			"You look around.\n<ansi fg=\"mobname\">A Culvert Slime</ansi> is here"},
		{"a one-word name is not an article",
			`You see <ansi fg="mobname">A</ansi>.`,
			`You see <ansi fg="mobname">A</ansi>.`},
		{"an item name is left to its own sites",
			`You see <ansi fg="itemname">A Sheaf of Grain</ansi>.`,
			`You see <ansi fg="itemname">A Sheaf of Grain</ansi>.`},
		{"a pet name is the player's choice",
			`You pat <ansi fg="petname">The Beast</ansi>.`,
			`You pat <ansi fg="petname">The Beast</ansi>.`},
		{"a name without an article is untouched",
			`You hit <ansi fg="mobname">Sewer Rat</ansi>.`,
			`You hit <ansi fg="mobname">Sewer Rat</ansi>.`},
	}
	for _, c := range cases {
		if got := LowerMidSentenceNameArticles(c.in); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// TestNormalizeLowersNameArticleInEveryCategory: the article stage runs even
// for a category that skips every style stage (room entry, the roster's room
// description), since the name's article is not the category's style.
func TestNormalizeLowersNameArticleInEveryCategory(t *testing.T) {
	in := `You see <ansi fg="mobname">A Culvert Slime</ansi> arrive`
	want := `You see <ansi fg="mobname">a Culvert Slime</ansi> arrive`
	for _, cat := range []Category{CategoryRoomEntry, CategoryRoomDescription, CategoryHitMelee, CategoryMobEmote} {
		got := Normalize(cat, in)
		if got != want && got != want+"." {
			t.Errorf("%v: got %q, want %q", cat, got, want)
		}
	}
}

// TestHideNamesHidesMidSentenceArticleSpelling: a corpse line names the dead
// one with a lowercase article ("corpse of a Culvert Slime"), and the hider is
// given the data spelling. It must still hide the name (#572).
func TestHideNamesHidesMidSentenceArticleSpelling(t *testing.T) {
	got := HideNames(`The corpse of a Culvert Slime crumbles to dust.`, []string{`A Culvert Slime`}, SightShapes)
	want := `The corpse of <ansi fg="combat-anon">a figure</ansi> crumbles to dust.`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

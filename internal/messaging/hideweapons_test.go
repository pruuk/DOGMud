package messaging

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/species"
)

// Spec F2 (ruling R8): a combat line names a weapon only at full sight.

const sword = `<ansi fg="item">Iron Longsword</ansi>`

func TestHideWeapons_FullSightUnchanged(t *testing.T) {
	in := `Kesh slashes you with their ` + sword + `!`
	if got := HideWeapons(in, SightFull, nil); got != in {
		t.Fatalf("full sight changed the line: %q", got)
	}
}

func TestHideWeapons_BelowFullSightReadsWeapon(t *testing.T) {
	for _, d := range []SightDecision{SightShapes, SightNone} {
		in := `something slashes you with their ` + sword + `!`
		got := HideWeapons(in, d, nil)
		want := `something slashes you with their <ansi fg="combat-anon">weapon</ansi>!`
		if got != want {
			t.Fatalf("sight %d:\n got %q\nwant %q", d, got, want)
		}
	}
}

func TestHideWeapons_ParryAttackToken(t *testing.T) {
	in := `You smoothly sweep aside the fumbled ` + sword + `!`
	got := HideWeapons(in, SightNone, nil)
	if strings.Contains(got, "Longsword") || !strings.Contains(got, "the fumbled <ansi fg=\"combat-anon\">weapon</ansi>!") {
		t.Fatalf("parry line kept the weapon: %q", got)
	}
}

func TestHideWeapons_ArticleAgreesWithWeapon(t *testing.T) {
	in := `A figure draws an <ansi fg="item">Ivory Dagger</ansi>.`
	got := HideWeapons(in, SightShapes, nil)
	want := `A figure draws a <ansi fg="combat-anon">weapon</ansi>.`
	if got != want {
		t.Fatalf("\n got %q\nwant %q", got, want)
	}
}

func TestHideWeapons_NestedDisplayNameTags(t *testing.T) {
	// DisplayName can carry a quest star before the name and an adjective
	// span after it, both inside the item tag.
	in := `something hits you with their <ansi fg="item"><ansi fg="questflag">★</ansi>Iron Longsword <ansi fg="black-bold">(cursed)</ansi></ansi> hard.`
	got := HideWeapons(in, SightNone, nil)
	want := `something hits you with their <ansi fg="combat-anon">weapon</ansi> hard.`
	if got != want {
		t.Fatalf("\n got %q\nwant %q", got, want)
	}
}

func TestHideWeapons_ShootItemnameTag(t *testing.T) {
	in := `A figure fires their <ansi fg="itemname">Longbow</ansi> at a figure!`
	got := HideWeapons(in, SightShapes, nil)
	if strings.Contains(got, "Longbow") {
		t.Fatalf("fg=itemname weapon survived: %q", got)
	}
}

func TestHideWeapons_UnarmedNamesStay(t *testing.T) {
	restore := species.SeedSpeciesForTest(map[int]*species.Species{
		1: {SpeciesId: 1, Name: "wolf", UnarmedName: "fangs"},
	})
	defer restore()
	for _, natural := range []string{"fists", "fangs", "Fangs"} {
		in := `something bites you with their <ansi fg="item">` + natural + `</ansi>!`
		if got := HideWeapons(in, SightNone, nil); got != in {
			t.Fatalf("natural weapon %q was hidden: %q", natural, got)
		}
	}
}

func TestHideWeapons_KeepsTheReadersOwnWeapon(t *testing.T) {
	in := `You slash something with your ` + sword + `, and it parries with their <ansi fg="item">Buckler Blade</ansi>.`
	got := HideWeapons(in, SightNone, []string{`<ansi fg="item">Iron Longsword</ansi>`})
	if !strings.Contains(got, "Iron Longsword") {
		t.Fatalf("reader's own weapon was hidden: %q", got)
	}
	if strings.Contains(got, "Buckler Blade") {
		t.Fatalf("the other party's weapon survived: %q", got)
	}
}

func TestHideWeapons_SentenceStartCapitalised(t *testing.T) {
	in := `The blow lands. ` + sword + ` bites deep.`
	got := HideWeapons(in, SightShapes, nil)
	if !strings.Contains(got, `<ansi fg="combat-anon">Weapon</ansi> bites deep.`) {
		t.Fatalf("sentence-start word not capitalised: %q", got)
	}
}

// The pipeline hides weapons from a shapes-only spectator of a combat line,
// after its a/an stage has already agreed the article with the real name.
func TestRenderForRecipient_ShapesSpectatorReadsWeapon(t *testing.T) {
	got := RenderForRecipient(RenderInput{
		Category:      CategoryHitMelee,
		Text:          `*** <ansi fg="mobname">Kesh</ansi> DEVASTATES <ansi fg="username">Sil</ansi> with a <ansi fg="item">Iron Longsword</ansi>! ***`,
		Channel:       ChannelVisual,
		SightDecision: SightShapes,
		LineWidth:     200,
	})
	if strings.Contains(got, "Longsword") {
		t.Fatalf("shapes spectator read the weapon: %q", got)
	}
	if !strings.Contains(got, "with a <ansi fg=\"combat-anon\">weapon</ansi>") {
		t.Fatalf("want \"with a weapon\" (never \"an weapon\"): %q", got)
	}
}

// A non-combat line keeps its item at shapes: R8 covers combat lines only.
func TestRenderForRecipient_NonCombatItemUntouched(t *testing.T) {
	got := RenderForRecipient(RenderInput{
		Category:      CategoryDefault,
		Text:          `<ansi fg="mobname">Kesh</ansi> picks up a <ansi fg="item">Torch</ansi>.`,
		Channel:       ChannelVisual,
		SightDecision: SightShapes,
		LineWidth:     200,
	})
	if !strings.Contains(got, "Torch") {
		t.Fatalf("non-combat item hidden: %q", got)
	}
}

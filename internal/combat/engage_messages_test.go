package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// Ruling A: a wolf squares up from bite.yaml's prepare pool, one draw for
// all three readers, in bite's hit band.
func TestEngageMessages_WolfSquaresUpFromBite(t *testing.T) {
	bite := slamSplitLoadFile(t, items.Bite)
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Bite: bite, items.Generic: slamSplitLoadFile(t, items.Generic),
	}))
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		9301: {SpeciesId: 9301, Name: "testwolf", UnarmedName: "jaws", NaturalAttack: items.Bite},
	}))
	actee := preAttackMatchers(t, bite, items.Prepare, func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToDefender })
	room := preAttackMatchers(t, bite, items.Prepare, func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToRoom })

	for range 20 {
		wolf := characters.New()
		wolf.SpeciesId = 9301
		wolf.Name = "Wolf"
		selka := characters.New()
		selka.Name = "Selka"
		lines := EngageMessages(wolf, selka, Mob, User)
		if lines.Category != messaging.CategoryHitNaturalSharp {
			t.Fatalf("engage category = %v, want CategoryHitNaturalSharp", lines.Category)
		}
		if l := strings.TrimSpace(slamSplitTag.ReplaceAllString(lines.Actee, ``)); !matchesAnyRe(l, actee) {
			t.Fatalf("defender's engage line is not from bite.yaml's prepare pool: %q", l)
		}
		if l := strings.TrimSpace(slamSplitTag.ReplaceAllString(lines.Observer, ``)); !matchesAnyRe(l, room) {
			t.Fatalf("room's engage line is not from bite.yaml's prepare pool: %q", l)
		}
	}
}

// The engage line is drawn by AttackSubtype directly, not
// meleeDisplaySubtype: a bow user squares up with shooting.yaml's own
// prepare lines, in the ranged band.
func TestEngageMessages_BowUserUsesShootingPrepare(t *testing.T) {
	shooting := slamSplitLoadFile(t, items.Shooting)
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Shooting: shooting, items.Generic: slamSplitLoadFile(t, items.Generic),
	}))
	actor := preAttackMatchers(t, shooting, items.Prepare, func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToAttacker })

	archer := characters.New()
	archer.Name = "Rurik"
	archer.Equipment.Weapon = items.Item{ItemId: 11, Spec: &items.ItemSpec{Type: items.Weapon, Subtype: items.Shooting, Name: "longbow"}}
	target := characters.New()
	target.Name = "Selka"
	lines := EngageMessages(archer, target, User, User)
	if lines.Category != messaging.CategoryHitRanged {
		t.Fatalf("bow engage category = %v, want CategoryHitRanged", lines.Category)
	}
	if l := strings.TrimSpace(slamSplitTag.ReplaceAllString(lines.Actor, ``)); !matchesAnyRe(l, actor) {
		t.Fatalf("bow user's engage line is not from shooting.yaml's prepare pool: %q", l)
	}
}

// D2 (a): a reader below full sight keeps the plain line with names hidden,
// because an authored line names the weapon.
func TestEngageLineFor_DarkReaderGetsThePlainLine(t *testing.T) {
	authored := "Wolf bares their jaws at you."
	plain := "Wolf prepares to fight you!"
	if got := EngageLineFor(messaging.SightFull, authored, plain, []string{"Wolf"}); got != authored {
		t.Errorf("full sight: got %q, want the authored line", got)
	}
	for _, d := range []messaging.SightDecision{messaging.SightShapes, messaging.SightNone} {
		got := EngageLineFor(d, authored, plain, []string{"Wolf"})
		if strings.Contains(got, "jaws") || strings.Contains(got, "Wolf") {
			t.Errorf("sight %v: %q names the weapon or the attacker", d, got)
		}
	}
	if got := EngageLineFor(messaging.SightFull, "", plain, []string{"Wolf"}); got != plain {
		t.Errorf("an unloaded store must fall back to the plain line, got %q", got)
	}
}

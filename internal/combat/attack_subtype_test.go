package combat

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// Spec 2026-10-16 #541 section 1: one weapon-type rule. The main-hand
// weapon's subtype, else the species' natural attack, else bare hands.
func TestAttackSubtype_OneRule(t *testing.T) {
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		9101: {SpeciesId: 9101, Name: "testwolf", UnarmedName: "jaws", NaturalAttack: items.Bite},
		9102: {SpeciesId: 9102, Name: "testhuman", UnarmedName: "fists"},
	}))
	sword := items.Item{ItemId: 10, Spec: &items.ItemSpec{Type: items.Weapon, Subtype: items.Slashing}}

	human := &characters.Character{SpeciesId: 9102}
	wolf := &characters.Character{SpeciesId: 9101}
	swordsman := &characters.Character{SpeciesId: 9102}
	swordsman.Equipment.Weapon = sword
	armedWolf := &characters.Character{SpeciesId: 9101}
	armedWolf.Equipment.Weapon = sword
	unknown := &characters.Character{SpeciesId: 424242}

	cases := []struct {
		name string
		char *characters.Character
		want items.ItemSubType
	}{
		{"weapon user", swordsman, items.Slashing},
		{"natural attacker", wolf, items.Bite},
		{"bare-handed humanoid", human, items.Unarmed},
		{"a weapon beats the species attack", armedWolf, items.Slashing},
		{"an unknown species is bare-handed", unknown, items.Unarmed},
	}
	for _, c := range cases {
		if got := AttackSubtype(c.char); got != c.want {
			t.Errorf("%s: AttackSubtype = %q, want %q", c.name, got, c.want)
		}
	}

	// The swing path asks the same rule per slot: an armed wolf's empty
	// off-hand still bites.
	if got := buildWeaponSetup(armedWolf, armedWolf, items.Item{}, 1, 2).weaponSubType; got != items.Bite {
		t.Errorf("armed wolf's empty off-hand swings as %q, want bite", got)
	}
}

// preAttackMatchers turns every template of one role of one pre-attack
// intensity into a regexp, tokens as wildcards, tags stripped.
func preAttackMatchers(t *testing.T, g *items.WeaponAttackMessageGroup, intensity items.Intensity,
	role func(items.TogetherMessages) items.SkillTieredMessages) []*regexp.Regexp {
	t.Helper()
	opts, ok := g.Options[intensity]
	if !ok {
		t.Fatalf("%s has no %s pool", g.OptionId, intensity)
	}
	var out []*regexp.Regexp
	s := role(opts.Together)
	for _, tier := range []items.MessageOptions{s.Beginner, s.Expert, s.Master} {
		for _, m := range tier {
			plain := slamSplitTag.ReplaceAllString(string(m), ``)
			out = append(out, regexp.MustCompile(`^`+slamSplitTokenRe.ReplaceAllString(regexp.QuoteMeta(plain), `.*?`)+`$`))
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s %s pool is empty", g.OptionId, intensity)
	}
	return out
}

func matchesAnyRe(line string, res []*regexp.Regexp) bool {
	for _, re := range res {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// Ruling B: a creature waits with its own natural attack. A wolf's wait
// lines come from bite.yaml, in bite's hit band, not from generic.
func TestGetWaitMessages_WolfWaitsWithItsBite(t *testing.T) {
	bite := slamSplitLoadFile(t, items.Bite)
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Bite:    bite,
		items.Generic: slamSplitLoadFile(t, items.Generic),
	}))
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		9103: {SpeciesId: 9103, Name: "testwolf", UnarmedName: "jaws", NaturalAttack: items.Bite},
	}))
	actee := preAttackMatchers(t, bite, items.Wait, func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToDefender })

	for range 20 {
		wolf := characters.New()
		wolf.SpeciesId = 9103
		wolf.Name = "Wolf"
		selka := characters.New()
		selka.Name = "Selka"
		res := GetWaitMessages(items.Wait, wolf, selka, Mob, User)
		if len(res.MessagesToTarget) == 0 {
			t.Fatal("the wolf sent its target no wait line")
		}
		msg := res.MessagesToTarget[0]
		if msg.Category != messaging.CategoryHitNaturalSharp {
			t.Errorf("wait line category = %v, want bite's CategoryHitNaturalSharp", msg.Category)
		}
		line := strings.TrimSpace(slamSplitTag.ReplaceAllString(msg.Text, ``))
		if !matchesAnyRe(line, actee) {
			t.Fatalf("wolf wait line is not from bite.yaml: %q", line)
		}
	}
}

package combat

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
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

// A bare-handed humanoid waits from unarmed.yaml, the file its swings use,
// on the unarmed hit band. Generic is seeded too, so the old fallback (an
// empty main-hand subtype falling to generic on CategoryHitMelee) is
// reachable and would be caught.
func TestGetWaitMessages_BareHandedHumanoidWaitsFromUnarmed(t *testing.T) {
	unarmed := slamSplitLoadFile(t, items.Unarmed)
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Unarmed: unarmed,
		items.Generic: slamSplitLoadFile(t, items.Generic),
	}))
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		9104: {SpeciesId: 9104, Name: "testhuman", UnarmedName: "fists"},
	}))
	actee := preAttackMatchers(t, unarmed, items.Wait, func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToDefender })

	for range 20 {
		brawler := characters.New()
		brawler.SpeciesId = 9104
		brawler.Name = "Brawler"
		selka := characters.New()
		selka.Name = "Selka"
		res := GetWaitMessages(items.Wait, brawler, selka, User, User)
		if len(res.MessagesToTarget) == 0 {
			t.Fatal("the brawler sent its target no wait line")
		}
		msg := res.MessagesToTarget[0]
		if msg.Category != messaging.CategoryHitUnarmed {
			t.Errorf("wait line category = %v, want CategoryHitUnarmed", msg.Category)
		}
		line := strings.TrimSpace(slamSplitTag.ReplaceAllString(msg.Text, ``))
		if !matchesAnyRe(line, actee) {
			t.Fatalf("bare-handed wait line is not from unarmed.yaml: %q", line)
		}
	}
}

// The "too entangled to defend" lines (filterDefensesForThirdParty) ride
// the attacker's own hit band: a wolf jumping on a grappled target sends
// them on bite's CategoryHitNaturalSharp, not the main-hand-only melee band.
func TestFilterDefensesForThirdParty_WolfEntangledLinesUseBiteBand(t *testing.T) {
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		9105: {SpeciesId: 9105, Name: "testwolf", UnarmedName: "jaws", NaturalAttack: items.Bite},
	}))
	wolf := characters.New()
	wolf.SetUserId(1)
	wolf.SpeciesId = 9105
	wolf.Name = "Wolf"
	other := characters.New()
	other.SetUserId(42)
	tgt := characters.New()
	tgt.SetUserId(2)
	tgt.Name = "Selka"
	if err := tgt.Position.TransitionToClinch(
		position.GrappleData{Partner: state.ActorRef{UserId: other.GetUserId()}},
		state.TransitionReason{Trigger: position.TriggerGrappleEntry},
	); err != nil {
		t.Fatalf("clinch: %v", err)
	}

	var res AttackResult
	filtered, third := filterDefensesForThirdParty(&res, wolf, tgt, nil)
	if !third || len(filtered) != 0 {
		t.Fatalf("fixture is not a defenceless third-party attack: third=%v filtered=%v", third, filtered)
	}
	if len(res.MessagesToTarget) == 0 || len(res.MessagesToSource) == 0 {
		t.Fatal("no entangled lines were sent")
	}
	for _, m := range append(res.MessagesToTarget, res.MessagesToSource...) {
		if m.Category != messaging.CategoryHitNaturalSharp {
			t.Errorf("entangled line %q category = %v, want CategoryHitNaturalSharp", m.Text, m.Category)
		}
	}
}

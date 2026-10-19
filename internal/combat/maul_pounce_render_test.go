package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// A bear's swing comes from maul.yaml and a cat's from pounce.yaml, both in
// the natural-sharp band; claws and generic are loaded too, so a species
// left on claws would match the wrong file.
func TestMaulAndPounceRenderFromTheirOwnFiles(t *testing.T) {
	subs := []items.ItemSubType{items.Maul, items.Pounce, items.Claws, items.Generic}
	pools := map[items.ItemSubType]*items.WeaponAttackMessageGroup{}
	for _, sub := range subs {
		pools[sub] = slamSplitLoadFile(t, sub)
	}
	t.Cleanup(items.SeedAttackMessagesForTest(pools))
	cases := []struct {
		file string
		want items.ItemSubType
	}{{"3-bear.yaml", items.Maul}, {"11-feline.yaml", items.Pounce}}
	seeded := map[int]*species.Species{}
	for _, c := range cases {
		sp := slamSplitLoadSpecies(t, c.file)
		seeded[sp.SpeciesId] = sp
	}
	t.Cleanup(species.SeedSpeciesForTest(seeded))
	for _, c := range cases {
		sp := slamSplitLoadSpecies(t, c.file)
		matchers := slamSplitActorMatchers(pools[c.want])
		t.Run(sp.Name, func(t *testing.T) {
			if got := CategoryForWeaponSubtype(c.want); got != messaging.CategoryHitNaturalSharp {
				t.Fatalf("%s band = %v, want CategoryHitNaturalSharp", c.want, got)
			}
			src, tgt := defenceFixture(1000)
			src.SpeciesId = sp.SpeciesId
			src.Name = "Rurik"
			tgt.Name = "Selka"
			tgt.HealthMax.Base = 100
			tgt.HealthMax.Recalculate()
			ws := buildWeaponSetup(src, tgt, items.Item{}, 0, 1)
			if ws.weaponSubType != c.want {
				t.Fatalf("%s swings as %q, want %q", sp.Name, ws.weaponSubType, c.want)
			}
			for range 20 {
				result := &AttackResult{}
				buildAttackMessages(result, src, tgt, ws, swingDamageParams{dmgMean: 20},
					10, 0, 0, 0, User, User, "", false, false)
				if len(result.MessagesToSource) == 0 {
					t.Fatalf("%s built no attacker line", sp.Name)
				}
				line := strings.TrimSpace(slamSplitTag.ReplaceAllString(result.MessagesToSource[0].Text, ``))
				if !slamSplitMatchesAny(line, matchers) {
					t.Fatalf("%s line is not from %s.yaml: %q", sp.Name, c.want, line)
				}
			}
		})
	}
}

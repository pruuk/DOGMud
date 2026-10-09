package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// Spec F2 (ruling R8), the participant half: a reader below full sight does
// not learn the OTHER party's weapon, and keeps their own. Lines below are
// the shapes the playtest quoted (run 6b3b017287df1ded): the defender's hit
// line, and the parry line whose {attack} is the attacker's weapon.
func TestHideIdentitiesInPersonalLines_HidesTheOtherPartysWeapon(t *testing.T) {
	atk := characters.New()
	atk.Name = "Ordel"
	atk.Equipment.Weapon = items.Item{ItemId: 999901, Spec: &items.ItemSpec{Name: "iron longsword"}}
	def := characters.New()
	def.Name = "Fold"
	def.Equipment.Weapon = items.Item{ItemId: 999902, Spec: &items.ItemSpec{Name: "oak staff"}}

	build := func() *AttackResult {
		return &AttackResult{
			MessagesToSource: []TaggedMessage{
				{Category: messaging.CategoryHitMelee, Text: `You slash <ansi fg="username">Fold</ansi> with your <ansi fg="item">Iron Longsword</ansi>!`},
				{Category: messaging.CategoryParry, Text: `<ansi fg="username">Fold</ansi> turns your blow aside with their <ansi fg="item">Oak Staff</ansi>.`},
			},
			MessagesToTarget: []TaggedMessage{
				{Category: messaging.CategoryHitMelee, Text: `In a sweeping motion, <ansi fg="username">Ordel</ansi> slashes you with their <ansi fg="item">Iron Longsword</ansi>!`},
				{Category: messaging.CategoryParry, Text: `You smoothly sweep aside the fumbled <ansi fg="item">Iron Longsword</ansi>!`},
			},
		}
	}

	t.Run("full sight: every weapon named", func(t *testing.T) {
		res := build()
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{sourceSight: messaging.SightFull, targetSight: messaging.SightFull})
		if !strings.Contains(res.MessagesToTarget[0].Text, "Iron Longsword") || !strings.Contains(res.MessagesToSource[1].Text, "Oak Staff") {
			t.Fatalf("full sight lost a weapon name: %q / %q", res.MessagesToTarget[0].Text, res.MessagesToSource[1].Text)
		}
	})

	for _, d := range []messaging.SightDecision{messaging.SightShapes, messaging.SightNone} {
		res := build()
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{sourceSight: d, targetSight: d})

		for _, m := range res.MessagesToTarget {
			if strings.Contains(m.Text, "Longsword") {
				t.Fatalf("sight %d: defender learned the attacker's weapon: %q", d, m.Text)
			}
			if !strings.Contains(m.Text, "weapon") {
				t.Fatalf("sight %d: defender line lacks the generic word: %q", d, m.Text)
			}
		}
		if !strings.Contains(res.MessagesToSource[0].Text, "your <ansi fg=\"item\">Iron Longsword</ansi>") {
			t.Fatalf("sight %d: attacker lost the name of their own weapon: %q", d, res.MessagesToSource[0].Text)
		}
		if strings.Contains(res.MessagesToSource[1].Text, "Oak Staff") {
			t.Fatalf("sight %d: attacker learned the defender's weapon: %q", d, res.MessagesToSource[1].Text)
		}
	}
}

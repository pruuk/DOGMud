package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// #572 (owner ruling 2026-10-19): every incoming hit, a weapon's or a natural
// attack's, tells the one hit how badly, in wound words and never a number,
// as the deflected and special-move lines already did. The attacker's line
// and the room's are unchanged, and a swing that dealt nothing gets none.
func TestIncomingHitCarriesDamageWord(t *testing.T) {
	subs := []items.ItemSubType{items.Slashing, items.Bite, items.Claws, items.Maul, items.Pounce, items.Lash, items.Slam, items.Generic}
	pools := map[items.ItemSubType]*items.WeaponAttackMessageGroup{}
	for _, sub := range subs {
		pools[sub] = slamSplitLoadFile(t, sub)
	}
	t.Cleanup(items.SeedAttackMessagesForTest(pools))

	for _, sub := range subs[:len(subs)-1] {
		t.Run(string(sub), func(t *testing.T) {
			src, tgt := defenceFixture(1000)
			src.Name = "Rurik"
			tgt.Name = "Selka"
			tgt.HealthMax.Base = 100
			tgt.HealthMax.Recalculate()
			ws := weaponSetup{weaponName: "teeth", weaponSubType: sub}

			// 10 of 100 health is the "light wounds" band.
			want := "(" + GetDamageDescription(10, 100) + ")"
			for range 20 {
				result := &AttackResult{}
				buildAttackMessages(result, src, tgt, ws, swingDamageParams{dmgMean: 20},
					10, 0, 0, 0, User, User, "", false, false)
				if len(result.MessagesToTarget) == 0 {
					t.Fatalf("%s built no defender line", sub)
				}
				got := strings.TrimSpace(slamSplitTag.ReplaceAllString(result.MessagesToTarget[0].Text, ``))
				if !strings.HasSuffix(got, want) {
					t.Fatalf("%s defender line lacks %q: %q", sub, want, got)
				}
				attacker := slamSplitTag.ReplaceAllString(result.MessagesToSource[0].Text, ``)
				if strings.Contains(attacker, want) {
					t.Errorf("%s attacker line gained the wound words: %q", sub, attacker)
				}
			}

			// A swing that dealt nothing reads no wound words.
			result := &AttackResult{}
			buildAttackMessages(result, src, tgt, ws, swingDamageParams{dmgMean: 20},
				0, 0, 0, 0, User, User, "", false, false)
			for _, m := range result.MessagesToTarget {
				if strings.Contains(m.Text, `fg="damage"`) {
					t.Errorf("%s zero-damage defender line has wound words: %q", sub, m.Text)
				}
			}
		})
	}
}

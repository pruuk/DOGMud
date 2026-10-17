package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// #564: a swing's message tier and its feint chance read the skill of the
// hand that SWUNG, not the main hand. The M6 wraith playtest saw a master-tier
// unarmed line from a fighter whose unarmed skill was apprentice: the main
// hand held a spear at weapon-combat 100, and buildAttackMessages read
// GetCombatSkillLevel(), which resolves from the main-hand weapon.
//
// Fixture: main hand holds a weapon (weapon-combat 100), the swing being
// narrated is the bare off hand (unarmed-combat 1). The unarmed pool carries a
// beginner line and a master line; only the beginner line may appear. At
// unarmed 1 the feint chance rounds to zero, while at the main hand's 100 it
// is the 75% cap, so a feint on any trial is the main-hand skill leaking in.
func TestBuildAttackMessages_TierAndFeintReadTheSwingingHand(t *testing.T) {
	const (
		beginnerHit  = "BEGINNER-HIT"
		masterHit    = "MASTER-HIT"
		beginnerMiss = "BEGINNER-MISS"
		masterMiss   = "MASTER-MISS"
	)
	tiered := func(beginner, master string) items.TogetherMessages {
		s := items.SkillTieredMessages{
			Beginner: items.MessageOptions{items.ItemMessage(beginner)},
			Master:   items.MessageOptions{items.ItemMessage(master)},
		}
		return items.TogetherMessages{ToAttacker: s, ToDefender: s, ToRoom: s}
	}
	group := &items.WeaponAttackMessageGroup{OptionId: items.Unarmed, Options: items.AttackTypes{}}
	for _, in := range []items.Intensity{items.Weak, items.Normal, items.Heavy} {
		group.Options[in] = items.AttackOptions{Together: tiered(beginnerHit, masterHit)}
	}
	group.Options[items.Miss] = items.AttackOptions{Together: tiered(beginnerMiss, masterMiss)}
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Unarmed: group,
	}))

	src, tgt := defenceFixture(1000)
	src.Name, tgt.Name = "Rurik", "Selka"
	src.RoomId, tgt.RoomId = 1, 1
	tgt.HealthMax.Base = 100
	tgt.HealthMax.Recalculate()
	src.Equipment.Weapon = items.Item{ItemId: 999999} // a non-fist weapon: weapon-combat
	src.Skills = map[string]int{string(skills.WeaponCombat): 100, string(skills.UnarmedCombat): 1}
	if got := src.GetCombatSkillLevel(); got != 100 {
		t.Fatalf("fixture: main-hand skill = %d, want 100", got)
	}
	if got := src.GetCombatSkillLevelFor(items.Item{}); got != 1 {
		t.Fatalf("fixture: off-hand (unarmed) skill = %d, want 1", got)
	}

	offHand := weaponSetup{weaponName: "fists", weaponSubType: items.Unarmed}
	sdp := swingDamageParams{dmgMean: 20}

	for i := 0; i < 200; i++ {
		hit := &AttackResult{}
		buildAttackMessages(hit, src, tgt, offHand, sdp, 15, 0, 0, 0, User, User, "", false, false)
		if got := hit.MessagesToSource[0].Text; got != beginnerHit {
			t.Fatalf("trial %d: off-hand hit narrated %q, want the unarmed beginner line", i, got)
		}

		miss := &AttackResult{}
		buildAttackMessages(miss, src, tgt, offHand, sdp, 0, 0, 0, 0, User, User, "", false, false)
		if got := miss.MessagesToSource[0].Text; got != beginnerMiss {
			t.Fatalf("trial %d: off-hand miss narrated %q, want the unarmed beginner miss (a master line or a feint means the main hand's skill was read)", i, got)
		}
	}
}

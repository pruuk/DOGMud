package aicompanion

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// #940: arrows a companion gathers after a fight go back into the bundle
// they came out of. When that bundle was a quest's pouch and it emptied, the
// fresh bundle keeps the quest mark, so the gathered arrows are still quest
// gear; arrows from a bought bundle stay ordinary.
func TestGatherArrowsKeepsTheQuestMark(t *testing.T) {
	const arrowsId = 94301
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		arrowsId: {ItemId: arrowsId, Name: "Bundle of Arrows", Type: items.Ammo, Uses: 20},
	}))
	m := &AICompanionModule{cfg: Config{RecoverArrows: true, RecoverArrowsMin: 0.5, RecoverArrowsMax: 0.5}}

	for _, quest := range []bool{true, false} {
		mob := &mobs.Mob{InstanceId: 94301}
		mob.Character.Stats.Strength.ValueAdj = 100
		c := &controller{mind: &Mind{}}
		f := &fightState{AmmoAtStart: 10, AmmoItemId: arrowsId, AmmoQuest: quest}

		if got := m.gatherArrows(c, mob, f); got != 5 {
			t.Fatalf("quest=%v: gathered %d, want 5", quest, got)
		}
		if len(mob.Character.Items) != 1 {
			t.Fatalf("quest=%v: carried %d bundles, want 1", quest, len(mob.Character.Items))
		}
		if mob.Character.Items[0].IsQuestGiven() != quest {
			t.Fatalf("quest=%v: the fresh bundle's mark is %v", quest, mob.Character.Items[0].IsQuestGiven())
		}
	}
}

// The source bundle's mark is read when the fight starts.
func TestAmmoBundleIsQuestGear(t *testing.T) {
	mob := &mobs.Mob{}
	mob.Character.Items = []items.Item{{ItemId: 7, Uses: 3, QuestGiven: true}}
	if !ammoBundleIsQuestGear(mob, 7) {
		t.Fatal("a quest pouch reads as quest gear")
	}
	mob.Character.Items[0].QuestGiven = false
	if ammoBundleIsQuestGear(mob, 7) {
		t.Fatal("a bought bundle does not")
	}
}

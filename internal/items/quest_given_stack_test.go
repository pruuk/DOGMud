package items

import "testing"

// #940: a quest's copy and a bought copy are never one inventory row or one
// bank slot.
func TestSameStack_QuestGivenIsItsOwnStack(t *testing.T) {
	bought := Item{ItemId: 5, Uses: 1}
	quest := Item{ItemId: 5, Uses: 1, QuestGiven: true}
	if SameStack(bought, quest) {
		t.Fatal("a quest copy stacked with a bought one")
	}
	if !SameStack(quest, Item{ItemId: 5, Uses: 1, QuestGiven: true}) {
		t.Fatal("two quest copies of the same item should stack")
	}
}

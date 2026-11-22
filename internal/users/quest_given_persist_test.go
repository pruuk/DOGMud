package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
)

// #940: a quest-given copy stays quest-given across a save and a load, and a
// bought copy of the same item stays unmarked. The mark rides on the copy as
// items.Item.QuestGiven (yaml "questgiven"), saved with the character.
func TestQuestGivenMarkSurvivesSaveAndLoad(t *testing.T) {
	withUserDataDir(t)
	const gearId = 94101
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		gearId: {ItemId: gearId, Name: "Drillyard Cudgel", NameSimple: "cudgel", Type: items.Weapon, Value: 12},
	}))
	idx := NewUserIndex()
	if err := idx.Create(); err != nil {
		t.Fatal(err)
	}
	if err := idx.AddUser(4101, "questgear"); err != nil {
		t.Fatal(err)
	}

	u := probeUser(4101, "questgear")
	given := items.New(gearId)
	given.QuestGiven = true
	bought := items.New(gearId)
	u.Character.Items = []items.Item{given, bought}

	p, err := PrepareUserWrite(u)
	if err != nil {
		t.Fatalf("PrepareUserWrite: %v", err)
	}
	if err := savequeue.Commit(p); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	back, err := LoadUser("questgear", true)
	if err != nil {
		t.Fatalf("LoadUser: %v", err)
	}
	if len(back.Character.Items) != 2 {
		t.Fatalf("items after load = %d, want 2", len(back.Character.Items))
	}
	marked := 0
	for _, itm := range back.Character.Items {
		if itm.IsQuestGiven() {
			marked++
		}
	}
	if marked != 1 {
		t.Fatalf("quest-given copies after load = %d, want 1 (the given copy marked, the bought one not)", marked)
	}
}

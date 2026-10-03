package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// A bonded AI companion's search rolls for a bauble only when the module
// says so (SearchOptions.BaubleForUserId), on her owner's account, and the
// find goes into her own pack, with the module told so she can react.
func TestSearch_Bauble_CompanionRollsForHerOwner(t *testing.T) {
	pinConfigForTest(t)
	room := newSearchTestRoom(9510)
	owner := newSearchFakeActor("Corvin", room, true, 7201)
	canCarry(owner)
	h := stubBaubleSearch(t, true, owner)

	her := newSearchTestMob(8902, "Tobin", 9510)
	her.Character.Stats.Strength.ValueAdj = 50
	mobs.SetInstanceForTest(her.InstanceId, her)
	t.Cleanup(func() { mobs.SetInstanceForTest(her.InstanceId, nil) })
	actor := newSearchMobActor("Tobin", room, her.InstanceId)
	actor.char = &her.Character

	var heard string
	companionai.SetBaubleFoundHandler(func(id int, name string, pocketed bool) {
		if id == her.InstanceId {
			heard = name
		}
	})
	t.Cleanup(func() { companionai.SetBaubleFoundHandler(nil) })

	if r := Search(actor, SearchOptions{}); h.rolls != 0 || r.BaubleFound {
		t.Fatal("without the module's say-so a companion's search does not roll")
	}
	her.Character.Cooldowns = nil // the search cooldown, between the two tries
	r := Search(actor, SearchOptions{BaubleForUserId: owner.userId})
	if h.rolls != 1 || !r.BaubleFound {
		t.Fatalf("with it, one roll on her owner's account: rolls=%d", h.rolls)
	}
	if len(h.deliveries) != 1 || h.deliveries[0].UserId != owner.userId || h.deliveries[0].ByMobInstanceId != her.InstanceId {
		t.Fatalf("the owner's find, turned up by her: %+v", h.deliveries)
	}
	if len(her.Character.Items) != 1 || !her.Character.Items[0].IsBauble() {
		t.Fatalf("it goes into her pack: %+v", her.Character.Items)
	}
	if len(owner.char.Items) != 0 {
		t.Fatal("not her owner's")
	}
	if heard == `` || !searchSaid(owner, "Tobin") || !strings.Contains(strings.Join(owner.sent, " "), "works it free") {
		t.Fatalf("the module hears of it (%q) and her owner is told: %q", heard, owner.sent)
	}
}

// If she is gone before the find is named, it is her owner's as usual.
func TestSearch_Bauble_CompanionGoneItIsTheOwners(t *testing.T) {
	pinConfigForTest(t)
	room := newSearchTestRoom(9511)
	owner := newSearchFakeActor("Corvin", room, true, 7202)
	canCarry(owner)
	h := stubBaubleSearch(t, true, owner)
	actor := newSearchMobActor("Tobin", room, 8903) // no live instance
	r := Search(actor, SearchOptions{BaubleForUserId: owner.userId})
	if !r.BaubleFound || len(h.deliveries) != 1 {
		t.Fatal("fixture: it was found")
	}
	if len(owner.char.Items) != 1 || !owner.char.Items[0].IsBauble() {
		t.Fatalf("with her gone it is her owner's: %+v", owner.char.Items)
	}
}

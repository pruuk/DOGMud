package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #282 at resolution: corpse selection spends nothing, so it runs before the
// reservation gate and before the component is consumed. A raise that cannot
// find a corpse names the corpse, and keeps the component.

const csoUserId = 9189

func seedResolveRaiser(t *testing.T, convictionMax int) (*users.UserRecord, func()) {
	t.Helper()
	u := users.NewTestUser(csoUserId, "resolver", "Resolver", uint64(csoUserId))
	u.Character.ConvictionMax.Value = convictionMax
	u.Character.Conviction = convictionMax
	cleanUsers := users.SeedUsersForTest(map[int]*users.UserRecord{csoUserId: u})
	events.DrainQueuedMessagesForTest(csoUserId)
	return u, func() {
		events.DrainQueuedMessagesForTest(csoUserId)
		cleanUsers()
	}
}

func raiseSpellForOrderTest(componentId int) *spells.SpellData {
	return &spells.SpellData{
		SpellId:              "testraise-resolve",
		SummonMobId:          1,
		SummonPetMultiplier:  1.0,
		SummonRequiresCorpse: true,
		SummonMinCorpsePool:  120,
		SummonComponentId:    componentId,
	}
}

func TestResolveSummon_RaiseAtCeiling_NoCorpse_NamesTheCorpse(t *testing.T) {
	u, cleanup := seedResolveRaiser(t, 50)
	defer cleanup()
	reserve := u.Character.CalcCompanionReserve(characters.CompanionReserveBase(1.0))
	if !u.Character.WouldBreachReservationCap(characters.PoolConviction, reserve) {
		t.Fatalf("fixture: reserve %d must breach the cap", reserve)
	}

	if resolveCompanionSummon(u, raiseSpellForOrderTest(0), "", &rooms.Room{RoomId: 999907}) {
		t.Fatal("a raise with no corpse must fail")
	}

	msgs := strings.Join(events.DrainQueuedMessagesForTest(csoUserId), "\n")
	if !strings.Contains(msgs, "no suitable remains") {
		t.Errorf("an empty room must be refused for the missing corpse; got:\n%s", msgs)
	}
	if strings.Contains(msgs, "reserve") {
		t.Errorf("the corpse reason must come first, not reservation; got:\n%s", msgs)
	}
}

func TestResolveSummon_RaiseNoCorpse_KeepsComponent(t *testing.T) {
	u, cleanup := seedResolveRaiser(t, 4000)
	defer cleanup()
	const componentId = 987654
	u.Character.Items = []items.Item{{ItemId: componentId}}

	if resolveCompanionSummon(u, raiseSpellForOrderTest(componentId), "", &rooms.Room{RoomId: 999908}) {
		t.Fatal("a raise with no corpse must fail")
	}
	if len(u.Character.Items) != 1 {
		t.Errorf("a raise refused for its corpse must not consume the component; %d items left",
			len(u.Character.Items))
	}
}

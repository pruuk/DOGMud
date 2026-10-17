package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #282: a raise refused for reservation hid the real reason it would fail.
// A player at the reservation ceiling with no usable corpse was told to free
// up reserve, did so, and then failed again for the corpse. Corpse selection
// spends nothing, so it is checked first and the player hears the reason that
// no amount of reserve would fix.

const crrUserId = 9188

func seedRaiseCaster(t *testing.T, convictionMax int) (*users.UserRecord, func()) {
	t.Helper()

	u := users.NewTestUser(crrUserId, "raiser", "Raiser", uint64(crrUserId))
	u.Character.Activity = activity.NewMachine()
	u.Character.ConvictionMax.Value = convictionMax
	u.Character.Conviction = convictionMax
	u.Character.Skills = map[string]int{"manifestation": 5}
	u.Character.SpellBook = map[string]int{"testraise-order": 1}

	cleanSpells := spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"testraise-order": {
			SpellId:    "testraise-order",
			Name:       "Test Raise Order",
			AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSelf,
			Schools:              []string{spells.SchoolManifestation},
			BaseFolds:            4,
			Cost:                 5,
			SummonMobId:          1,
			SummonPetMultiplier:  1.0,
			SummonRequiresCorpse: true,
			SummonMinCorpsePool:  120,
		},
	})
	cleanUsers := users.SeedUsersForTest(map[int]*users.UserRecord{crrUserId: u})
	events.DrainQueuedMessagesForTest(crrUserId)

	return u, func() {
		events.DrainQueuedMessagesForTest(crrUserId)
		cleanUsers()
		cleanSpells()
	}
}

// faintCorpse is a mob corpse whose stat pool sits below the spell's minimum.
func faintCorpse() rooms.Corpse {
	c := rooms.Corpse{MobId: 1}
	c.Character.Name = "rat"
	c.Character.Stats.Strength.Training = 10
	return c
}

func castRaise(t *testing.T, u *users.UserRecord, room *rooms.Room, rest string) string {
	t.Helper()
	handled, err := Cast(rest, u, room, 0)
	if err != nil || !handled {
		t.Fatalf("cast errored: handled=%v err=%v", handled, err)
	}
	if u.Character.Activity != nil && u.Character.Activity.IsCasting() {
		t.Error("a raise with no usable corpse must be refused before it channels")
	}
	return strings.Join(events.DrainQueuedMessagesForTest(crrUserId), "\n")
}

func requireBreach(t *testing.T, u *users.UserRecord) {
	t.Helper()
	reserve := u.Character.CalcCompanionReserve(characters.CompanionReserveBase(1.0))
	if !u.Character.WouldBreachReservationCap(characters.PoolConviction, reserve) {
		t.Fatalf("fixture: reserve %d must breach the cap %d, or the test proves nothing",
			reserve, u.Character.ReservationCap(characters.PoolConviction))
	}
}

func TestCast_RaiseAtCeiling_NoCorpse_NamesTheCorpse(t *testing.T) {
	u, cleanup := seedRaiseCaster(t, 50)
	defer cleanup()
	requireBreach(t, u)

	msgs := castRaise(t, u, &rooms.Room{RoomId: 999904}, "testraise-order")

	if !strings.Contains(msgs, "no suitable remains") {
		t.Errorf("an empty room must be refused for the missing corpse; got:\n%s", msgs)
	}
	if strings.Contains(msgs, "reserve") {
		t.Errorf("the corpse reason must come first, not reservation; got:\n%s", msgs)
	}
}

func TestCast_RaiseAtCeiling_FaintCorpse_NamesTheCorpse(t *testing.T) {
	u, cleanup := seedRaiseCaster(t, 50)
	defer cleanup()
	requireBreach(t, u)

	room := &rooms.Room{RoomId: 999905, Corpses: []rooms.Corpse{faintCorpse()}}
	msgs := castRaise(t, u, room, "testraise-order corpse")

	if !strings.Contains(msgs, "too faint") {
		t.Errorf("a too-faint corpse must be named as the reason; got:\n%s", msgs)
	}
	if strings.Contains(msgs, "reserve") {
		t.Errorf("the corpse reason must come first, not reservation; got:\n%s", msgs)
	}
	if len(room.Corpses) != 1 {
		t.Errorf("a refused raise must not consume the corpse; %d left", len(room.Corpses))
	}
}

// Within the ceiling, an unusable corpse used to channel the whole cast and
// spend the conviction before refusing at resolution. It is now refused at
// initiation, like every other raise that cannot land (U7b admission rule).
func TestCast_RaiseWithinCeiling_NoCorpse_ChargesNothing(t *testing.T) {
	u, cleanup := seedRaiseCaster(t, 4000)
	defer cleanup()
	before := u.Character.Conviction

	msgs := castRaise(t, u, &rooms.Room{RoomId: 999906}, "testraise-order")

	if !strings.Contains(msgs, "no suitable remains") {
		t.Errorf("an empty room must be refused for the missing corpse; got:\n%s", msgs)
	}
	if u.Character.Conviction != before {
		t.Errorf("a refused raise charged conviction: had %d, left with %d", before, u.Character.Conviction)
	}
}

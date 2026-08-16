package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// A refused binding must cost the caster nothing.
//
// Spell conviction is charged per round as the cast CHANNELS (the upkeep
// ApplyCost in internal/hooks/combat_shared_helpers.go), while the U7b
// reservation gates in companion_summon.go and charm_spell.go sit at
// RESOLUTION. A doomed summon therefore channelled to completion, spent the
// pool, and only then refused: an adversarial playtest measured a full 30
// conviction for a refused water elemental and 42 for a refused magma, three
// times running, one of which also rolled skill progression on the way. Both
// `help companion` and `help manifestation` promise the refusal costs nothing.
//
// The fix gates at cast INITIATION, so the fixture below never enters the
// Casting state at all and no upkeep round can run.

const csrUserId = 9187

func seedSummonCaster(t *testing.T, convictionMax int) (*users.UserRecord, *rooms.Room, func()) {
	t.Helper()

	u := users.NewTestUser(csrUserId, "binder", "Binder", uint64(csrUserId))
	u.Character.Activity = activity.NewMachine()
	u.Character.ConvictionMax.Value = convictionMax
	u.Character.Conviction = convictionMax
	u.Character.Skills = map[string]int{"manifestation": 5}
	u.Character.SpellBook = map[string]int{"testsummon-ceiling": 1}

	cleanSpells := spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"testsummon-ceiling": {
			SpellId:             "testsummon-ceiling",
			Name:                "Test Summon Ceiling",
			Type:                spells.Neutral,
			Schools:             []string{spells.SchoolManifestation},
			BaseFolds:           4,
			Cost:                5,
			SummonMobId:         1,
			SummonPetMultiplier: 1.0,
		},
	})
	cleanUsers := users.SeedUsersForTest(map[int]*users.UserRecord{csrUserId: u})
	room := &rooms.Room{RoomId: 999903}
	events.DrainQueuedMessagesForTest(csrUserId)

	return u, room, func() {
		events.DrainQueuedMessagesForTest(csrUserId)
		cleanUsers()
		cleanSpells()
	}
}

func TestCast_RefusedSummon_ChargesNoConviction(t *testing.T) {
	// A small pool, so one ordinary companion costs more than the whole
	// ceiling allows on its own. This is the playtest's exact situation: a
	// FIRST companion, at zero reservation, refused outright.
	u, room, cleanup := seedSummonCaster(t, 50)
	defer cleanup()

	reserve := u.Character.CalcCompanionReserve(characters.CompanionReserveBase(1.0))
	if !u.Character.WouldBreachReservationCap(characters.PoolConviction, reserve) {
		t.Fatalf("fixture: reserve %d must breach the cap %d, or the test proves nothing",
			reserve, u.Character.ReservationCap(characters.PoolConviction))
	}
	before := u.Character.Conviction

	handled, err := Cast("testsummon-ceiling", u, room, 0)
	if err != nil || !handled {
		t.Fatalf("cast errored: handled=%v err=%v", handled, err)
	}

	if u.Character.Conviction != before {
		t.Errorf("a refused summon charged conviction: had %d, left with %d",
			before, u.Character.Conviction)
	}
	if u.Character.Activity != nil && u.Character.Activity.IsCasting() {
		t.Error("a refused summon must never enter the Casting state; every " +
			"round it channels charges the pool it was refused for")
	}

	msgs := strings.Join(events.DrainQueuedMessagesForTest(csrUserId), "\n")
	if !strings.Contains(msgs, "reserve") {
		t.Errorf("the refusal must name reservation as the cause; got:\n%s", msgs)
	}
	if !strings.Contains(msgs, "That alone") {
		t.Errorf("a first companion at zero reservation must be told the DEMAND "+
			"is too large, not that it already holds something; got:\n%s", msgs)
	}
}

// The gate must bite only on a genuine breach. With room to spare the same
// spell is not refused, or the fix would simply have broken summoning.
func TestCast_AffordableSummon_IsNotRefused(t *testing.T) {
	u, room, cleanup := seedSummonCaster(t, 4000)
	defer cleanup()

	reserve := u.Character.CalcCompanionReserve(characters.CompanionReserveBase(1.0))
	if u.Character.WouldBreachReservationCap(characters.PoolConviction, reserve) {
		t.Fatalf("fixture: reserve %d must FIT under the cap %d",
			reserve, u.Character.ReservationCap(characters.PoolConviction))
	}

	handled, err := Cast("testsummon-ceiling", u, room, 0)
	if err != nil || !handled {
		t.Fatalf("cast errored: handled=%v err=%v", handled, err)
	}

	msgs := strings.Join(events.DrainQueuedMessagesForTest(csrUserId), "\n")
	if strings.Contains(msgs, "in reserve") || strings.Contains(msgs, "That alone") {
		t.Errorf("an affordable summon must not be refused by the ceiling; got:\n%s", msgs)
	}
}

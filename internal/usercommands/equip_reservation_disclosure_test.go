package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// Equipping a reserving item was completely silent.
//
// A 2026-08-15 playtest put on gear that took health, stamina and conviction
// into reserve, heard nothing, and met a refusal several actions later with no
// way to connect the two. This drives the real `equip` command, because the
// disclosure being CORRECT in internal/characters is worth nothing if the
// command never asks for it.

// seedDisclosureItems layers the two items this file needs over whatever
// seedAllRegistries put in place. SeedItemsForTest replaces the whole registry,
// so both the reserving and the plain item live here together.
func seedDisclosureItems(t *testing.T) func() {
	t.Helper()
	return items.SeedItemsForTest(map[int]*items.ItemSpec{
		999960: {
			ItemId:               999960,
			Name:                 "draining collar",
			NameSimple:           "collar",
			Description:          "A collar that drinks from its wearer.",
			Type:                 items.Neck,
			Subtype:              items.Wearable,
			ReserveHealthPct:     0.20,
			ReserveStaminaPct:    0.10,
			ReserveConvictionPct: 0.10,
		},
		999961: {
			ItemId:      999961,
			Name:        "plain pendant",
			NameSimple:  "pendant",
			Description: "An ordinary pendant.",
			Type:        items.Neck,
			Subtype:     items.Wearable,
		},
	})
}

func equipAndCollect(t *testing.T, itemId int, name string) []string {
	t.Helper()

	user, room := getTestUserAndRoom(t)
	user.Character.Equipment.Neck = items.Item{}

	// NewTestUser writes the pool maxima straight to .Value, which is DERIVED:
	// the Validate() inside the equip path recomputes it from .Base and would
	// leave every pool at 1, making a percentage reservation floor to nothing.
	user.Character.HealthMax.Base = 400
	user.Character.StaminaMax.Base = 200
	user.Character.ConvictionMax.Base = 500
	user.Character.Validate(true)

	user.Character.StoreItem(items.Item{ItemId: itemId})
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Equip(name, user, room, 0)
	if err != nil || !handled {
		t.Fatalf("equip errored: handled=%v err=%v", handled, err)
	}
	if user.Character.Equipment.Neck.ItemId != itemId {
		t.Fatalf("fixture: %q did not end up worn (neck holds %d)", name,
			user.Character.Equipment.Neck.ItemId)
	}

	return events.DrainQueuedMessagesForTest(user.UserId)
}

func findReservationLine(msgs []string) string {
	for _, m := range msgs {
		if strings.Contains(m, "in reserve") {
			return m
		}
	}
	return ``
}

func TestEquip_DisclosesReservationWhenGearTakesAShare(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedDisclosureItems(t)()

	line := findReservationLine(equipAndCollect(t, 999960, "draining collar"))
	if line == `` {
		t.Fatal("equipping an item that reserves health, stamina and conviction " +
			"said nothing about reservation")
	}

	for _, pool := range []string{"health", "stamina", "conviction"} {
		if !strings.Contains(line, pool) {
			t.Errorf("the equip line does not name %s: %q", pool, line)
		}
	}
	if strings.ContainsAny(line, "0123456789") {
		t.Errorf("the equip line shows a raw number: %q", line)
	}
}

func TestEquip_SaysNothingWhenNothingIsReserved(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedDisclosureItems(t)()

	if line := findReservationLine(equipAndCollect(t, 999961, "plain pendant")); line != `` {
		t.Errorf("an ordinary equip produced a reservation disclosure: %q", line)
	}
}

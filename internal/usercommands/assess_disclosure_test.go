package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Full-command check of the assess reservation disclosure: supported forms
// come from the raise spells' own corpse-pool gates, and each band line
// describes the Conviction reservation without leaking a raw number.

const adUserId = 9131

func seedAssessFixture(t *testing.T, convictionMax int) (*users.UserRecord, *rooms.Room, func()) {
	t.Helper()

	cleanSpells := spells.SeedSpellsForTest(map[string]*spells.SpellData{
		// Reserve is derived, never authored: the pet multiplier is the only
		// dial. Skeleton 0.50 and golem 1.00 mirror the live YAMLs, giving
		// bases of 140 and 280 against CompanionReserveDefault 280.
		"raise-skeleton": {
			SpellId: "raise-skeleton", Name: "Raise Skeleton",
			SummonMobId: 300, SummonRequiresCorpse: true,
			SummonMinCorpsePool: 30, SummonPetMultiplier: 0.50,
		},
		"raise-golem": {
			SpellId: "raise-golem", Name: "Raise Golem",
			SummonMobId: 305, SummonRequiresCorpse: true,
			SummonMinCorpsePool: 500, SummonPetMultiplier: 1.00,
		},
	})

	u := users.NewTestUser(adUserId, "assessor", "Assessor", uint64(adUserId))
	u.Character.ConvictionMax.Value = convictionMax
	cleanUsers := users.SeedUsersForTest(map[int]*users.UserRecord{adUserId: u})

	// Deliberately a bare Character rather than characters.New(): assess scores
	// a corpse with StatPoolTotal, which is sum(Base) - speciesBase +
	// sum(Training). New() rolls a gaussian Base for all six stats, and this
	// test binary has no species roster to cancel it against, so a New()
	// fixture would score ~640 instead of the 40 the spell gates below are
	// written for — and would vary run to run. A real mob corpse carries its
	// species baseline in Base and cancels exactly; species 0 here contributes
	// no baseline, so the pool is precisely the authored Training.
	corpse := rooms.Corpse{
		MobId: 1234,
	}
	corpse.Character.Name = "goblin"
	corpse.Character.Stats.Strength.Training = 20
	corpse.Character.Stats.Vitality.Training = 20 // total 40 → skeleton only

	room := &rooms.Room{RoomId: 999901, Corpses: []rooms.Corpse{corpse}}

	events.DrainQueuedMessagesForTest(adUserId)
	return u, room, func() {
		events.DrainQueuedMessagesForTest(adUserId)
		cleanUsers()
		cleanSpells()
	}
}

func TestAssess_DisclosesReservationBand(t *testing.T) {
	// Big pool: the skeleton's base 140 at manifestation 0 costs 140 * 1.10 =
	// 154. The band is measured against the ceiling (2640 on a 4000 pool), and
	// 154 is under a sixteenth of that → "a slight part".
	u, room, cleanup := seedAssessFixture(t, 4000)
	defer cleanup()

	handled, err := Assess("goblin", u, room, events.CmdSecretly)
	if err != nil || !handled {
		t.Fatalf("Assess failed: handled=%v err=%v", handled, err)
	}

	msgs := strings.Join(events.DrainQueuedMessagesForTest(adUserId), "\n")
	if !strings.Contains(msgs, "It could sustain: skeleton.") {
		t.Errorf("supported list wrong (want skeleton only from the spell gates):\n%s", msgs)
	}
	if !strings.Contains(msgs, "Raising a skeleton would set aside a slight part of your conviction") {
		t.Errorf("missing/incorrect reservation disclosure:\n%s", msgs)
	}
	if strings.Contains(msgs, "could not spare") {
		t.Errorf("affordable reservation flagged as unaffordable:\n%s", msgs)
	}
	if strings.Contains(msgs, "154") {
		t.Errorf("raw reserve number leaked to the player:\n%s", msgs)
	}
}

func TestAssess_FlagsUnaffordableReservation(t *testing.T) {
	// Tiny pool: the same 154 reserve on a 150 max is more than the whole
	// pool, so it is both over-pool and past the reservation ceiling.
	u, room, cleanup := seedAssessFixture(t, 150)
	defer cleanup()

	handled, err := Assess("goblin", u, room, events.CmdSecretly)
	if err != nil || !handled {
		t.Fatalf("Assess failed: handled=%v err=%v", handled, err)
	}

	msgs := strings.Join(events.DrainQueuedMessagesForTest(adUserId), "\n")
	if !strings.Contains(msgs, "more than your spirit could hold") {
		t.Errorf("over-pool reservation not described as such:\n%s", msgs)
	}
	if !strings.Contains(msgs, "could not spare") {
		t.Errorf("unaffordable reservation not flagged:\n%s", msgs)
	}
}

package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const tgUserId = 9122

func seedTalkGoldUser(t *testing.T, gold int) (*users.UserRecord, func()) {
	t.Helper()
	u := users.NewTestUser(tgUserId, "talkgold", "Talkgold", uint64(tgUserId))
	u.Character.Gold = gold
	clean := users.SeedUsersForTest(map[int]*users.UserRecord{tgUserId: u})
	events.DrainQueuedMessagesForTest(tgUserId)
	return u, func() {
		events.DrainQueuedMessagesForTest(tgUserId)
		clean()
	}
}

// buildPlayerState's ChargeGold backs dialogue chargesGold: it takes the
// whole price or nothing, and names the amount with a thousands separator.
func TestDialogueChargeGold_TakesAllOrNothing(t *testing.T) {
	u, cleanup := seedTalkGoldUser(t, 1400)
	defer cleanup()
	ps := buildPlayerState(u)

	if ps.ChargeGold(25000) {
		t.Error("ChargeGold reported success for a charge larger than the player's gold")
	}
	if u.Character.Gold != 1400 {
		t.Errorf("gold = %d; want 1400 after a refused charge", u.Character.Gold)
	}
	if msgs := strings.Join(events.DrainQueuedMessagesForTest(tgUserId), "\n"); strings.Contains(msgs, "You pay") {
		t.Errorf("a refused charge told the player they paid:\n%s", msgs)
	}

	u.Character.Gold = 25000
	if !ps.ChargeGold(25000) {
		t.Fatal("ChargeGold refused an affordable charge")
	}
	if u.Character.Gold != 0 {
		t.Errorf("gold = %d; want 0", u.Character.Gold)
	}
	if msgs := strings.Join(events.DrainQueuedMessagesForTest(tgUserId), "\n"); !strings.Contains(msgs, "25,000 gold") {
		t.Errorf("payment line should read 25,000 gold:\n%s", msgs)
	}
}

func TestDialogueGiveGold_ThousandsSeparator(t *testing.T) {
	u, cleanup := seedTalkGoldUser(t, 0)
	defer cleanup()
	buildPlayerState(u).GiveGold(12500)
	if msgs := strings.Join(events.DrainQueuedMessagesForTest(tgUserId), "\n"); !strings.Contains(msgs, "12,500 gold") {
		t.Errorf("gift line should read 12,500 gold:\n%s", msgs)
	}
}

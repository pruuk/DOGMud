package users

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/presence"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// presenceUser is a menu test user whose presence machine sits in want.
func presenceUser(t *testing.T, want presence.State) *UserRecord {
	t.Helper()
	u := menuUser()
	u.Character.Presence = presence.NewPlayerPresence()
	if err := u.Character.Presence.TransitionTo(presence.Active, state.TransitionReason{}); err != nil {
		t.Fatalf("to Active: %v", err)
	}
	switch want {
	case presence.Idle:
		if err := u.Character.Presence.TransitionTo(presence.Idle, state.TransitionReason{}); err != nil {
			t.Fatalf("to Idle: %v", err)
		}
	case presence.AFK:
		if err := u.Character.Presence.TransitionToAFK(presence.AFKData{Manual: true}, state.TransitionReason{}); err != nil {
			t.Fatalf("to AFK: %v", err)
		}
	}
	return u
}

func TestNoteActivity_RecordsTheInputRoundAndWakesIdle(t *testing.T) {
	orig := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCountForTest(orig) })
	util.SetRoundCountForTest(424242)

	u := presenceUser(t, presence.Idle)
	u.SetLastInputRound(1)
	u.NoteActivity()
	if got := u.GetLastInputRound(); got != 424242 {
		t.Fatalf("last input round %d, want 424242", got)
	}
	if s := u.Character.Presence.State(); s != presence.Active {
		t.Fatalf("presence %v, want Active", s)
	}
}

func TestWakePresence_ManualAFKSaysSo(t *testing.T) {
	u := presenceUser(t, presence.AFK)
	events.DrainQueuedMessagesForTest(u.UserId)
	u.WakePresence()
	if s := u.Character.Presence.State(); s != presence.Active {
		t.Fatalf("presence %v, want Active", s)
	}
	msgs := strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "")
	if !strings.Contains(msgs, "You are no longer AFK.") {
		t.Fatalf("messages %q", msgs)
	}
}

func TestWakePresence_ActiveIsQuiet(t *testing.T) {
	u := presenceUser(t, presence.Active)
	events.DrainQueuedMessagesForTest(u.UserId)
	u.WakePresence()
	if msgs := events.DrainQueuedMessagesForTest(u.UserId); len(msgs) != 0 {
		t.Fatalf("messages %q", msgs)
	}
}

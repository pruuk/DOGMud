package menu

import "testing"

func TestInterruptLine(t *testing.T) {
	cases := map[CloseReason][2]string{
		CloseCombat:      {"You break off as the fight begins.", "You break off your talk with Kael as the fight begins."},
		CloseBusy:        {"You break off to focus on your work.", "You break off your talk with Kael to focus on your work."},
		CloseControlled:  {"You break off as you are seized.", "You break off your talk with Kael as you are seized."},
		CloseAsleep:      {"You break off as you fall asleep.", "You break off your talk with Kael as you fall asleep."},
		CloseQuitting:    {"You break off to meditate.", "You break off your talk with Kael to meditate."},
		CloseRoomChange:  {"You break off as you leave.", "You break off your talk with Kael as you leave."},
		CloseContextLost: {"You break off.", "You break off your talk with Kael."},
	}
	for r, want := range cases {
		if got := InterruptLine(r, ""); got != want[0] {
			t.Errorf("%v generic: %q, want %q", r, got, want[0])
		}
		if got := InterruptLine(r, "talk with Kael"); got != want[1] {
			t.Errorf("%v topic: %q, want %q", r, got, want[1])
		}
	}
	for _, silent := range []CloseReason{CloseNone, CloseChosen, CloseLeft, CloseOtherCommand, CloseReplaced, CloseDead, CloseFollow} {
		if got := InterruptLine(silent, "talk with Kael"); got != "" {
			t.Errorf("%v should be silent, got %q", silent, got)
		}
	}
}

func TestFollowLines(t *testing.T) {
	f, l, r := FollowLines("Mira", "Brannoc", "talk with Kael")
	if f != "You break off your talk with Kael to follow Brannoc." ||
		l != "Mira breaks off a talk with Kael to follow you." ||
		r != "Mira breaks off a talk with Kael and follows Brannoc." {
		t.Fatalf("topic lines: %q / %q / %q", f, l, r)
	}
	f, l, r = FollowLines("Mira", "Brannoc", "")
	if f != "You stop what you were doing to follow Brannoc." ||
		l != "Mira stops what they were doing to follow you." ||
		r != "Mira stops what they were doing and follows Brannoc." {
		t.Fatalf("generic lines: %q / %q / %q", f, l, r)
	}
}

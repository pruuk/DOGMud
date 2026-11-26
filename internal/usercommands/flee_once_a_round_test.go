package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// nextFleeRound moves the round clock on one round. A flee may be started
// once every round (#951), so a test that stands for several rounds of
// fleeing must start each attempt in a round of its own.
func nextFleeRound(t *testing.T) {
	t.Helper()
	util.SetRoundCountForTest(util.GetRoundCount() + 1)
	t.Cleanup(util.ResetRoundCountForTest)
}

// #951: a second flee in the round a flee was started in (and here also
// failed in) is refused with its own line and costs nothing; the next
// round's flee goes ahead.
func TestFlee_SecondAttemptInARoundIsRefusedWithItsLine(t *testing.T) {
	util.SetRoundCountForTest(2_000_000)
	t.Cleanup(util.ResetRoundCountForTest)
	u, room, cleanup := fleeFixture(t, 9261, 99853, 0)
	defer cleanup()
	engageFleeFixture(t, u)

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}
	// The fixture room has no exits, so the attempt fails back into the fight.
	if out := actions.ResolveFlee(actions.NewUserActorInRoom(u, room), room); !out.NoExit {
		t.Fatalf("fixture: outcome %+v, want a NoExit failure", out)
	}
	events.DrainQueuedMessagesForTest(u.UserId)
	before := u.Character.Stamina

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}
	if u.Character.Stamina != before {
		t.Errorf("refused flee spent stamina: %d -> %d", before, u.Character.Stamina)
	}
	if u.Character.IsDisengaging() {
		t.Error("refused flee entered Disengaging")
	}
	want := fleeRefusalText[actions.FleeRefuseThisRound]
	got := strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "\n")
	if want == "" || !strings.Contains(got, want) {
		t.Fatalf("second flee said %q, want the once-a-round line %q", got, want)
	}
	if strings.Contains(got, "attempt to flee") {
		t.Errorf("refused flee claimed an attempt: %q", got)
	}

	util.SetRoundCountForTest(2_000_001)
	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}
	if !u.Character.IsDisengaging() {
		t.Fatal("flee in the next round was not accepted")
	}
}

// Every refusal BeginFlee can return has a line, so no refused flee goes
// silent for a player.
func TestFlee_EveryRefusalHasALine(t *testing.T) {
	for r := actions.FleeRefuseRooted; r <= actions.FleeRefuseThisRound; r++ {
		if fleeRefusalText[r] == "" {
			t.Errorf("FleeRefusal %d has no player line", r)
		}
	}
}

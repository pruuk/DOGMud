package forager

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
)

func TestForagerArrivalListener_IgnoresNonForagerVendorEvents(t *testing.T) {
	e := events.PatrolWaypointArrival{
		MobInstanceId: 12345,
		PatrolId:      "marsh_forager_delivery",
		ArrivalEvent:  "",
	}
	got := ForagerArrivalListener(e)
	if got != events.Continue {
		t.Errorf("expected events.Continue for non-forager_vendor event, got %v", got)
	}
}

func TestForagerArrivalListener_IgnoresMissingMob(t *testing.T) {
	e := events.PatrolWaypointArrival{
		MobInstanceId: 999999, // not a live instance
		PatrolId:      "marsh_forager_delivery",
		ArrivalEvent:  "forager_vendor",
		RoomId:        4102,
	}
	got := ForagerArrivalListener(e)
	if got != events.Continue {
		t.Errorf("expected events.Continue when mob missing, got %v", got)
	}
}

func TestForagerArrivalListener_IgnoresNonPatrolWaypointArrivalEvent(t *testing.T) {
	// The listener is registered on PatrolWaypointArrival; if some
	// other event somehow gets routed here, it should no-op cleanly.
	got := ForagerArrivalListener(events.PatrolCompleted{
		MobInstanceId: 12345,
		PatrolId:      "marsh_forager_delivery",
		RoomId:        4102,
	})
	if got != events.Continue {
		t.Errorf("expected events.Continue for non-PatrolWaypointArrival event, got %v", got)
	}
}

package mobs

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
)

// TestOffShift_FollowsTheSchedule: a keeper whose home room (where the shop
// is keyed) is the forge, 5678 in the Kerra fixture, is on shift there during
// the forge segment and off shift in every awake segment that sends her to
// another room, or whenever she stands somewhere other than home. Asleep she
// is not off shift: sleep refuses trade on its own line.
func TestOffShift_FollowsTheSchedule(t *testing.T) {
	s := kerraTestSchedule()
	registerScheduleForTest(s)
	defer unregisterScheduleForTest(s.Id)

	m := &Mob{ScheduleId: s.Id, HomeRoomId: 5678}
	m.Character.Conditions = conditions.New()
	m.Character.RoomId = 5678

	for hour, want := range map[int]bool{7: true, 10: false, 17: false, 19: true, 23: true} {
		if got := m.offShiftAt(hour); got != want {
			t.Errorf("at the forge, hour %d: OffShift = %v, want %v", hour, got, want)
		}
	}
	m.Character.RoomId = 9012 // in the tavern during forge hours
	if !m.offShiftAt(10) {
		t.Error("away from home in forge hours must be off shift")
	}

	none := &Mob{HomeRoomId: 5678}
	none.Character.RoomId = 9012
	if none.offShiftAt(19) {
		t.Error("a mob with no schedule is never off shift")
	}
}

func TestValidateSchedule_OffShiftLine(t *testing.T) {
	s := kerraTestSchedule()
	s.Segments[2].OffShiftLine = "Not here; come by the forge."
	if err := validateScheduleStandalone(s); err == nil || !strings.Contains(err.Error(), "offshift_line") {
		t.Errorf("a ';' in offshift_line must be refused, got %v", err)
	}
	s.Segments[2].OffShiftLine = "Not here. Come by the forge tomorrow."
	if err := validateScheduleStandalone(s); err != nil {
		t.Errorf("a clean offshift_line must validate, got %v", err)
	}
	s.Segments[3].Activity = "sleeping"
	s.Segments[3].OffShiftLine = "Go away."
	if err := validateScheduleStandalone(s); err == nil || !strings.Contains(err.Error(), "sleeping") {
		t.Errorf("an offshift_line on a sleeping segment must be refused, got %v", err)
	}
}

package justice

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	stampTestUserId = 8841
	stampTestRoomA  = 9841
	stampTestRoomB  = 9842
	stampTestRound  = uint64(1000)
)

// arrestStampScene is a wanted player (surrender policy) in room A with
// three guards of one cell-owning faction, every justice seam pinned. It
// counts the declarations the guards speak and records whom they haul.
type arrestStampScene struct {
	player       *users.UserRecord
	roomA, roomB *rooms.Room
	guards       []*mobs.Mob
	declared     int
	hauled       []int
}

func newArrestStampScene(t *testing.T) *arrestStampScene {
	t.Helper()
	s := &arrestStampScene{}

	u := users.NewTestUser(stampTestUserId, "wanted", "Wanted", 0)
	u.Character.ArrestPolicy = characters.ArrestSurrender
	u.Character.RoomId = stampTestRoomA
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{stampTestUserId: u}))
	s.player = u

	s.roomA = &rooms.Room{RoomId: stampTestRoomA}
	s.roomB = &rooms.Room{RoomId: stampTestRoomB}
	s.roomA.AddPlayer(stampTestUserId)
	t.Cleanup(func() {
		s.roomA.RemovePlayer(stampTestUserId)
		s.roomB.RemovePlayer(stampTestUserId)
	})

	for i := 0; i < 3; i++ {
		g := &mobs.Mob{MobId: 1, InstanceId: 8850 + i, Groups: []string{"guard", "test_guards"}}
		g.Character.Name = "Guard"
		s.guards = append(s.guards, g)
	}

	origFactions, origBounty, origCell := guardFactionsFn, openFactionBountyFn, cellRoomFn
	origSay, origArrest := guardSayFn, executeArrestFn
	t.Cleanup(func() {
		guardFactionsFn, openFactionBountyFn, cellRoomFn = origFactions, origBounty, origCell
		guardSayFn, executeArrestFn = origSay, origArrest
	})
	guardFactionsFn = func(*mobs.Mob) []string { return []string{"test_guards"} }
	openFactionBountyFn = func(int, map[string]bool) bool { return true } // wanted: SeverityArrest
	cellRoomFn = func(string) int { return 5106 }
	guardSayFn = func(_ *rooms.Room, _ *mobs.Mob, line string) {
		if strings.Contains(line, "under arrest") {
			s.declared++
		}
	}
	executeArrestFn = func(_ *characters.Character, userId int, _ string, _ bool) bool {
		s.hauled = append(s.hauled, userId)
		return true
	}
	return s
}

// moveTo walks the player into room `to` the way rooms.MoveToRoom does:
// out of the old room, into the new one, then the RoomChange event its
// listeners hear (hooks.RegisterListeners wires LapseArrestStampOnMove).
func (s *arrestStampScene) moveTo(to *rooms.Room) {
	from := s.roomA
	if s.player.Character.RoomId == stampTestRoomB {
		from = s.roomB
	}
	from.RemovePlayer(stampTestUserId)
	to.AddPlayer(stampTestUserId)
	s.player.Character.RoomId = to.RoomId
	LapseArrestStampOnMove(events.RoomChange{
		UserId:     stampTestUserId,
		FromRoomId: from.RoomId,
		ToRoomId:   to.RoomId,
	})
}

func (s *arrestStampScene) stamp() (round, room uint64, ok bool) {
	round, okRound := miscDataRound(s.player.Character.MiscData, keyArrestPendingRound)
	room, okRoom := miscDataRound(s.player.Character.MiscData, keyArrestPendingRoom)
	return round, room, okRound && okRoom
}

// #241: every guard in the room stamped its own MiscData and declared on its
// own, so three guards spoke three declarations in one round. The stamp is
// the player's: the first guard declares, the others see it and wait.
func TestArrestStamp_ThreeGuardsDeclareOnce(t *testing.T) {
	s := newArrestStampScene(t)
	for _, g := range s.guards {
		RunGuardEnforcement(g, s.roomA, stampTestRound)
	}
	if s.declared != 1 {
		t.Fatalf("three guards declared %d times in one round, want 1", s.declared)
	}
	if round, room, ok := s.stamp(); !ok || round != stampTestRound || room != stampTestRoomA {
		t.Fatalf("player stamp = round %d room %d (ok %v), want round %d room %d", round, room, ok, stampTestRound, stampTestRoomA)
	}
	for _, g := range s.guards {
		for k := range g.Character.MiscData {
			if strings.HasPrefix(k, "justice_arrest_pending") {
				t.Errorf("guard %d carries %s; the stamp belongs on the player", g.InstanceId, k)
			}
		}
	}

	for _, g := range s.guards {
		RunGuardEnforcement(g, s.roomA, stampTestRound+1)
	}
	if s.declared != 1 || len(s.hauled) != 0 {
		t.Fatalf("within the grace: %d declarations, %d hauls; want 1 and 0", s.declared, len(s.hauled))
	}
}

// Staying put through the grace is still an arrest: any guard in the room
// hauls once it has run, and the haul clears the stamp.
func TestArrestStamp_StayingThroughTheGraceIsHauled(t *testing.T) {
	s := newArrestStampScene(t)
	RunGuardEnforcement(s.guards[0], s.roomA, stampTestRound)
	RunGuardEnforcement(s.guards[1], s.roomA, stampTestRound+arrestGraceRounds())

	if len(s.hauled) != 1 || s.hauled[0] != stampTestUserId {
		t.Fatalf("hauled %v, want the player once", s.hauled)
	}
	if s.declared != 1 {
		t.Errorf("declared %d times, want 1", s.declared)
	}
	if _, _, ok := s.stamp(); ok {
		t.Errorf("the haul must clear the stamp")
	}
}

// The defect from the issue: declared, the player walked off, and the same
// guard met later somewhere else hauled them at once with no new word. A
// declaration holds only in its room: elsewhere the guard declares afresh.
func TestArrestStamp_AGuardElsewhereDeclaresAfresh(t *testing.T) {
	s := newArrestStampScene(t)
	RunGuardEnforcement(s.guards[0], s.roomA, stampTestRound)
	s.moveTo(s.roomB)
	later := stampTestRound + arrestGraceRounds() + 1
	RunGuardEnforcement(s.guards[0], s.roomB, later)

	if len(s.hauled) != 0 {
		t.Fatalf("hauled %v in another room with no new declaration", s.hauled)
	}
	if s.declared != 2 {
		t.Fatalf("declared %d times, want a fresh declaration (2)", s.declared)
	}
	if round, room, ok := s.stamp(); !ok || round != later || room != stampTestRoomB {
		t.Fatalf("player stamp = round %d room %d (ok %v), want round %d room %d", round, room, ok, later, stampTestRoomB)
	}
}

// A declaration lapses when the player walks off during the grace (owner
// call), even if they step back into the same room inside the window: the
// next sighting declares afresh rather than hauling on the old word.
func TestArrestStamp_LeavingAndComingBackDeclaresAfresh(t *testing.T) {
	s := newArrestStampScene(t)
	RunGuardEnforcement(s.guards[0], s.roomA, stampTestRound)
	s.moveTo(s.roomB)
	s.moveTo(s.roomA)
	back := stampTestRound + arrestGraceRounds()
	RunGuardEnforcement(s.guards[0], s.roomA, back)

	if len(s.hauled) != 0 {
		t.Fatalf("hauled %v on a declaration the player walked away from", s.hauled)
	}
	if s.declared != 2 {
		t.Fatalf("declared %d times, want a fresh declaration (2)", s.declared)
	}
	if round, room, ok := s.stamp(); !ok || round != back || room != stampTestRoomA {
		t.Fatalf("player stamp = round %d room %d (ok %v), want round %d room %d", round, room, ok, back, stampTestRoomA)
	}
}

// Only leaving the declared room lapses it. The RoomChange for the player's
// own arrival into that room can be heard after a guard has already declared
// there; it must not drop the fresh stamp, or the next guard declares twice.
func TestArrestStamp_ArrivalDoesNotLapseTheStamp(t *testing.T) {
	s := newArrestStampScene(t)
	RunGuardEnforcement(s.guards[0], s.roomA, stampTestRound)
	LapseArrestStampOnMove(events.RoomChange{UserId: stampTestUserId, FromRoomId: stampTestRoomB, ToRoomId: stampTestRoomA})
	LapseArrestStampOnMove(events.RoomChange{MobInstanceId: 8850, FromRoomId: stampTestRoomA, ToRoomId: stampTestRoomB})
	RunGuardEnforcement(s.guards[1], s.roomA, stampTestRound+arrestGraceRounds())

	if s.declared != 1 || len(s.hauled) != 1 {
		t.Fatalf("%d declarations and %d hauls; want 1 and 1", s.declared, len(s.hauled))
	}
}

// A declaration in the same room lapses too, once it is older than the
// grace plus a window as long again: the guard declares afresh.
func TestArrestStamp_AnOldDeclarationLapses(t *testing.T) {
	s := newArrestStampScene(t)
	RunGuardEnforcement(s.guards[0], s.roomA, stampTestRound)
	RunGuardEnforcement(s.guards[0], s.roomA, stampTestRound+2*arrestGraceRounds()+1)

	if len(s.hauled) != 0 || s.declared != 2 {
		t.Fatalf("%d hauls and %d declarations; want 0 and 2", len(s.hauled), s.declared)
	}
}

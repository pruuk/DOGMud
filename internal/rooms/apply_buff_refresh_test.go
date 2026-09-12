package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Buff ids clear of other rooms package fixtures (sightTestInfraredBuffId is
// 7401).
const (
	refreshTestHeldBuffId    = 7301 // "Test Zone Press": already held, must refresh
	refreshTestGrantedBuffId = 7302 // "Test Zone Grant": not yet held, event path
)

// refreshTestSetTriggersLeft mutates a held buff's remaining triggers
// directly, mirroring internal/hooks/buff_room_text_test.go's unexported
// expire() helper (that helper is not visible outside package hooks, so it
// is mirrored here rather than imported).
func refreshTestSetTriggersLeft(t *testing.T, list []*buffs.Buff, buffId, left int) {
	t.Helper()
	for _, b := range list {
		if b.BuffId == buffId {
			b.TriggersLeft = left
			return
		}
	}
	t.Fatalf("buff %d not found on held list", buffId)
}

// A room mutator's playerbuffids run every round. A player who already holds
// the buff must have it REFRESHED (TriggersLeft reset), not skipped until it
// lapses and gets re-added a round later: the skip is what turned slice C's
// authored notices into a start/end loop every few rounds. A player who does
// not yet hold the buff still goes through the normal grant path, which is
// the async events.Buff queue (UserRecord.AddBuff only enqueues; it does not
// apply the buff synchronously), so it must NOT already show up on
// Character.HasBuff immediately after the call.
func TestApplyBuffIdToPlayers_HeldBuffIsRefreshedNotRelapsed(t *testing.T) {
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		refreshTestHeldBuffId:    {BuffId: refreshTestHeldBuffId, Name: "Test Zone Press", TriggerCount: 3, RoundInterval: 1},
		refreshTestGrantedBuffId: {BuffId: refreshTestGrantedBuffId, Name: "Test Zone Grant", TriggerCount: 3, RoundInterval: 1},
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7311: users.NewTestUser(7311, "presskeeper", "Presskeeper", 97311),
		7312: users.NewTestUser(7312, "newcomer", "Newcomer", 97312),
	}))

	r := &Room{RoomId: 7310}
	r.AddPlayer(7311)
	r.AddPlayer(7312)

	held := users.GetByUserId(7311)
	if err := held.Character.AddBuff(refreshTestHeldBuffId, false); err != nil {
		t.Fatalf("precondition: could not grant the held buff: %v", err)
	}
	// Drive it down as if two of its three triggers had already fired.
	refreshTestSetTriggersLeft(t, held.Character.Buffs.List, refreshTestHeldBuffId, 1)

	newcomer := users.GetByUserId(7312)
	if newcomer.Character.HasBuff(refreshTestGrantedBuffId) {
		t.Fatal("precondition: the newcomer should not start with the granted buff")
	}

	// events.Buff carries no per-instance payload beyond ids, and this test's
	// package (rooms) cannot process the queue into Message events without
	// importing package hooks, which would be a cycle (hooks already imports
	// rooms). There is also no DrainQueuedBuffForTest helper in package
	// events (grepped: only Message, Broadcast, CharacterDied, Input,
	// PlayerAttackedMob, PatrolWaypointArrival/Completed, ItemOwnership,
	// VitalsChanged, SkillUsed have one). So "no start event was queued" is
	// verified behaviorally below rather than by draining the queue: this
	// call is a hooks-free step, so if it ever went through the async event
	// path for the held buff, TriggersLeft would still read back as 1 (the
	// event would sit unprocessed), not 3. Reading 3 back synchronously is
	// only possible through the direct Character.AddBuff refresh call. The
	// messages queue is also drained and asserted empty, as an additional,
	// admittedly weaker, confirmatory check (weaker because nothing in this
	// test turns a queued events.Buff into a Message either way).
	events.DrainQueuedMessagesForTest(7311)
	events.DrainQueuedMessagesForTest(7312)

	r.ApplyBuffIdToPlayers([]int{refreshTestHeldBuffId, refreshTestGrantedBuffId}, "area")

	if got := len(held.Character.Buffs.List); got != 1 {
		t.Fatalf("held player's buff list has %d entries, want 1 (refreshed in place, not removed and re-added)", got)
	}
	if got := held.Character.Buffs.List[0].TriggersLeft; got != 3 {
		t.Errorf("held player's TriggersLeft = %d, want 3 (refreshed synchronously)", got)
	}

	if got := events.DrainQueuedMessagesForTest(7311); len(got) != 0 {
		t.Errorf("held player received queued messages %v, want none: a refresh queues no event and so renders no start text", got)
	}

	// The player without the buff goes through the async grant path
	// (UserRecord.AddBuff), which only enqueues events.Buff; it does not
	// apply the buff synchronously. Confirming it did NOT appear yet is what
	// distinguishes this call from the held player's synchronous refresh.
	if newcomer.Character.HasBuff(refreshTestGrantedBuffId) {
		t.Error("newcomer already carries the granted buff synchronously; expected the async event path (grant not yet applied)")
	}
}

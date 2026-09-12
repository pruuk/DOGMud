package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Buff ids clear of other rooms package fixtures (sightTestInfraredBuffId is
// 7401, seenByVeilBuffId is 7621).
const (
	refreshTestHeldBuffId    = 7301 // "Test Zone Press": already held, must refresh
	refreshTestGrantedBuffId = 7302 // "Test Zone Grant": not yet held, event path
	refreshTestMobBuffId     = 7303 // "Test Zone Press (mob)": mob-side refresh
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
// Character.HasBuff immediately after the call, and it must queue exactly one
// Buff event, while the refreshed holder must queue none.
func TestApplyBuffIdToPlayers_HeldBuffIsRefreshedNotRelapsed(t *testing.T) {
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		refreshTestHeldBuffId:    {BuffId: refreshTestHeldBuffId, Name: "Test Zone Press", TriggerCount: 3, RoundInterval: 1},
		refreshTestGrantedBuffId: {BuffId: refreshTestGrantedBuffId, Name: "Test Zone Grant", TriggerCount: 3, RoundInterval: 1},
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7311: users.NewTestUser(7311, "presskeeper", "Presskeeper", 97311),
		7312: users.NewTestUser(7312, "newcomer", "Newcomer", 97312),
	}))

	// Separate rooms, each with a single mutator buff id: ApplyBuffIdToPlayers
	// applies every id in its list to every player in the room, so sharing a
	// room (or a buffIds slice) between the held and the granted case would
	// have each player also receive a grant for the other case's id, muddying
	// the event counts this test asserts on.
	heldRoom := &Room{RoomId: 7310}
	heldRoom.AddPlayer(7311)
	grantRoom := &Room{RoomId: 7313}
	grantRoom.AddPlayer(7312)

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

	// Discard anything left over from AddBuff/SeedUsersForTest setup above,
	// then read only what ApplyBuffIdToPlayers itself queues.
	events.DrainQueuedBuffsForTest(7311)
	events.DrainQueuedBuffsForTest(7312)

	heldRoom.ApplyBuffIdToPlayers([]int{refreshTestHeldBuffId}, "area")
	grantRoom.ApplyBuffIdToPlayers([]int{refreshTestGrantedBuffId}, "area")

	if got := len(held.Character.Buffs.List); got != 1 {
		t.Fatalf("held player's buff list has %d entries, want 1 (refreshed in place, not removed and re-added)", got)
	}
	if got := held.Character.Buffs.List[0].TriggersLeft; got != 3 {
		t.Errorf("held player's TriggersLeft = %d, want 3 (refreshed synchronously)", got)
	}
	if got := events.DrainQueuedBuffsForTest(7311); len(got) != 0 {
		t.Errorf("held player has %d queued Buff events, want 0: a refresh queues no event and so renders no start text", len(got))
	}

	// The player without the buff goes through the async grant path
	// (UserRecord.AddBuff), which only enqueues events.Buff; it does not
	// apply the buff synchronously. Confirming it did NOT appear yet is what
	// distinguishes this call from the held player's synchronous refresh, and
	// confirming exactly one Buff event landed proves the grant still happens.
	if newcomer.Character.HasBuff(refreshTestGrantedBuffId) {
		t.Error("newcomer already carries the granted buff synchronously; expected the async event path (grant not yet applied)")
	}
	got := events.DrainQueuedBuffsForTest(7312)
	if len(got) != 1 {
		t.Fatalf("newcomer has %d queued Buff events, want 1", len(got))
	}
	if got[0].BuffId != refreshTestGrantedBuffId || got[0].Source != "area" {
		t.Errorf("queued Buff event = %+v, want BuffId %d Source \"area\"", got[0], refreshTestGrantedBuffId)
	}
}

// The mob-side room applier has the same lapse-and-reapply shape as the
// player one: ApplyBuffIdToMobs must refresh a mob that already holds the
// buff instead of skipping it until it lapses and gets re-added.
func TestApplyBuffIdToMobs_HeldBuffIsRefreshedNotRelapsed(t *testing.T) {
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		refreshTestMobBuffId: {BuffId: refreshTestMobBuffId, Name: "Test Zone Press (mob)", TriggerCount: 3, RoundInterval: 1},
	}))

	const mobInstanceId = 7321
	m := &mobs.Mob{InstanceId: mobInstanceId}
	m.Character.Name = "Test Guard"
	m.Character.Buffs = buffs.New()
	mobs.SetInstanceForTest(mobInstanceId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(mobInstanceId, nil) })

	r := &Room{RoomId: 7320}
	r.AddMob(mobInstanceId)

	if err := m.Character.AddBuff(refreshTestMobBuffId, false); err != nil {
		t.Fatalf("precondition: could not grant the held buff: %v", err)
	}
	// Drive it down as if two of its three triggers had already fired.
	refreshTestSetTriggersLeft(t, m.Character.Buffs.List, refreshTestMobBuffId, 1)

	r.ApplyBuffIdToMobs([]int{refreshTestMobBuffId}, "area")

	if got := len(m.Character.Buffs.List); got != 1 {
		t.Fatalf("mob's buff list has %d entries, want 1 (refreshed in place, not removed and re-added)", got)
	}
	if got := m.Character.Buffs.List[0].TriggersLeft; got != 3 {
		t.Errorf("mob's TriggersLeft = %d, want 3 (refreshed synchronously)", got)
	}
}

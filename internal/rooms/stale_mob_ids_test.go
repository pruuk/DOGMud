package rooms

import (
	"testing"
)

// Playtest 2026-08-08: "Sala the Mender #1" and "#2" both rendered in
// "Also here:", but `look sala`, `look mender`, `look 2.sala` and `look Sala`
// all returned "Look at what???". A second look showed no Sala at all.
//
// Root cause: r.mobs held instance ids whose instances no longer existed.
//
//  1. GetMobs(FindAll) returned r.mobs verbatim, so stale ids rendered as
//     phantom NPCs.
//  2. findMobByName called mobs.GetInstance(mId) and dereferenced the result
//     with no nil check (rooms.go, the only unguarded GetInstance in the
//     file). A stale id panicked there; recovery upstream swallowed it and the
//     player saw "Look at what???" for EVERY mob in the room, because the
//     name-collection loop never finished.
//
// Instance id 0 is never a live mob, so it is a reliable stand-in for a stale
// id in these tests without needing mob templates loaded.

const staleMobId = 0

// The nil deref. Against the pre-fix code this panics instead of returning.
func TestFindMobByName_StaleIdDoesNotPanic(t *testing.T) {
	r := &Room{RoomId: 5209, mobs: []int{staleMobId}}

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("findMobByName panicked on a stale mob id: %v", rec)
		}
	}()

	if id, err := r.findMobByName("sala"); err == nil || id != 0 {
		t.Errorf("findMobByName = (%d, %v), want (0, error) when only a stale id is present", id, err)
	}
}

// FindByName is the path `look` actually uses.
func TestFindByName_StaleIdDoesNotPanic(t *testing.T) {
	r := &Room{RoomId: 5209, mobs: []int{staleMobId, staleMobId}}

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("FindByName panicked on stale mob ids: %v", rec)
		}
	}()

	playerId, mobId := r.FindByName("sala")
	if playerId != 0 || mobId != 0 {
		t.Errorf("FindByName = (%d, %d), want (0, 0)", playerId, mobId)
	}
}

// Phantom rendering: "Also here:" is built from GetMobs, which must not
// surface ids with no live instance.
func TestGetMobs_ExcludesStaleIds(t *testing.T) {
	r := &Room{RoomId: 5209, mobs: []int{staleMobId, staleMobId}}

	if got := r.GetMobs(FindAll); len(got) != 0 {
		t.Errorf("GetMobs(FindAll) = %v, want empty; stale ids must not render as phantom NPCs", got)
	}
}

func TestGetMobs_EmptyRoomIsSafe(t *testing.T) {
	r := &Room{RoomId: 5209}
	if got := r.GetMobs(FindAll); len(got) != 0 {
		t.Errorf("GetMobs on an empty room = %v, want empty", got)
	}
}

// GetMobDuplicateIndex already guarded nil, but it shares the ordering
// contract with findMobByName, so pin that it stays safe too.
func TestGetMobDuplicateIndex_StaleIdIsSafe(t *testing.T) {
	r := &Room{RoomId: 5209, mobs: []int{staleMobId}}

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("GetMobDuplicateIndex panicked on a stale mob id: %v", rec)
		}
	}()

	if got := r.GetMobDuplicateIndex(staleMobId); got != 0 {
		t.Errorf("GetMobDuplicateIndex = %d, want 0", got)
	}
}

// Targeting by #id must also survive a stale entry rather than panicking.
func TestFindMobByName_HashTargetingWithStaleId(t *testing.T) {
	r := &Room{RoomId: 5209, mobs: []int{staleMobId}}

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("findMobByName panicked on #-targeting with a stale id: %v", rec)
		}
	}()

	if id, err := r.findMobByName("#1"); err == nil || id != 0 {
		t.Errorf("findMobByName(\"#1\") = (%d, %v), want (0, error)", id, err)
	}
}

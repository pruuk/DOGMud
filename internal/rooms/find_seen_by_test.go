package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	seenByRoomId        = 7600
	seenByViewerId      = 7601
	seenByHiderId       = 7602
	seenByHiddenGuardId = 7611
	seenByGuardId       = 7612
	seenByVeilBuffId    = 7621
)

func seenByHide(t *testing.T, c *characters.Character) {
	t.Helper()
	reason := state.TransitionReason{Trigger: "find_seen_by_test"}
	if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		t.Fatalf("concealing: %v", err)
	}
	c.Awareness.ResolveConcealment(true, reason)
	if !c.IsHidden() {
		t.Fatal("precondition: the fixture should now be hidden")
	}
}

// seenByRoom holds the viewer Aliceia, a hidden player Kesh, and two guards:
// the first hidden, the second not.
func seenByRoom(t *testing.T) (*Room, *characters.Character) {
	t.Helper()
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		seenByVeilBuffId: {BuffId: seenByVeilBuffId, Name: "Test Veil", Flags: []buffs.Flag{buffs.SeeHidden}},
	}))
	viewer := users.NewTestUser(seenByViewerId, "aliceia", "Aliceia", 97601)
	hider := users.NewTestUser(seenByHiderId, "kesh", "Kesh", 97602)
	seenByHide(t, hider.Character)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		seenByViewerId: viewer,
		seenByHiderId:  hider,
	}))

	r := &Room{RoomId: seenByRoomId}
	r.AddPlayer(seenByViewerId)
	r.AddPlayer(seenByHiderId)
	for _, g := range []struct {
		id     int
		hidden bool
	}{{seenByHiddenGuardId, true}, {seenByGuardId, false}} {
		m := &mobs.Mob{InstanceId: g.id}
		m.Character.Name = "Guard"
		m.Character.RoomId = seenByRoomId
		m.Character.Buffs = buffs.New()
		m.Character.Awareness = awareness.NewMachine()
		if g.hidden {
			seenByHide(t, &m.Character)
		}
		mobs.SetInstanceForTest(g.id, m)
		id := g.id
		t.Cleanup(func() { mobs.SetInstanceForTest(id, nil) })
		r.AddMob(g.id)
	}
	return r, viewer.Character
}

func TestFindByNameSeenBy_HiddenPlayerCannotBeNamed(t *testing.T) {
	r, viewer := seenByRoom(t)
	if pId, _ := r.FindByNameSeenBy(viewer, "kesh"); pId != 0 {
		t.Errorf("a viewer without see-hidden named the hidden player (%d)", pId)
	}
	if pId, _ := r.FindByNameSeenBy(viewer, "@7602"); pId != 0 {
		t.Errorf("@id must respect perception too, got %d", pId)
	}
	if pId, _ := r.FindByName("kesh"); pId != seenByHiderId {
		t.Errorf("FindByName, the unfiltered form for staff and mobs, = %d, want %d", pId, seenByHiderId)
	}
}

func TestFindByNameSeenBy_SeeHiddenNamesThem(t *testing.T) {
	r, viewer := seenByRoom(t)
	if err := viewer.AddBuff(seenByVeilBuffId, true); err != nil {
		t.Fatalf("applying see-hidden: %v", err)
	}
	if pId, _ := r.FindByNameSeenBy(viewer, "kesh"); pId != seenByHiderId {
		t.Errorf("with see-hidden = %d, want %d", pId, seenByHiderId)
	}
	if _, mId := r.FindByNameSeenBy(viewer, "2.guard"); mId != seenByGuardId {
		t.Errorf("with see-hidden, 2.guard = %d, want the second guard %d", mId, seenByGuardId)
	}
}

func TestFindByNameSeenBy_OrdinalsCountOnlyWhatYouPerceive(t *testing.T) {
	r, viewer := seenByRoom(t)
	if _, mId := r.FindByNameSeenBy(viewer, "guard"); mId != seenByGuardId {
		t.Errorf("guard = %d, want the only guard the viewer perceives, %d", mId, seenByGuardId)
	}
	if _, mId := r.FindByNameSeenBy(viewer, "2.guard"); mId != 0 {
		t.Errorf("2.guard = %d, want nothing: the viewer perceives one guard", mId)
	}
	if _, mId := r.FindByNameSeenBy(viewer, "#7611"); mId != 0 {
		t.Errorf("#id must respect perception too, got %d", mId)
	}
}

func TestFindByNameSeenBy_NilViewerIsFindByName(t *testing.T) {
	r, _ := seenByRoom(t)
	for _, name := range []string{"kesh", "guard", "2.guard", "@7602", "#7611"} {
		wantP, wantM := r.FindByName(name)
		gotP, gotM := r.FindByNameSeenBy(nil, name)
		if gotP != wantP || gotM != wantM {
			t.Errorf("%q: nil viewer = (%d, %d), FindByName = (%d, %d)", name, gotP, gotM, wantP, wantM)
		}
	}
}

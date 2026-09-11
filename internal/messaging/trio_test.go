package messaging

import "testing"

type fakeRecipient struct {
	cats  []Category
	texts []string
}

func (f *fakeRecipient) SendText(cat Category, text string) {
	f.cats = append(f.cats, cat)
	f.texts = append(f.texts, text)
}

type fakeBroadcaster struct {
	calls int
	cat   Category
	text  string
	names []string
	excl  []int
	sight map[int]SightDecision
}

func (f *fakeBroadcaster) SendTextVisualHidingNames(cat Category, txt string, names []string, excludeUserIds ...int) {
	f.calls++
	f.cat = cat
	f.text = txt
	f.names = append([]string(nil), names...)
	f.excl = append([]int(nil), excludeUserIds...)
}

func (f *fakeBroadcaster) ParticipantSight(userId int) SightDecision {
	if d, ok := f.sight[userId]; ok {
		return d
	}
	return SightFull
}

func TestSendTrioDeliversAllThreeWithTheirOwnCategories(t *testing.T) {
	actor, actee := &fakeRecipient{}, &fakeRecipient{}
	room := &fakeBroadcaster{}

	SendTrio(Trio{
		Actor:    Say(CategorySystem, "you hit"),
		Actee:    Say(CategorySystem, "you are hit"),
		Observer: Say(CategoryBash, "a hits b"),
	}, Audience{Actor: actor, ActorId: 7, Actee: actee, ActeeId: 9, Room: room})

	if len(actor.texts) != 1 || actor.texts[0] != "you hit" {
		t.Fatalf("actor got %v", actor.texts)
	}
	if len(actee.texts) != 1 || actee.texts[0] != "you are hit" {
		t.Fatalf("actee got %v", actee.texts)
	}
	if room.calls != 1 || room.text != "a hits b" {
		t.Fatalf("room got %d calls, %q", room.calls, room.text)
	}
	if actor.cats[0] != CategorySystem || room.cat != CategoryBash {
		t.Fatalf("categories not preserved per role: actor %v room %v", actor.cats[0], room.cat)
	}
}

func TestSendTrioExcludesActorAndActeeFromTheRoom(t *testing.T) {
	room := &fakeBroadcaster{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "a"),
		Actee:    Say(CategorySystem, "b"),
		Observer: Say(CategoryBash, "c"),
	}, Audience{Actor: &fakeRecipient{}, ActorId: 7, Actee: &fakeRecipient{}, ActeeId: 9, Room: room})

	if len(room.excl) != 2 || room.excl[0] != 7 || room.excl[1] != 9 {
		t.Fatalf("exclusions = %v, want [7 9]", room.excl)
	}
}

func TestSendTrioOmitsZeroIdsFromExclusions(t *testing.T) {
	room := &fakeBroadcaster{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "a"),
		Actee:    NoLine,
		Observer: Say(CategoryBash, "c"),
	}, Audience{Actor: &fakeRecipient{}, ActorId: 7, Actee: nil, ActeeId: 0, Room: room})

	if len(room.excl) != 1 || room.excl[0] != 7 {
		t.Fatalf("exclusions = %v, want [7]", room.excl)
	}
}

func TestSendTrioSkipsActeeWhenTheActeeIsAMob(t *testing.T) {
	actor := &fakeRecipient{}
	room := &fakeBroadcaster{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "you hit"),
		Actee:    Say(CategorySystem, "never delivered"),
		Observer: Say(CategoryBash, "a hits b"),
	}, Audience{Actor: actor, ActorId: 7, Actee: nil, Room: room})

	if len(actor.texts) != 1 {
		t.Fatalf("actor got %v", actor.texts)
	}
	if room.calls != 1 {
		t.Fatalf("room calls = %d, want 1", room.calls)
	}
}

func TestSendTrioSkipsActorWhenTheActorIsAMob(t *testing.T) {
	actee := &fakeRecipient{}
	room := &fakeBroadcaster{}
	SendTrio(Trio{
		Actor:    NoLine,
		Actee:    Say(CategorySystem, "you are hit"),
		Observer: Say(CategoryBash, "a hits b"),
	}, Audience{Actor: nil, Actee: actee, ActeeId: 9, Room: room})

	if len(actee.texts) != 1 || actee.texts[0] != "you are hit" {
		t.Fatalf("actee got %v", actee.texts)
	}
	if room.calls != 1 {
		t.Fatalf("room calls = %d, want 1", room.calls)
	}
}

func TestSendTrioNoLineObserverSendsNoBroadcast(t *testing.T) {
	room := &fakeBroadcaster{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "private aside"),
		Actee:    NoLine,
		Observer: NoLine,
	}, Audience{Actor: &fakeRecipient{}, ActorId: 7, Room: room})

	if room.calls != 0 {
		t.Fatalf("room calls = %d, want 0", room.calls)
	}
}

func TestSendTrioNilRoomIsSafe(t *testing.T) {
	actor := &fakeRecipient{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "a"),
		Actee:    NoLine,
		Observer: Say(CategoryBash, "c"),
	}, Audience{Actor: actor, ActorId: 7})

	if len(actor.texts) != 1 {
		t.Fatalf("actor got %v", actor.texts)
	}
}

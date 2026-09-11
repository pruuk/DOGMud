package messaging

import "testing"

func TestSendTrioHidesTheOtherPartyByEachReadersOwnSight(t *testing.T) {
	actor, actee := &fakeRecipient{}, &fakeRecipient{}
	room := &fakeBroadcaster{sight: map[int]SightDecision{7: SightShapes, 9: SightNone}}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, `You kick <ansi fg="mobname">Bobrick</ansi>!`),
		Actee:    Say(CategorySystem, `<ansi fg="username">Aliceia</ansi> kicks you!`),
		Observer: Say(CategoryKick, `Aliceia kicks Bobrick!`),
	}, Audience{
		Actor: actor, ActorId: 7, ActorName: "Aliceia",
		Actee: actee, ActeeId: 9, ActeeName: "Bobrick",
		Room: room,
	})

	if got, want := actor.texts[0], `You kick <ansi fg="combat-anon">a figure</ansi>!`; got != want {
		t.Errorf("actor (shapes) read %q, want %q", got, want)
	}
	if got, want := actee.texts[0], `<ansi fg="combat-anon">Something</ansi> kicks you!`; got != want {
		t.Errorf("actee (no sight) read %q, want %q", got, want)
	}
	if len(room.names) != 2 || room.names[0] != "Aliceia" || room.names[1] != "Bobrick" {
		t.Errorf("room was handed names %v, want [Aliceia Bobrick]", room.names)
	}
	if room.text != `Aliceia kicks Bobrick!` {
		t.Errorf("the observer line is hidden per observer by the Broadcaster, not before it: got %q", room.text)
	}
}

func TestSendTrioWithoutNamesIsUnchangedInTheDark(t *testing.T) {
	actor := &fakeRecipient{}
	room := &fakeBroadcaster{sight: map[int]SightDecision{7: SightNone}}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "You kick Bobrick!"),
		Actee:    NoLine,
		Observer: NoLine,
	}, Audience{Actor: actor, ActorId: 7, Room: room})

	if actor.texts[0] != "You kick Bobrick!" {
		t.Fatalf("a caller that names no one must get its text untouched, got %q", actor.texts[0])
	}
}

func TestSendTrioNilRoomHidesNothing(t *testing.T) {
	actor := &fakeRecipient{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "You kick Bobrick!"),
		Actee:    NoLine,
		Observer: NoLine,
	}, Audience{Actor: actor, ActorId: 7, ActeeName: "Bobrick"})

	if actor.texts[0] != "You kick Bobrick!" {
		t.Fatalf("with no room there is no light to judge, got %q", actor.texts[0])
	}
}

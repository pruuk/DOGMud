package questengine

import "testing"

func TestGuideLine_EveryCase(t *testing.T) {
	cases := []struct {
		d    Destination
		want string
	}{
		{Destination{}, ""},
		{Destination{Kind: DestAnywhere}, "You can do this anywhere."},
		{Destination{Kind: DestUnknown}, "Destination unknown."},
		{Destination{Kind: DestRoom, NextRoom: 2, NextDir: "north"}, "Head north."},
		{Destination{Kind: DestRoom, NextRoom: 2, NextDir: "up"}, "Head up."},
		{Destination{Kind: DestRoom, NextRoom: 2, NextDir: "gate"}, "Take the gate."},
		{Destination{Kind: DestArea, Label: "the unquiet grove", Inside: true}, "You are in the unquiet grove."},
		{Destination{Kind: DestZone, Label: "Pothole Coulee", Inside: true}, "You are in Pothole Coulee."},
		{Destination{Kind: DestArea, Npc: "a wolf", Fallback: true, Inside: true}, "You are where a wolf is usually found."},
		{Destination{Kind: DestArea, Npc: "the Dock Constable or Jesset", Fallback: true, Inside: true},
			"You are where the Dock Constable or Jesset is usually found."},
		{Destination{Kind: DestMob, Npc: "Velk", Inside: true}, "You are where you need to be."},
		{Destination{Kind: DestRoom, Inside: true}, "You are where you need to be."},
		{Destination{Kind: DestRoom}, "You can't find a way there from here."},
	}
	for _, c := range cases {
		if got := GuideLine(c.d); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.d, got, c.want)
		}
	}
}

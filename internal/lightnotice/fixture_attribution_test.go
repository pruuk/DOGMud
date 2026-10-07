package lightnotice

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// lampAgrees (lighting plan 6): the room's own light is blamed only when it
// moved the same way as the room. Each row is checked on lampAgrees itself
// and on the cause attribute then names.
func TestLampAgreesOnlyWhenTheLampMovedWithTheRoom(t *testing.T) {
	base := rooms.LightTerms{SkyFilter: 1, Dark: 0, Fixture: 0, CarriedLight: 0}
	terms := func(level int, sky float64, hasLamp bool, lamp int, fixture float64) rooms.LightTerms {
		x := base
		x.Level, x.Sky, x.HasLamp, x.Lamp, x.Fixture = level, sky, hasLamp, lamp, fixture
		return x
	}
	cases := []struct {
		name       string
		a, b       rooms.LightTerms
		wantAgrees bool
		wantCause  Cause
	}{
		{"room lighter, lamp dimmer: the sky brightened past a dimming lamp",
			terms(40, 30, true, 52, 0), terms(60, 60, true, 35, 0), false, CauseSky},
		{"room darker, lamp brighter: noon to midnight on a backstreet",
			terms(69, 69, false, 0, 0), terms(40, 10, true, 35, 0), false, CauseSky},
		{"both brighter: the lamps light on a dark street",
			terms(30, 30, false, 0, 0), terms(54, 30, true, 52, 0), true, CauseLamp},
		{"both darker: the lamps go out",
			terms(54, 30, true, 52, 0), terms(30, 30, false, 0, 0), true, CauseLamp},
		{"a lamp that changed but contributes nothing: an authored lamp 0 lit as the sky fell",
			terms(60, 60, false, 0, 0), terms(40, 40, true, 0, 0), false, CauseSky},
		{"a move both ways agrees: the lamp out, a fixture up, the room darker",
			terms(54, 30, true, 52, 0), terms(45, 30, false, 0, 44), true, CauseLamp},
	}
	for _, c := range cases {
		if got := lampAgrees(c.a, c.b); got != c.wantAgrees {
			t.Errorf("%s: lampAgrees = %v, want %v", c.name, got, c.wantAgrees)
		}
		prev := record{roomId: 1, terms: c.a}
		now := observation{roomId: 1, terms: c.b}
		if got := attribute(prev, now); got != c.wantCause {
			t.Errorf("%s: attribute = %q, want %q", c.name, got, c.wantCause)
		}
	}
}

// X5 and Rule 12: a fixture changing notifies as lamp; a carried light that
// dims while still lit, or a second carried light arriving, notifies as
// carried. Each case reads eyes against the pre-5e attribute, which keyed
// carried on presence and lamp on the room's static Lamp int only.
func TestAttributionNamesFixturesAndScheduledCarriedLight(t *testing.T) {
	// 4111 at midwinter midnight: moonlight 34, no lamp of its own.
	night := rooms.LightTerms{Level: 34, Sky: 34, SkyFilter: 1, Dark: lightscale.Absent(),
		Fixture: lightscale.Absent(), CarriedLight: lightscale.Absent()}
	with := func(f func(*rooms.LightTerms)) rooms.LightTerms { t2 := night; f(&t2); return t2 }

	cases := []struct {
		name string
		prev record
		now  observation
		want Cause
	}{
		{"the arch lantern is lit at dusk",
			rec(1, messaging.BandShapes, night),
			obs(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 54; x.Fixture = 52 })),
			CauseLamp},
		{"the arch lantern is snuffed at dawn",
			rec(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 54; x.Fixture = 52 })),
			obs(1, messaging.BandShapes, night),
			CauseLamp},
		{"a fixture's pulse crosses a band",
			rec(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 45; x.Fixture = 36 })),
			obs(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 52; x.Fixture = 50 })),
			CauseLamp},
		{"a sunstone fades while still lit",
			rec(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 52; x.Carried = true; x.CarriedLight = 46 })),
			obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 41; x.Carried = true; x.CarriedLight = 30 })),
			CauseCarried},
		{"a second light arrives where one already is",
			rec(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 45; x.Carried = true; x.CarriedLight = 40 })),
			obs(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 52; x.Carried = true; x.CarriedLight = 50 })),
			CauseCarried},
		{"control: a steady fixture and lantern leave a sky change to the sky",
			rec(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 54; x.Sky = 50; x.Fixture = 52 })),
			obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 49; x.Sky = 20; x.Fixture = 52 })),
			CauseSky},
	}
	for _, c := range cases {
		if got := c.now.bandAt(c.now.terms.Level); got != c.now.band {
			t.Fatalf("%s: fixture inconsistent: sight reads %v at %d, case claims %v", c.name, got, c.now.terms.Level, c.now.band)
		}
		if got := attribute(c.prev, c.now); got != c.want {
			t.Errorf("%s: attribute = %q, want %q", c.name, got, c.want)
		}
	}
}

package lightnotice

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

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

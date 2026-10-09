package lightnotice

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

var (
	termsLit    = rooms.LightTerms{Level: 60, Sky: 60}
	termsDim    = rooms.LightTerms{Level: 40, Sky: 40}
	termsBright = rooms.LightTerms{Level: 80, Sky: 80}
)

func rec(room int, b messaging.Band, terms rooms.LightTerms) record {
	return record{roomId: room, band: b, terms: terms}
}

// obs builds an observation. bandAt is optional and defaults to sight(0): an
// observer with no night-sight bonus, so TestDecideTriggerRules (which never
// checks a cause) does not have to supply one.
func obs(room int, b messaging.Band, terms rooms.LightTerms, bandAt ...func(int) messaging.Band) observation {
	f := sight(0)
	if len(bandAt) > 0 {
		f = bandAt[0]
	}
	return observation{roomId: room, band: b, terms: terms, bandAt: f}
}

// recSight is rec read through a night-sight window of strength (no infra
// reach).
func recSight(room int, b messaging.Band, terms rooms.LightTerms, strength int) record {
	r := rec(room, b, terms)
	r.sight = sightWindow{strength: strength}
	return r
}

// obsSight is obs for an observer whose window is strength (no infra reach):
// bandAt reads through it and the observation carries it.
func obsSight(room int, b messaging.Band, terms rooms.LightTerms, strength int) observation {
	o := obs(room, b, terms, sight(strength))
	o.sight = sightWindow{strength: strength}
	return o
}

// sight returns a bandAt for an observer with the given night-sight strength
// and the shipped 25/50 edges (LightBlindBelow, LightDimBelow).
func sight(strength int) func(int) messaging.Band {
	return func(light int) messaging.Band { return messaging.BandThroughWindow(light, strength, 0, 25, 50, 75) }
}

func TestDecideTriggerRules(t *testing.T) {
	cases := []struct {
		name  string
		prev  record
		now   observation
		trig  Trigger
		speak bool
		tr    Transition
	}{
		{"move to darker speaks", rec(1, messaging.BandFaces, termsLit), obs(2, messaging.BandShapes, termsDim), TriggerMove, true, DarkerShapes},
		{"move to lighter is silent", rec(1, messaging.BandShapes, termsDim), obs(2, messaging.BandFaces, termsLit), TriggerMove, false, ""},
		{"move into dazzle speaks", rec(1, messaging.BandFaces, termsLit), obs(2, messaging.BandDazzled, termsBright), TriggerMove, true, IntoDazzle},
		{"combat darker speaks", rec(1, messaging.BandFaces, termsLit), obs(1, messaging.BandShapes, termsDim), TriggerCombatRound, true, DarkerShapes},
		{"combat lighter speaks", rec(1, messaging.BandShapes, termsDim), obs(1, messaging.BandFaces, termsLit), TriggerCombatRound, true, LighterFaces},
		{"command darker speaks", rec(1, messaging.BandFaces, termsLit), obs(1, messaging.BandShapes, termsDim), TriggerCommand, true, DarkerShapes},
		{"command lighter speaks", rec(1, messaging.BandShapes, termsDim), obs(1, messaging.BandFaces, termsLit), TriggerCommand, true, LighterFaces},
		{"quiet never speaks", rec(1, messaging.BandFaces, termsLit), obs(1, messaging.BandShapes, termsDim), TriggerQuiet, false, ""},
		{"same band is silent", rec(1, messaging.BandFaces, termsLit), obs(1, messaging.BandFaces, termsDim), TriggerCommand, false, ""},
		{"command sees a room change to lighter and is silent", rec(1, messaging.BandShapes, termsDim), obs(2, messaging.BandFaces, termsLit), TriggerCommand, false, ""},
		{"command sees a room change to darker and speaks", rec(1, messaging.BandFaces, termsLit), obs(2, messaging.BandShapes, termsDim), TriggerCommand, true, DarkerShapes},
	}
	for _, c := range cases {
		n, speak, next := decide(c.prev, true, c.now, c.trig)
		if speak != c.speak {
			t.Errorf("%s: speak = %v, want %v", c.name, speak, c.speak)
		}
		if speak && n.transition != c.tr {
			t.Errorf("%s: transition = %q, want %q", c.name, n.transition, c.tr)
		}
		if next.band != c.now.band || next.roomId != c.now.roomId || next.quiet {
			t.Errorf("%s: the observed band must always be recorded, got %+v", c.name, next)
		}
	}
}

func TestDecideSilentRecording(t *testing.T) {
	now := obs(1, messaging.BandShapes, termsDim)

	if _, speak, next := decide(record{}, false, now, TriggerCommand); speak || next.band != messaging.BandShapes {
		t.Errorf("first check must record silently, speak=%v next=%+v", speak, next)
	}

	asleep := now
	asleep.asleep = true
	_, speak, next := decide(rec(1, messaging.BandFaces, termsLit), true, asleep, TriggerCommand)
	if speak || !next.quiet || next.band != messaging.BandFaces || next.roomId != 1 {
		t.Errorf("a sleeper gets no notice, is marked quiet, and keeps the OLD record, speak=%v next=%+v", speak, next)
	}

	blind := now
	blind.blinded = true
	_, speak, next = decide(rec(1, messaging.BandFaces, termsLit), true, blind, TriggerCombatRound)
	if speak || !next.quiet || next.band != messaging.BandFaces || next.roomId != 1 {
		t.Errorf("a blinded player gets no notice, is marked quiet, and keeps the OLD record, speak=%v next=%+v", speak, next)
	}

	quiet := rec(1, messaging.BandFaces, termsLit)
	quiet.quiet = true
	_, speak, next = decide(quiet, true, now, TriggerCommand)
	if speak || next.quiet || next.band != messaging.BandShapes {
		t.Errorf("the first check after quiet records silently and clears quiet, speak=%v next=%+v", speak, next)
	}

	_, speak, _ = decide(next, true, obs(1, messaging.BandFaces, termsLit), TriggerCommand)
	if !speak {
		t.Error("the check after a silent resync must speak on a real change")
	}
}

func TestTransitionOf(t *testing.T) {
	cases := []struct {
		from, to messaging.Band
		want     Transition
	}{
		{messaging.BandDark, messaging.BandShapes, LighterShapes},
		{messaging.BandDark, messaging.BandFaces, LighterFaces},
		{messaging.BandShapes, messaging.BandFaces, LighterFaces},
		{messaging.BandDark, messaging.BandDazzled, IntoDazzle},
		{messaging.BandFaces, messaging.BandDazzled, IntoDazzle},
		{messaging.BandDazzled, messaging.BandFaces, DarkerFaces},
		{messaging.BandDazzled, messaging.BandShapes, DarkerShapes},
		{messaging.BandFaces, messaging.BandShapes, DarkerShapes},
		{messaging.BandFaces, messaging.BandDark, DarkerDark},
		{messaging.BandDazzled, messaging.BandDark, DarkerDark},
	}
	for _, c := range cases {
		if got := transitionOf(c.from, c.to); got != c.want {
			t.Errorf("%v -> %v = %q, want %q", c.from, c.to, got, c.want)
		}
	}
}

// TestAttribution's fixtures are ALL physically consistent: every now.band is
// what now.bandAt (the observer's current sight) actually reads at
// now.terms.Level, checked below as a fixture sanity assertion, so an
// inconsistent case fails loudly instead of quietly asserting a cause for a
// band change that could never happen. sight(0)'s edges land Level 60 (the
// shared base) in faces and Level 40 in shapes, which is why most cases move
// between exactly those two levels.
func TestAttribution(t *testing.T) {
	base := rooms.LightTerms{Level: 60, Sky: 55, SkyFilter: 1, Lamp: 40, HasLamp: true}
	with := func(f func(*rooms.LightTerms)) rooms.LightTerms { t2 := base; f(&t2); return t2 }

	cases := []struct {
		name string
		prev record
		now  observation
		want Cause
	}{
		{"a different room is movement",
			rec(1, messaging.BandFaces, base), obs(2, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40 })), CauseMovement},
		{"carried light leaving",
			rec(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Carried = true })), obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40 })), CauseCarried},
		{"lamp value changing",
			rec(1, messaging.BandFaces, base), obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Lamp = 20 })), CauseLamp},
		{"weather occlusion",
			rec(1, messaging.BandFaces, base), obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.SkyFilter = 0.5; x.Sky = 30 })), CauseWeather},
		{"the sky alone",
			rec(1, messaging.BandFaces, base), obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Sky = 30 })), CauseSky},
		{"sky gone entirely counts as the sky",
			rec(1, messaging.BandFaces, base), obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Sky = lightscale.Absent() })), CauseSky},

		// Eyes cases: the room's light terms are IDENTICAL (or the sky drifts
		// the way it does every round regardless); only the observer's own
		// sight, expressed as a different bandAt on prev's read versus now's,
		// changed. prev's band is what that OLD sight read at the shared
		// Level; now's bandAt is the NEW sight.

		{"eyes, darker: a night-sight draught expires",
			rec(1, messaging.BandFaces, rooms.LightTerms{Level: 40, Sky: 55, Lamp: 40, HasLamp: true}),
			obs(1, messaging.BandShapes, rooms.LightTerms{Level: 40, Sky: 55, Lamp: 40, HasLamp: true}, sight(0)),
			CauseEyes},
		{"eyes beats a same-direction sky drift: this is the bug the counterfactual fixes",
			rec(1, messaging.BandFaces, rooms.LightTerms{Level: 40, Sky: 55, Lamp: 40, HasLamp: true}),
			obs(1, messaging.BandShapes, rooms.LightTerms{Level: 39, Sky: 54.5, Lamp: 40, HasLamp: true}, sight(0)),
			CauseEyes},
		{"eyes, lighter: a night-sight draught is drunk",
			rec(1, messaging.BandShapes, rooms.LightTerms{Level: 40, Sky: 55, Lamp: 40, HasLamp: true}),
			obs(1, messaging.BandFaces, rooms.LightTerms{Level: 40, Sky: 55, Lamp: 40, HasLamp: true}, sight(24)),
			CauseEyes},
		{"eyes, into dazzle: the draught also lowers the dazzle edge",
			rec(1, messaging.BandFaces, rooms.LightTerms{Level: 60, Sky: 55, Lamp: 40, HasLamp: true}),
			obs(1, messaging.BandDazzled, rooms.LightTerms{Level: 60, Sky: 55, Lamp: 40, HasLamp: true}, sight(24)),
			CauseEyes},

		// A term also changed here, so the counterfactual must clear before
		// the switch gets a chance to (mis)fire on it.

		{"lamp into dazzle",
			rec(1, messaging.BandFaces, rooms.LightTerms{Level: 70, Sky: 55, Lamp: 40, HasLamp: true}),
			obs(1, messaging.BandDazzled, rooms.LightTerms{Level: 80, Sky: 55, Lamp: 60, HasLamp: true}, sight(0)),
			CauseLamp},
		{"sky, dazzled to faces",
			rec(1, messaging.BandDazzled, rooms.LightTerms{Level: 55, Sky: 55, Lamp: 40, HasLamp: true}),
			obs(1, messaging.BandFaces, rooms.LightTerms{Level: 45, Sky: 45, Lamp: 40, HasLamp: true}, sight(24)),
			CauseSky},

		// Precedence: two terms move at once, carried/lamp must win over sky.

		{"precedence: carried change beats a sky drift",
			rec(1, messaging.BandFaces, rooms.LightTerms{Level: 60, Sky: 55, Lamp: 40, HasLamp: true, Carried: true}),
			obs(1, messaging.BandShapes, rooms.LightTerms{Level: 40, Sky: 54.5, Lamp: 40, HasLamp: true, Carried: false}, sight(0)),
			CauseCarried},
		{"precedence: lamp change beats a sky drift",
			rec(1, messaging.BandFaces, rooms.LightTerms{Level: 60, Sky: 55, Lamp: 40, HasLamp: true}),
			obs(1, messaging.BandShapes, rooms.LightTerms{Level: 40, Sky: 54.5, Lamp: 20, HasLamp: true}, sight(0)),
			CauseLamp},
		{"a room with no sky on either side still attributes the lamp",
			rec(1, messaging.BandFaces, rooms.LightTerms{Level: 60, Sky: lightscale.Absent(), Lamp: 40, HasLamp: true}),
			obs(1, messaging.BandShapes, rooms.LightTerms{Level: 40, Sky: lightscale.Absent(), Lamp: 20, HasLamp: true}, sight(0)),
			CauseLamp},

		// #448: the record keeps the sight window. When the eyes changed and
		// the OLD light through the NEW eyes moves the band the way it went
		// (not necessarily all the way), the eyes take the blame even though
		// a light drifted at the same moment.

		{"eyes moved the band and a carried light drifted the same way: eyes",
			recSight(1, messaging.BandFaces, rooms.LightTerms{Level: 40, Carried: true, CarriedLight: 40}, 24),
			obsSight(1, messaging.BandDark, rooms.LightTerms{Level: 20, Carried: true, CarriedLight: 20}, 0),
			CauseEyes},
		{"eyes changed but read the old light the same: the light is named",
			recSight(1, messaging.BandFaces, rooms.LightTerms{Level: 60, Carried: true, CarriedLight: 60}, 10),
			obsSight(1, messaging.BandDark, rooms.LightTerms{Level: 20, Carried: true, CarriedLight: 20}, 0),
			CauseCarried},
		{"eyes moved the band the other way: the light is named",
			recSight(1, messaging.BandFaces, rooms.LightTerms{Level: 60, Sky: 60}, 0),
			obsSight(1, messaging.BandShapes, rooms.LightTerms{Level: 10, Sky: 10}, 24),
			CauseSky},
	}
	for _, c := range cases {
		if got := c.now.bandAt(c.now.terms.Level); got != c.now.band {
			t.Fatalf("%s: fixture is inconsistent: bandAt(now.Level) = %q, but now.band = %q", c.name, got, c.now.band)
		}
		if got := attribute(c.prev, c.now); got != c.want {
			t.Errorf("%s: cause = %q, want %q", c.name, got, c.want)
		}
	}
}

// The record keeps the sight window it was read through (#448), so the next
// check can tell the eyes changed.
func TestDecideRecordsTheSightWindow(t *testing.T) {
	now := obsSight(1, messaging.BandShapes, termsDim, 12)
	now.sight.reach = 30
	_, _, next := decide(record{}, false, now, TriggerCommand)
	if next.sight != (sightWindow{strength: 12, reach: 30}) {
		t.Errorf("the record must keep the window, got %+v", next.sight)
	}
}

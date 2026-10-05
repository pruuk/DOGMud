package lightnotice

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

const noticePallId = 9791 // magnitude darkness: the pall's shape

// Ruling D4: a darkness arriving, lapsing or changing strength is its own
// cause, checked first among the terms, never the eyes and never carried.
func TestAttributionNamesTheDarkness(t *testing.T) {
	lit := rooms.LightTerms{Level: 60, Sky: 55, SkyFilter: 1, Lamp: 40, HasLamp: true, Dark: lightscale.Absent()}
	with := func(f func(*rooms.LightTerms)) rooms.LightTerms { t2 := lit; f(&t2); return t2 }
	cases := []struct {
		name string
		prev record
		now  observation
		want Cause
	}{
		{"a darkness arrives",
			rec(1, messaging.BandFaces, lit),
			obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Darkened = true; x.Dark = 20 })),
			CauseDarkness},
		{"a darkness lapses",
			rec(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Darkened = true; x.Dark = 20 })),
			obs(1, messaging.BandFaces, lit),
			CauseDarkness},
		{"a darkness changes strength",
			rec(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Darkened = true; x.Dark = 20 })),
			obs(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 55; x.Darkened = true; x.Dark = 5 })),
			CauseDarkness},
		{"precedence: darkness beats a carried light changing with it",
			rec(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Carried = true })),
			obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Darkened = true; x.Dark = 20 })),
			CauseDarkness},
		{"control: a steady darkness does not claim a lamp change",
			rec(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Darkened = true; x.Dark = 20 })),
			obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Lamp = 20; x.Darkened = true; x.Dark = 20 })),
			CauseLamp},
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

// Through Check: a pall cast in a lit room is announced from the darkness
// pool, and its expiry too; neither blames the eyes.
func TestCheckNamesADarknessOnCastAndOnExpiry(t *testing.T) {
	u, r1, _ := seedLampWorld(t)
	r1.AddPlayer(u.UserId) // a carried darkness counts only for someone the room lists
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		noticePallId: {ConditionId: noticePallId, Name: "Test Pall", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {UsesMagnitude: true}},
			Flags:   []conditions.Flag{conditions.Adjustable, conditions.Cancellable}},
	}))
	drain := captureFor(t, 1)

	Check(u, TriggerQuiet) // lamp 60: faces
	if !u.Character.Conditions.AddConditionMagnitude(noticePallId, 4, 90) {
		t.Fatal("fixture: the pall did not land")
	}
	Check(u, TriggerCommand) // 60 - 90 = -30: dark
	got := drain()
	if len(got) != 1 || !containsAny(got[0], Pool(CauseDarkness, DarkerDark, false)) {
		t.Fatalf("want one darkness darker_dark line on the cast, got %q", got)
	}

	u.Character.Conditions.RemoveCondition(noticePallId) // expires it; LightNow reads nothing
	Check(u, TriggerCommand)
	got = drain()
	if len(got) != 1 || !containsAny(got[0], Pool(CauseDarkness, LighterFaces, false)) {
		t.Fatalf("want one darkness lighter_faces line on expiry, got %q", got)
	}
}

// Ruling D7: Check queues SightBandChanged the first time and on a change of
// band, and not on a repeat.
func TestCheckQueuesSightBandChangedOnlyOnAChange(t *testing.T) {
	u, r1, _ := seedLampWorld(t)
	events.DrainQueuedSightBandChangedForTest(u.UserId)

	Check(u, TriggerQuiet)
	if got := events.DrainQueuedSightBandChangedForTest(u.UserId); len(got) != 1 {
		t.Fatalf("the first check must report the band once, got %d", len(got))
	}
	Check(u, TriggerCommand)
	if got := events.DrainQueuedSightBandChangedForTest(u.UserId); len(got) != 0 {
		t.Fatalf("a repeat check in the same band must queue nothing, got %d", len(got))
	}
	r1.Lamp = rooms.LampPtr(30)
	Check(u, TriggerCommand)
	if got := events.DrainQueuedSightBandChangedForTest(u.UserId); len(got) != 1 {
		t.Fatalf("a change of band must queue one update, got %d", len(got))
	}
	Forget(u.UserId)
	Check(u, TriggerQuiet)
	if got := events.DrainQueuedSightBandChangedForTest(u.UserId); len(got) != 1 {
		t.Fatalf("after Forget (logout) the next check must report the band again, got %d", len(got))
	}
}

package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Sight gates close-out, #333 and #215 (owner 2026-10-08). An observer who
// sees nothing gets a HEARING roll against a sneak, at SneakHearingMult of
// its detection score; the superhearing flag skips the multiplier. A notice
// names the sneaker only as far as the observer's eyes allow: the name at
// clear sight, "a figure" at shapes, and at none only that someone was
// heard. The same contest runs in two places, the sneak command (Sneak) and
// a sneaking arrival (EntryDetection), and both follow the rule.

const (
	hearSuperCond = 9631
	hearInfraCond = 9632
)

var hearTag = regexp.MustCompile(`<[^>]*>`)

// newDarkDetectWorld is newDetectWorld with the lamp out: a plain-eyed
// observer in it sees nothing (SightNone). mult pins SneakHearingMult.
func newDarkDetectWorld(t *testing.T, mult float64) *detectWorld {
	t.Helper()
	pinDetectionKnobs(t)
	c := configs.GetConfig()
	c.Balance.SneakHearingMult = configs.ConfigFloat(mult)
	configs.SetConfigForTest(t, c)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		hearSuperCond: {ConditionId: hearSuperCond, Name: "Test Sharp Ears", RoundInterval: 1, TriggerCount: 50,
			Flags: []conditions.Flag{conditions.SuperHearing}},
		hearInfraCond: {ConditionId: hearInfraCond, Name: "Test Heat Eyes", RoundInterval: 1, TriggerCount: 50,
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))
	dest := &rooms.Room{RoomId: 9800, Zone: "MoveDetect", SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(0)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9800: dest}, map[string]*rooms.ZoneConfig{}))
	return &detectWorld{dest: dest}
}

// hearLines is what uid's client prints, tags stripped.
func hearLines(uid int) []string {
	var out []string
	for _, m := range events.DrainQueuedMessageEventsForTest(uid) {
		out = append(out, strings.TrimSpace(hearTag.ReplaceAllString(m.Text, "")))
	}
	return out
}

func TestCalcHearingScore_PaysTheKnobUnlessSuperhearing(t *testing.T) {
	w := newDarkDetectWorld(t, 0.75)
	_, obs := w.place(t, "player", 9811, "Watcher")
	obs.Stats.Perception.ValueAdj = 200

	require.InDelta(t, 150.0, CalcHearingScore(obs), 0.001, "Perception 200 at 0.75")

	require.True(t, obs.Conditions.AddCondition(hearSuperCond, true))
	require.InDelta(t, 200.0, CalcHearingScore(obs), 0.001, "superhearing skips the multiplier")
}

// sneakObserverScore prices an observer by ear exactly when it sees nothing.
func TestSneakObserverScore_EarOnlyAtSightNone(t *testing.T) {
	w := newDarkDetectWorld(t, 0.5)
	_, blind := w.place(t, "player", 9811, "Watcher")
	blind.Stats.Perception.ValueAdj = 200
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(blind, w.dest))

	score, sight := sneakObserverScore(blind, w.dest)
	require.Equal(t, messaging.SightNone, sight)
	require.InDelta(t, CalcHearingScore(blind), score, 0.001)

	_, heat := w.place(t, "player", 9812, "Seer")
	heat.Stats.Perception.ValueAdj = 200
	require.True(t, heat.Conditions.AddCondition(hearInfraCond, true))
	score, sight = sneakObserverScore(heat, w.dest)
	require.Equal(t, messaging.SightShapes, sight)
	require.InDelta(t, CalcDetectionScore(heat, w.dest), score, 0.001)
}

// A sharp observer in the dark no longer spots a sneaker by sight: with the
// knob pinned near zero its ear cannot win. On master it rolled at the sight
// ramp's 0.80 floor and won.
func TestSneak_BlindObserverRollsByEar(t *testing.T) {
	w := newDarkDetectWorld(t, 0.01)
	_, obs := w.place(t, "player", 9811, "Watcher")
	obs.Stats.Perception.ValueAdj = 1000
	actor, mc := w.place(t, "player", 9810, "Sneak")
	mc.Stats.Dexterity.ValueAdj = 100
	events.DrainQueuedMessageEventsForTest(9811)

	got := Sneak(actor)

	require.True(t, got.Success, "a blind observer at 1000 x 0.01 cannot hear a sneaker at 100")
	require.Empty(t, hearLines(9811))
}

// Superhearing skips the multiplier, so the same sharp observer hears it.
func TestSneak_SuperhearingObserverStillHears(t *testing.T) {
	w := newDarkDetectWorld(t, 0.01)
	_, obs := w.place(t, "player", 9811, "Watcher")
	obs.Stats.Perception.ValueAdj = 1000
	require.True(t, obs.Conditions.AddCondition(hearSuperCond, true))
	actor, mc := w.place(t, "player", 9810, "Sneak")
	mc.Stats.Dexterity.ValueAdj = 0
	events.DrainQueuedMessageEventsForTest(9811)

	got := Sneak(actor)

	require.False(t, got.Success)
	require.Equal(t, []string{"You hear someone trying to move quietly."}, hearLines(9811),
		"heard, never named")
}

// The observer's notice follows its sight, for both contests.
func TestSneakNotice_FollowsObserverSight(t *testing.T) {
	cases := []struct {
		name  string
		infra bool
		want  string
	}{
		{"blind observer hears", false, "You hear someone trying to move quietly."},
		{"heat-sighted observer sees a figure", true, "A figure tries to hide but you notice them."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDarkDetectWorld(t, 0.75)
			_, obs := w.place(t, "player", 9811, "Watcher")
			obs.Stats.Perception.ValueAdj = 1000
			if c.infra {
				require.True(t, obs.Conditions.AddCondition(hearInfraCond, true))
			}
			actor, mc := w.place(t, "player", 9810, "Sneak")
			mc.Stats.Dexterity.ValueAdj = 0
			events.DrainQueuedMessageEventsForTest(9811)

			require.False(t, Sneak(actor).Success)
			require.Equal(t, []string{c.want}, hearLines(9811))
		})
	}
}

func TestEntryDetectionNotice_FollowsObserverSight(t *testing.T) {
	cases := []struct {
		name  string
		infra bool
		want  string
	}{
		{"blind observer hears", false, "You hear someone trying to move quietly."},
		{"heat-sighted observer sees a figure", true, "A figure slips into the room but you notice them."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDarkDetectWorld(t, 0.75)
			_, obs := w.place(t, "player", 9811, "Watcher")
			obs.Stats.Perception.ValueAdj = 1000
			if c.infra {
				require.True(t, obs.Conditions.AddCondition(hearInfraCond, true))
			}
			mover, mc := w.place(t, "player", 9810, "Sneak")
			mc.Stats.Dexterity.ValueAdj = 0
			hideForMove(t, mc)
			mc.SetMiscData(`sneaking`, true)
			events.DrainQueuedMessageEventsForTest(9811)

			got := EntryDetection(mover, w.dest, true)

			require.False(t, got.StillSneaking)
			require.Equal(t, []string{c.want}, hearLines(9811))
		})
	}
}

// A sneaking arrival meets the same ear: a sharp blind observer with the knob
// near zero does not catch it.
func TestEntryDetection_BlindObserverRollsByEar(t *testing.T) {
	w := newDarkDetectWorld(t, 0.01)
	_, obs := w.place(t, "player", 9811, "Watcher")
	obs.Stats.Perception.ValueAdj = 1000
	mover, mc := w.place(t, "player", 9810, "Sneak")
	mc.Stats.Dexterity.ValueAdj = 100
	hideForMove(t, mc)
	mc.SetMiscData(`sneaking`, true)

	got := EntryDetection(mover, w.dest, true)

	require.True(t, got.StillSneaking)
}

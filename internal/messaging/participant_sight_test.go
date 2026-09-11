package messaging

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
)

// sightLight is a RoomVisibility with a fixed light level: 0 dark, 1 lit.
type sightLight int

func (l sightLight) GetVisibility() int { return int(l) }

const (
	sightInfraredBuffId = 9101
	sightNightBuffId    = 9102
	sightSleepBuffId    = 9103
)

// sightChar returns a fresh character carrying the given test flags. The three
// flag buffs are seeded once per test, so applying one never replaces another.
func sightChar(t *testing.T, flags ...buffs.Flag) *characters.Character {
	t.Helper()
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		sightInfraredBuffId: {BuffId: sightInfraredBuffId, Name: "Test Infrared", Flags: []buffs.Flag{buffs.InfraredVision}},
		sightNightBuffId:    {BuffId: sightNightBuffId, Name: "Test Night", Flags: []buffs.Flag{buffs.NightVision}},
		sightSleepBuffId:    {BuffId: sightSleepBuffId, Name: "Test Sleep", Flags: []buffs.Flag{buffs.Sleeping}},
	}))
	c := newChar(t)
	ids := map[buffs.Flag]int{
		buffs.InfraredVision: sightInfraredBuffId,
		buffs.NightVision:    sightNightBuffId,
		buffs.Sleeping:       sightSleepBuffId,
	}
	for _, f := range flags {
		if err := c.AddBuff(ids[f], true); err != nil {
			t.Fatalf("applying %s: %v", f, err)
		}
	}
	return c
}

func TestParticipantSight(t *testing.T) {
	cases := []struct {
		name  string
		light sightLight
		flags []buffs.Flag
		blind bool
		want  SightDecision
	}{
		{name: "lit room", light: 1, want: SightFull},
		{name: "dark room", light: 0, want: SightNone},
		{name: "dark with night vision", light: 0, flags: []buffs.Flag{buffs.NightVision}, want: SightFull},
		{name: "dark with infrared", light: 0, flags: []buffs.Flag{buffs.InfraredVision}, want: SightShapes},
		{name: "blinded in a lit room", light: 1, blind: true, want: SightNone},
		{name: "blinded with infrared in the dark", light: 0, flags: []buffs.Flag{buffs.InfraredVision}, blind: true, want: SightNone},
		// Sleep is NOT a factor: a sleeper struck in a lit room is told what hit them.
		{name: "sleeping in a lit room", light: 1, flags: []buffs.Flag{buffs.Sleeping}, want: SightFull},
		{name: "sleeping with infrared in the dark", light: 0, flags: []buffs.Flag{buffs.Sleeping, buffs.InfraredVision}, want: SightShapes},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := sightChar(t, tc.flags...)
			if tc.blind {
				setBlinded(t, c)
			}
			if got := ParticipantSight(c, tc.light); got != tc.want {
				t.Fatalf("ParticipantSight = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParticipantSight_NilObserverSeesFully(t *testing.T) {
	if got := ParticipantSight(nil, sightLight(0)); got != SightFull {
		t.Fatalf("nil observer = %v, want SightFull, matching the other predicates", got)
	}
}

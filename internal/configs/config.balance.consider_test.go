package configs

import "testing"

func TestValidateConsider_Defaults(t *testing.T) {
	b := Balance{}
	b.Validate()
	want := map[string][2]float64{
		"PaceMoments":      {float64(b.ConsiderPaceMoments), 2},
		"PaceShort":        {float64(b.ConsiderPaceShort), 6},
		"PaceLong":         {float64(b.ConsiderPaceLong), 20},
		"BandUntouched":    {float64(b.ConsiderBandUntouched), 0.05},
		"BandHopelessDent": {float64(b.ConsiderBandHopelessDent), 0.05},
		"BandScratches":    {float64(b.ConsiderBandScratches), 0.25},
		"BandHurt":         {float64(b.ConsiderBandHurt), 0.6},
		"BandBarely":       {float64(b.ConsiderBandBarely), 1.0},
		"BandLethal":       {float64(b.ConsiderBandLethal), 3.0},
		"ClearReadZ":       {float64(b.ConsiderClearReadZ), 1.0},
	}
	for name, gw := range want {
		if gw[0] != gw[1] {
			t.Errorf("Consider%s = %v, want default %v", name, gw[0], gw[1])
		}
	}
}

// A ladder out of order would make a band unreachable; it resets to the
// shipped ladder rather than guessing which value was meant.
func TestValidateConsider_OutOfOrderLaddersReset(t *testing.T) {
	b := Balance{ConsiderPaceMoments: 10, ConsiderPaceShort: 6, ConsiderPaceLong: 20,
		ConsiderBandScratches: 2.0, ConsiderBandHurt: 0.6}
	b.validateConsider()
	if b.ConsiderPaceMoments != 2 || b.ConsiderPaceShort != 6 || b.ConsiderPaceLong != 20 {
		t.Errorf("pace ladder = %v/%v/%v, want 2/6/20", b.ConsiderPaceMoments, b.ConsiderPaceShort, b.ConsiderPaceLong)
	}
	if b.ConsiderBandUntouched != 0.05 || b.ConsiderBandScratches != 0.25 ||
		b.ConsiderBandHurt != 0.6 || b.ConsiderBandBarely != 1.0 || b.ConsiderBandLethal != 3.0 {
		t.Errorf("band ladder not reset: %v/%v/%v/%v/%v, want 0.05/0.25/0.6/1/3",
			b.ConsiderBandUntouched, b.ConsiderBandScratches, b.ConsiderBandHurt,
			b.ConsiderBandBarely, b.ConsiderBandLethal)
	}
}

func TestValidateConsider_ValidCustomLadderKept(t *testing.T) {
	b := Balance{ConsiderPaceMoments: 1, ConsiderPaceShort: 3, ConsiderPaceLong: 9,
		ConsiderBandUntouched: 0.1, ConsiderBandScratches: 0.3, ConsiderBandHurt: 0.7,
		ConsiderBandBarely: 1.5, ConsiderBandLethal: 4.0}
	b.validateConsider()
	if b.ConsiderPaceMoments != 1 || b.ConsiderPaceShort != 3 || b.ConsiderPaceLong != 9 {
		t.Errorf("pace ladder changed: %v/%v/%v", b.ConsiderPaceMoments, b.ConsiderPaceShort, b.ConsiderPaceLong)
	}
	if b.ConsiderBandUntouched != 0.1 || b.ConsiderBandScratches != 0.3 ||
		b.ConsiderBandHurt != 0.7 || b.ConsiderBandBarely != 1.5 || b.ConsiderBandLethal != 4.0 {
		t.Errorf("band ladder changed: %v/%v/%v/%v/%v", b.ConsiderBandUntouched,
			b.ConsiderBandScratches, b.ConsiderBandHurt, b.ConsiderBandBarely, b.ConsiderBandLethal)
	}
}

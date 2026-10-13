package configs

// validateConsider sets defaults for the consider fight-read knobs (#301).
// Paces and bands are strict ascending ladders; a ladder out of order would
// make a step unreachable, so it resets to the shipped values whole.
func (b *Balance) validateConsider() {
	if b.ConsiderPaceMoments <= 0 {
		b.ConsiderPaceMoments = 2
	}
	if b.ConsiderPaceShort <= 0 {
		b.ConsiderPaceShort = 6
	}
	if b.ConsiderPaceLong <= 0 {
		b.ConsiderPaceLong = 20
	}
	if !(b.ConsiderPaceMoments < b.ConsiderPaceShort && b.ConsiderPaceShort < b.ConsiderPaceLong) {
		b.ConsiderPaceMoments, b.ConsiderPaceShort, b.ConsiderPaceLong = 2, 6, 20
	}

	if b.ConsiderBandUntouched <= 0 {
		b.ConsiderBandUntouched = 0.05
	}
	if b.ConsiderBandHopelessDent <= 0 {
		b.ConsiderBandHopelessDent = 0.05
	}
	if b.ConsiderBandScratches <= 0 {
		b.ConsiderBandScratches = 0.25
	}
	if b.ConsiderBandHurt <= 0 {
		b.ConsiderBandHurt = 0.6
	}
	if b.ConsiderBandBarely <= 0 {
		b.ConsiderBandBarely = 1.0
	}
	if b.ConsiderBandLethal <= 0 {
		b.ConsiderBandLethal = 3.0
	}
	if !(b.ConsiderBandUntouched < b.ConsiderBandScratches &&
		b.ConsiderBandScratches < b.ConsiderBandHurt &&
		b.ConsiderBandHurt < b.ConsiderBandBarely &&
		b.ConsiderBandBarely < b.ConsiderBandLethal) {
		b.ConsiderBandUntouched, b.ConsiderBandScratches, b.ConsiderBandHurt,
			b.ConsiderBandBarely, b.ConsiderBandLethal = 0.05, 0.25, 0.6, 1.0, 3.0
	}

	if b.ConsiderClearReadZ <= 0 {
		b.ConsiderClearReadZ = 1.0
	}
}

package configs

import "testing"

// #375 phase 3a: the search difficulties. Absent or non-positive falls back to
// the shipped 175 (plain search) and 125 (feature search); a set value stays.
func TestSearchHiddenDifficultyDefaults(t *testing.T) {
	for _, tc := range []struct {
		room, feature         float64
		wantRoom, wantFeature float64
	}{
		{0, 0, 175, 125},
		{-1, -5, 175, 125},
		{200, 90, 200, 90},
	} {
		b := Balance{SearchRoomHiddenDifficulty: ConfigFloat(tc.room), SearchFeatureHiddenDifficulty: ConfigFloat(tc.feature)}
		b.validateMisc()
		if float64(b.SearchRoomHiddenDifficulty) != tc.wantRoom || float64(b.SearchFeatureHiddenDifficulty) != tc.wantFeature {
			t.Errorf("in %v/%v: got %v/%v, want %v/%v", tc.room, tc.feature,
				b.SearchRoomHiddenDifficulty, b.SearchFeatureHiddenDifficulty, tc.wantRoom, tc.wantFeature)
		}
	}
}

package messaging

import "testing"

// TestBlankLineAllowedIsTheScreenSet pins the categories a deliberate blank
// line may ride (#610): the screen categories, and no combat, ambient,
// speech or refusal category. It walks the whole enum, so a new category is
// refused until someone decides it is a screen.
func TestBlankLineAllowedIsTheScreenSet(t *testing.T) {
	want := map[Category]bool{
		CategorySystem:          true,
		CategoryScreen:          true,
		CategoryAdmin:           true,
		CategoryRoomDescription: true,
	}
	for c := CategoryDefault; c < categoryMax; c++ {
		if got := BlankLineAllowed(c); got != want[c] {
			t.Errorf("BlankLineAllowed(%s) = %v, want %v", c, got, want[c])
		}
	}
}

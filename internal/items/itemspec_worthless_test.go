package items

import "testing"

// Validate prices an item with no authored value; worthless opts out of that,
// and overrides any authored value, so the field means one thing.
func TestValidate_WorthlessKeepsNoValue(t *testing.T) {
	priced := ItemSpec{ItemId: 975701, Name: "priced"}
	if err := priced.Validate(); err != nil {
		t.Fatalf("priced: %v", err)
	}
	if priced.Value < 1 {
		t.Fatalf("the control must be priced, or this proves nothing: %d", priced.Value)
	}

	for _, authored := range []int{0, 40} {
		spec := ItemSpec{ItemId: 975702, Name: "prop", Worthless: true, Value: authored}
		if err := spec.Validate(); err != nil {
			t.Fatalf("worthless: %v", err)
		}
		if spec.Value != 0 {
			t.Errorf("authored value %d: Value = %d, want 0", authored, spec.Value)
		}
	}
}

package combat

import (
	"reflect"
	"testing"
)

// The dark-combat bucketing (M6 slice 2) reads a per-swing defence result.
// AttackResult.DefenseUsed is set on a defensive win and never cleared, so it
// cannot answer "did THIS swing's defence win". SwingEvent.Defended does.
func TestSwingEvent_HasPerSwingDefended(t *testing.T) {
	f, ok := reflect.TypeOf(SwingEvent{}).FieldByName("Defended")
	if !ok {
		t.Fatal("SwingEvent has no Defended field")
	}
	if f.Type.Kind() != reflect.Bool {
		t.Fatalf("SwingEvent.Defended is %v, want bool", f.Type)
	}
}

func TestHitResolution_DefenceWonFeedsDefended(t *testing.T) {
	cases := []struct {
		r    hitResolution
		want bool
	}{
		{hitResolution{}, false},
		{hitResolution{defended: true}, true},
		{hitResolution{defenseCrit: true}, true},
	}
	for _, c := range cases {
		if got := swingEventFor(c.r).Defended; got != c.want {
			t.Fatalf("swingEventFor(%+v).Defended = %v, want %v", c.r, got, c.want)
		}
	}
}

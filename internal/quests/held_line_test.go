package quests

import (
	"strings"
	"testing"
)

func TestValidate_HeldLineNeedsAPlainItemGain(t *testing.T) {
	for _, tc := range []struct {
		name string
		t    TriggerDef
		want string
	}{
		{"wrong event", TriggerDef{Event: "room_enter", HeldLine: "x"}, "held_line needs a plain item_gain"},
		{"reason", TriggerDef{Event: "item_gain", Reason: "steal", HeldLine: "x"}, "held_line needs a plain item_gain"},
		{"room", TriggerDef{Event: "item_gain", Room: 5, HeldLine: "x"}, "held_line needs a plain item_gain"},
		{"count", TriggerDef{Event: "item_gain", Count: 2, HeldLine: "x"}, "held_line needs a plain item_gain"},
	} {
		err := validateAreaCount(tc.t)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.want)
		}
	}
	if err := validateAreaCount(TriggerDef{Event: "item_gain", Item: 5, HeldLine: "x"}); err != nil {
		t.Errorf("a plain item_gain may carry held_line: %v", err)
	}
}

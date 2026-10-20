package events

import "testing"

func TestCmdTyped_IsADistinctBit(t *testing.T) {
	for _, f := range []EventFlag{CmdSkipScripts, CmdSecretly, CmdIsRequeue, CmdBlockInput, CmdUnBlockInput, CmdFollowStep} {
		if f&CmdTyped != 0 {
			t.Fatalf("CmdTyped overlaps flag %b", f)
		}
	}
	var f EventFlag
	if f.Has(CmdTyped) {
		t.Fatal("zero flags must not read as typed")
	}
	f = CmdTyped
	if !f.Has(CmdTyped) {
		t.Fatal("CmdTyped not detected")
	}
}

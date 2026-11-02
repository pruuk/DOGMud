package questloop

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// #375 phase 3b slice 4: two triggers that differ only by area, zone, count
// or distinct are different ways in, so their signatures differ.
func TestTriggerSignatureAreaFields(t *testing.T) {
	td := &quests.TriggerDef{Event: "room_enter", Area: "grove", Distinct: "all", Actions: []quests.ActionDef{{Grant: "5-end"}}}
	if got, want := triggerSignature(td), "room_enter area=grove distinct=all grants=5-end"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	td = &quests.TriggerDef{Event: "mob_death", Mob: 9, Zone: "Thornwall", Count: 3, Actions: []quests.ActionDef{{Grant: "5-end"}}}
	if got, want := triggerSignature(td), "mob_death mob=9 zone=Thornwall count=3 grants=5-end"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// #375 phase 7: two item_gain triggers that differ only by reason are different
// ways in.
func TestTriggerSignatureReason(t *testing.T) {
	td := &quests.TriggerDef{Event: "item_gain", Item: 60, Reason: "steal", Actions: []quests.ActionDef{{Grant: "5-end"}}}
	if got, want := triggerSignature(td), "item_gain item=60 reason=steal grants=5-end"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

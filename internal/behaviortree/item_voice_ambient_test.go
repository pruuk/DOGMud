package behaviortree

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
)

func pinAmbientBudget(t *testing.T, rounds int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.AmbientListenerBudgetRounds = configs.ConfigInt(rounds)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ambient.ResetForTest())
}

// An item's ambient (item_idle) line counts toward the listener budget
// (#317): a listener over budget skips the room line, and so does a holder
// over budget skip their own line.
func TestItemIdleLine_SkipsAListenerOverBudget(t *testing.T) {
	seedVoiceWorld(t)
	pinAmbientBudget(t, 5)
	ambient.ChargeForTest(voiceListenerId, 5005)
	if !TryItemBehavior(voiceEvent("item_idle"), wornProbe(voiceProbeItemId, 1)) {
		t.Fatal("the idle line must still go out to the holder")
	}
	if got := drained(voiceListenerId); strings.Contains(got, "mutters") {
		t.Errorf("the listener is over budget and must skip the room line, got %q", got)
	}
	if got := drained(voiceBearerId); !strings.Contains(got, "says") {
		t.Errorf("the holder has budget and must read the item's line, got %q", got)
	}
}

func TestItemIdleLine_SkipsAHolderOverBudget(t *testing.T) {
	seedVoiceWorld(t)
	pinAmbientBudget(t, 5)
	ambient.ChargeForTest(voiceBearerId, 5005)
	TryItemBehavior(voiceEvent("item_idle"), wornProbe(voiceProbeItemId, 1))
	if got := drained(voiceBearerId); strings.Contains(got, "says") {
		t.Errorf("the holder is over budget and must skip the item's line, got %q", got)
	}
	if got := drained(voiceListenerId); !strings.Contains(got, "mutters") {
		t.Errorf("the listener has budget and must hear the room line, got %q", got)
	}
}

// An event line (on_equip) is not ambient and is never budgeted.
func TestItemEventLine_IsNeverBudgeted(t *testing.T) {
	seedVoiceWorld(t)
	pinAmbientBudget(t, 5)
	ambient.ChargeForTest(voiceBearerId, 5005)
	ambient.ChargeForTest(voiceListenerId, 5005)
	TryItemBehavior(voiceEvent("on_equip"), wornProbe(voiceProbeItemId, 1))
	if got := drained(voiceListenerId); !strings.Contains(got, "Up we go") {
		t.Errorf("an event line is exempt; the listener must hear it, got %q", got)
	}
}

const voiceHolderIdleTree = `
speech:
  on_idle: ["Psst."]
tree:
  type: action
  event: item_idle
  do: speak
  pool: on_idle
  to: holder
  paced: false
`

const voiceHolderIdleItemId = 9903

// A holder-only idle line is heard by the holder alone, so only the holder
// is charged: a bystander keeps their budget (controller fix, Task 7 review).
func TestItemIdleLine_HolderOnlyChargesOnlyTheHolder(t *testing.T) {
	seedVoiceWorld(t)
	pinAmbientBudget(t, 5)
	// SeedItemsForTest replaces the registry, so the probe item comes along.
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		voiceProbeItemId:      {ItemId: voiceProbeItemId, Name: "Probe Shield", Type: items.Offhand, Behavior: "voice_nodes_probe"},
		voiceHolderIdleItemId: {ItemId: voiceHolderIdleItemId, Name: "Whisper Ring", Type: items.Offhand, Behavior: "voice_holder_idle"},
	}))
	LoadItemTreeForTest(t, "voice_holder_idle", voiceHolderIdleTree)

	if !TryItemBehavior(voiceEvent("item_idle"), wornProbe(voiceHolderIdleItemId, 2)) {
		t.Fatal("the holder-only idle line must go out")
	}
	if got := drained(voiceBearerId); !strings.Contains(got, "Psst.") {
		t.Fatalf("the holder must read the line, got %q", got)
	}
	if got := drained(voiceListenerId); got != "" {
		t.Errorf("a bystander hears nothing of a holder-only line, got %q", got)
	}
	// Now a second item speaks to the whole room. The holder was charged by
	// the line above and skips it; the bystander was not, and hears it.
	if !TryItemBehavior(voiceEvent("item_idle"), wornProbe(voiceProbeItemId, 1)) {
		t.Fatal("the room-wide idle line must still go out")
	}
	if got := drained(voiceBearerId); strings.Contains(got, "says") {
		t.Errorf("the holder was charged by the holder-only line and must skip this one, got %q", got)
	}
	if got := drained(voiceListenerId); !strings.Contains(got, "mutters") {
		t.Errorf("the bystander was not charged and must hear the room line, got %q", got)
	}
}

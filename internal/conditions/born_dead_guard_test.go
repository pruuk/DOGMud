package conditions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// bornDeadExceptions are shipped conditions that are DELIBERATELY inert when
// applied non-permanently, keyed by condition id with the reason.
//
// Keep this map as small as the truth allows. An entry here is a claim that
// nothing should ever apply the condition through an ordinary, non-permanent
// path, which includes the `setcondition` admin command.
var bornDeadExceptions = map[int]string{
	9: "Hidden is granted only by stealth paths that pass permanent=true " +
		"(see internal/actions/shadow.go and the behaviortree skullduggery " +
		"actions); an ordinary apply is not a supported way to become hidden",
	133: "Refraction is a rift construct's nature, held permanently through " +
		"its mob conditionids (the Splitlight, 9837); it is never applied " +
		"for a time",
}

// TestNoShippedConditionIsBornDead pins the invariant that a condition you can
// apply must actually be held afterwards.
//
// 🔴 THE DEFECT THIS EXISTS FOR. Condition.Expired() is TriggersLeft <= 0, and
// AddCondition seeds TriggersLeft from the spec's TriggerCount. A condition
// authored with no `triggercount` therefore gets 0 and is EXPIRED THE INSTANT
// IT IS APPLIED: GetConditions hides it and HasFlag skips it, so its flags
// never take effect and nothing reports an error.
//
// ConditionSpec.Validate does not catch this. Its `TriggerCount < 1` check is
// nested inside `if b.TriggerRate != ""`, so a pure flag condition with no
// triggerrate skips the guard entirely.
//
// That is how condition 85 (InfraredVision) shipped inert. It was applied
// successfully by `setcondition 85`, reported as applied, and then did
// nothing, so infrared vision never worked for anyone by any route a player
// or admin could reach. Three separate playtests recorded the shapes sight
// tier as "unverified" before the cause was found, and each time the playtest
// PROFILE was blamed.
func TestNoShippedConditionIsBornDead(t *testing.T) {
	mudlog.SetupLogger(nil, "", "", false)
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString("../../_datafiles/world/dogmud")
	// Condition 0 derives its TriggerCount from this at Validate time and
	// refuses 0; a test binary never reads config.yaml.
	cfg.Network.LogoutRounds = 3
	configs.SetConfigForTest(t, cfg)
	LoadDataFiles()

	ids := GetAllConditionIds()
	if len(ids) == 0 {
		t.Fatal("no conditions loaded; this test cannot fail and so proves nothing")
	}

	checked := 0
	for _, id := range ids {
		spec := GetConditionSpec(id)
		if spec == nil || spec.IsStacking() {
			// A stacking record is applied only through AddConditionMagnitude,
			// which supplies its own trigger count.
			continue
		}
		checked++

		bs := New()
		if !bs.AddCondition(id, false) {
			// Refused for a reason of its own (poison immunity and the like).
			continue
		}
		held := bs.GetConditions(id)
		reason, excepted := bornDeadExceptions[id]

		if len(held) == 0 {
			if excepted {
				continue
			}
			t.Errorf("condition %d (%s) is BORN DEAD: applied non-permanently it is "+
				"immediately expired, so its flags %v never take effect and nothing "+
				"reports an error. Author a `triggercount` of at least 1, or add it to "+
				"bornDeadExceptions with a reason.",
				id, spec.Name, spec.Flags)
			continue
		}
		if excepted {
			t.Errorf("condition %d (%s) is listed in bornDeadExceptions (%q) but is in "+
				"fact held after an ordinary apply. Remove the stale entry.",
				id, spec.Name, reason)
		}
	}

	if checked == 0 {
		t.Fatal("no non-stacking conditions checked; the loop cannot fail")
	}
	t.Logf("checked %d shipped conditions", checked)
}

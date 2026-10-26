package guards

import (
	"sort"
	"strconv"
	"strings"
	"testing"
)

// acquireSeamFile is the one file allowed to call Character.StoreItem for an
// acquisition: it stores and then raises ItemOwnership{Gained}.
const acquireSeamFile = "internal/acquire/acquire.go"

// storeItemAllowlist names every other production file that calls a method
// named StoreItem, with the reason it is not an acquisition. The guard is
// syntactic (it matches the method name, not the receiver type), like the
// other call guards here, so the Pet.StoreItem callers are listed too. Keyed
// by file, not line, so edits do not break it. A new entry needs a reason a
// reviewer accepts: anything that hands a player an item belongs in
// internal/acquire so quest item_gain triggers hear it.
var storeItemAllowlist = map[string]string{
	"internal/actions/remove_equip.go":            "unequip and displaced gear go back to the player's own pack",
	"internal/actions/transfer.go":                "rollback to the giver when the receiver refuses",
	"internal/actions/steal_pocket.go":            "a bauble moved into the mark's own pocket",
	"internal/characters/inventory.go":            "ReturnToPack: an own-container move",
	"internal/usercommands/admin.rename.go":       "admin re-store of an item after an edit",
	"internal/usercommands/admin.redescribe.go":   "admin re-store of an item after an edit",
	"internal/usercommands/get.go":                "Pet.StoreItem: a companion's inventory, not a player's",
	"internal/usercommands/give.go":               "Pet.StoreItem: a companion's inventory, not a player's",
	"internal/playtestprofiles/overlay.go":        "playtest harness setup",
	"internal/behaviortree/actions_wagon.go":      "cargo handling on a wagon, not a player acquisition",
	"internal/caravan/arrival_listener.go":        "caravan cargo handling",
	"internal/caravan/cargo_handoff.go":           "caravan cargo handling",
	"internal/caravan/import_load.go":             "caravan cargo handling",
	"internal/caravan/visit.go":                   "caravan cargo handling",
	"internal/ferry/factor.go":                    "ferry factor cargo handling",
	"internal/hooks/mob_equip_best_floor_item.go": "mob inventory (ruling 2026-10-26: no quest listener acts on mobs)",
	"internal/hooks/NewRound_MobRoundTick.go":     "mob inventory (ruling 2026-10-26: no quest listener acts on mobs)",
	"internal/mobs/crafter.go":                    "mob inventory (ruling 2026-10-26: no quest listener acts on mobs)",
	"modules/aicompanion/combat.go":               "mob inventory (companion; ruling 2026-10-26)",
	"modules/aicompanion/cooking.go":              "mob inventory (companion; ruling 2026-10-26)",
}

// TestStoreItemOnlyThroughTheAcquireSeam fails on any StoreItem call outside
// internal/acquire/acquire.go that is not an allowlisted non-acquisition move.
func TestStoreItemOnlyThroughTheAcquireSeam(t *testing.T) {
	calls, _ := walkGoCalls(t, map[string]bool{"StoreItem": true})
	seen := map[string]bool{}
	var violations []string
	for _, c := range calls {
		seen[c.file] = true
		if c.file == acquireSeamFile {
			continue
		}
		if _, ok := storeItemAllowlist[c.file]; ok {
			continue
		}
		violations = append(violations, c.file+":"+strconv.Itoa(c.line))
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("StoreItem called outside internal/acquire (use acquire.AcquireItem, or allowlist the file with a reason):\n  %s",
			strings.Join(violations, "\n  "))
	}
	if !seen[acquireSeamFile] {
		t.Fatalf("the scan never saw %s: the guard cannot find calls", acquireSeamFile)
	}
	for file := range storeItemAllowlist {
		if !seen[file] {
			t.Errorf("allowlist entry %s has no StoreItem call any more: remove it", file)
		}
	}
}

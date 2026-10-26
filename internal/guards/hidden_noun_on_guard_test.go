package guards

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// checkHiddenNounOn returns one message per hidden noun of the room whose
// `on` does not name the visible thing it is tucked into (#375 phase 3a,
// owner rulings 6 and 7). `on` must be the canonical key of a room noun (an
// alias such as `stones: ":floor"` never matches: search compares `on` with
// the feature's canonical name), or the key of another hidden noun of the
// same room, for a chain. A chain must end at a room noun: a hidden noun
// tucked into itself, or into a loop of hidden nouns, can never be found.
// Keys compare case-insensitively, as actions.searchFeatureSecrets does.
func checkHiddenNounOn(roomId int, r *rooms.Room) []string {
	var bad []string
	keys := make([]string, 0, len(r.HiddenNouns))
	for k := range r.HiddenNouns {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// parent resolves on to a room noun ("", true), a hidden noun (key,
	// true) or nothing (_, false).
	parent := func(on string) (string, bool, string) {
		for name, desc := range r.Nouns {
			if strings.EqualFold(name, on) {
				if strings.HasPrefix(desc, ":") {
					return "", false, fmt.Sprintf("names the alias %q; use the noun it points at, %q", name, strings.TrimPrefix(desc, ":"))
				}
				return "", true, ""
			}
		}
		for _, k := range keys {
			if strings.EqualFold(k, on) {
				return k, true, ""
			}
		}
		return "", false, "names no room noun or hidden noun of the room"
	}

	for _, k := range keys {
		where := fmt.Sprintf("room %d hidden noun %q", roomId, k)
		on := strings.TrimSpace(r.HiddenNouns[k].On)
		if on == "" {
			bad = append(bad, where+": has no `on` (the visible thing it is tucked into)")
			continue
		}
		hidden, ok, why := parent(on)
		if !ok {
			bad = append(bad, fmt.Sprintf("%s: on %q %s", where, on, why))
			continue
		}
		// Follow a chain of hidden nouns to the room noun it must end at.
		seen := map[string]bool{k: true}
		for hidden != "" {
			if seen[hidden] {
				bad = append(bad, fmt.Sprintf("%s: on %q is part of a loop of hidden nouns that never reaches a room noun", where, on))
				break
			}
			seen[hidden] = true
			next := strings.TrimSpace(r.HiddenNouns[hidden].On)
			if next == "" {
				break // reported on its own entry
			}
			if hidden, ok, _ = parent(next); !ok {
				break // reported on its own entry
			}
		}
	}
	return bad
}

// TestHiddenNounsNameWhereTheyAreTucked: every shipped hidden noun names, in
// `on`, a room noun or another hidden noun of its room.
func TestHiddenNounsNameWhereTheyAreTucked(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()

	roomIds := rooms.GetAllRoomIds()
	sort.Ints(roomIds)
	if len(roomIds) == 0 {
		t.Fatal("loaded no rooms: the guard cannot run")
	}
	checked := 0
	for _, rid := range roomIds {
		r := rooms.LoadRoom(rid)
		if r == nil {
			t.Fatalf("room %d failed to load", rid)
		}
		checked += len(r.HiddenNouns)
		for _, msg := range checkHiddenNounOn(rid, r) {
			t.Error(msg)
		}
	}
	// Floor: eleven rooms shipped hidden nouns when this guard was written.
	// None found means the scan, not the content, is broken.
	if checked == 0 {
		t.Fatalf("found no hidden noun in %d rooms: the guard checked nothing", len(roomIds))
	}
}

// TestHiddenNounOnGuard_CanFail proves the check reports each planted fault
// and accepts a room noun, a differently cased key and a chain.
func TestHiddenNounOnGuard_CanFail(t *testing.T) {
	nouns := map[string]string{"floor": "Cracked flags.", "stones": ":floor", "Elgar's grave": "A marker."}
	mk := func(hidden map[string]rooms.HiddenNoun) *rooms.Room {
		return &rooms.Room{Nouns: nouns, HiddenNouns: hidden}
	}

	bad := map[string]map[string]rooms.HiddenNoun{
		"no on":         {"disc": {}},
		"blank on":      {"disc": {On: "  "}},
		"unknown noun":  {"disc": {On: "chandelier"}},
		"alias":         {"disc": {On: "stones"}},
		"self":          {"disc": {On: "disc"}},
		"loop":          {"a": {On: "b"}, "b": {On: "a"}},
		"chain to none": {"a": {On: "b"}, "b": {On: "chandelier"}},
	}
	for name, hidden := range bad {
		if got := checkHiddenNounOn(1, mk(hidden)); len(got) == 0 {
			t.Errorf("planted %s not reported", name)
		}
	}

	good := map[string]map[string]rooms.HiddenNoun{
		"room noun":   {"disc": {On: "floor"}},
		"case folded": {"mark": {On: "elgar's grave"}},
		"chain":       {"seam": {On: "floor"}, "pouch": {On: "seam"}},
	}
	for name, hidden := range good {
		if got := checkHiddenNounOn(1, mk(hidden)); len(got) != 0 {
			t.Errorf("%s wrongly reported: %v", name, got)
		}
	}
}

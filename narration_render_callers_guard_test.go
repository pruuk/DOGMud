package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// narrationRenderCallers is every production file allowed to call
// narration.Render, with why. The Kind B stores (conditions, spells, quests) are
// deliberately ABSENT: they reach the core only through textutil.Narrate,
// which always passes narration.FirstPicker. A store calling Render itself
// could pass the default picker and consume a global random draw per
// narrated phase (see narration.FirstPicker), which no golden can see.
var narrationRenderCallers = map[string]string{
	"internal/behaviortree/actions_item_voice.go": "Kind A: item tree speech pools (item behaviour slice 2), single role (the item speaks); the default picker is deliberate, so a seeded util.Rand replays it",
	"internal/combat/grapple_narration.go":        "Kind A: grapple's crit-failure and disarm events, the special-move store's only internal/combat consumer (both are shared with internal/mobcommands/usercommands twins, so they cannot live in move_narration.go without a cycle) (M4e-1)",
	"internal/combat/taunt_messages.go":           "Kind A: the taunt store's coordinated triad",
	"internal/gossip/gossip.go":                   "Kind A: gossip template pools, single role (the gossiping NPC)",
	"internal/grapplemessaging/render.go":         "Kind A: the grapple store's coordinated triad",
	"internal/items/defensive_messages.go":        "Kind A: the defence store's coordinated triad",
	"internal/items/attack_messages.go":           "Kind A: the combat-message store's coordinated triad and ranged quartet (M3 item 8)",
	"internal/lightnotice/store.go":               "Kind A: light-band change notices, single role (the observer is told); the default picker is deliberate, a notice is rare and variety is the point (lighting plan 3d)",
	"internal/mobcommands/move_narration.go":      "Kind A: the special-move store's per-event triad, shared call-site helper for kick and the twelve mob verbs after it (M4e-1 Task 7)",
	"internal/usercommands/move_narration.go":     "Kind A: the special-move store's per-event triad, the player-side twin of mobcommands/move_narration.go, shared call-site helper for the player special-move verbs (M4e-1b)",
	"internal/spells/casting_messages.go":         "Kind A: the caster-only casting pools",
	"internal/textutil/narrate.go":                "the ONE door for the Kind B stores; must pass narration.FirstPicker",

	// The first and only entry outside internal/. Weather is a module, and
	// modules/weather/content carries its own purity rule (arch_test.go) that
	// forbids internal/* imports except for a one-package allowlist naming
	// narration; see that file before adding a second module here.
	//
	// It is also the arc's ONLY ACTORLESS store: an ambient line has no Actor
	// and no Actee, so renderAmbient populates Observer alone rather than
	// inventing a subject for the weather.
	"modules/weather/content/emotes.go": "Kind A: weather ambient emote pools, single role (the room is told)",
}

var narrationRenderCallRE = regexp.MustCompile(`narration\.Render\(`)

// The textutil door must hand Render FirstPicker, and must never name the
// default picker at all.
//
// This regex assumes narrate.go holds exactly ONE narration.Render call, which
// is true today (a single 31-line door). Go source carries no literal ";"
// between statements, so with (?s) the `[^;]*?` span can reach past the end of
// a call; a second Render call added later could satisfy the pattern while an
// earlier one passes a different picker. The DefaultPicker substring ban below
// is the load-bearing half. If the door ever grows a second call, switch this
// to an AST check of each call's third argument, the way
// condition_apply_path_guard_test.go inspects call arguments.
var textutilFirstPickerRE = regexp.MustCompile(`(?s)narration\.Render\([^;]*?narration\.FirstPicker\)`)

func TestNarrationRenderIsCalledOnlyByRegisteredStores(t *testing.T) {
	found := map[string]bool{}
	for _, root := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					// A test elsewhere can create and remove a temp file under
					// the tree while packages test in parallel; a vanished
					// entry has nothing to scan.
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			if narrationRenderCallRE.Match(src) {
				found[filepath.ToSlash(path)] = true
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if len(found) == 0 {
		t.Fatal("no narration.Render call found anywhere; the walk is broken, not the code")
	}

	var unregistered, stale []string
	for f := range found {
		if _, ok := narrationRenderCallers[f]; !ok {
			unregistered = append(unregistered, f)
		}
	}
	for f := range narrationRenderCallers {
		if !found[f] {
			stale = append(stale, f)
		}
	}
	sort.Strings(unregistered)
	sort.Strings(stale)
	if len(unregistered) > 0 {
		t.Errorf("narration.Render is called from unregistered file(s):\n  %s\n\nA Kind B store (conditions, spells, quests) must render through textutil.Narrate, never Render directly. A new Kind A store registers here with a reason.", strings.Join(unregistered, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("registered Render caller(s) no longer call it:\n  %s\n\nRemove the entry only after confirming the store did not lose its rendering.", strings.Join(stale, "\n  "))
	}

	door, err := os.ReadFile("internal/textutil/narrate.go")
	if err != nil {
		t.Fatalf("read the textutil door: %v", err)
	}
	if !textutilFirstPickerRE.Match(door) {
		t.Errorf("internal/textutil/narrate.go must call narration.Render with narration.FirstPicker as the picker; a single-variant store must not consume a random draw")
	}
	if strings.Contains(string(door), "DefaultPicker") {
		t.Errorf("internal/textutil/narrate.go names DefaultPicker; the Kind B door must never draw")
	}
}

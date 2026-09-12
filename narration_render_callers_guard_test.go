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
// narration.Render, with why. The Kind B stores (buffs, spells, quests) are
// deliberately ABSENT: they reach the core only through textutil.Narrate,
// which always passes narration.FirstPicker. A store calling Render itself
// could pass the default picker and consume a global random draw per
// narrated phase (see narration.FirstPicker), which no golden can see.
var narrationRenderCallers = map[string]string{
	"internal/combat/taunt_messages.go":    "Kind A: the taunt store's coordinated triad",
	"internal/grapplemessaging/render.go":  "Kind A: the grapple store's coordinated triad",
	"internal/items/defensive_messages.go": "Kind A: the defence store's coordinated triad",
	"internal/itemvoices/itemvoices.go":    "Kind A: sentient item voices, single role",
	"internal/spells/casting_messages.go":  "Kind A: the caster-only casting pools",
	"internal/textutil/narrate.go":         "the ONE door for the Kind B stores; must pass narration.FirstPicker",
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
// buff_apply_path_guard_test.go inspects call arguments.
var textutilFirstPickerRE = regexp.MustCompile(`(?s)narration\.Render\([^;]*?narration\.FirstPicker\)`)

func TestNarrationRenderIsCalledOnlyByRegisteredStores(t *testing.T) {
	found := map[string]bool{}
	for _, root := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
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
		t.Errorf("narration.Render is called from unregistered file(s):\n  %s\n\nA Kind B store (buffs, spells, quests) must render through textutil.Narrate, never Render directly. A new Kind A store registers here with a reason.", strings.Join(unregistered, "\n  "))
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

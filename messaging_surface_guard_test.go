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

// surfaceScope classifies WHY a registered key spelling is player-facing text,
// and by extension which arc/owner is responsible for cleaning it up.
type surfaceScope int

const (
	// narration is an event narrated at the player as something happens --
	// combat swings, movement, spell effects, idle chatter. The messaging
	// unification arc owns these.
	narration surfaceScope = iota
	// content is authored text a player reads on request -- room and item
	// descriptions, dialogue, help text.
	content
	// config is not player prose at all -- colour aliases, keyword tables, or
	// other structural data that happens to contain a stem like "message" or
	// "text" in its key name.
	config
)

// surfaceEntry documents one registered text-bearing YAML key spelling: the
// scope it belongs to and a one-line reason a reviewer can trust without
// re-deriving it.
type surfaceEntry struct {
	Scope  surfaceScope
	Reason string
}

// textSurfaceRegistry is the locked inventory of every text-bearing YAML key
// spelling this guard's own walk finds appearing in 2+ files -- a SCHEMA key,
// per splitSchemaContent below, meaning some loader owns it and it recurs by
// construction rather than by author coincidence. A spelling found in exactly
// one file is author-invented content (the overwhelming case is room `nouns:`
// children -- there are thousands of them) and does NOT need an entry here.
//
// Left EMPTY as of the sweep that added this guard (messaging arc M0, task 5).
// TestEveryTextSurfaceIsRegistered is EXPECTED TO FAIL until the follow-up
// task populates this map -- that failure is this task's deliverable: it
// proves the walk actually reaches the data.
//
// This guard owns its OWN walk of _datafiles/world/dogmud rather than reading
// tools/messaging_surface_audit.py's output. Two implementations that must
// agree would drift against each other; these are two instruments with
// different jobs -- the Python tool is a human-facing survey report, this is
// a CI-enforced recurrence guard.
var textSurfaceRegistry = map[string]surfaceEntry{}

// messagingSurfaceSkipDirs mirrors tools/messaging_surface_audit.py's
// SKIP_DIRS: runtime state, not authored content. Instance saves mirror
// templates, user saves are per-player, shops/guilds/moderation are living
// state (see CLAUDE.md).
var messagingSurfaceSkipDirs = map[string]bool{
	"mobs.instances":  true,
	"rooms.instances": true,
	"users":           true,
	"shops":           true,
	"guilds":          true,
	"moderation":      true,
	"plugin-data":     true,
	"warehouses":      true,
}

// messagingSurfaceKeyStems mirrors tools/messaging_surface_audit.py's
// KEY_STEMS: a key is a text candidate if its name CONTAINS any of these,
// deliberately substring rather than word-boundary matching. Over-reporting
// costs a registry line; under-reporting hides a surface.
var messagingSurfaceKeyStems = []string{
	"text", "message", "msg", "lines", "hint", "prose", "desc",
	"say", "emote", "voice", "phrase", "greeting", "taunt",
}

// messagingSurfaceAudienceKeys mirrors tools/messaging_surface_audit.py's
// AUDIENCE_KEYS: audience/role keys carry no stem but ARE the narration shape.
var messagingSurfaceAudienceKeys = map[string]bool{
	"toattacker": true, "todefender": true, "toroom": true, "observers": true,
	"controller": true, "controlled": true, "together": true, "separate": true,
	"options": true, "optionid": true,
}

// messagingSurfaceKeyRE mirrors tools/messaging_surface_audit.py's KEY_RE.
// Keys appear at line start, after a sequence dash, or inside a flow mapping
// (opened by `{` or continued by `,`). Multi-word keys are real (room
// `nouns:` blocks use author-chosen phrases like `hunt pool:`), and
// apostrophes occur too (`hunter's blind:`).
var messagingSurfaceKeyRE = regexp.MustCompile(`(?i)(?:^|[-{,])\s*([a-z_][a-z0-9_' -]*?)\s*:`)

// messagingSurfaceValueStartRE mirrors VALUE_START_RE: where a quoted value
// begins. Everything from there on is prose, and a colon inside prose ("She
// said: run") must not be mistaken for a key.
var messagingSurfaceValueStartRE = regexp.MustCompile(`:\s*["']`)

// messagingSurfaceBlockScalarOpenRE mirrors BLOCK_SCALAR_OPEN_RE: a YAML
// block-scalar opener (`key: |`, `key: >-`, `key: |2`, optionally with a
// trailing comment). Once seen, every following blank line or line indented
// MORE than this one is the scalar's VALUE, not new keys -- even if that
// value contains a colon.
var messagingSurfaceBlockScalarOpenRE = regexp.MustCompile(`:\s*[|>][+\-]?\d*\s*(?:#.*)?$`)

// messagingSurfaceKeysInLine returns the lowercased key spellings found on
// one line, scanning only up to where a quoted value begins.
func messagingSurfaceKeysInLine(line string) []string {
	head := line
	if loc := messagingSurfaceValueStartRE.FindStringIndex(line); loc != nil {
		head = line[:loc[0]+1]
	}
	var keys []string
	for _, m := range messagingSurfaceKeyRE.FindAllStringSubmatch(head, -1) {
		key := strings.ToLower(strings.TrimSpace(m[1]))
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

// messagingSurfaceIsCandidate mirrors is_candidate: an audience key, or any
// key whose name contains one of the stems.
func messagingSurfaceIsCandidate(key string) bool {
	if messagingSurfaceAudienceKeys[key] {
		return true
	}
	for _, stem := range messagingSurfaceKeyStems {
		if strings.Contains(key, stem) {
			return true
		}
	}
	return false
}

// messagingSurfaceIndent counts leading spaces (YAML indentation is spaces,
// never tabs).
func messagingSurfaceIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// messagingSurfaceCandidateKeysInFile scans one YAML file for candidate key
// spellings, block-scalar aware: once a `key: |` / `key: >` line opens a
// block, its blank or more-indented continuation lines are values, not new
// keys, until the first line indented at or below the opener's indent.
func messagingSurfaceCandidateKeysInFile(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	inBlock := false
	blockIndent := 0
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, "\r")
		if inBlock {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || messagingSurfaceIndent(line) > blockIndent {
				continue
			}
			inBlock = false
			// Falls through -- not a continuation, parsed normally below.
		}
		for _, key := range messagingSurfaceKeysInLine(line) {
			if messagingSurfaceIsCandidate(key) {
				found[key] = true
			}
		}
		if messagingSurfaceBlockScalarOpenRE.MatchString(line) {
			inBlock = true
			blockIndent = messagingSurfaceIndent(line)
		}
	}
	return found, nil
}

// messagingSurfaceWalk walks worldDir and returns, for every candidate key
// spelling found, the set of repo-relative files it appeared in.
func messagingSurfaceWalk(worldDir string) (map[string]map[string]bool, error) {
	keyFiles := map[string]map[string]bool{}
	err := filepath.WalkDir(worldDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if messagingSurfaceSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		rel, rerr := filepath.Rel(".", path)
		if rerr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)

		keys, kerr := messagingSurfaceCandidateKeysInFile(path)
		if kerr != nil {
			// An unreadable file is a filesystem problem, not this test's --
			// skip it rather than fail the whole walk on it.
			return nil
		}
		for key := range keys {
			if keyFiles[key] == nil {
				keyFiles[key] = map[string]bool{}
			}
			keyFiles[key][rel] = true
		}
		return nil
	})
	return keyFiles, err
}

// messagingSurfaceSplitSchema mirrors split_schema_content: a spelling found
// in 2+ files is schema (a loader reads it, so it recurs by construction); a
// spelling found in exactly 1 file is author-invented content (e.g. a room
// `nouns:` child) and is dropped here. Returns key -> one example file.
func messagingSurfaceSplitSchema(keyFiles map[string]map[string]bool) map[string]string {
	schema := map[string]string{}
	for key, files := range keyFiles {
		if len(files) < 2 {
			continue
		}
		var example string
		for f := range files {
			if example == "" || f < example {
				example = f
			}
		}
		schema[key] = example
	}
	return schema
}

// TestEveryTextSurfaceIsRegistered fails when this guard's own walk finds a
// schema-level text-bearing YAML key spelling that is not in
// textSurfaceRegistry, AND fails when a registered spelling no longer appears
// in 2+ files anywhere the walk looks.
//
// Both directions matter. A guard that only checks the first rots the moment
// a surface is renamed or deleted: the stale entry sits there forever,
// looking like coverage it no longer provides. This is M0 of the messaging
// unification arc -- the arc exists because curated inventories rot, and a
// hand-built store list already missed `idlemessages`, 1,285 occurrences and
// the largest single narration surface in the game.
//
// textSurfaceRegistry is deliberately EMPTY as of this task. Every schema key
// the walk finds is therefore reported unregistered, and the test fails. That
// failure is the deliverable: it proves the walk reaches the data. The
// follow-up task populates the registry, classifying each key's Scope
// (narration / content / config) with a reason.
//
// If you are here because this test failed on a genuinely new spelling: add
// it to textSurfaceRegistry with the scope that fits and a one-line reason.
// If you are here because a spelling vanished: find out what deleted the
// surface (`git log -S<key> -- _datafiles/world/dogmud` is a good start)
// before removing the entry -- a silently deleted narration surface is
// exactly the kind of regression this guard exists to catch.
func TestEveryTextSurfaceIsRegistered(t *testing.T) {
	worldDir := filepath.Join("_datafiles", "world", "dogmud")
	if _, err := os.Stat(worldDir); err != nil {
		t.Fatalf("world data not found at %s (test must run from the repo root): %v", worldDir, err)
	}

	keyFiles, err := messagingSurfaceWalk(worldDir)
	if err != nil {
		t.Fatalf("walk %s: %v", worldDir, err)
	}
	if len(keyFiles) == 0 {
		t.Fatal("no text-bearing keys found at all -- the walk is broken, not the data")
	}

	schema := messagingSurfaceSplitSchema(keyFiles)

	var unregistered []string
	for key, example := range schema {
		if _, ok := textSurfaceRegistry[key]; !ok {
			unregistered = append(unregistered, key+"  (e.g. "+example+")")
		}
	}
	sort.Strings(unregistered)

	var stale []string
	for key := range textSurfaceRegistry {
		if _, ok := schema[key]; !ok {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)

	if len(unregistered) > 0 {
		t.Errorf("%d text-bearing YAML key spelling(s) appear in 2+ files but are "+
			"not registered in textSurfaceRegistry:\n  %s\n\n"+
			"Each of these is a SCHEMA key -- some loader reads it and it recurs "+
			"across files by construction, unlike a one-off author-invented content "+
			"key (e.g. a room `nouns:` child), which needs no entry. Add a "+
			"textSurfaceRegistry entry for each, picking the scope that fits: "+
			"narration (an event narrated at the player -- the messaging arc owns "+
			"it), content (authored text a player reads on request), or config (not "+
			"player prose at all -- a colour alias or keyword table). Give each a "+
			"one-line reason.",
			len(unregistered), strings.Join(unregistered, "\n  "))
	}

	if len(stale) > 0 {
		t.Errorf("%d textSurfaceRegistry entr(y/ies) no longer appear in 2+ files "+
			"anywhere under %s:\n  %s\n\n"+
			"Either the surface was renamed or removed -- `git log -S<key> -- "+
			"_datafiles/world/dogmud` is a good way to find out what changed it -- "+
			"or it dropped to a single file and is now author-invented content "+
			"rather than a schema key. Either way, remove the stale entry from "+
			"textSurfaceRegistry once you understand why it disappeared. Do not "+
			"remove it just to make the test pass without checking first: a "+
			"disappearing narration surface is exactly the regression this guard "+
			"exists to catch.",
			len(stale), worldDir, strings.Join(stale, "\n  "))
	}
}

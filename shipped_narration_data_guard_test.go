package main

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/gossip"
	"github.com/GoMudEngine/GoMud/internal/grapplemessaging"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/tips"
	weathercontent "github.com/GoMudEngine/GoMud/modules/weather/content"
	yamlv2 "gopkg.in/yaml.v2"
	yamlv3 "gopkg.in/yaml.v3"
)

// shippedWorldRoot is the world the server actually serves.
//
// It is spelled out rather than read from the config on purpose: a test binary
// does not read config.yaml, so configs.GetFilePathsConfig().DataFiles returns
// the Go default `_datafiles/world/default` (internal/configs/config.filepaths.go:23),
// a different and largely vestigial world. A guard pointed at that world would
// pass while every shipped store was broken.
//
// The relative path resolves because `go test` runs a package's binary with
// that package's directory as the working directory, and this package is the
// repo root. Nothing here may be moved into a subdirectory without re-rooting
// these paths.
const shippedWorldRoot = "_datafiles/world/dogmud"

// positionControlPath is the position_control store. M4b-1 moved it under the
// world tree from `_datafiles/messages/`, so production now resolves it as
// <configured world>/messaging/position_control.yaml via
// hooks.positionMessagesPath rather than from a hardcoded literal.
const positionControlPath = shippedWorldRoot + "/messaging/position_control.yaml"

// grappleOutcomesPath is what grapplemessaging.DataFilesPath resolves to under
// the shipped config.
const grappleOutcomesPath = shippedWorldRoot + "/messaging/grapple_outcomes.yaml"

// TestShippedNarrationDataValidates loads every narration store from the
// shipped data path and fails the BUILD when one does not validate.
//
// Why this exists: three stores used to log and continue rather than panicking
// (taunt, grapple, position_control), so bad data in them reached players as
// silence with only a log line. M4b-1's two-tier policy moved all three into
// the event tier, where they now fail the boot, but that only helps someone who
// boots the server; this catches the same data at BUILD time, which is where
// the mistake is actually made. It is the pattern weather already uses
// (modules/weather/content/biome_coupling_test.go), and weather stays in the
// ambient tier where this guard is the ONLY net.
//
// It loads from _datafiles/world/dogmud explicitly rather than through the
// config, because a test binary does not read config.yaml: it would get the
// Go default (_datafiles/world/default), which is a different, vestigial
// world (internal/configs/config.filepaths.go:23).
//
// A subtest over a store whose loader already panics still earns its place: it
// names the store and the record in the failure instead of a boot stack trace.
//
// This guard lands BEFORE M4b's role-key renames, deliberately. A store whose
// Go struct no longer declares the tag its shipped file uses unmarshals to the
// zero value, which for the three logging stores means silence in play and
// nothing at all in a green test run.
func TestShippedNarrationDataValidates(t *testing.T) {
	t.Run("taunt", func(t *testing.T) {
		// combat.LoadTauntMessageFiles panics at boot as of M4b-1; this names
		// the offending record instead of handing an operator a stack trace.
		checkFlatStore[string, *combat.TauntMessageGroup](t, "taunt-messages", shippedWorldRoot+"/taunt-messages")
	})

	t.Run("grapple_outcomes", func(t *testing.T) {
		// hooks.LoadGrappleMessaging panics at boot as of M4b-1. It used to log
		// the load error and substitute an EMPTY library, so every grapple line
		// degraded to a debug string with nothing but one log line to say why.
		lib, err := grapplemessaging.Load(grappleOutcomesPath)
		if err != nil {
			t.Fatalf("grapple_outcomes: %v", err)
		}
		total := len(lib.Advancements) + len(lib.Degradations) + len(lib.Reversals) +
			len(lib.Escapes) + len(lib.Holds) + len(lib.StrikingApex) + len(lib.Gradients)
		if total == 0 {
			t.Fatal("grapple_outcomes loaded zero keys: the store is shipped, so zero means the load failed silently")
		}
		// Production calls this too, at boot, and as of M4b-1 panics on it
		// rather than mudlog.Warn'ing each violation. Here it fails the build,
		// which is earlier and names every violation rather than the first.
		for _, e := range grapplemessaging.ValidateCompleteness(lib) {
			t.Errorf("grapple_outcomes: %v", e)
		}
	})

	t.Run("position_control", func(t *testing.T) {
		checkPositionControl(t)
	})

	t.Run("defence", func(t *testing.T) {
		// Keyed by items.DefencePool, exactly as items.LoadDataFiles keys it.
		// Instantiating the same generic with the same key type is what keeps
		// the guard from reading a normalised variant of production's index.
		checkFlatStore[items.DefencePool, *items.DefenseMessageGroup](t, "defense-messages", shippedWorldRoot+"/defense-messages")
	})

	t.Run("combat_messages", func(t *testing.T) {
		checkFlatStore[items.ItemSubType, *items.WeaponAttackMessageGroup](t, "combat-messages", shippedWorldRoot+"/combat-messages")
	})

	// Sentient item speech lives in item trees since item behaviour slice 2
	// (behaviors/items/<tree>.yaml speech:). ValidateItemBehaviors checks
	// every pool at boot, and TestEveryShippedItemBehaviorResolves runs it
	// over the shipped world.

	t.Run("casting", func(t *testing.T) {
		path := shippedWorldRoot + "/casting-messages.yaml"
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("casting-messages: %v", err)
		}
		// yaml.v2, matching internal/spells/casting_messages.go.
		var cm spells.CastingMessages
		if err := yamlv2.Unmarshal(data, &cm); err != nil {
			t.Fatalf("casting-messages: parse %s: %v", path, err)
		}
		if err := cm.Validate(); err != nil {
			t.Errorf("casting-messages: %v", err)
		}
	})

	t.Run("conditions", func(t *testing.T) {
		// THE ONE STORE WHOSE VALIDATION READS A BALANCE KNOB.
		//
		// ConditionSpec.Validate special-cases conditionId 0 (Meditating, the
		// logout condition): it OVERWRITES the authored triggercount with
		// Network.LogoutRounds and then refuses a count below 1
		// (internal/conditions/conditionspec.go:330). A test binary does not
		// read config.yaml, so that knob comes back as the Go default 0 and
		// the whole store fails to load, on data the server boots on happily.
		//
		// So this subtest reads the real config, which is also the honest
		// thing to guard: a config.yaml shipping LogoutRounds 0 or dropping
		// the key panics the boot, and nothing else would catch it.
		// SetConfigForTest snapshots first and self-registers the restore, so
		// the mutation does not leak into the rest of this test binary; the
		// pattern is internal/characters/poolmax_test.go's withRepoRoot.
		//
		// The store path stays the literal above, NOT the reloaded
		// FilePaths.DataFiles, so the guard keeps reading the shipped world
		// even if a local config.yaml points somewhere else.
		//
		// ReloadConfig logs, and a test binary has no logger until something
		// installs one, so slog nil-dereferences. "LOW" maps to Warn, which
		// keeps the reload quiet. boot_smoke_test.go does the same.
		mudlog.SetupLogger(nil, `LOW`, ``, false)

		configs.SetConfigForTest(t, configs.GetConfig())
		if err := configs.ReloadConfig(); err != nil {
			t.Fatalf("conditions: reload config: %v", err)
		}

		loaded := checkFlatStore[int, *conditions.ConditionSpec](t, "conditions", shippedWorldRoot+"/conditions")
		// conditions.LoadDataFiles panics through ValidateLoadedFlags on an
		// unknown flag. That walks the package's own globals, which this test
		// never populates, so the per-spec method is called directly.
		for id, spec := range loaded {
			if err := spec.ValidateFlags(); err != nil {
				t.Errorf("condition %d: %v", id, err)
			}
		}
	})

	t.Run("spells", func(t *testing.T) {
		checkFlatStore[string, *spells.SpellData](t, "spells", shippedWorldRoot+"/spells")
	})

	t.Run("quests", func(t *testing.T) {
		checkFlatStore[int, *quests.Quest](t, "quests", shippedWorldRoot+"/quests")
	})

	t.Run("crafting", func(t *testing.T) {
		checkFlatStore[string, *crafting.RecipeSpec](t, "crafting", shippedWorldRoot+"/recipes")
	})

	t.Run("gossip", func(t *testing.T) {
		path := shippedWorldRoot + "/gossip_templates.yaml"
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("gossip_templates: %v", err)
		}
		loaded := map[string][]string{}
		if err := yamlv2.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("gossip_templates: parse %s: %v", path, err)
		}
		if len(loaded) == 0 {
			t.Fatal("gossip_templates loaded zero keys: the store is shipped, so zero means the load failed silently")
		}
		if err := gossip.Validate(loaded); err != nil {
			t.Errorf("gossip_templates: %v", err)
		}
	})

	t.Run("tips", func(t *testing.T) {
		path := shippedWorldRoot + "/tips.yaml"
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("tips: %v", err)
		}
		var file struct {
			Tips []string `yaml:"tips"`
		}
		if err := yamlv2.Unmarshal(data, &file); err != nil {
			t.Fatalf("tips: parse %s: %v", path, err)
		}
		if len(file.Tips) == 0 {
			t.Fatal("tips loaded zero lines: the store is shipped, so zero means the load failed silently")
		}
		if err := tips.Validate(file.Tips); err != nil {
			t.Errorf("tips: %v", err)
		}
	})

	t.Run("weather_emotes", func(t *testing.T) {
		checkWeatherEmotes(t)
	})

	t.Run("special-moves", func(t *testing.T) {
		dir := filepath.Join(shippedWorldRoot, "narration", "special-moves")
		groups, err := fileloader.LoadAllFlatFiles[string, *movenarration.MoveNarrationGroup](dir)
		if err != nil {
			t.Fatalf("loading %s: %v", dir, err)
		}
		if len(groups) == 0 {
			t.Fatalf("no special-move files loaded from %s", dir)
		}
		for id, g := range groups {
			if err := g.Validate(); err != nil {
				t.Errorf("move %q: %v", id, err)
			}
		}
	})
}

// narrationStoreWalkRoots is exactly what TestNoLegacyRoleKeysInShippedData
// walks: fourteen paths covering the fifteen shipped narration stores
// (messaging/ holds two of them). It is a list of stores rather than a walk of
// the world root, and both exclusions that buys are load-bearing.
//
// _datafiles/world/default is OUT OF BOUNDS. The owner ruled on 2026-09-17
// that the vestigial default world is left alone, so it still carries the old
// spellings on purpose: 39 conditions, 2 quests and 8 combat-messages files
// were deliberately not rewritten by M4b-1. That world cannot boot anyway (it
// has no defense-messages directory, so the item loader panics first) and no
// test loads a narration store from it, because every loader call in a test
// points FilePaths.DataFiles at the dogmud world or a temp dir first. A walk
// that reached it would fail on data nobody serves.
//
// Everything under the dogmud world that is not a narration store is out of
// scope too. Other stores own some of these spellings legitimately:
// internal/behaviortree reads `room_text` and `user_text` action params out of
// behaviors/ (internal/behaviortree/actions_dialogue.go:40), and
// internal/items reads `on_use_room_text`. Those are different stores in a
// different arc. Naming the narration stores keeps them out by construction,
// which cannot rot the way a path exemption can. behaviors/items is the one
// part of behaviors/ that is a narration store: since item behaviour slice 2
// it holds the sentient item speech pools itemvoices/ used to.
var narrationStoreWalkRoots = []string{
	shippedWorldRoot + "/taunt-messages",
	shippedWorldRoot + "/combat-messages",
	shippedWorldRoot + "/defense-messages",
	shippedWorldRoot + "/behaviors/items", // sentient item speech (item behaviour slice 2)
	shippedWorldRoot + "/conditions",
	shippedWorldRoot + "/spells",
	shippedWorldRoot + "/quests",
	shippedWorldRoot + "/recipes",
	shippedWorldRoot + "/messaging",
	shippedWorldRoot + "/weather/emotes",
	shippedWorldRoot + "/casting-messages.yaml",
	shippedWorldRoot + "/gossip_templates.yaml",
	shippedWorldRoot + "/tips.yaml",
	shippedWorldRoot + "/narration/special-moves",
}

// legacyRoleKeysAnyStore are retired spellings that no narration store may use
// anywhere, mapped to what replaced them.
//
// The table mirrors tools/messaging_token_rewrite.py's KEY_GROUPS, which is
// what actually performed the renames, so the guard bans exactly the
// vocabulary the slice retired and nothing it invented. `controlled` is here
// with one documented exemption; see legacyKeyIsExempt.
var legacyRoleKeysAnyStore = map[string]string{
	// combat, defence, taunt (commit e2e6795e4)
	"toattacker":     "actor",
	"todefender":     "actee",
	"toroom":         "observer",
	"toattackerroom": "observer",
	"todefenderroom": "remote_observer",

	// grapple and position_control sides (620188c7f, 93fbc3ccc)
	"controller": "actor",
	"controlled": "actee",
	"observers":  "observer",
	"partner":    "actee",

	// conditions (acd556e82)
	"start_user_text":   "start_actee",
	"start_room_text":   "start_observer",
	"trigger_user_text": "trigger_actee",
	"trigger_room_text": "trigger_observer",
	"end_user_text":     "end_actee",
	"end_room_text":     "end_observer",

	// spells (acd556e82)
	"cast_user_text":  "cast_actor",
	"cast_room_text":  "cast_observer",
	"wait_user_text":  "wait_actor",
	"wait_room_text":  "wait_observer",
	"magic_user_text": "magic_actor",
	"magic_room_text": "magic_observer",

	// quests (acd556e82)
	"playermessage": "actor",
	"roommessage":   "observer",
	"send_text":     "actor",
	// `room_text` IS banned here, and the plan's warning that it might not be
	// safe to ban was checked rather than assumed. It was a real quest trigger
	// action key and acd556e82 renamed it to `observer`; no shipped quest in
	// either world authors it any more (grep over dogmud/quests and
	// default/quests: zero hits). Its one surviving reader,
	// internal/behaviortree, reads it out of behaviors/, which this walk does
	// not cover. So within these roots the spelling is retired, full stop.
	"room_text": "observer",

	// crafting recipes (acd556e82)
	"success_message":      "success_actor",
	"success_room_message": "success_observer",
	"failure_message":      "failure_actor",
	"failure_room_message": "failure_observer",
}

// legacyRoleKeysMessagingOnly are spellings that are retired inside
// messaging/ but are ordinary, live keys elsewhere, so banning them worldwide
// would be a guard that fails on correct data.
//
// `room` is the proof: internal/quests/triggers.go:13 declares TriggerDef.Room
// with the yaml tag "room" as a trigger's room filter, and 102 shipped quest
// lines author it. A blanket ban would redden every one of them.
// `attacker`, `target` and `self` are the same shape of word: they were
// position_control and grapple_outcomes role keys (the `position` and
// `grapple` groups in the rewrite tool) and nothing else in these stores uses
// them, but they are plausible future keys for a store that never had the old
// vocabulary, so the ban stays where the rename happened.
var legacyRoleKeysMessagingOnly = map[string]string{
	"self":     "actor",
	"attacker": "actor",
	"target":   "actee",
	"room":     "observer",
}

// legacyKeyIsExempt carves out the one place a banned spelling is correct
// authored data.
//
// `controlled` is BOTH a retired role key and a live gradient STATE name. In
// position_control.yaml the sides used to be spelled controller/controlled and
// are now actor/actee, but the gradient states are in_control,
// losing_control, neutral, becoming_controlled and controlled, and the state
// keeps its spelling because renaming it would turn
// gradient_messages.actor.actee into nonsense. So after Task 7,
// `gradient_messages.actor.controlled.actor` is legal and correct, and a guard
// that banned the word outright would fail on shipped data.
//
// The discriminator is the same one the rewrite tool used: nesting depth. The
// sides are direct children of gradient_messages and the states are one level
// below them. This scopes by ancestor PATH rather than by column, which is the
// stricter form of the same rule: `controlled` is exempt only as a grandchild
// of gradient_messages in that one file. A `controlled:` reintroduced as a
// side, as an audience key inside a state, or anywhere in any other file, is
// still caught.
func legacyKeyIsExempt(path, key string, ancestors []string) bool {
	return key == "controlled" &&
		filepath.ToSlash(path) == positionControlPath &&
		len(ancestors) == 2 &&
		ancestors[0] == "gradient_messages"
}

// TestNoLegacyRoleKeysInShippedData fails the build when a narration YAML file
// still spells a role the old way. The renames of M4b-1 are only durable if a
// newly authored file cannot reintroduce the old vocabulary, and a store whose
// struct no longer declares the tag would load that file SILENTLY EMPTY, which
// is the exact failure this slice exists to make impossible.
//
// It walks narrationStoreWalkRoots, which is the shipped dogmud world's
// narration stores only; the scoping and the reasons for it are documented on
// that variable, on the two ban tables, and on legacyKeyIsExempt.
//
// It inspects MAPPING KEYS from a parsed yaml.v3 node tree, not lines of text.
// Half these spellings are ordinary English words, so a line-oriented scan
// would have to guess whether `room:` inside a block scalar is a key or prose.
// The node tree does not guess, and it hands over a real ancestor path, which
// is what the gradient-state exemption is keyed on.
//
// It logs how many files and how many keys it inspected, and refuses to pass
// on a walk that found nothing. An absence guard whose walk silently scans
// zero files passes in 0.00s and proves nothing; this repo has been bitten by
// exactly that.
func TestNoLegacyRoleKeysInShippedData(t *testing.T) {
	filesInspected := 0
	keysInspected := 0

	for _, root := range narrationStoreWalkRoots {
		files := yamlFilesUnder(t, root)
		if len(files) == 0 {
			t.Errorf("walk root %s yielded zero YAML files: the guard would scan nothing there", root)
			continue
		}
		for _, path := range files {
			filesInspected++
			keysInspected += checkFileForLegacyRoleKeys(t, path)
		}
	}

	if keysInspected == 0 {
		t.Fatal("inspected zero mapping keys: the walk found nothing, so a green run proves nothing")
	}
	// A floor, not a pin. Content volume moves; a walk collapsing to a handful
	// of files does not happen for a legitimate reason.
	if filesInspected < 100 {
		t.Errorf("inspected only %d files across %d walk roots: expected the whole narration tree", filesInspected, len(narrationStoreWalkRoots))
	}
	t.Logf("inspected %d YAML files and %d mapping keys across %d narration store roots",
		filesInspected, keysInspected, len(narrationStoreWalkRoots))
}

// knownConditionObserverPhaseKeys are the only mapping keys ending in
// "observer" the conditions store may author. inObserverRole matches any key
// ending in that suffix, which is deliberately wide (conditions keys its
// three phases start_observer/trigger_observer/end_observer, not a bare
// "observer"), but the width cuts both ways: a fourth phase key, say
// "foo_observer", would also satisfy the suffix match and would be silently
// waved through by observerIdentityGuardContentSafeViaCode's file-level
// exemption for conditions/*.yaml, even though nothing has proven that a
// sender narrates it with a hidden name. Each key here is safe only because a
// specific, audited sender is proven to pass the holder's plain name into
// HideNames for it:
var knownConditionObserverPhaseKeys = map[string]bool{
	// Narrated by Condition_ApplyConditions.go, which passes the holder's
	// plain name into HideNames when a condition is first applied.
	"start_observer": true,
	// Narrated by NewRound_UserRoundTick.go and NewRound_MobRoundTick.go,
	// both of which pass the holder's plain name into HideNames on every
	// round a condition fires.
	"trigger_observer": true,
	// Narrated by NewTurn_PruneConditions.go's sendConditionEndRoomText,
	// which passes the holder's plain name into HideNames when a condition
	// expires.
	"end_observer": true,
}

// TestConditionObserverPhaseKeysAreKnown fails the build when the conditions
// store authors a mapping key ending in "observer" outside the three phase
// keys named in knownConditionObserverPhaseKeys.
//
// Why this exists: inObserverRole's suffix match and the conditions/*.yaml
// entries in observerIdentityGuardContentSafeViaCode are keyed to a STORE, not
// to the three specific phase keys that store happens to ship today. Someone
// authoring a fourth phase key (say foo_observer) would have it accepted as
// an observer role by the suffix match, and TestObserverIdentityTagsAreAnonymizable
// would wave it through silently because the whole conditions/*.yaml file is
// exempted by path. That leaves the new key either dead content or, if a
// sender is later wired to narrate it without passing names through
// HideNames, a live name leak that no guard would catch. This test pins the
// vocabulary so a new phase key has to earn its way into the known set rather
// than riding in on the file-level exemption.
func TestConditionObserverPhaseKeysAreKnown(t *testing.T) {
	dir := shippedWorldRoot + "/conditions"
	files := yamlFilesUnder(t, dir)
	if len(files) == 0 {
		t.Fatalf("walk root %s yielded zero YAML files: the guard would scan nothing there", dir)
	}

	filesInspected := 0
	keysFound := 0
	for _, path := range files {
		filesInspected++
		keysFound += checkFileForUnknownObserverPhaseKeys(t, path)
	}

	if keysFound == 0 {
		t.Fatal("inspected zero observer-suffixed keys: the walk found nothing, so a green run proves nothing")
	}
	t.Logf("inspected %d YAML files and %d observer-suffixed keys under %s", filesInspected, keysFound, dir)
}

// checkFileForUnknownObserverPhaseKeys reports every mapping key ending in
// "observer" in one conditions file that is not in
// knownConditionObserverPhaseKeys, and returns how many observer-suffixed
// keys it looked at (known or not), so the caller can prove the walk is not
// silently inspecting nothing.
func checkFileForUnknownObserverPhaseKeys(t *testing.T, path string) int {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: %v", path, err)
		return 0
	}

	found := 0
	dec := yamlv3.NewDecoder(bytes.NewReader(data))
	for {
		var doc yamlv3.Node
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			t.Errorf("%s: parse: %v", path, err)
			return found
		}
		found += walkNodeForUnknownObserverPhaseKeys(t, path, &doc)
	}
	return found
}

func walkNodeForUnknownObserverPhaseKeys(t *testing.T, path string, n *yamlv3.Node) int {
	t.Helper()

	found := 0
	switch n.Kind {
	case yamlv3.DocumentNode, yamlv3.SequenceNode:
		for _, child := range n.Content {
			found += walkNodeForUnknownObserverPhaseKeys(t, path, child)
		}
	case yamlv3.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if strings.HasSuffix(k.Value, "observer") {
				found++
				if !knownConditionObserverPhaseKeys[k.Value] {
					t.Errorf("%s:%d: unknown observer phase key %q: inObserverRole's suffix match accepts it and observerIdentityGuardContentSafeViaCode's file-level exemption for conditions/*.yaml would wave it through silently, but no audited sender is proven to narrate it with a hidden name. Add an audited sender that passes the holder's plain name into HideNames for this phase, then add the key to knownConditionObserverPhaseKeys.",
						filepath.ToSlash(path), k.Line, k.Value)
				}
			}
			found += walkNodeForUnknownObserverPhaseKeys(t, path, v)
		}
	}
	return found
}

// yamlFilesUnder returns every .yaml/.yml file at or under root. root may name
// a single file, which three of the stores are.
func yamlFilesUnder(t *testing.T, root string) []string {
	t.Helper()

	info, err := os.Stat(root)
	if err != nil {
		t.Errorf("walk root %s: %v", root, err)
		return nil
	}
	if !info.IsDir() {
		return []string{root}
	}

	var out []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if ext := filepath.Ext(path); ext == ".yaml" || ext == ".yml" {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Errorf("walk root %s: %v", root, err)
	}
	return out
}

// checkFileForLegacyRoleKeys reports every banned mapping key in one file and
// returns how many keys it looked at, so the caller can prove the walk is not
// silently inspecting nothing.
func checkFileForLegacyRoleKeys(t *testing.T, path string) int {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: %v", path, err)
		return 0
	}

	// Multi-document files are not used by these stores today, but decoding in
	// a loop costs nothing and means a second document could not hide a key.
	keys := 0
	dec := yamlv3.NewDecoder(bytes.NewReader(data))
	for {
		var doc yamlv3.Node
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			// A parse failure is TestShippedNarrationDataValidates' business,
			// but reporting it here too beats scanning zero keys quietly.
			t.Errorf("%s: parse: %v", path, err)
			return keys
		}
		keys += walkNodeForLegacyRoleKeys(t, path, &doc, nil)
	}
	return keys
}

func walkNodeForLegacyRoleKeys(t *testing.T, path string, n *yamlv3.Node, ancestors []string) int {
	t.Helper()

	keys := 0
	switch n.Kind {
	case yamlv3.DocumentNode, yamlv3.SequenceNode:
		for _, child := range n.Content {
			keys += walkNodeForLegacyRoleKeys(t, path, child, ancestors)
		}
	case yamlv3.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			keys++
			reportLegacyRoleKey(t, path, k, ancestors)
			keys += walkNodeForLegacyRoleKeys(t, path, v, append(ancestors, k.Value))
		}
	}
	return keys
}

// isRoleKeyedNarrationStore reports whether path belongs to a store whose YAML
// keys ARE narration roles, so the M4b-1 role-key ban applies to it.
//
// The ban cannot be repo-wide: `room` is a quest TRIGGER FILTER
// (internal/quests/triggers.go), and `target`/`self` are legitimate words in
// other stores, so a blanket ban would redden about a hundred shipped quest
// lines. It is scoped by store instead. messaging/ was the only such store
// until M4e added narration/, and a store added here without being added to
// this list would simply not be checked, which is how the special-move store
// shipped outside the ban until this was noticed.
func isRoleKeyedNarrationStore(path string) bool {
	slash := filepath.ToSlash(path)
	for _, root := range []string{
		shippedWorldRoot + "/messaging/",
		shippedWorldRoot + "/narration/",
	} {
		if strings.HasPrefix(slash, root) {
			return true
		}
	}
	return false
}

func reportLegacyRoleKey(t *testing.T, path string, key *yamlv3.Node, ancestors []string) {
	t.Helper()

	replacement, banned := legacyRoleKeysAnyStore[key.Value]
	if !banned && isRoleKeyedNarrationStore(path) {
		replacement, banned = legacyRoleKeysMessagingOnly[key.Value]
	}
	if !banned || legacyKeyIsExempt(path, key.Value, ancestors) {
		return
	}

	where := "(top level)"
	if len(ancestors) > 0 {
		where = strings.Join(ancestors, ".")
	}
	t.Errorf("%s:%d: legacy role key %q under %s: M4b-1 renamed it to %q, and the store's Go struct no longer declares the old tag, so this file would load SILENTLY EMPTY",
		filepath.ToSlash(path), key.Line, key.Value, where, replacement)
}

// checkFlatStore loads one fileloader-backed store through the SAME generic
// instantiation production uses, then asserts the store is non-empty and every
// record validates.
//
// fileloader.LoadAllFlatFiles already calls Validate on each record and refuses
// duplicate ids, so a nil error is most of the contract. Validate is called
// again per record anyway, because the loader's error names the FILE and this
// names the ID, and the id is what a role-key rename breaks.
func checkFlatStore[K comparable, T fileloader.Loadable[K]](t *testing.T, store, dir string) map[K]T {
	t.Helper()

	loaded, err := fileloader.LoadAllFlatFiles[K, T](dir)
	if err != nil {
		t.Fatalf("%s: load %s: %v", store, dir, err)
	}
	if len(loaded) == 0 {
		t.Fatalf("%s loaded zero records from %s: the store is shipped, so zero means the load failed silently", store, dir)
	}
	for id, rec := range loaded {
		if err := rec.Validate(); err != nil {
			t.Errorf("%s record %v: %v", store, id, err)
		}
	}
	return loaded
}

// posGuardTriple mirrors hooks.submissionMsgTriple.
//
// The production type is unexported and the package exposes no seam onto it,
// so the shape is mirrored here rather than exporting API for a test. The same
// trade is already made in internal/narration/snapshot_test.go, which mirrors
// this file for the golden.
//
// The mirror is decoded STRICTLY: if the shipped file renames a role key and
// this mirror is not renamed with it, the unknown key fails the decode rather
// than yielding a silently empty triple. That is the direction M4b renames
// travel, and it is the failure this whole test exists to catch.
type posGuardTriple struct {
	Attacker string `yaml:"actor"`
	Target   string `yaml:"actee"`
	Room     string `yaml:"observer"`
}

// posGuardFile mirrors the whole shipped file so it can be decoded strictly.
//
// hooks.positionMessageTemplates parses only stamina_warning and submission.
// gradient_messages and transition_messages are authored in the same file but
// read by NOBODY (the live gradient and transition prose comes from
// internal/grapplemessaging), so requiring text in them would guard data
// production does not use, and the narration snapshot golden covers them. They
// are declared as yaml.Node so a strict decode does not trip over them while
// still refusing an unknown key inside the two blocks that matter.
type posGuardFile struct {
	Gradient   yamlv3.Node `yaml:"gradient_messages"`
	Transition yamlv3.Node `yaml:"transition_messages"`
	Stamina    struct {
		Self string `yaml:"actor"`
		Room string `yaml:"observer"`
	} `yaml:"stamina_warning"`
	Submission struct {
		Opening                map[string]posGuardTriple `yaml:"opening"`
		EscapeBad              posGuardTriple            `yaml:"escape_bad"`
		Neutral                posGuardTriple            `yaml:"neutral"`
		OutcomeMercy           posGuardTriple            `yaml:"outcome_mercy"`
		OutcomeSubdue          posGuardTriple            `yaml:"outcome_subdue"`
		OutcomeCrippleArm      posGuardTriple            `yaml:"outcome_cripple_arm"`
		OutcomeCrippleShoulder posGuardTriple            `yaml:"outcome_cripple_shoulder"`
		OutcomeLethal          posGuardTriple            `yaml:"outcome_lethal"`
		CritFlag               posGuardTriple            `yaml:"crit_flag"`
	} `yaml:"submission"`
}

func checkPositionControl(t *testing.T) {
	t.Helper()

	data, err := os.ReadFile(positionControlPath)
	if err != nil {
		t.Fatalf("position_control: %v", err)
	}
	// yaml.v3, matching internal/hooks/Position_Messaging.go, but with
	// KnownFields on. Production decodes leniently, so a role key renamed on
	// disk and not in Go silently yields an empty string; here it is an error
	// that names the offending key.
	var f posGuardFile
	dec := yamlv3.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		t.Fatalf("position_control: parse %s: %v", positionControlPath, err)
	}

	if f.Stamina.Self == "" || f.Stamina.Room == "" {
		t.Errorf("position_control stamina_warning: actor=%q observer=%q, both must carry text", f.Stamina.Self, f.Stamina.Room)
	}
	if len(f.Submission.Opening) == 0 {
		t.Fatal("position_control submission.opening loaded zero keys: the store is shipped, so zero means the load failed silently")
	}

	full := []struct {
		name string
		tri  posGuardTriple
	}{
		{"escape_bad", f.Submission.EscapeBad},
		{"neutral", f.Submission.Neutral},
		{"outcome_mercy", f.Submission.OutcomeMercy},
		{"outcome_subdue", f.Submission.OutcomeSubdue},
		{"outcome_cripple_arm", f.Submission.OutcomeCrippleArm},
		{"outcome_cripple_shoulder", f.Submission.OutcomeCrippleShoulder},
		{"outcome_lethal", f.Submission.OutcomeLethal},
	}
	for key, tri := range f.Submission.Opening {
		full = append(full, struct {
			name string
			tri  posGuardTriple
		}{"opening." + key, tri})
	}
	for _, c := range full {
		if c.tri.Attacker == "" || c.tri.Target == "" || c.tri.Room == "" {
			t.Errorf("position_control submission.%s: every role must carry text (actor=%q actee=%q observer=%q)",
				c.name, c.tri.Attacker, c.tri.Target, c.tri.Room)
		}
	}

	// crit_flag is the one deliberate exception. It is a PREFIX fragment
	// glued onto the actor's line, so its actee and observer are authored
	// empty on purpose. Requiring all three here would be a guard that fails
	// on correct data. The exemption is by KEY, not by tag, so M4b-1's rename
	// left it working: crit_flag is still absent from the `full` list above.
	if f.Submission.CritFlag.Attacker == "" {
		t.Error("position_control submission.crit_flag: actor must carry text")
	}
}

// checkWeatherEmotes loads the ambient store the way modules/weather does:
// os.DirFS over the world root, then the two exported content loaders.
//
// Weather never panics by documented intent (an emote file that fails to parse
// costs silence, not a boot), so a build-time check is the ONLY thing standing
// behind it. LoadEmotes swallows a missing directory and returns empty tables
// with a nil error, which is exactly the silent-empty failure mode this test
// exists for, hence the length assertions.
//
// DELIBERATELY THIN, and here is what it does not cover. A section dropped
// from one table (say `outdoor:` renamed) parses to an empty map with no
// error, and this subtest would still see a non-empty Tables and pass. The
// rule that catches that lives in the module, in
// modules/weather/content/shipped_emotes_test.go, which pins the table count
// at 9 and requires a non-empty outdoor default per table; per-pool depth and
// the biome coupling are in biome_coupling_test.go. Those are not duplicated
// here. What this subtest adds is that the shipped tree still loads non-empty
// at all, that a pool below the depth floor fails, and that weather is named
// in the failure alongside the other thirteen stores.
func checkWeatherEmotes(t *testing.T) {
	t.Helper()

	worldFS := os.DirFS(shippedWorldRoot)

	tables, err := weathercontent.LoadEmotes(worldFS, "weather/emotes")
	if err != nil {
		t.Fatalf("weather emotes: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("weather emotes loaded zero tables: the store is shipped, so zero means the load failed silently")
	}

	seasonal, err := weathercontent.LoadSeasonalEmotes(worldFS, "weather/emotes/seasons")
	if err != nil {
		t.Fatalf("weather seasonal emotes: %v", err)
	}
	if len(seasonal) == 0 {
		t.Fatal("weather seasonal emotes loaded zero tables: the store is shipped, so zero means the load failed silently")
	}
}

// observerIdentityGuardRoots is what TestObserverIdentityTagsAreAnonymizable
// walks: the two content trees Task 4c's bug lives in (see that task's design
// note and internal/combat/context.md). combat-messages relies ENTIRELY on
// its own `<ansi fg="{actortype}">{actor}</ansi>` markup to give
// messaging.Anonymize something to strip: buildAttackMessages
// (internal/combat/combat_helpers.go) substitutes a bare player name into
// {actor} with no code-level tagging of its own (only a Mob actor gets
// GetMobName(0) override), so a malformed or missing tag in the CONTENT is
// the only thing standing between a dark room and a leaked name. That is
// exactly the bug Item 2 fixed (12 unclosed <ansi fg="{actortype}"> tags in
// bite/claws/slam.yaml) and this guard is what stops a thirteenth from
// shipping unnoticed. defense-messages is walked too: a NEW pool authored
// without Item 1's Go-layer self-tagging (or a hand-edit that strips it) can
// only be caught here -- see observerIdentityGuardContentSafeViaCode for why
// its TEN existing files do not fail this guard despite most of them still
// authoring the unregistered "mob"/"user" alias or no tag at all.
var observerIdentityGuardRoots = []string{
	shippedWorldRoot + "/combat-messages",
	shippedWorldRoot + "/defense-messages",
	shippedWorldRoot + "/conditions",
	shippedWorldRoot + "/narration/special-moves",
}

// observerIdentityGuardContentSafeViaCode names the files across all four
// walk roots whose {actor}/{actee}/{actor_plain}/{actee_plain} placeholders
// this guard would otherwise flag, but does not fail on, because every one of
// them is proven to render through a Go function that already substitutes an
// identity string this content-only walk cannot see.
//
// THE KEY IS THE PATH RELATIVE TO shippedWorldRoot ("conditions/9-hidden.yaml",
// "narration/special-moves/drain.yaml", "defense-messages/dodge.yaml"), never a
// bare basename. combat-messages and narration/special-moves both ship a
// drain.yaml, gore.yaml, maul.yaml, pounce.yaml and throttle.yaml; a
// basename-only key would let marking the special-moves twin safe silently
// also exempt the combat-messages twin, which earns its safety a completely
// different way (its own ansi tagging, not SendTrio) and was never meant to
// be exempted at all. That collision shipped for one commit (M5 PR 1 Task 3)
// before being caught and re-keyed here: five combat-messages files were
// briefly merely logged instead of enforced. Every lookup builds this same
// store-relative, slash-normalised path from the file it is checking, so the
// two stores can never collide again regardless of what they name their
// files.
//
// Three separate runtime guarantees are recorded here, one per group below.
// A file NOT in this map is enforced like any other; a new entry earns its
// place only with the same kind of proof.
//
// defense-messages/*.yaml (ten files) -> a Go function that already
// substitutes a fully self-tagged identity string
// (`<ansi fg="mobname">Name</ansi>` or `<ansi fg="username">Name</ansi>`,
// built by meleeIdentityTag or the pre-existing taunt/spell defence path).
// messaging.nameTagPattern matches that SUBSTITUTED span wherever it lands in
// the final text, regardless of what -- an unregistered "mob"/"user" alias,
// or nothing at all -- wraps the placeholder in the authored YAML (confirmed
// against internal/combat's darkness identity tests and a standalone regex
// check; see Task 4c's Item 1 commit). A purely static, content-only guard
// cannot see that runtime guarantee, so recognising "mob"/"user" as
// anonymizableAliases was rejected (it would blind this guard to a genuinely
// bad alias in combat-messages, which has no such runtime fallback); naming
// the files here instead keeps the alias check strict everywhere it still
// matters:
//
//	dodge.yaml, parry.yaml, block.yaml    -> combat.sendDefenseMessages (Item 1)
//	counter-dodge/parry/block/quell.yaml  -> combat.fillCounterMessages (Item 1)
//	counter-defy.yaml                     -> combat.BuildCounterTauntMessages
//	                                          (fixed alongside Item 1: this
//	                                          guard caught it substituting a
//	                                          raw name exactly like its two
//	                                          fixed siblings in counter.go)
//	quell.yaml, defy.yaml                 -> combat.RenderChannelDefenceMessages,
//	                                          called from
//	                                          internal/hooks/spell_resolution.go
//	                                          and mobcommands/usercommands
//	                                          taunt.go -- already safe before
//	                                          Task 4c, per its own bug report
//
// A SECOND safety route covers conditions and special-moves, widened into
// this guard's walk in M5 PR 1 Task 3. Both rest on the same idea as
// defense-messages above (a Go layer that substitutes an identity string
// this content-only walk cannot see), but neither one tags that string with
// an ansi alias at all, so nameTagPattern never matches it; the runtime
// guarantee instead comes from messaging.HideNames scanning the RENDERED
// text for the substituted plain name and redacting it per reader. That
// holds regardless of which token the YAML happened to author -- {actee},
// {actee_plain}, {actor} or {actor_plain} -- because HideNames matches the
// substituted name as a substring wherever it lands, tagged or bare
// (internal/messaging/hidenames.go: hideOneName also strips the identity tag
// around a match). This is why the entries below cover every file in each
// store that authors a name-referencing observer line at all, not only the
// files that happen to author a bare `_plain` token -- 48 entries where the
// plan anticipated 16, discovered by investigating the Step 3 red run rather
// than trimming it to match the plan:
//
//	conditions/*.yaml               -> ConditionSpec.Narrate always maps
//	                                    {actee}/{actee_plain} to the holder
//	                                    (ActorName is permanently empty until
//	                                    M6 gives a condition a caster --
//	                                    internal/conditions/narration.go:45-50),
//	                                    and all three phases pass that same
//	                                    holder plain name to HideNames: start
//	                                    Condition_ApplyConditions.go:179,
//	                                    trigger UserRoundTick
//	                                    and MobRoundTick, end
//	                                    sendConditionEndRoomText
//	                                    (NewTurn_PruneConditions.go), which
//	                                    since #220 judges a light's or a
//	                                    darkness's end line against a snapshot
//	                                    of the room before it ran out, shapes
//	                                    tier included, so 1-illumination.yaml
//	                                    relies on HideNames like the rest.
//	                                    inObserverRole was widened from an
//	                                    exact "observer"/"remote_observer"
//	                                    match to a suffix match, because
//	                                    conditions keys its three phases
//	                                    start_observer/trigger_observer/
//	                                    end_observer -- with the exact match
//	                                    the widened root inspected zero
//	                                    condition placeholders, which was not
//	                                    "conditions are clean", it was
//	                                    "conditions were never inspected". See
//	                                    TestObserverIdentityTagsAreAnonymizable's
//	                                    per-root placeholder check, which now
//	                                    catches that regression directly.
//	narration/special-moves/*.yaml  -> messaging.SendTrio hides Audience
//	                                    ActorName and ActeeName from every
//	                                    reader by that reader's
//	                                    ParticipantSight, for every event a
//	                                    special-move file authors, not only
//	                                    the two (grapple.yaml, throw.yaml)
//	                                    that happen to also author a `_plain`
//	                                    token. Confirmed for all fourteen
//	                                    shipped files by grepping every
//	                                    mobcommands/usercommands move handler
//	                                    for its SendTrio call site. throw.yaml
//	                                    additionally relies on M5 PR 1 Task 2,
//	                                    which stopped its interrupt event
//	                                    passing NoName.
//
// This map staying at 48 entries does not mean the guard went vacuous: a
// clean run inspects 153 files and 4358 {actor}/{actee} placeholders across
// the four roots, of which 732 are logged here as exempt and the remaining
// 3626 -- the large majority -- are still fully enforced (see the t.Logf line
// at the end of TestObserverIdentityTagsAreAnonymizable for the current
// numbers).
var observerIdentityGuardContentSafeViaCode = map[string]bool{
	"defense-messages/dodge.yaml":         true,
	"defense-messages/parry.yaml":         true,
	"defense-messages/block.yaml":         true,
	"defense-messages/quell.yaml":         true,
	"defense-messages/defy.yaml":          true,
	"defense-messages/counter-dodge.yaml": true,
	"defense-messages/counter-parry.yaml": true,
	"defense-messages/counter-block.yaml": true,
	"defense-messages/counter-quell.yaml": true,
	"defense-messages/counter-defy.yaml":  true,
	// Conditions: all three narration phases pass the holder's plain name
	// into HideNames (start Condition_ApplyConditions.go:179, trigger
	// UserRoundTick and MobRoundTick, end
	// sendConditionEndRoomText). Applies to the store's every
	// name-referencing observer line, not only the ones that author a
	// `_plain` token; see the doc comment above.
	"conditions/0-meditating.yaml":         true,
	"conditions/1-illumination.yaml":       true, // a light: its end line is judged against the room before it ran out (#220)
	"conditions/2-stunned.yaml":            true,
	"conditions/3-blinded.yaml":            true,
	"conditions/9-hidden.yaml":             true,
	"conditions/29-night_vision.yaml":      true,
	"conditions/31-empathic_shroud.yaml":   true,
	"conditions/38-conviction_armor.yaml":  true,
	"conditions/39-venom.yaml":             true,
	"conditions/40-spore_toxin.yaml":       true,
	"conditions/47-minor_antidote.yaml":    true,
	"conditions/48-clarity_tonic.yaml":     true,
	"conditions/49-fire_resistance.yaml":   true,
	"conditions/50-greater_healing.yaml":   true,
	"conditions/51-berserker_elixir.yaml":  true,
	"conditions/52-chrysalis_shell.yaml":   true,
	"conditions/65-cats_eye_draught.yaml":  true,
	"conditions/78-toxic_cloud.yaml":       true,
	"conditions/94-cold_discharge.yaml":    true,
	"conditions/96-hull_discharge.yaml":    true,
	"conditions/97-arc_trap.yaml":          true,
	"conditions/100-blood_frenzy.yaml":     true,
	"conditions/102-disrupted.yaml":        true,
	"conditions/106-searing_backlash.yaml": true,
	"conditions/107-rimefrost.yaml":        true,
	"conditions/108-static_shock.yaml":     true,
	"conditions/109-reeling.yaml":          true,
	"conditions/110-mired.yaml":            true,
	"conditions/111-ensnared.yaml":         true,
	"conditions/112-paralysed.yaml":        true,
	"conditions/114-cursed.yaml":           true,
	"conditions/115-rending_bleed.yaml":    true,
	"conditions/116-terrified.yaml":        true,
	"conditions/119-conviction_ward.yaml":  true,
	// Messaging M6 slice 1: the other two wards, narrated by the same start
	// (narrateConditionStart, through sendConditionStartRoomText) and end
	// (sendConditionEndRoomText) senders as 119, each with the holder's plain
	// name handed to HideNames.
	"conditions/135-conviction_bulwark.yaml": true,
	"conditions/136-chrysalis_cocoon.yaml":   true,
	// Lighting plan 5c: the vision spells and the tincture, narrated by the
	// same three phases as 29 and 65 above.
	"conditions/128-night_sight.yaml":       true,
	"conditions/129-heat_sight.yaml":        true,
	"conditions/130-pitsense_tincture.yaml": true,
	// Lighting plan 5d: Chrysalis Pall. Its start line goes out through
	// sendConditionStartRoomText, which as a darkness routes it through
	// SendTextVisualToSnapshot with the holder's plain name, judged against
	// the room before the pall landed (ruling D6 as amended by the owner,
	// 2026-10-05); its end line through sendConditionEndRoomText with the
	// holder's plain name.
	"conditions/131-chrysalis_pall.yaml": true,
	// Special moves: messaging.SendTrio hides Audience ActorName and ActeeName
	// from every reader by that reader's ParticipantSight. Applies to all
	// fourteen shipped files, not only the two that author a `_plain` token;
	// see the doc comment above. The store-relative key is what keeps these
	// five from colliding with their combat-messages basename twins.
	"narration/special-moves/bash.yaml":      true,
	"narration/special-moves/charge.yaml":    true,
	"narration/special-moves/drain.yaml":     true,
	"narration/special-moves/gore.yaml":      true,
	"narration/special-moves/grapple.yaml":   true,
	"narration/special-moves/hamstring.yaml": true,
	"narration/special-moves/kick.yaml":      true,
	"narration/special-moves/maul.yaml":      true,
	"narration/special-moves/pounce.yaml":    true,
	"narration/special-moves/rake.yaml":      true,
	"narration/special-moves/shoot.yaml":     true,
	"narration/special-moves/throttle.yaml":  true,
	"narration/special-moves/throw.yaml":     true, // SendTrio hides both Audience names by reader sight; the interrupt event stopped passing NoName in M5 PR 1 Task 2
	"narration/special-moves/trip.yaml":      true,
}

// anonymizableAliases are the ansi aliases messaging.Anonymize's
// nameTagPattern recognises (internal/messaging/anonymize.go: username,
// mobname, petname, each with an optional -suffix this guard does not need to
// reproduce since content never authors the suffixed forms), plus the two
// content placeholders internal/combat resolves to them BEFORE Anonymize ever
// sees the rendered text: {actortype} becomes "username" or "mobname"
// (TokenActorType, set from SourceTarget in combat.go/combat_helpers.go), and
// {acteetype} the same for the other side.
var anonymizableAliases = map[string]bool{
	"username":    true,
	"mobname":     true,
	"petname":     true,
	"{actortype}": true,
	"{acteetype}": true,
}

// identityPlaceholderPattern finds a bare {actor}, {actee}, {actor_plain} or
// {actee_plain} token in an authored line. The two _plain variants are the
// dangerous ones: they are untagged by definition, so messaging.Anonymize,
// which strips identity TAGS only, cannot see them at all. They are safe only
// when the delivery path hands the name to HideNames, which is a runtime
// property this content-only guard cannot verify and which
// observerIdentityGuardContentSafeViaCode therefore records by hand.
//
// It does not match inside {actortype}/{acteetype}: those do not end in `}`
// immediately after "actor"/"actee", so the exact-token anchor is enough.
var identityPlaceholderPattern = regexp.MustCompile(`\{actor\}|\{actee\}|\{actor_plain\}|\{actee_plain\}`)

// identityTagSpanPattern finds one whole `<ansi fg="ALIAS">content</ansi>`
// span and captures both the alias and the content, mirroring
// messaging.nameTagPattern's shape but keeping the alias general so this
// guard can judge it against anonymizableAliases itself, rather than only
// ever matching the three aliases Anonymize already accepts -- the whole
// point is to also catch a span whose alias is something else (e.g. "mob"
// or "user", the unregistered aliases Task 4c's bug report names).
var identityTagSpanPattern = regexp.MustCompile(`<ansi fg="([^"]+)">([^<]*)</ansi>`)

// TestObserverIdentityTagsAreAnonymizable fails the build when an observer or
// remote_observer line in combat-messages/ or defense-messages/ names {actor}
// or {actee} outside a CLOSED ansi tag whose alias messaging.Anonymize
// recognises (counting {actortype}/{acteetype}, which internal/combat
// resolves to a recognised alias before Anonymize runs). An infrared-only
// (SightShapes) observer in a dark room reads whatever text these two stores
// produce; a placeholder that Anonymize cannot find a tag for reaches that
// observer as a real name, exactly as Task 4c's bug report measured (231 of
// 2553 observer lines, before Items 1 and 2 fixed the two failure modes: an
// unregistered content alias substituted with a bare name, and a tag that
// opens but never closes).
//
// It inspects the RENDERED TEXT of each observer/remote_observer scalar, not
// its role key -- unlike TestNoLegacyRoleKeysInShippedData, which checks
// mapping keys, this walks to the scalar leaves and regex-matches the
// authored string itself, because the defect here is inside the string, not
// in what it is filed under.
//
// It logs how many files and how many {actor}/{actee} placeholders it
// inspected, and refuses to pass on a walk that found nothing, for the same
// reason TestNoLegacyRoleKeysInShippedData does: a walk that silently scans
// zero files or zero placeholders would pass in 0.00s and prove nothing.
//
// It also checks that count PER ROOT, not only summed across all four. The
// summed check alone cannot see one root going dark while the others keep
// the total comfortably positive -- which is exactly what happened to
// conditions before inObserverRole was widened to a suffix match: conditions
// keys its three phases start_observer/trigger_observer/end_observer, none
// of which equalled the old exact "observer"/"remote_observer" check, so
// that root silently inspected zero placeholders while combat-messages and
// defense-messages carried the overall count and the test stayed green. A
// future narrowing of inObserverRole back toward exact equality would
// reproduce that silently; the per-root check below catches it directly.
func TestObserverIdentityTagsAreAnonymizable(t *testing.T) {
	filesInspected := 0
	placeholdersInspected := 0
	exemptFindings := 0
	placeholdersByRoot := make(map[string]int, len(observerIdentityGuardRoots))

	for _, root := range observerIdentityGuardRoots {
		files := yamlFilesUnder(t, root)
		if len(files) == 0 {
			t.Errorf("walk root %s yielded zero YAML files: the guard would scan nothing there", root)
			continue
		}
		for _, path := range files {
			filesInspected++
			placeholders, exempt := checkFileForUnanonymizableIdentities(t, path)
			placeholdersInspected += placeholders
			exemptFindings += exempt
			placeholdersByRoot[root] += placeholders
		}
	}

	if placeholdersInspected == 0 {
		t.Fatal("inspected zero {actor}/{actee} placeholders: the walk found nothing, so a green run proves nothing")
	}
	for _, root := range observerIdentityGuardRoots {
		if placeholdersByRoot[root] == 0 {
			t.Errorf("walk root %s inspected zero {actor}/{actee} placeholders: either it has no matching content or inObserverRole stopped recognising its observer-role keys", root)
		}
	}
	// A floor, not a pin. Content volume moves; a walk collapsing to a
	// handful of files does not happen for a legitimate reason. Four roots
	// now: combat-messages, defense-messages, conditions, special-moves. A
	// clean run on 2026-09-22 inspected 153 files; 100 is comfortably below
	// that (leaves room for content to move without a false trip) while
	// still catching a walk that collapses back toward the old two-root,
	// ten-file floor.
	if filesInspected < 100 {
		t.Errorf("inspected only %d files across %d walk roots: expected the whole combat, defense, condition and special-move message tree", filesInspected, len(observerIdentityGuardRoots))
	}
	t.Logf("inspected %d YAML files and %d {actor}/{actee} placeholders across %d walk roots (%d placeholders logged, not failed, under observerIdentityGuardContentSafeViaCode)",
		filesInspected, placeholdersInspected, len(observerIdentityGuardRoots), exemptFindings)
	for _, root := range observerIdentityGuardRoots {
		t.Logf("  %s: %d placeholders", root, placeholdersByRoot[root])
	}
}

// checkFileForUnanonymizableIdentities reports every unanonymizable
// {actor}/{actee} placeholder in one file's observer/remote_observer lines
// (t.Errorf, or t.Logf for a file named in
// observerIdentityGuardContentSafeViaCode) and returns how many placeholders
// it looked at and how many of those were logged rather than failed, so the
// caller can prove the walk is not silently inspecting nothing and can report
// the exemption's size honestly.
func checkFileForUnanonymizableIdentities(t *testing.T, path string) (placeholders, exempt int) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: %v", path, err)
		return 0, 0
	}

	relPath, err := filepath.Rel(shippedWorldRoot, path)
	if err != nil {
		t.Errorf("%s: relative to %s: %v", path, shippedWorldRoot, err)
		return 0, 0
	}
	contentSafeViaCode := observerIdentityGuardContentSafeViaCode[filepath.ToSlash(relPath)]

	dec := yamlv3.NewDecoder(bytes.NewReader(data))
	for {
		var doc yamlv3.Node
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			t.Errorf("%s: parse: %v", path, err)
			return placeholders, exempt
		}
		p, e := walkNodeForObserverIdentities(t, path, &doc, nil, contentSafeViaCode)
		placeholders += p
		exempt += e
	}
	return placeholders, exempt
}

func walkNodeForObserverIdentities(t *testing.T, path string, n *yamlv3.Node, ancestors []string, contentSafeViaCode bool) (placeholders, exempt int) {
	t.Helper()

	switch n.Kind {
	case yamlv3.DocumentNode, yamlv3.SequenceNode:
		for _, child := range n.Content {
			p, e := walkNodeForObserverIdentities(t, path, child, ancestors, contentSafeViaCode)
			placeholders += p
			exempt += e
		}
	case yamlv3.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			p, e := walkNodeForObserverIdentities(t, path, v, append(ancestors, k.Value), contentSafeViaCode)
			placeholders += p
			exempt += e
		}
	case yamlv3.ScalarNode:
		if n.Tag == "!!str" && inObserverRole(ancestors) {
			p, e := checkLineForUnanonymizableIdentities(t, path, n, contentSafeViaCode)
			placeholders += p
			exempt += e
		}
	}
	return placeholders, exempt
}

// inObserverRole reports whether an observer-facing key appears anywhere in
// the ancestor key path, not only as the immediate parent -- combat-messages
// nests an intensity band (beginner/expert/master) between the role key and
// the scalar list, defense-messages does not, and this check must hold for
// both without caring which.
//
// It matches by SUFFIX ("observer"), not exact equality, because conditions
// does not use a bare "observer"/"remote_observer" key at all: its three
// narration phases key their room-facing text as start_observer,
// trigger_observer and end_observer. An exact-equality check silently walks
// every condition file and finds nothing -- confirmed empirically before this
// suffix match was added: with only the roots and pattern widened, the guard
// found zero condition placeholders, which is not "conditions are clean", it
// is "conditions were never inspected". combat-messages, defense-messages and
// special-moves all key their room-facing text as bare "observer" or
// "remote_observer", both of which also satisfy a suffix match, so this is a
// pure widening with no narrowing risk to the two original stores.
func inObserverRole(ancestors []string) bool {
	for _, a := range ancestors {
		if strings.HasSuffix(a, "observer") {
			return true
		}
	}
	return false
}

// checkLineForUnanonymizableIdentities finds every {actor}/{actee}
// placeholder in one scalar's text and reports the ones that do not sit
// inside a closed identity tag: it maps every
// `<ansi fg="ALIAS">content</ansi>` span in the text first, then checks each
// placeholder's byte range falls fully inside one whose alias
// anonymizableAliases recognises. A placeholder inside a span whose alias is
// something else (the "mob"/"user" bug) or inside no span at all (a bare
// token, or one behind a tag that opened but never closed) is a violation --
// t.Errorf normally, t.Logf (and counted as exempt) when contentSafeViaCode
// is true, i.e. this file is named in observerIdentityGuardContentSafeViaCode
// because its renderer already substitutes a self-tagged identity
// regardless of the content's own markup.
func checkLineForUnanonymizableIdentities(t *testing.T, path string, n *yamlv3.Node, contentSafeViaCode bool) (placeholders, exempt int) {
	t.Helper()

	text := n.Value

	type safeSpan struct{ start, end int }
	var safeSpans []safeSpan
	for _, m := range identityTagSpanPattern.FindAllStringSubmatchIndex(text, -1) {
		alias := text[m[2]:m[3]]
		if anonymizableAliases[alias] {
			safeSpans = append(safeSpans, safeSpan{start: m[4], end: m[5]})
		}
	}

	for _, ph := range identityPlaceholderPattern.FindAllStringIndex(text, -1) {
		placeholders++
		safe := false
		for _, sp := range safeSpans {
			if ph[0] >= sp.start && ph[1] <= sp.end {
				safe = true
				break
			}
		}
		if safe {
			continue
		}
		if contentSafeViaCode {
			exempt++
			t.Logf("%s:%d: [content-safe-via-code, not enforced] %s is not anonymizable by content alone: %q",
				filepath.ToSlash(path), n.Line, text[ph[0]:ph[1]], text)
			continue
		}
		t.Errorf("%s:%d: %s is not anonymizable: it does not sit inside a closed ansi tag whose alias messaging.Anonymize recognises\n\tline: %s",
			filepath.ToSlash(path), n.Line, text[ph[0]:ph[1]], text)
	}
	return placeholders, exempt
}

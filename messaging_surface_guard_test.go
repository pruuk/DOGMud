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
var textSurfaceRegistry = map[string]surfaceEntry{
	// -- Spell narration: internal/spells/spells.go SpellData, four actor/room
	// x cast/wait fields (plus magic_user_text/magic_room_text, which don't
	// clear the 2-file threshold on their own spelling yet). --
	"cast_user_text": {narration, "internal/spells/spells.go SpellData.CastUserText -- actor-side line narrated the instant a spell is cast (e.g. spells/blood-boil.yaml)."},
	"cast_room_text": {narration, "internal/spells/spells.go SpellData.CastRoomText -- room-side line narrated the instant a spell is cast, paired with cast_user_text."},
	"wait_user_text": {narration, "internal/spells/spells.go SpellData.WaitUserText -- actor-side line narrated during a spell's cast-time channel/wait."},
	"wait_room_text": {narration, "internal/spells/spells.go SpellData.WaitRoomText -- room-side line narrated during a spell's cast-time channel/wait, paired with wait_user_text."},

	// -- Buff narration: internal/buffs/buffspec.go BuffSpec, all six fields
	// present on the 101 buff YAML files (start/trigger/end x user/room). --
	"start_user_text":   {narration, "internal/buffs/buffspec.go BuffSpec.StartUserText -- actor-side line narrated when a buff is applied; one of six start/trigger/end x user/room fields across 101 buff files."},
	"start_room_text":   {narration, "internal/buffs/buffspec.go BuffSpec.StartRoomText -- room-side line narrated when a buff is applied, paired with start_user_text."},
	"trigger_user_text": {narration, "internal/buffs/buffspec.go BuffSpec.TriggerUserText -- actor-side line narrated each time a periodic buff tick fires (e.g. poison, regen)."},
	"trigger_room_text": {narration, "internal/buffs/buffspec.go BuffSpec.TriggerRoomText -- room-side line narrated each time a periodic buff tick fires, paired with trigger_user_text."},
	"end_user_text":     {narration, "internal/buffs/buffspec.go BuffSpec.EndUserText -- actor-side line narrated when a buff expires or is removed."},
	"end_room_text":     {narration, "internal/buffs/buffspec.go BuffSpec.EndRoomText -- room-side line narrated when a buff expires or is removed, paired with end_user_text."},

	// -- Crafting narration: internal/crafting/crafting.go Recipe, 126 recipe
	// files. Crafting currently has NO audience split -- a single message,
	// not actor/room pairs like spells and buffs. --
	"success_message": {narration, "internal/crafting/crafting.go Recipe.SuccessMessage -- narrated crafting-outcome line on a successful craft, 126 recipe files; no user/room split exists for crafting."},
	"failure_message": {narration, "internal/crafting/crafting.go Recipe.FailureMessage -- narrated crafting-outcome line on a failed craft, paired with success_message; same no-audience-split gap."},

	// -- Enchanting narration: internal/enchantments/enchantments.go
	// EnchantSpec. description_suffix below is the CONTENT half of this
	// same struct -- appended prose, not a narrated event. --
	"tier_up_message": {narration, "internal/enchantments/enchantments.go EnchantSpec.TierUpMessage -- narrated line sent to the player when an enchantment advances a tier."},

	// -- Quest step narration: internal/quests/quests.go Quest.PlayerMessage /
	// RoomMessage, fired when a quest step completes. --
	"playermessage": {narration, "internal/quests/quests.go Quest.PlayerMessage -- actor-side line narrated when a quest step completes."},
	"roommessage":   {narration, "internal/quests/quests.go Quest.RoomMessage -- room-side line narrated when a quest step completes, paired with playermessage."},

	// -- Quest trigger narration: internal/quests/triggers.go. Both are
	// dash-prefixed list items ("- npc_say:", "- send_text: ..."), which an
	// earlier version of this walk's key regex could not see. --
	"npc_say":   {narration, "internal/quests/triggers.go QuestTrigger.NpcSay (*NpcSayDef) -- a quest trigger that makes a mob speak scripted lines with per-line delay/speaker/emote (see modules/gmcp/gmcp.Quest.go); 32 quest files, dash-prefixed."},
	"send_text": {narration, "internal/quests/triggers.go QuestTrigger.SendText -- a quest trigger sending a message to the player only (modules/gmcp/gmcp.Quest.go: \"message to the player only\"); 46 quest files, dash-prefixed."},

	// room_text is genuinely overloaded but every hit is narration: the
	// bare (non-prefixed) spelling is QuestTrigger.RoomText -- "message to
	// the whole room", pairing with send_text -- on 13 quest files, plus one
	// behaviortree action param (internal/behaviortree/actions_dialogue.go,
	// getStringParam(params, "room_text")) that drives a mob's scripted
	// room-facing speech/emote when a behavior-tree event fires.
	"room_text": {narration, "Two narrated surfaces share this bare spelling: internal/quests/triggers.go QuestTrigger.RoomText (room half of send_text, 13 quest files) and the room_text action param read by internal/behaviortree/actions_dialogue.go (mob speaks/emotes to the room on a behavior-tree event). Do not confuse with the *_room_text spellings above, which are separate distinct keys on spells/buffs."},

	// user_text is the behaviortree-only counterpart of room_text: an action
	// param, not a struct field, read the same way by
	// internal/behaviortree/actions_dialogue.go.
	"user_text": {narration, "internal/behaviortree/actions_dialogue.go getStringParam(params, \"user_text\") -- drives a mob's scripted actor-facing speech/emote (e.g. the \"respond\" action) when a behavior-tree event fires, seen in behaviors/**/*.yaml."},

	// -- Room/zone ambient narration. --
	"idlemessages": {narration, "internal/rooms/rooms.go Room.IdleMessages and internal/rooms/zoneconfig.go ZoneConfig.IdleMessages -- room/zone ambient flavour lines, 1,285 occurrences, the largest narration surface in the game. Read by internal/hooks/NewRound_UserRoundTick.go."},
	"message":      {narration, "internal/rooms/spawninfo.go SpawnInfo.Message -- custom line narrated to the room when a spawn-list creature appears, replacing the default spawn announcement; 57 room files."},

	// -- Combat/attack/defence/taunt message triad. All nine of these keys
	// are structural selector/audience keys rather than prose themselves --
	// the actual lines live inside the maps they key into -- but they ARE
	// the narration shape (see messagingSurfaceAudienceKeys' own comment),
	// spanning internal/combat/taunt_messages.go, internal/items/
	// attack_messages.go and internal/items/defensive_messages.go. --
	"toattacker": {narration, "Attacker-side phrasing key shared by combat/taunt_messages.go TauntMessages.ToAttacker, items/attack_messages.go and items/defensive_messages.go -- combat/attack/defence/taunt message triad."},
	"todefender": {narration, "Defender-side phrasing key, same triad as toattacker (taunt_messages.go, attack_messages.go, defensive_messages.go)."},
	"toroom":     {narration, "Room-observer phrasing key, same triad as toattacker; ToRoom on TauntMessages/AttackMessages/DefensiveMessages."},
	"together":   {narration, "items/attack_messages.go and items/defensive_messages.go Together field -- joint attacker+defender phrasing, paired with separate, in the same message triad."},
	"separate":   {narration, "items/attack_messages.go and items/defensive_messages.go Separate field -- independent attacker/defender phrasing, paired with together."},
	"optionid":   {narration, "combat/taunt_messages.go, items/attack_messages.go, items/defensive_messages.go OptionId field -- an identifier/selector (e.g. a DefenseType or ItemSubType), not prose itself, but it selects which tier of the message triad's Options map plays; part of the narration shape, not content."},
	"options":    {narration, "The map of tiered/intensity message pools selected by optionid, same combat/attack/defence/taunt triad; the prose lives one level down inside this map."},

	// -- Sentient item voice narration: internal/itemvoices/itemvoices.go
	// VoiceSpec, one YAML per voice, consumed by the pinnacle per-round tick
	// for items with a voice_id. --
	"lines":    {narration, "Overloaded but every schema hit is narration: itemvoices.go VoiceSpec.Lines (sentient-item chatter pools), quests/triggers.go NpcSayDef.Lines (npc_say scripted speech), and conversations/conversation.go ConversationDef.Lines (ambient NPC-NPC exchange, see CLAUDE.md NPC<->NPC Conversations). A handful of room `nouns:` children (e.g. \"flood lines\") coincidentally reuse this spelling as author content and are a known false positive of the 2-file heuristic -- see washing lines below for the same pattern."},
	"on_taunt": {narration, "internal/itemvoices/itemvoices.go validVoiceEvents[\"on_taunt\"] -- an event-name key nested under a VoiceSpec's lines: map, selecting the line pool played when a sentient item's bearer taunts. A selector key like optionid, not prose itself, but part of the same narration shape."},

	// -- voice_id / voiceid: TWO SPELLINGS OF THE SAME CONCEPT, drifted
	// between two schemas that must agree for sentient-item chatter to
	// resolve. Neither value is prose -- both are foreign-key identifiers --
	// so both file as config. This drift is a consolidation target for a
	// later stage of the messaging arc, not fixed here. --
	"voice_id": {config, "internal/items/itemspec.go ItemSpec.VoiceId, yaml tag \"voice_id\" (with underscore) -- a sentient item's reference to its itemvoices/<id>.yaml file. Same concept as voiceid below, spelled differently; not player prose, an identifier."},
	"voiceid":  {config, "internal/itemvoices/itemvoices.go VoiceSpec.VoiceId, yaml tag \"voiceid\" (no underscore) -- the voice file's own self-identifying id, matched against items' voice_id. Same concept as voice_id above, spelled differently; not player prose, an identifier."},

	// -- Content: authored text read on request, not narrated as an event. --
	"description":         {content, "The single most overloaded key in the schema -- generic authored description field spanning achievements, biomes, buffs, conversations, factions, items, mobs, mutations, patrols, quests, rooms, schedules, species, spells, users and facts.yaml. Universally read on request (look/examine/status/identify), never narrated as an event."},
	"description_suffix":  {content, "internal/enchantments/enchantments.go EnchantSpec.DescriptionSuffix -- prose appended to an item's description once enchanted; read via look/examine, not narrated. Sibling field tier_up_message on the same struct IS narration -- see above."},
	"descriptionmodifier": {content, "internal/mutators/mutators.go Mutator.DescriptionModifier (*TextModifier) -- text injected into a mutated entity's description; read on request like description above."},
	"hidden_description":  {content, "internal/rooms/rooms.go Room.HiddenDescription -- revealed only after a successful search/perception check, but still authored content read on request rather than an event narration."},
	"hint":                {content, "internal/quests/quests.go Quest.Hint -- quest-log guidance text shown to the player on request via the journal/quest command, 67 quest files."},
	"hints":               {content, "Dominated (286 of 287 files) by internal/dialogue/types.go's Hints field -- narrator-perspective text describing dialogue options (see CLAUDE.md Dialogue Voice & Trigger Discoverability), read on request when a player enters a dialogue node. One outlier file, the top-level _datafiles/world/dogmud/hints.yaml, reuses the identical spelling for periodic gameplay tips broadcast every ~5 minutes by internal/hooks/NewRound_BroadcastHints.go -- that single surface is narration-shaped but is outvoted by the dialogue usage; filed as content with this noted as a known gap."},
	"greetings":           {content, "internal/dialogue/types.go DialogueTree.Greetings ([]Greeting) -- NPC greeting variants shown when a dialogue tree is entered; dialogue content, outside the messaging arc's scope."},
	"text":                {content, "Heavily overloaded: internal/dialogue/types.go's Text field dominates by file count (286 dialogue files, NPC spoken content read via talk/ask -- see CLAUDE.md Dialogue Voice). The identical spelling is ALSO genuine narration elsewhere: internal/behaviortree/actions_dialogue.go's text action param (say/emote actions, 44 behavior files), internal/conversations/conversation.go ConversationLine.Text (ambient NPC-NPC exchange, 18 files), internal/quests/triggers.go SayLineDef.Text (npc_say lines), and internal/mutators/mutators.go Mutator.Text. Filed as content because dialogue is the overwhelming majority; the narration uses are a consolidation target for a later arc stage."},

	// washing lines: a room `nouns:` child, not schema at all -- it only
	// crossed the 2-file threshold because three unrelated rooms happen to
	// describe the same noun. Same pattern as the room-noun tail of "lines"
	// above.
	"washing lines": {content, "A room `nouns:` child on rooms/new_plymouth_common/5613.yaml, rooms/new_plymouth_docks/5519.yaml and rooms/new_plymouth_old_quarter/6033.yaml -- author-chosen noun text, not a schema key, that only crossed the 2-file threshold because three unrelated rooms independently used the same noun phrase. Known limitation of the file-count heuristic, not a real recurring surface."},
}

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

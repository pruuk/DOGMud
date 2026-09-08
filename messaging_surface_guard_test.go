package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

	// -- Item on-use narration: internal/items/itemspec.go ItemSpec, YAML-
	// driven use effects (replaces JS onUse/onCommand_use). Method E find:
	// on_use_user_text is used in exactly one data file
	// (materials-40000/40042-herbalism_recipe_page.yaml) so the 2-file
	// threshold alone would never have registered it; it is schema by Go
	// struct tag regardless. Sibling on_use_room_text is not registered here
	// because it does not appear in any data file today -- nothing for the
	// walk to find, so it cannot be stale either. --
	"on_use_user_text": {narration, "internal/items/itemspec.go ItemSpec.OnUseUserText -- narrated to the player via user.SendText(messaging.CategorySystem, ...) in internal/usercommands/use.go when they `use` the item; found in exactly ONE data file (materials-40000/40042-herbalism_recipe_page.yaml), promoted to schema by Method E (Go yaml struct tag) rather than the 2-file threshold."},

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

	// -- Grapple outcome narration: internal/grapplemessaging/loader.go
	// TemplateTriad (Controller/Controlled/Observers) and GradientTriad
	// (Observers only), rendered by RenderOutcome, consumed by
	// internal/hooks/Position_GrappleTick.go. All three live in the single
	// data file _datafiles/world/dogmud/messaging/grapple_outcomes.yaml, so
	// none clears the 2-file threshold on file COUNT alone -- each key
	// recurs many times (30-40 occurrences) within that one file, across
	// many outcome entries. Promoted to schema by Method E (Go yaml struct
	// tag), same shape as on_use_user_text above. --
	"controller": {narration, "internal/grapplemessaging/loader.go TemplateTriad.Controller -- second-person line shown to the grapple's controlling side; grapple_outcomes.yaml is the only data file, 37 occurrences within it, promoted to schema by Method E."},
	"controlled": {narration, "internal/grapplemessaging/loader.go TemplateTriad.Controlled -- second-person line shown to the grapple's controlled side, paired with controller; same single-file/Method-E shape."},
	"observers":  {narration, "internal/grapplemessaging/loader.go TemplateTriad.Observers and GradientTriad.Observers -- third-person line broadcast to the room during a grapple outcome/gradient event; same single-file/Method-E shape as controller/controlled."},

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

	// taunt_pull: matched by the "taunt" stem but is a plain bool toggle, not
	// prose -- promoted to schema by Method E (found in exactly one data
	// file, materials-40000/40185-aegis_of_mockery.yaml).
	"taunt_pull": {config, "internal/items/itemspec.go ItemSpec.TauntPull (bool) -- \"sentient chatter on_taunt also pulls the bearer's target's aggro (Aegis)\"; a toggle, not player-facing text, despite matching the taunt stem. Found in exactly ONE data file, promoted to schema by Method E (Go yaml struct tag)."},

	// emote: a genuine Method E COLLISION, not a real hit. internal/quests/
	// triggers.go SayLineDef.Emote (bool, dash-prefixed under an npc_say
	// lines: list) shares this exact spelling, but no quest file in
	// _datafiles/world/dogmud actually sets that field today (grepped: zero
	// matches; the only quest-file occurrences of the substring "emote" are
	// prose inside "#" comments, which this walk does not treat as keys).
	// The one data-file hit Method E's own walk finds is
	// ansi-aliases.yaml's `emote: 144` -- an ANSI colour-alias numeric code
	// (internal/templates/templates.go loads this file via
	// ansitags.LoadAliases), unrelated to the quest field. Filed as config
	// because the only real usage today is that colour alias, not prose.
	// Same pattern as washing lines below: a spelling collision, not a
	// recurring schema surface.
	"emote": {config, "Method E collision: internal/quests/triggers.go SayLineDef.Emote (bool) shares this spelling but is unused in any quest data file today (zero matches). The one real hit is ansi-aliases.yaml's `emote: 144`, an ANSI colour-alias code loaded by ansitags.LoadAliases (internal/templates/templates.go) -- not player prose."},

	// -- Content: authored text read on request, not narrated as an event. --
	"description":         {content, "The single most overloaded key in the schema -- generic authored description field spanning achievements, biomes, buffs, conversations, factions, items, mobs, mutations, patrols, quests, rooms, schedules, species, spells, users and facts.yaml. Universally read on request (look/examine/status/identify), never narrated as an event."},
	"description_suffix":  {content, "internal/enchantments/enchantments.go EnchantSpec.DescriptionSuffix -- prose appended to an item's description once enchanted; read via look/examine, not narrated. Sibling field tier_up_message on the same struct IS narration -- see above."},
	"descriptionmodifier": {content, "internal/mutators/mutators.go Mutator.DescriptionModifier (*TextModifier) -- text injected into a mutated entity's description; read on request like description above."},
	"hidden_description":  {content, "internal/rooms/rooms.go Room.HiddenDescription -- revealed only after a successful search/perception check, but still authored content read on request rather than an event narration."},
	"corpse_description":  {content, "internal/mobs/mobs.go Mob.CorpseDescription -- overrides the default corpse look-text; rendered via user.SendText(messaging.CategoryRoomDescription, ...) in internal/usercommands/look.go, i.e. read on `look` at the corpse like description above, not narrated as an event. Found in exactly ONE data file (mobs/thornwall_city/374-caravan_wagon.yaml), promoted to schema by Method E (Go yaml struct tag) rather than the 2-file threshold."},
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

// messagingSurfaceGoRoots mirrors tools/messaging_surface_audit.py's
// GO_ROOTS: the two directories walked for Go yaml struct tags.
var messagingSurfaceGoRoots = []string{"internal", "modules"}

// messagingSurfaceYAMLTagRE mirrors tools/messaging_surface_audit.py's
// YAML_TAG_RE: captures the key spelling out of a `yaml:"key,omitempty"`
// struct tag.
var messagingSurfaceYAMLTagRE = regexp.MustCompile(`yaml:"([a-z_][a-z0-9_]*)`)

// messagingSurfaceGoYAMLTagKeys walks internal/ and modules/ (skipping
// _test.go files, same as tools/messaging_surface_audit.py's walk_go) for
// every Go yaml struct tag spelling.
//
// Method E -- Go struct tags are the AUTHORITATIVE schema. A key declared as
// a yaml tag is read by a loader by definition, however many data files
// happen to use it today. This closes the false-negative direction of the
// file-count proxy: corpse_description and on_use_user_text are real fields
// used in one data file each, invisible to the 2-file threshold alone.
//
// This guard walks Go source itself rather than reading
// tools/messaging_surface_audit.py's output -- same reasoning as
// messagingSurfaceWalk above: two instruments that must independently agree,
// not one feeding the other.
func messagingSurfaceGoYAMLTagKeys() (map[string]bool, error) {
	keys := map[string]bool{}
	for _, rootName := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(rootName, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				// An unreadable file is a filesystem problem, not this
				// test's -- skip it rather than fail the whole walk on it.
				return nil
			}
			for _, line := range strings.Split(string(data), "\n") {
				for _, m := range messagingSurfaceYAMLTagRE.FindAllStringSubmatch(line, -1) {
					keys[strings.ToLower(m[1])] = true
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// messagingSurfaceSplitSchema mirrors split_schema_content: a spelling is
// schema if it is found in 2+ files (a loader reads it, so it recurs by
// construction) OR it is a text candidate that also appears as a Go
// `yaml:"..."` struct tag under internal/ or modules/ (Method E) -- a loader
// reads it by definition, whatever the file count happens to be today.
// Everything else is dropped here as author-invented content (e.g. a room
// `nouns:` child). Returns key -> one example file.
func messagingSurfaceSplitSchema(keyFiles map[string]map[string]bool, yamlTagKeys map[string]bool) map[string]string {
	schema := map[string]string{}
	for key, files := range keyFiles {
		if len(files) < 2 && !(messagingSurfaceIsCandidate(key) && yamlTagKeys[key]) {
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
//
// Both directions were verified on 2026-08-31. Adding an unregistered
// `whispered_room_text` key to TWO probe buffs failed the guard by name; adding
// a `nonexistent_probe_text` registry entry failed it as stale; and a
// `solo_probe_text` key in a SINGLE file correctly did not require
// registration, proving the 2-file schema threshold. Re-verify the same way
// after any change to the walk -- two probe files, not one, or the threshold
// makes a single probe silently pass.
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

	yamlTagKeys, err := messagingSurfaceGoYAMLTagKeys()
	if err != nil {
		t.Fatalf("walk Go yaml struct tags under %v: %v", messagingSurfaceGoRoots, err)
	}

	schema := messagingSurfaceSplitSchema(keyFiles, yamlTagKeys)

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

// -----------------------------------------------------------------------
// Narration viewpoint guard (messaging arc M1, task 2)
//
// This extends the guard above with a SECOND, unrelated recurrence check. It
// shares the file because it shares the idiom (a locked registry a walk is
// checked against, both directions), not because it shares data: the walk
// above reads YAML key spellings, this one reads Go call sites. Two
// registries, two walks, one guard file.
//
// BACKGROUND. docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md
// ruled 247 narration sites where an event narrates fewer than all three
// viewpoints (actor/actee/observer, see that doc's "What this is" section):
// 240 are correctly incomplete (the actee is a mob, the target is a door, the
// line is a private refusal), 7 are genuine defects -- a duplicated code path
// that copied an effect and dropped its narration. That audit is POINT IN
// TIME. This guard is what keeps it from silently rotting: it re-derives the
// set of incomplete sites from source on every run and fails when that set
// drifts from narrationViewpointRegistry, in either direction.
//
// THREE DELIBERATE DEPARTURES FROM THE WRITTEN M1 PLAN, be aware before
// changing this:
//
//  1. Keyed on file + a NORMALIZED LITERAL, never file:line. 247 entries keyed
//     by line number across ~80 files would go red on any edit that adds a
//     line above a narration site -- and M2's whole job is to rewrite these
//     sites, so it would go red for every entry at once and say nothing
//     useful. A literal fingerprint is stable under line movement and still
//     catches an edit, a delete, or an add as a key-set difference, exactly
//     the contract narrationSiteKey documents below.
//  2. Does NOT read tools/narration_viewpoint_scan.py's output. That scanner
//     is a textual heuristic (regex anchor + a 16-line trailing window) and
//     mislabelled 26% of the 247 sites during the audit: it cannot see the
//     sendVisualRoomText wrapper all of internal/hooks uses, it matches the
//     actee only by variable-name prefix (target*/victim*/...), and its
//     window pairs a refusal branch with a success-path message it can never
//     reach at runtime. Baking those three bugs into a CI gate would enshrine
//     them. This guard does its own go/ast walk and takes VERDICTS from the
//     audit table, not viewpoint labels.
//  3. Registers the narration events THIS GUARD'S OWN WALK actually finds
//     incomplete, not "every narration site in the game." There are roughly
//     2,100 actor-directed sends and 340 room-directed sends under internal/
//     and modules/ combined; a registry that size would be unmaintainable and
//     would churn on every commit. See narrationSiteKey and
//     narrationCandidateEvent for the exact, narrower population this guard
//     actually covers, and "SCOPE, MEASURED" below for the numbers.
//
// SCOPE, MEASURED. This guard's own walk, run against the 2026-09-07 audit's
// 80 files, finds 127 candidate events (its own definition of "candidate,"
// below) where the audit's scanner found 128 in the YNY/YYN pattern (actor
// plus exactly one of actee/observer -- see narrationCandidateEvent). 105
// land on the exact line the audit cites; the rest land on a different line
// for the SAME event, because the scanner's line is wherever ITS regex
// anchored, not necessarily the event's first call (that doc's own row for
// usercommands/boot.go:32 is anchored on a `user.SendText` line whose real
// companions sit five lines later; this walk anchors on the actee line
// instead). Every one of those apparent misses was checked by hand against
// source before being folded into the registry below with the audit's own
// verdict and reason.
//
// This guard does NOT attempt two harder patterns the audit also covers, and
// says so plainly rather than faking coverage:
//
//   - Pure single-viewpoint events (audit pattern YNN -- actor only, nothing
//     else in the same event at all). 63 of the 247 audit rows are this
//     shape, and the overwhelming majority of them are ordinary refusals
//     ("You don't have that.") that were never meant to carry a second
//     viewpoint -- narrationCandidateEvent below deliberately excludes them,
//     mirroring the scanner's own stated intent ("single-viewpoint: refusal
//     territory, not this arc"). The cost: two of the seven confirmed
//     defects, actions/salvage.go:202 and :207, are ALSO pattern YNN (a
//     player-salvage branch missing BOTH the actee and the observer, where
//     the sibling mob branch has the observer) and are therefore outside
//     what this guard can see. They remain real, tracked defects in the
//     audit; this guard simply cannot distinguish "a deliberate solo
//     refusal" from "a solo line that a sibling branch proves should not be
//     solo" from one event's shape alone.
//   - Cross-branch asymmetry. usercommands/equip.go:218 (the third gap this
//     guard cannot see) is the same shape as the salvage pair: one branch of
//     an if/else omits the room broadcast its sibling branch sends. Catching
//     that needs comparing SIBLING branches against each other, a materially
//     different (and harder) analysis than "does this one event address all
//     three viewpoints." Not attempted here.
//
// So: this guard covers 4 of the audit's 7 confirmed defects -- the ones
// that really are "one event, missing a viewpoint": hooks/spell_resolution.go's
// buff case, usercommands/rally.go and warcry.go's Resonant Larynx fold, and
// usercommands/admin.zap.go's engaged-target path -- and does not claim the
// other 3. A false "covers everything" would be worse than this honest gap.
//
// narrationCallViewpoint identifies which of the three roles (see the audit
// doc's "What this is" table) a single recognised call addresses.
type narrationCallViewpoint int

const (
	viewpointActor narrationCallViewpoint = iota
	viewpointActee
	viewpointObserver
)

// narrationVerdictKind mirrors the audit's two live verdicts. Its third,
// "unsure," was never actually used (0 of 247) and has no code path here.
type narrationVerdictKind int

const (
	verdictCorrect narrationVerdictKind = iota
	verdictGap
)

// narrationEntry is one registered narration event: the ruling (correct, or
// a genuine gap the messaging arc still owes a fix), which of the three
// viewpoints the event actually addresses today, and a one-line reason. Most
// entries trace to an audit row, and for those Reason and Verdict are the
// audit's own so the two documents cannot silently drift onto different
// facts about the same site. The rest are events this guard's more accurate
// walk finds that the audit's scanner never surfaced at all (most often
// because the site uses the sendVisualRoomText wrapper, which the scanner's
// regex cannot see, or because the scanner's actee/observer name patterns
// are narrower than this guard's) -- each was read against source before
// being added and says so in its reason.
type narrationEntry struct {
	Verdict  narrationVerdictKind
	Actor    bool
	Actee    bool
	Observer bool
	Reason   string
}

// narrationSiteKey is the registry key for one candidate narration event:
// the repo-relative file (internal/ and modules/ prefixes stripped, matching
// the audit's own "Site" column convention) joined to a normalized literal
// fingerprint by "|". Never file:line -- see the guard's header comment for
// why.
func narrationSiteKey(relFile, literal string) string {
	return relFile + "|" + narrationNormalizeLiteral(literal)
}

// narrationLiteralMaxRunes caps the fingerprint so a trivial rewrap of a long
// line does not change the key, while still keeping distinct nearby messages
// distinguishable.
const narrationLiteralMaxRunes = 80

// narrationNormalizeLiteral strips the source-level quoting a *ast.BasicLit
// carries in its Value (", ' or `), collapses internal whitespace runs to a
// single space, and truncates to narrationLiteralMaxRunes runes.
func narrationNormalizeLiteral(raw string) string {
	trimmed := raw
	if len(trimmed) >= 2 {
		first, last := trimmed[0], trimmed[len(trimmed)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') || (first == '`' && last == '`') {
			trimmed = trimmed[1 : len(trimmed)-1]
		}
	}
	collapsed := strings.Join(strings.Fields(trimmed), " ")
	runes := []rune(collapsed)
	if len(runes) > narrationLiteralMaxRunes {
		runes = runes[:narrationLiteralMaxRunes]
	}
	return string(runes)
}

// narrationStringLiteralIn finds the first string literal reachable from
// expr: directly, one level into a wrapped call (fmt.Sprintf("...", args...)
// used as an argument), or across a "a" + "b" concatenation. Returns "" when
// none is found (a message built entirely from variables/params, e.g. a
// dialogue tree's user_text, or forwarded through a helper like
// util.SplitStringNL(replyMsg, 80)).
func narrationStringLiteralIn(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return e.Value
		}
	case *ast.CallExpr:
		for _, a := range e.Args {
			if lit := narrationStringLiteralIn(a); lit != "" {
				return lit
			}
		}
	case *ast.BinaryExpr:
		if lit := narrationStringLiteralIn(e.X); lit != "" {
			return lit
		}
		return narrationStringLiteralIn(e.Y)
	}
	return ""
}

// narrationCallFingerprint returns the text this call's key is built from: a
// literal string when one of call's arguments carries one (per
// narrationStringLiteralIn), otherwise the printed source of the call's
// argument list itself (e.g. "playerMsg, roomMsg" or
// "util.SplitStringNL(replyMsg, 80)"), which is stable and still distinct
// per call site even when the message text is entirely data-driven.
func narrationCallFingerprint(fset *token.FileSet, call *ast.CallExpr) string {
	for _, arg := range call.Args {
		if lit := narrationStringLiteralIn(arg); lit != "" {
			return lit
		}
	}
	var buf bytes.Buffer
	for i, arg := range call.Args {
		if i > 0 {
			buf.WriteString(", ")
		}
		if err := printer.Fprint(&buf, fset, arg); err != nil {
			buf.WriteString("?")
		}
	}
	return buf.String()
}

// narrationCall is one recognised SendText-shaped call: which viewpoint it
// addresses, the source line (for diagnostics only -- never part of the
// registry key), and its fingerprint text.
type narrationCall struct {
	viewpoint narrationCallViewpoint
	line      int
	fp        string
}

// narrationRecognizeCall reports whether call is one of the four narration
// shapes named in the guard's header comment, and if so its viewpoint:
//
//   - actor:    SendText on a receiver literally named "user" or "actor"
//   - actee:    SendText on any OTHER receiver identifier -- deliberately
//     broader than tools/narration_viewpoint_scan.py's
//     target*/victim*/defender*/recipient*/other*/receiver* name-prefix
//     match, which is exactly why that scanner missed
//     usercommands/admin.zap.go's `u.SendText` actee (see the audit's
//     "Confirmed defects" #5).
//   - observer: room.SendText / SendTextVisual / SendTextToUser on a
//     receiver literally named "room", or a call to the free function
//     sendVisualRoomText (internal/hooks/NewRound_DoCombat_helpers.go:401).
//
// KNOWN BLIND SPOT, same shape as tools/narration_viewpoint_scan.py's own
// ACTOR/OBSERVER regexes: a send whose receiver is a room or actor value
// under a DIFFERENT name (destRoom, oldRoom, uRoom, gotoRoom; buyer, caster)
// is invisible to the matching role and falls into the broad "any other
// identifier" actee bucket instead. A handful of registered entries below
// are affected -- e.g. usercommands/admin.teleport.go:98's `gotoRoom.SendText`
// really is an observer line, just not one this walk's Observer recognizer
// can see -- and their Reason says so rather than letting the raw
// Actor/Actee/Observer booleans misstate what the code actually does.
func narrationRecognizeCall(call *ast.CallExpr) (narrationCallViewpoint, bool) {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		recv, ok := fun.X.(*ast.Ident)
		if !ok {
			return 0, false
		}
		switch fun.Sel.Name {
		case "SendText":
			switch recv.Name {
			case "user", "actor":
				return viewpointActor, true
			case "room":
				return viewpointObserver, true
			default:
				return viewpointActee, true
			}
		case "SendTextVisual", "SendTextToUser":
			if recv.Name == "room" {
				return viewpointObserver, true
			}
		}
	case *ast.Ident:
		if fun.Name == "sendVisualRoomText" {
			return viewpointObserver, true
		}
	}
	return 0, false
}

// narrationBlockTerminates reports whether body's LAST statement is a
// return, break, continue, goto, or a call to panic -- the shape of a guard
// clause that bails out, as opposed to a plain conditional decoration.
func narrationBlockTerminates(body *ast.BlockStmt) bool {
	if len(body.List) == 0 {
		return false
	}
	switch s := body.List[len(body.List)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return s.Tok == token.BREAK || s.Tok == token.CONTINUE || s.Tok == token.GOTO
	case *ast.ExprStmt:
		if call, ok := s.X.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "panic" {
				return true
			}
		}
	}
	return false
}

// narrationHasImmediateTrailingCall reports whether a narration call is
// reachable, unconditionally, by continuing to scan stmts from fromIdx --
// i.e. before hitting another branch, loop, switch, or terminating
// statement. Used to tell an if/else that just PICKS WORDING (e.g.
// usercommands/equip.go's `if iSpec.Type == items.Offhand {...} else
// {...}`, both single actor lines, followed by one unconditional room
// broadcast) from an if/else whose branches are alternate OUTCOMES with
// nothing shared afterward (e.g. actions/salvage.go's recovered/not-recovered
// split, the last statement in its enclosing block).
func narrationHasImmediateTrailingCall(stmts []ast.Stmt, fromIdx int) bool {
	for i := fromIdx; i < len(stmts); i++ {
		switch s := stmts[i].(type) {
		case *ast.ExprStmt:
			if call, ok := s.X.(*ast.CallExpr); ok {
				if _, ok := narrationRecognizeCall(call); ok {
					return true
				}
			}
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.ReturnStmt, *ast.BranchStmt:
			return false
		}
	}
	return false
}

// narrationScanFuncLits finds any function literal embedded in stmt (a
// closure passed to go/defer, assigned to a variable, etc.) and walks its
// body as its own standalone scope, appending whatever it finds to *events.
func narrationScanFuncLits(fset *token.FileSet, stmt ast.Stmt, events *[][]narrationCall) {
	ast.Inspect(stmt, func(n ast.Node) bool {
		if fl, ok := n.(*ast.FuncLit); ok {
			narrationCollectStandalone(fset, fl.Body.List, events)
			return false
		}
		return true
	})
}

// narrationCollectFromBlock walks stmts in order, appending recognised calls
// to *current and splitting off completed events into *events at each
// boundary. It is called two ways: as the owner of a fresh scope (only from
// narrationCollectStandalone, which finalizes *current when this returns),
// and recursively to FLATTEN a transparent nested block into the caller's
// still-open *current (an if with no else that does not terminate, a bare
// scoping block, or -- see narrationHasImmediateTrailingCall -- an if/else
// that only picks wording before a shared unconditional continuation). This
// function itself never finalizes; only narrationCollectStandalone does,
// which is what lets the transparent case keep accumulating into the same
// event across the recursive call instead of being cut short the moment the
// nested scope returns.
func narrationCollectFromBlock(fset *token.FileSet, stmts []ast.Stmt, current *[]narrationCall, events *[][]narrationCall) {
	finalize := func() {
		if len(*current) > 0 {
			*events = append(*events, *current)
			*current = nil
		}
	}
	for idx, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.ExprStmt:
			if call, ok := s.X.(*ast.CallExpr); ok {
				if vp, ok := narrationRecognizeCall(call); ok {
					*current = append(*current, narrationCall{
						viewpoint: vp,
						line:      fset.Position(call.Pos()).Line,
						fp:        narrationCallFingerprint(fset, call),
					})
					continue
				}
			}
			narrationScanFuncLits(fset, stmt, events)

		case *ast.IfStmt:
			terminates := narrationBlockTerminates(s.Body)
			if s.Else == nil && !terminates {
				// Transparent decoration -- e.g. the "actee looked up once,
				// nil when the target is a mob" idiom the bash/kick/trip
				// family shares. Flatten into the surrounding event.
				narrationCollectFromBlock(fset, s.Body.List, current, events)
				continue
			}
			if elseBlock, ok := s.Else.(*ast.BlockStmt); ok && !terminates &&
				narrationHasImmediateTrailingCall(stmts, idx+1) {
				// Phrasing-only if/else feeding a shared unconditional
				// continuation -- fold both branches into the current event
				// rather than splitting.
				narrationCollectFromBlock(fset, s.Body.List, current, events)
				narrationCollectFromBlock(fset, elseBlock.List, current, events)
				continue
			}
			// A genuine outcome split (has an else with nothing guaranteed
			// to follow) or a guard clause that bails: whatever was
			// accumulating before this if is a complete event on its own,
			// and each branch is its own fresh event.
			finalize()
			narrationCollectStandalone(fset, s.Body.List, events)
			switch e := s.Else.(type) {
			case *ast.BlockStmt:
				narrationCollectStandalone(fset, e.List, events)
			case *ast.IfStmt:
				narrationCollectStandalone(fset, []ast.Stmt{e}, events)
			}

		case *ast.BlockStmt:
			// A bare scoping block, not a branch -- transparent.
			narrationCollectFromBlock(fset, s.List, current, events)

		case *ast.ForStmt:
			finalize()
			narrationCollectStandalone(fset, s.Body.List, events)
		case *ast.RangeStmt:
			finalize()
			narrationCollectStandalone(fset, s.Body.List, events)

		case *ast.SwitchStmt:
			finalize()
			for _, c := range s.Body.List {
				if cc, ok := c.(*ast.CaseClause); ok {
					narrationCollectStandalone(fset, cc.Body, events)
				}
			}
		case *ast.TypeSwitchStmt:
			finalize()
			for _, c := range s.Body.List {
				if cc, ok := c.(*ast.CaseClause); ok {
					narrationCollectStandalone(fset, cc.Body, events)
				}
			}

		default:
			narrationScanFuncLits(fset, stmt, events)
		}
	}
}

// narrationCollectStandalone owns a fresh event scope: a function body, a
// loop body, a switch case body, or one branch of a split if/else. It is the
// only place that finalizes a trailing accumulator, which is what makes the
// transparent-flatten recursion in narrationCollectFromBlock safe -- that
// recursive call shares this scope's *current and must NOT finalize on its
// own return.
func narrationCollectStandalone(fset *token.FileSet, stmts []ast.Stmt, events *[][]narrationCall) {
	var current []narrationCall
	narrationCollectFromBlock(fset, stmts, &current, events)
	if len(current) > 0 {
		*events = append(*events, current)
	}
}

// narrationCandidateEvent reports whether ev -- one grouped narration event
// -- is a candidate this guard tracks, and if so its key and which
// viewpoints it addresses. A candidate has an actor call (this guard is
// actor-anchored, same restriction as tools/narration_viewpoint_scan.py's
// own ACTOR regex -- see the header comment's departure #3) AND addresses
// exactly one of {actee, observer}, i.e. is missing exactly one of the
// other two. Excluded: an event with all three (nothing missing) and a pure
// single-viewpoint actor-only event (audit pattern YNN, "refusal territory,
// not this arc" -- see the header comment's SCOPE, MEASURED section).
func narrationCandidateEvent(relFile string, ev []narrationCall) (key string, hasActor, hasActee, hasObserver bool, ok bool) {
	if len(ev) == 0 {
		return "", false, false, false, false
	}
	for _, c := range ev {
		switch c.viewpoint {
		case viewpointActor:
			hasActor = true
		case viewpointActee:
			hasActee = true
		case viewpointObserver:
			hasObserver = true
		}
	}
	if !hasActor || hasActee == hasObserver {
		return "", hasActor, hasActee, hasObserver, false
	}
	return narrationSiteKey(relFile, ev[0].fp), hasActor, hasActee, hasObserver, true
}

// narrationGoRoots mirrors messagingSurfaceGoRoots -- the same two directories,
// reused rather than redeclared.
var narrationGoRoots = messagingSurfaceGoRoots

// narrationCandidateSite is one candidate this guard's walk found: its
// viewpoints (for diagnostics) and its source line (for diagnostics only --
// never part of the key).
type narrationCandidateSite struct {
	relFile     string
	line        int
	hasActor    bool
	hasActee    bool
	hasObserver bool
}

// narrationWalk walks narrationGoRoots and returns every candidate event
// (see narrationCandidateEvent), keyed by narrationSiteKey.
func narrationWalk() (map[string]narrationCandidateSite, error) {
	sites := map[string]narrationCandidateSite{}
	fset := token.NewFileSet()
	for _, root := range narrationGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				// A syntax error is the compiler's problem to report, not
				// this test's.
				return nil
			}
			rel := filepath.ToSlash(path)
			rel = strings.TrimPrefix(rel, "internal/")
			rel = strings.TrimPrefix(rel, "modules/")
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				var events [][]narrationCall
				narrationCollectStandalone(fset, fd.Body.List, &events)
				for _, ev := range events {
					key, hasActor, hasActee, hasObserver, ok := narrationCandidateEvent(rel, ev)
					if !ok {
						continue
					}
					sites[key] = narrationCandidateSite{
						relFile: rel, line: ev[0].line,
						hasActor: hasActor, hasActee: hasActee, hasObserver: hasObserver,
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return sites, nil
}

// narrationViewpointsLabel renders a candidate's viewpoints for a failure
// message, e.g. "actor+observer, missing actee".
func narrationViewpointsLabel(s narrationCandidateSite) string {
	var have []string
	if s.hasActor {
		have = append(have, "actor")
	}
	if s.hasActee {
		have = append(have, "actee")
	}
	if s.hasObserver {
		have = append(have, "observer")
	}
	var missing []string
	if !s.hasActor {
		missing = append(missing, "actor")
	}
	if !s.hasActee {
		missing = append(missing, "actee")
	}
	if !s.hasObserver {
		missing = append(missing, "observer")
	}
	return strings.Join(have, "+") + ", missing " + strings.Join(missing, "+")
}

// narrationViewpointRegistry is the locked set of narration events this
// guard's own walk (narrationWalk, via narrationCandidateEvent) finds
// candidates today: 141 entries. 106 trace to a row in
// docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, quoted
// verbatim in Reason so the two documents describe the same fact rather than
// two independently-typed ones that can drift apart; a handful land on a
// different line than the audit's own "Site" column because the audit's
// scanner anchors on whichever line ITS regex matched, not necessarily the
// event's first call (see the header comment's SCOPE, MEASURED section).
// The remaining 35 are events this walk finds that the audit's scanner never
// surfaced -- most often the sendVisualRoomText wrapper, or a room/actor
// value under a name other than "room"/"user"/"actor" -- each read against
// source and marked as such in its own Reason.
//
// Four entries carry verdictGap: hooks/spell_resolution.go's buff case
// (missing the room broadcast its sibling heal case has),
// usercommands/rally.go and usercommands/warcry.go's Resonant Larynx fold
// (silently reapplying the paired buff with no SendText), and
// usercommands/admin.zap.go's engaged-target path (drops the victim to 1 HP
// with no message). These stay registered, not fixed, on purpose: the
// contract is set equality with what the walk finds today, and fixing one
// makes its event complete, which drops it out of the walk and turns this
// entry stale -- exactly the signal that tells whoever ships the fix to also
// update the audit and this registry.
var narrationViewpointRegistry = map[string]narrationEntry{
	"actions/defuse.go|<ansi fg=\"green\">You carefully disarm the trap mechanism.</ansi>":                                  {verdictCorrect, true, false, true, "audit: exit trap disarmed -- target is the lock (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, actions/defuse.go:269)"},
	"actions/defuse.go|<ansi fg=\"red-bold\">The trap triggers as you fumble the mechanism!</ansi>":                         {verdictCorrect, true, false, true, "audit: trap fires after a failed defuse -- target is the trap, not a character (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, actions/defuse.go:161)"},
	"actions/forage.go|You crouch low and begin searching the ground carefully...":                                          {verdictCorrect, true, false, true, "audit: player forages the ground -- solo action, no target person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, actions/forage.go:78)"},
	"actions/mutation_cocoon.go|<ansi fg=\"cyan-bold\">You fold inward; a hard shell snaps shut and the fight lose":         {verdictCorrect, true, false, true, "audit: cocoon mutation self-buff, drops room aggro -- self-buff, no actee (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, actions/mutation_cocoon.go:42)"},
	"actions/mutation_venom_coat.go|<ansi fg=\"green-bold\">You flex, and venom weeps slick across your weapons.</ansi":     {verdictCorrect, true, false, true, "audit: venom-coat self-buff -- self-targeted buff (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, actions/mutation_venom_coat.go:23)"},
	"actions/plant.go|<ansi fg=\"mobname\">%s</ansi> catches you in the act!":                                               {verdictCorrect, true, false, true, "audit: failed plant on a mob, caught -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, actions/plant.go:202)"},
	"actions/plant.go|<ansi fg=\"mobname\">%s</ansi> spots you slipping something into":                                     {verdictCorrect, true, false, true, "audit: failed plant in a container, spotted -- container has no owner; the spotter is an observer (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, actions/plant.go:431)"},
	"actions/search.go|You snoop around for a bit...\\n":                                                                    {verdictCorrect, true, false, true, "audit: player begins searching the room -- solo action (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, actions/search.go:112)"},
	"actions/shadow.go|You begin shadowing <ansi fg=\"username\">%s</ansi>,":                                                {verdictCorrect, true, true, false, "shadow is a covert-observation skill; the target is privately notified they are being shadowed via a separate SendText this walk groups elsewhere, but the room is deliberately not told, which would defeat the point of a stealth skill. Not part of the 2026-09-07 audit; read against source for this guard."},
	"actions/steal.go|<ansi fg=\"mobname\">%s</ansi> catches you in the act!":                                               {verdictCorrect, true, false, true, "audit: failed steal from a mob, caught -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, actions/steal.go:286)"},
	"actions/steal.go|<ansi fg=\"mobname\">%s</ansi> spots you reaching into the":                                           {verdictCorrect, true, false, true, "audit: failed steal from a container, spotted -- container has no owner (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, actions/steal.go:554)"},
	"behaviortree/actions_dialogue.go|messaging.CategoryNPCDialogue, textutil.SubstituteTokens(userText, tokenCtx)":         {verdictCorrect, true, false, true, "audit: NPC dialogue delivered to the asking player -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, behaviortree/actions_dialogue.go:37)"},
	"cleanup/cleanup.go|You trash the <ansi fg=\"item\">%s</ansi> for good.":                                                {verdictCorrect, true, false, true, "trashing your own item from your own backpack; the room line exists (room.SendText two lines below, gated on not sneaking), so this is genuinely actor+observer, no actee since destroying your own item has no third party. Not part of the audit; read against source for this guard."},
	"follow/follow.go|<ansi fg=\"username\">%s</ansi> stopped following you.":                                               {verdictCorrect, true, true, false, "the followed party privately learns their follower stopped; a follow relationship change has no room-facing component in this codebase. Not part of the audit; read against source for this guard."},
	"follow/follow.go|You start following <ansi fg=\"username\">%s</ansi>.":                                                 {verdictCorrect, true, true, false, "starting to follow someone notifies the two parties only, same as the stop case above; no room broadcast for a private relationship state change. Not part of the audit; read against source for this guard."},
	"hooks/NewRound_DoCombat_helpers.go|<ansi fg=\"red\">You lose your concentration as you hit the ground!</ansi>":         {verdictCorrect, true, false, true, "a spell interrupted by falling prone; the room sees the concentration break via sendVisualRoomText two lines below, actor+observer, no actee since concentration breaking is self-only. Not part of the audit; read against source for this guard."},
	"hooks/NewRound_DoCombat_helpers.go|<ansi fg=\"red\">Your concentration shatters — you cannot hold the fold while grap": {verdictCorrect, true, false, true, "a spell interrupted by a grapple breaking concentration, the GrappleBroke sibling of the ProneBroke case at line 570; same shape, sendVisualRoomText broadcasts, no actee."},
	"hooks/NewRound_DoCombat_helpers.go|<ansi fg=\"red-bold\"><ansi fg=\"%s\">%s</ansi> blocks you from fleeing!</ansi>":    {verdictCorrect, true, true, false, "flee blocked by another combatant; uRoom.SendText broadcasts the block to the room, but uRoom is not the literal identifier room this walk's Observer recognizer matches (see the guard's header comment on that blind spot). A real room broadcast exists; this walk just cannot see it under this variable name."},
	"hooks/NewRound_DoCombat_helpers.go|You flee to the <ansi fg=\"exit\">%s</ansi> exit!":                                  {verdictCorrect, true, true, false, "flee succeeds; uRoom.SendText broadcasts the flee to the room two lines below, same uRoom-name blind spot as the block case above, a real room broadcast this walk cannot see under this identifier."},
	"hooks/NewRound_UserRoundTick.go|You attempt to stand, but slip back down in the chaos of battle!":                      {verdictCorrect, true, false, true, "automatic recovery from prone fails; same shape as the success case above, actor+observer via sendVisualRoomText, no actee."},
	"hooks/NewRound_UserRoundTick.go|You scramble to your feet!":                                                            {verdictCorrect, true, false, true, "automatic recovery from prone succeeds; the room sees it via sendVisualRoomText, actor+observer, no actee since recovering from prone is self-only."},
	"hooks/charm_spell.go|<ansi fg=\"cyan\">%s's eyes glaze as your will takes hold. It is yours.</ansi>":                   {verdictCorrect, true, false, true, "a charm spell binds a mob; sendVisualRoomText broadcasts it to the room two lines below, actee is a mob."},
	"hooks/pinnacle_tick.go|<ansi fg=\"item\">%s</ansi> says, \"<ansi fg=\"yellow\">%s</ansi>\"":                            {verdictCorrect, true, false, true, "audit: sentient item chatter -- item speech has no actee (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, hooks/pinnacle_tick.go:527)"},
	"hooks/spell_foldanchor.go|A Chrysalis anchor locks into place here.":                                                   {verdictCorrect, true, false, true, "audit: Chrysalis fold anchor set -- self-targeted spell (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, hooks/spell_foldanchor.go:18)"},
	"hooks/spell_foldrecall.go|<ansi fg=\"username\">%s</ansi> folds through the Veil and vanishes!":                        {verdictCorrect, true, true, false, "the departure broadcast on the room the caster LEFT; oldRoom.SendText is a genuine room broadcast this walk cannot see under that identifier (see the guard's header comment on the room-variable-name blind spot), so this walk reads it as actor+actee when it is really actor+observer."},
	"hooks/spell_purgeaffliction.go|<ansi fg=\"green\">You purge the afflictions from your body.</ansi>":                    {verdictCorrect, true, false, true, "self-cast purge (caster targets themselves); sendVisualRoomText broadcasts to the room two lines below, no actee since there is no separate target. Sibling of the audited spell_purgeaffliction.go row for the other-target branch, which the audit already ruled full trio."},
	"hooks/spell_resolution.go|<ansi fg=\"cyan-bold\">Your %s scrambles %s's focus -- its spell collapses!</ansi>":          {verdictCorrect, true, false, true, "a disruption spell interrupts a mid-cast mob; actee is a mob, sendVisualRoomText broadcasts to the room."},
	"hooks/spell_resolution.go|<ansi fg=\"red\">Your spell backfires violently, wounding you!</ansi>":                       {verdictCorrect, true, false, true, "backfire on a fumbled cast, the mob-target resolution path; actee is a mob, sendVisualRoomText broadcasts. Sibling of the identical backfire text at line 395 in the same file."},
	"hooks/spell_resolution.go|You concentrate on the <ansi fg=\"item\">%s</ansi>...":                                       {verdictCorrect, true, false, true, "identify-via-spell concentration flavor; sendVisualRoomText broadcasts to the room, no actee since the target is an item, not a person."},
	"hooks/spell_resolution.go|Your %s afflicts %s!%s":                                                                      {verdictCorrect, true, false, true, "a DoT spell afflicts a mob; actee is a mob, sendVisualRoomText broadcasts. Mob-target sibling of the audited spell_resolution.go:1004 purge-on-player row."},
	"hooks/spell_resolution.go|Your %s strikes %s! (<ansi fg=\"damage\">%s</ansi>)%s":                                       {verdictCorrect, true, false, true, "a damage spell strikes a mob; actee is a mob, sendVisualRoomText broadcasts to the room. Mob-target sibling of the audited spell_resolution.go:983 row (the player-target branch of the same switch), which the audit already ruled full trio via the wrapper."},
	"hooks/spell_resolution.go|Your %s takes effect on %s!%s":                                                               {verdictCorrect, true, false, true, "a generic effect takes hold on a mob; actee is a mob, sendVisualRoomText broadcasts. Mob-target sibling of the audited spell_resolution.go:1034 heal-on-player row."},
	"hooks/spell_resolution.go|Your %s takes effect on <ansi fg=\"username\">%s</ansi>!%s":                                  {verdictGap, true, true, false, "audit: buff spell on a player -- sibling `case \"heal\"` calls `sendVisualRoomText` at ~:1044; `case \"buff\"` never does (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, hooks/spell_resolution.go:1075)"},
	"hooks/spell_resolution.go|Your spell erupts outward but finds no targets.":                                             {verdictCorrect, true, false, true, "a disruption spell that finds no targets; sendVisualRoomText broadcasts it, no actee since nothing was hit. Same sendVisualRoomText-wrapper visibility this walk has and tools/narration_viewpoint_scan.py's regex does not, per the guard's header comment."},
	"usercommands/admin.item.go|You wave your hands around and <ansi fg=\"item\">%s</ansi> appears from thin air a":         {verdictCorrect, true, false, true, "audit: admin conjures an item -- no player target (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/admin.item.go:127)"},
	"usercommands/admin.locate.go|<ansi fg=\"username\">%s</ansi> is in room #<ansi fg=\"yellow-bold\">%d</ansi> - <an":     {verdictCorrect, true, true, false, "an admin locate command; the located player is privately told someone is looking for them (actee), but there is deliberately no room broadcast for an admin tool."},
	"usercommands/admin.mob.go|You wave your hands around and <ansi fg=\"mobname\">%s</ansi> appears in the air a":          {verdictCorrect, true, false, true, "audit: admin spawns a mob -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/admin.mob.go:203)"},
	"usercommands/admin.paz.go|You illuminate <ansi fg=\"mobname\">%s</ansi> with a %s!":                                    {verdictCorrect, true, false, true, "audit: admin paz-illuminates a mob -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/admin.paz.go:33)"},
	"usercommands/admin.paz.go|You paz yourself with a":                                                                     {verdictCorrect, true, false, true, "audit: admin paz-illuminates self -- self-targeted (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/admin.paz.go:58)"},
	"usercommands/admin.redescribe.go|You chant softly and wave your hand over the <ansi fg=\"item\">%s</ansi>. Success!":   {verdictCorrect, true, false, true, "audit: admin redescribes an item -- target is an item (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/admin.redescribe.go:50)"},
	"usercommands/admin.rename.go|You chant softly and wave your hand over the <ansi fg=\"item\">%s</ansi>. Success!":       {verdictCorrect, true, false, true, "audit: admin renames a backpack item -- target is an item (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/admin.rename.go:52)"},
	"usercommands/admin.skillset.go|Your \"<ansi fg=\"skill\">%s</ansi>\" skill level has been set to <ansi fg=\"red\">%d<": {verdictCorrect, true, true, false, "audit: confirmation, one skill set -- invisible stat edit, no public component (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/admin.skillset.go:89)"},
	"usercommands/admin.spawn.go|You wave your hands around and <ansi fg=\"container\">%s</ansi> appears from thin ":        {verdictCorrect, true, false, true, "audit: admin conjures a container -- target is a container (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/admin.spawn.go:50)"},
	"usercommands/admin.spawn.go|You wave your hands around and <ansi fg=\"gold\">%d gold</ansi> appears from thin ":        {verdictCorrect, true, false, true, "audit: admin conjures gold -- gold is not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/admin.spawn.go:76)"},
	"usercommands/admin.spawn.go|You wave your hands around pathetically.":                                                  {verdictCorrect, true, false, true, "audit: spawn fails, hand-wave flourish -- no target (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/admin.spawn.go:89)"},
	"usercommands/admin.teleport.go|Moved to room %d.":                                                                      {verdictCorrect, true, true, false, "gotoRoom.SendText broadcasts an arrival flash to the destination room two lines below, a genuine room broadcast this walk cannot see under the gotoRoom identifier (see the guard's header comment on the room-variable-name blind spot)."},
	"usercommands/admin.zap.go|You zap <ansi fg=\"mobname\">%s</ansi> with a %s!":                                           {verdictCorrect, true, false, true, "audit: admin zaps engaged mob -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/admin.zap.go:70)"},
	"usercommands/admin.zap.go|You zap <ansi fg=\"username\">%s</ansi> with a %s!":                                          {verdictGap, true, false, true, "audit: admin zaps engaged player -- the parallel explicit-target path at :46 tells the victim; this one drops them to 1 HP silently (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/admin.zap.go:82)"},
	"usercommands/afk.go|You are no longer AFK.":                                                                            {verdictCorrect, true, false, true, "audit: player returns from AFK -- self state change (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/afk.go:28)"},
	"usercommands/afk.go|You are now AFK. Type <ansi fg=\"command\">afk</ansi> again to return.":                            {verdictCorrect, true, false, true, "audit: goes AFK with no message -- self state change (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/afk.go:48)"},
	"usercommands/afk.go|You are now AFK: %s":                                                                               {verdictCorrect, true, false, true, "audit: goes AFK with a message -- self state change (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/afk.go:43)"},
	"usercommands/appraise.go|You give <ansi fg=\"mobname\">%s</ansi> %d gold to appraise <ansi fg=\"itemname\">%s":         {verdictCorrect, true, false, true, "audit: player pays a merchant to appraise -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/appraise.go:75)"},
	"usercommands/attack.go|You prepare to enter into mortal combat with %s.":                                               {verdictCorrect, true, false, true, "audit: player commits to attacking a mob -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/attack.go:251)"},
	"usercommands/boot.go|<ansi fg=\"alert-5\">You have been disconnected by staff.</ansi> %s":                              {verdictCorrect, true, true, false, "audit: admin boots a player -- boot is server-wide, not room-scoped; no room to observe (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/boot.go:32)"},
	"usercommands/break.go|You break off combat.":                                                                           {verdictCorrect, true, false, true, "audit: player breaks off combat -- no single target; opponents are covered by the room line (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/break.go:17)"},
	"usercommands/character.go|<ansi fg=\"username\">":                                                                      {verdictCorrect, true, false, true, "audit: player hires an alt as a companion -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/character.go:437)"},
	"usercommands/character.go|You dematerialize as <ansi fg=\"username\">":                                                 {verdictCorrect, true, false, true, "audit: player swaps to an alt -- self-targeted (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/character.go:292)"},
	"usercommands/deletecharacter.go|<ansi fg=\"username\">%s</ansi>'s form dissolves into shimmering dust.":                {verdictCorrect, true, false, true, "a player deletes their own character; the room sees the dissolve via room.SendTextVisual just above, no actee since there is no separate target for a self-deletion."},
	"usercommands/dismiss.go|You release <ansi fg=\"mobname\">%s</ansi>. It dissolves back into the energies th":            {verdictCorrect, true, false, true, "audit: companion dismissed peacefully -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/dismiss.go:104)"},
	"usercommands/dismiss.go|You sever the bond with <ansi fg=\"mobname\">%s</ansi>.":                                       {verdictCorrect, true, false, true, "releasing a charmed companion; actee is a mob (the companion), room sees the release."},
	"usercommands/drink.go|You drink the <ansi fg=\"itemname\">%s</ansi>.":                                                  {verdictCorrect, true, false, true, "audit: drinks a potion normally -- self-targeted (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/drink.go:227)"},
	"usercommands/drink.go|You drink the <ansi fg=\"itemname\">%s</ansi>...":                                                {verdictCorrect, true, false, true, "audit: drinks a spoiled potion, retches -- self-targeted (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/drink.go:169)"},
	"usercommands/drop.go|You drop %d item(s).":                                                                             {verdictCorrect, true, false, true, "audit: \"drop all\" succeeds -- items are not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/drop.go:127)"},
	"usercommands/drop.go|You drop <ansi fg=\"gold\">%d gold</ansi> on the floor.":                                          {verdictCorrect, true, false, true, "audit: drop gold succeeds -- gold is not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/drop.go:100)"},
	"usercommands/drop.go|You drop the <ansi fg=\"item\">%s</ansi>.":                                                        {verdictCorrect, true, false, true, "audit: drop item succeeds -- item is not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/drop.go:147)"},
	"usercommands/eat.go|You eat some of the <ansi fg=\"itemname\">%s</ansi>.":                                              {verdictCorrect, true, false, true, "audit: eat succeeds -- item is not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/eat.go:67)"},
	"usercommands/emote.go|You Emote: %s":                                                                                   {verdictCorrect, true, false, true, "audit: emote via alias -- free-form emote has no target (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/emote.go:29)"},
	"usercommands/emote.go|You emote.":                                                                                      {verdictCorrect, true, false, true, "audit: emote with no text -- free-form emote has no target (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/emote.go:17)"},
	"usercommands/equip.go|You equip your <ansi fg=\"item\">%s</ansi> in your %s.":                                          {verdictCorrect, true, false, true, "audit: equips an offhand item into an arm slot -- solo self-action (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/equip.go:229)"},
	"usercommands/equip.go|You remove your <ansi fg=\"item\">%s</ansi> and return it to your backpack.":                     {verdictCorrect, true, false, true, "audit: displaced item returned (shared path) -- solo self-action (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/equip.go:274)"},
	"usercommands/equip.go|You wear your <ansi fg=\"item\">%s</ansi>.":                                                      {verdictCorrect, true, false, true, "audit: wears a wearable -- solo self-action (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/equip.go:285)"},
	"usercommands/equip.go|You wield your <ansi fg=\"item\">%s</ansi>. You're feeling dangerous.":                           {verdictCorrect, true, false, true, "audit: wields a non-wearable -- solo self-action (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/equip.go:293)"},
	"usercommands/get.go|You can't get the <ansi fg=\"noun\">%s</ansi>":                                                     {verdictCorrect, true, false, true, "audit: \"get\" a room noun, refused -- fixture is not a person; failure flavor still broadcasts (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/get.go:693)"},
	"usercommands/get.go|You dig out the <ansi fg=\"itemname\">%s</ansi> from where it was stashed.":                        {verdictCorrect, true, false, true, "audit: stashed item retrieved -- item is not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/get.go:664)"},
	"usercommands/get.go|You pick up %d item(s).":                                                                           {verdictCorrect, true, false, true, "audit: floor sweep succeeds -- items are not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/get.go:64)"},
	"usercommands/get.go|You pick up <ansi fg=\"gold\">%d gold</ansi> from the <ansi fg=\"container\">%s</ans":              {verdictCorrect, true, false, true, "audit: container gold taken -- container is not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/get.go:520)"},
	"usercommands/get.go|You pick up <ansi fg=\"gold\">%d gold</ansi>.":                                                     {verdictCorrect, true, false, true, "audit: room gold taken -- gold is not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/get.go:592)"},
	"usercommands/get.go|You pick up the <ansi fg=\"itemname\">%s</ansi>.":                                                  {verdictCorrect, true, false, true, "audit: floor item retrieved -- item is not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/get.go:672)"},
	"usercommands/get.go|You remove a <ansi fg=\"itemname\">%s</ansi> from %s.":                                             {verdictCorrect, true, false, true, "audit: take item from pet -- pet is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/get.go:469)"},
	"usercommands/get.go|You take <ansi fg=\"gold\">%d gold</ansi> from the <ansi fg=\"mob-corpse\">%s</ansi>":              {verdictCorrect, true, false, true, "audit: corpse gold taken -- corpse is not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/get.go:394)"},
	"usercommands/get.go|You take the <ansi fg=\"itemname\">%s</ansi> from the <ansi fg=\"container\">%s</ans":              {verdictCorrect, true, false, true, "audit: container item taken -- container is not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/get.go:553)"},
	"usercommands/get.go|You take the <ansi fg=\"itemname\">%s</ansi> from the <ansi fg=\"mob-corpse\">%s</an":              {verdictCorrect, true, false, true, "audit: corpse item taken -- corpse is not a person (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/get.go:430)"},
	"usercommands/give.go|You count out <ansi fg=\"gold\">%d gold</ansi> and put it back in your pocket.":                   {verdictCorrect, true, false, true, "audit: gives gold to self -- self-targeted (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/give.go:105)"},
	"usercommands/give.go|You give <ansi fg=\"gold\">%d gold</ansi> to <ansi fg=\"username\">%s</ansi>.":                    {verdictCorrect, true, false, true, "audit: gives gold to a mob -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/give.go:176)"},
	"usercommands/give.go|You give the <ansi fg=\"item\">%s</ansi> to <ansi fg=\"mobname\">%s</ansi>.":                      {verdictCorrect, true, false, true, "audit: gives an item to a mob -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/give.go:227)"},
	"usercommands/give.go|You give the <ansi fg=\"itemname\">%s</ansi> to %s.":                                              {verdictCorrect, true, false, true, "audit: gives an item to a pet -- pet has no client (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/give.go:299)"},
	"usercommands/go.go|%s follows you.":                                                                                    {verdictCorrect, true, true, false, "a pet follows its owner through an exit; this line is a private notice to the owner, and the pet's own room-entry narration is handled separately elsewhere. Not part of the audit; read against source for this guard."},
	"usercommands/go.go|You're bumping into walls.":                                                                         {verdictCorrect, true, false, true, "audit: player bumps into a wall -- acts on no one (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/go.go:897)"},
	"usercommands/go.go|messaging.CategorySystem, playerMsg":                                                                {verdictCorrect, true, false, true, "audit: player unlocks a door -- acts on an exit, not a character (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/go.go:98)"},
	"usercommands/guild.go|<ansi fg=\"username\">%s</ansi> invites you to join <ansi fg=\"yellow-bold\">%s</ans":            {verdictCorrect, true, true, false, "a guild invite is sent privately to the invitee (actee); guild management has no room-facing component anywhere in this file."},
	"usercommands/guild.go|You have been removed from <ansi fg=\"yellow-bold\">%s</ansi>.":                                  {verdictCorrect, true, true, false, "a guild kick notifies the removed member privately (actee); same no-room-component reasoning as the invite."},
	"usercommands/inventory.go|<ansi fg=\"yellow\">%d grenade(s) have destabilized and dissolved into putrid resi":          {verdictCorrect, true, false, true, "audit: own grenades destabilize in the backpack -- self-directed accident (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/inventory.go:94)"},
	"usercommands/lock.go|You use a key to relock the <ansi fg=\"container\">%s</ansi>.":                                    {verdictCorrect, true, false, true, "audit: relocks a container with a key -- target is an object (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/lock.go:63)"},
	"usercommands/lock.go|You use a key to relock the <ansi fg=\"exit\">%s</ansi> lock.":                                    {verdictCorrect, true, false, true, "audit: relocks an exit with a key -- target is an object (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/lock.go:119)"},
	"usercommands/lock.go|You use your <ansi fg=\"item\">%s</ansi> to lock the <ansi fg=\"container\">%s</ansi":             {verdictCorrect, true, false, true, "audit: locks a container, keys it -- target is an object (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/lock.go:85)"},
	"usercommands/lock.go|You use your <ansi fg=\"item\">%s</ansi> to lock the <ansi fg=\"exit\">%s</ansi> exi":             {verdictCorrect, true, false, true, "audit: locks an exit, keys it -- target is an object (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/lock.go:141)"},
	"usercommands/look.go|": {verdictCorrect, true, false, true, "audit: looks at a backpack item -- target is an item; observer at :310 (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/look.go:301)"},
	"usercommands/look.go|<ansi fg=\"username\">%s</ansi> is examining the <ansi fg=\"noun\">%s</ansi>.":          {verdictCorrect, true, false, true, "examining a room noun, an object rather than a person; room.SendTextVisual broadcasts it, no actee possible."},
	"usercommands/look.go|<ansi fg=\"username\">%s</ansi> is looking at %s.":                                      {verdictCorrect, true, false, true, "looking at a mob; room.SendTextVisual broadcasts it (gated on not sneaking), no actee since the target is a mob."},
	"usercommands/look.go|You look at %s":                                                                         {verdictCorrect, true, false, true, "audit: looks at a pet -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/look.go:407)"},
	"usercommands/look.go|You look at the <ansi fg=\"%s\">%s</ansi>.":                                             {verdictCorrect, true, false, true, "audit: looks at a corpse -- corpse is not a live recipient (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/look.go:431)"},
	"usercommands/look.go|You peer toward the %s.":                                                                {verdictCorrect, true, false, true, "audit: peers toward a direction -- target is a direction (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/look.go:276)"},
	"usercommands/loot.go|You take <ansi fg=\"gold\">%d gold</ansi> from the <ansi fg=\"mob-corpse\">%s</ansi>":   {verdictCorrect, true, false, true, "audit: loot-all takes corpse gold -- corpse is not a person; observer deferred to :125 (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/loot.go:117)"},
	"usercommands/party.go|<ansi fg=\"username\">%s</ansi> declined the invitation.":                              {verdictCorrect, true, true, false, "the party leader is privately told an invite was declined (actee); same no-room-component reasoning as the invite."},
	"usercommands/party.go|You invited <ansi fg=\"username\">%s</ansi> to your party.":                            {verdictCorrect, true, true, false, "a party invite is sent privately to the invitee (actee); party management, like guild management, has no room-facing component."},
	"usercommands/pet.go|You name your pet: %s.":                                                                  {verdictCorrect, true, false, true, "naming your own pet; room.SendTextVisual broadcasts it, no actee since a pet has no client. Sibling pattern to the audited pet.go:68 row, which the audit already covers."},
	"usercommands/pet.go|You pet %s":                                                                              {verdictCorrect, true, false, true, "audit: pets a pet -- pet is a mob; the owner is another room observer (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/pet.go:68)"},
	"usercommands/picklock.go|":                                                                                   {verdictCorrect, true, false, true, "audit: lockpick breaks -- target is a lock (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/picklock.go:226)"},
	"usercommands/put.go|You place <ansi fg=\"gold\">%d gold</ansi> into the <ansi fg=\"container\">%s</ansi>":    {verdictCorrect, true, false, true, "audit: places gold into a container -- target is a container (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/put.go:112)"},
	"usercommands/rally.go|<ansi fg=\"cyan-bold\">You rally your allies with an inspiring shout that steadies":    {verdictCorrect, true, false, true, "audit: rally succeeds -- AoE buff, no single actee; members notified in the loop at :58 (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/rally.go:40)"},
	"usercommands/rally.go|<ansi fg=\"red-bold\">Your layered voice looses a thunderous war cry in the same b":    {verdictGap, true, false, true, "audit: Resonant Larynx war-cry fold -- the fold loop applies `AddCondition`/`AddBuff(79)` with no `SendText`, while the main loop at ~:56-60 notifies each member (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/rally.go:72)"},
	"usercommands/read.go|You look at <ansi fg=\"item\">%s</ansi>...":                                             {verdictCorrect, true, false, true, "audit: reads a held item -- self-targeted (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/read.go:48)"},
	"usercommands/remove.go|You remove your <ansi fg=\"item\">%s</ansi> and return it to your backpack.":          {verdictCorrect, true, false, true, "audit: removes an equipped item -- target is an item (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/remove.go:74)"},
	"usercommands/renameself.go|The world ripples briefly — you are now known as <ansi fg=\"username\">":          {verdictCorrect, true, false, true, "audit: rename succeeds -- self-directed (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/renameself.go:89)"},
	"usercommands/reply.go|messaging.CategoryWhisper, util.SplitStringNL(replyMsg, 80)":                           {verdictCorrect, true, true, false, "a whisper reply; the recipient is the actee, and a whisper is deliberately private, no room broadcast for any whisper in this codebase."},
	"usercommands/report.go|<ansi fg=\"whisper\"><ansi fg=\"username\">%s</ansi> reports to you: %s</ansi>":       {verdictCorrect, true, true, false, "audit: whisper-reports vitals to one player -- deliberately private; no room broadcast in this branch (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/report.go:79)"},
	"usercommands/report.go|You report: %s":                                                                       {verdictCorrect, true, false, true, "audit: reports vitals to the room -- broadcast with no specific target (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/report.go:86)"},
	"usercommands/sell.go|You sell %d <ansi fg=\"itemname\">%s</ansi> for <ansi fg=\"gold\">%d gold</ansi>.":      {verdictCorrect, true, false, true, "audit: sells multiple items -- merchant is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/sell.go:105)"},
	"usercommands/sell.go|You sell a <ansi fg=\"itemname\">%s</ansi> for <ansi fg=\"gold\">%d gold</ansi>.":       {verdictCorrect, true, false, true, "audit: sells a single item -- merchant is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/sell.go:100)"},
	"usercommands/shoot.go|messaging.CategorySurpriseAttack, surpriseShotRevealedText":                            {verdictCorrect, true, true, false, "this walk's grouping merges three sequential, unrelated private lines in the same function (the surprise-reveal notice, the engaged-aim cue, and a defended target's shortage note) into one event, because none of the intervening ifs has an else or terminates, a known imprecision noted in the guard's header comment. Each piece is independently a private, targeted message with no room component by design; there is no narration defect here."},
	"usercommands/show.go|You show the <ansi fg=\"item\">%s</ansi> to <ansi fg=\"mobname\">%s</ansi>.":            {verdictCorrect, true, false, true, "audit: shows an item to a mob -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/show.go:84)"},
	"usercommands/skill.cast.go|cast_started":                                                                     {verdictCorrect, true, false, true, "audit: spell cast begins -- fires before any target is affected; the target is reached by the room line (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/skill.cast.go:387)"},
	"usercommands/stand.go|You struggle to your feet!":                                                            {verdictCorrect, true, false, true, "audit: stands up from prone -- self-directed (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/stand.go:102)"},
	"usercommands/stash.go|You stash the <ansi fg=\"itemname\">%s</ansi>. To get it back, try <ansi fg=\"comma":   {verdictCorrect, true, false, true, "audit: stashes an item in the room -- self-directed, own item (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/stash.go:35)"},
	"usercommands/suicide.go|You are revived in a shower of magical sparks!":                                      {verdictCorrect, true, false, true, "audit: revive-on-death buff fires -- self-directed (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/suicide.go:46)"},
	"usercommands/target.go|You shift your focus to <ansi fg=\\\"mobname\\\">%s</ansi>!":                          {verdictCorrect, true, false, true, "audit: uncontested target switch onto a mob -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/target.go:191)"},
	"usercommands/taunt.go|messaging.CategorySystem, fmt.Sprintf(pullMsgs[util.Rand(len(pullMsgs))], target":      {verdictCorrect, true, false, true, "audit: taunt pulls mob aggro -- `AggroPulled` is only set when `target.MobInstanceId > 0`, so the actee is always a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/taunt.go:169)"},
	"usercommands/throw.go|<ansi fg=\"cyan-bold\">The blast shatters %s's concentration -- its spell collapse":    {verdictCorrect, true, false, true, "audit: grenade interrupts a mob's cast -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/throw.go:352)"},
	"usercommands/throw.go|<ansi fg=\"red-bold\">Your throw goes horribly wrong — the projectile detonates in":    {verdictCorrect, true, false, true, "audit: grenade fumbles and backfires -- self-directed backfire (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/throw.go:320)"},
	"usercommands/throw.go|<ansi fg=\"yellow-bold\">You hurl the <ansi fg=\"itemname\">%s</ansi> into the fray!":  {verdictCorrect, true, false, true, "audit: hurls a grenade into a room of mobs -- AoE against mobs, no single actee (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/throw.go:284)"},
	"usercommands/throw.go|messaging.CategoryDodge, string(triad.ToRoom), user.UserId":                            {verdictCorrect, true, false, true, "audit: grenade fully defended by a mob -- actee is a mob (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/throw.go:402)"},
	"usercommands/unlock.go|You use a key to unlock the <ansi fg=\"container\">%s</ansi>.":                        {verdictCorrect, true, false, true, "audit: unlocks a container with a key -- target is an object (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/unlock.go:58)"},
	"usercommands/unlock.go|You use a key to unlock the <ansi fg=\"exit\">%s</ansi> lock.":                        {verdictCorrect, true, false, true, "audit: unlocks an exit with a key -- target is an object (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/unlock.go:114)"},
	"usercommands/unlock.go|You use your <ansi fg=\"item\">%s</ansi> to unlock the <ansi fg=\"container\">%s</an": {verdictCorrect, true, false, true, "audit: unlocks a container, keys it -- target is an object (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/unlock.go:80)"},
	"usercommands/unlock.go|You use your <ansi fg=\"item\">%s</ansi> to unlock the <ansi fg=\"exit\">%s</ansi> e": {verdictCorrect, true, false, true, "audit: unlocks an exit, keys it -- target is an object (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/unlock.go:136)"},
	"usercommands/use.go|": {verdictCorrect, true, false, true, "audit: crafting container produces its item -- target is a container (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/use.go:62)"},
	"usercommands/use.go|You use the <ansi fg=\"itemname\">%s</ansi>.":                                          {verdictCorrect, true, false, true, "audit: uses a consumable -- target is the item (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/use.go:91)"},
	"usercommands/usercommands.go|<ansi fg=\"cyan\">You lose your concentration as you flee!</ansi>":            {verdictCorrect, true, false, true, "audit: loses concentration while fleeing a cast -- self-event (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/usercommands.go:430)"},
	"usercommands/warcry.go|<ansi fg=\"cyan-bold\">Your layered voice weaves a rallying cry into the same brea": {verdictGap, true, false, true, "audit: Resonant Larynx rally fold -- mirror of `rally.go:72`: the fold loop at :81-96 applies the buff with no `SendText`, while the main loop at :54-59 notifies each member (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/warcry.go:76)"},
	"usercommands/warcry.go|<ansi fg=\"red-bold\">You let out a thunderous warcry that ignites the fighting sp": {verdictCorrect, true, false, true, "audit: warcry buffs the party -- each member gets their own `SendText` at :58-59 (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/warcry.go:42)"},
	"usercommands/whisper.go|messaging.CategoryWhisper, util.SplitStringNL(whisperMsg, 80)":                     {verdictCorrect, true, true, false, "a whisper; the recipient is the actee, deliberately private with no room broadcast, same as reply.go."},
}

// TestNarrationSitesMatchViewpointAudit fails when this guard's own AST walk
// (narrationWalk) finds a candidate narration event (see
// narrationCandidateEvent for exactly what qualifies) that is not accounted
// for in narrationViewpointRegistry, AND fails when a registered site's key
// is no longer found by the walk at all -- because the event became
// complete (a gap was fixed), the call was deleted, or its literal changed
// enough that the fingerprint no longer matches.
//
// Both directions matter, same reasoning as TestEveryTextSurfaceIsRegistered
// above: a guard that only checks new sites rots the moment an existing one
// is fixed or rewritten, since the stale entry just sits there looking like
// coverage it no longer provides.
//
// If you are here because this test failed on a genuinely new incomplete
// site: read it against source, decide whether the missing viewpoint is
// correct or a gap (same standard the audit doc's "Confirmed defects"
// section applies -- is this a duplicated code path that copied an effect
// and dropped its narration?), and add a narrationViewpointRegistry entry
// with that verdict and a one-line reason.
//
// If you are here because a registered key vanished: find out why before
// removing the entry. `git log -p -- <file>` around the site is a good
// start. A verdictGap entry going stale is GOOD NEWS -- it means the gap was
// fixed -- but the audit doc should be updated to match before the entry is
// deleted, so the two do not drift onto different facts about the same
// history. A verdictCorrect entry going stale because the surrounding code
// was rewritten needs the same read-before-delete care as the YAML guard
// above.
func TestNarrationSitesMatchViewpointAudit(t *testing.T) {
	for _, root := range narrationGoRoots {
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("%s not found (test must run from the repo root): %v", root, err)
		}
	}

	found, err := narrationWalk()
	if err != nil {
		t.Fatalf("walk %v for narration sites: %v", narrationGoRoots, err)
	}
	if len(found) == 0 {
		t.Fatal("no candidate narration events found at all -- the walk is broken, not the data")
	}

	var unregistered []string
	for key, site := range found {
		if _, ok := narrationViewpointRegistry[key]; !ok {
			unregistered = append(unregistered, key+"  (line "+strconv.Itoa(site.line)+", "+narrationViewpointsLabel(site)+")")
		}
	}
	sort.Strings(unregistered)

	var stale []string
	for key := range narrationViewpointRegistry {
		if _, ok := found[key]; !ok {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)

	if len(unregistered) > 0 {
		t.Errorf("%d candidate narration event(s) are missing a viewpoint but are "+
			"not registered in narrationViewpointRegistry:\n  %s\n\n"+
			"Read each against source and rule it the way "+
			"docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md's "+
			"\"Confirmed defects\" section does: is the missing viewpoint correct "+
			"(the actee is a mob, the target is an object, the line is a private "+
			"refusal) or is this a duplicated code path that copied an effect and "+
			"dropped its narration (a gap)? Add a narrationViewpointRegistry entry "+
			"with that verdict, the viewpoints the event actually addresses, and a "+
			"one-line reason.",
			len(unregistered), strings.Join(unregistered, "\n  "))
	}

	if len(stale) > 0 {
		t.Errorf("%d narrationViewpointRegistry entr(y/ies) are no longer found "+
			"incomplete by this guard's walk:\n  %s\n\n"+
			"Either the event became complete (a gap was fixed -- update the audit "+
			"doc to match before removing the entry, so the two do not drift onto "+
			"different facts about the same history), the call was deleted, or its "+
			"literal changed enough that the fingerprint no longer matches. "+
			"`git log -p -- <file>` around the site is a good way to find out which. "+
			"Do not remove a stale entry just to make the test pass without "+
			"checking first.",
			len(stale), strings.Join(stale, "\n  "))
	}
}

// ---------------------------------------------------------------------------
// M2 literal freeze
//
// The messaging arc's M2 slice rewrites the 23 files below onto
// messaging.SendTrio. Their player-facing text is hand-rolled fmt.Sprintf
// literals, NOT a data store, so M1's goldens under
// internal/narration/testdata/stores/ do not cover a single line of it.
//
// This freezes the multiset of string literals in each file. A refactor that
// drops a line, alters a string, or reorders a pool changes the multiset and
// fails here. It says nothing about rendered output -- that is the honest
// limit, and it is the same tradeoff M1 made for its Group C inventory.
//
// Import declarations are skipped, because the migration legitimately adds an
// import to files that did not already have one.
//
// WHEN A FINGERPRINT LEGITIMATELY CHANGES (the seven output changes in the M2
// spec's section 4), update it IN THE SAME COMMIT as the text change, so the
// diff shows both together.
// ---------------------------------------------------------------------------

var m2FrozenFiles = map[string]string{
	"internal/usercommands/bash.go":     "1d8d25c0ceac07fb5fc976e15c8a26acf0614a398f69438c9ec94bc253513759",
	"internal/usercommands/drain.go":    "3a32f21ca0b34c14ef6cb625cd8b56263b6ef66ffece7038b362b494d2710257",
	"internal/usercommands/gore.go":     "cda7cd477eacf9f2cdb8d5f90803d09930755a74d875bd20f8de42326bbca3b4",
	"internal/usercommands/kick.go":     "f5dc3b8af7b4f702e3a8f7879b413d21d78b73d2c4a820088adfb322d57ca3ed",
	"internal/usercommands/maul.go":     "2abb23ead58ccd3af940cab05d01462df4899982e91f3440dbe1510d9790fb5b",
	"internal/usercommands/pounce.go":   "4cb8896a5627a3ea05cae952f4b48f1d4a7c73b8f8a4f0ab58b0adb8152b85d0",
	"internal/usercommands/rake.go":     "c192d3cde4c80035226f2af63cceacbb98f3d6cc5c142a4685e6d4b14d31fb5a",
	"internal/usercommands/throttle.go": "6678a1d6fc74f6b2619a4d0ed35c82c4c88c5f41c31d9ef6986731123ba62471",
	"internal/usercommands/trip.go":     "a72aba7fdbff9b09d84f27f419d0ef17b7b5eddd1586604b0515d4f6ac0023a2",
	"internal/usercommands/grapple.go":  "8e64bb48750d09f9581c6b7a0a3c0ec9eadcd4befbe94d9553b25d8c2c452a85",
	"internal/usercommands/shoot.go":    "d63942e7087292a898ce1730bcdf5b90f7c9dcaa08891a1a3af7ce4687d90376",
	"internal/usercommands/throw.go":    "44fc7829103b0dea6a1ccdba8787ceafa42519f78dccb4659e3e38b74ca98851",
	"internal/mobcommands/bash.go":      "fa5082a09245e01e30d5b967f687e443e0cedec587511234d5b0b41404146503",
	"internal/mobcommands/drain.go":     "e28eb92ae670bb746ae9009c14f4056984f8ca68230e9531c63d30348faff8eb",
	"internal/mobcommands/gore.go":      "c9f3a5c218732ebdc1bdcd141736443c98d9fa8c0b08e6f4d6a4e1f687f4a270",
	"internal/mobcommands/kick.go":      "622a3209b47a6ed5941a60900b57171083fd2116d6d7351e85a953dde45f073d",
	"internal/mobcommands/maul.go":      "8cdcc16a2fa36a5f9cf05ed52d9ef7a385809bc06f418b25caad92166ed2cc6e",
	"internal/mobcommands/pounce.go":    "4de98c9a45b40fb2a2dabd4dfb6033ebebe3573f3280e79bf46f7a1a9df7c866",
	"internal/mobcommands/rake.go":      "94973b45a77e548bf5a085a7005ce3ab83916c9a5b9f0cb18906d3045a64e2d2",
	"internal/mobcommands/throttle.go":  "caa10903bd759b2a0461a502a83fb0f59c2ebe80fe1eaeacfbba07f602886d0b",
	"internal/mobcommands/trip.go":      "42f5206e6a08bccd4673067e31882b7a5147524c971dd0f31f0403a5086808bd",
	"internal/mobcommands/grapple.go":   "7183a365181000f6ccec5b3500657ef0a758ed663aa8140e9487a2ffc9372df8",
	"internal/mobcommands/shoot.go":     "7ec3cee39749e1a422778894adf9b43b1be66efaedffd4f1f7b09f0171fd53e1",
}

// m2LiteralFingerprint returns a stable hash of every string literal in the
// file outside its import declarations.
func m2LiteralFingerprint(path string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return "", err
	}
	var lits []string
	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			continue
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			bl, ok := n.(*ast.BasicLit)
			if ok && bl.Kind == token.STRING {
				lits = append(lits, bl.Value)
			}
			return true
		})
	}
	sort.Strings(lits)
	sum := sha256.Sum256([]byte(strings.Join(lits, "\x00")))
	return hex.EncodeToString(sum[:]), nil
}

func TestM2LiteralsAreFrozen(t *testing.T) {
	var drift []string
	for path, want := range m2FrozenFiles {
		got, err := m2LiteralFingerprint(path)
		if err != nil {
			t.Fatalf("fingerprint %s (test must run from the repo root): %v", path, err)
		}
		if want == "" {
			drift = append(drift, path+"  RECORD: "+got)
			continue
		}
		if got != want {
			drift = append(drift, path+"\n    want "+want+"\n    got  "+got)
		}
	}
	sort.Strings(drift)
	if len(drift) > 0 {
		t.Errorf("%d M2-frozen file(s) have a different set of string literals "+
			"than recorded:\n  %s\n\n"+
			"During the M2 migration this means text was lost or altered by a "+
			"refactor that was supposed to move it unchanged -- find the "+
			"dropped or edited literal rather than re-recording the hash. "+
			"Re-record ONLY when the commit deliberately changes player-facing "+
			"text (the seven output changes in the M2 spec's section 4), and "+
			"do it in that same commit.",
			len(drift), strings.Join(drift, "\n  "))
	}
}

// TestM2FrozenFilesAllCarryText stops a vacuous entry being added to
// m2FrozenFiles.
//
// WHY THIS EXISTS. The first draft of the freeze listed both
// skill_move_defence.go files. Neither contains any player-facing text at all
// -- every line they speak comes from the authored defence store via
// combat.RenderChannelDefenceMessages -- so their only string literals are the
// two `""` in `== ""` comparisons. Their fingerprints were IDENTICAL to each
// other, and could not have moved no matter what the migration did to them.
// The freeze silently claimed to protect two files it could not protect.
//
// Those two are covered instead by M1's store goldens, extended by PR #112
// with 84 melee lines: internal/narration/testdata/stores/defense_messages.golden.
//
// A file whose text lives in a store does not belong in this list. This test
// makes that a build failure rather than a thing someone notices later.
func TestM2FrozenFilesAllCarryText(t *testing.T) {
	var vacuous []string
	for path := range m2FrozenFiles {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s (test must run from the repo root): %v", path, err)
		}
		substantive := 0
		for _, decl := range file.Decls {
			if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
				continue
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				bl, ok := n.(*ast.BasicLit)
				// len > 2 excludes `""` and backtick-empty: a quoted literal
				// carries two delimiter characters of its own.
				if ok && bl.Kind == token.STRING && len(bl.Value) > 2 {
					substantive++
				}
				return true
			})
		}
		if substantive == 0 {
			vacuous = append(vacuous, path)
		}
	}
	sort.Strings(vacuous)
	if len(vacuous) > 0 {
		t.Errorf("%d file(s) in m2FrozenFiles carry no string literal with any "+
			"content, so their fingerprint cannot move and the freeze protects "+
			"nothing:\n  %s\n\n"+
			"Remove them. A file whose player-facing text comes from an authored "+
			"store is guarded by that store's golden, not by this literal freeze.",
			len(vacuous), strings.Join(vacuous, "\n  "))
	}
}

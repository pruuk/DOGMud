package guards

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// secondPersonWord matches the reader being addressed. "you're" matches too:
// the apostrophe is a word boundary.
var secondPersonWord = regexp.MustCompile(`(?i)\b(you|your|yours|yourself|yourselves)\b`)

// quotedSpeech matches a double-quoted span: speech, which the guard skips.
var quotedSpeech = regexp.MustCompile(`"[^"]*"`)

// secondPersonEmoteAllowlist names the deliberate exceptions, keyed
// "<list> | <owner file key> | <emote text>". Each needs a reason. It is empty:
// every shipped case was reworded to "its foe" in #515, including the Ascended
// species line the issue floated as a room-wide address.
var secondPersonEmoteAllowlist = map[string]string{}

// TestMobFightEmotesDoNotAddressEveryoneAsYou is the #515 guard. A mob's
// `emote` command goes to everyone in the room through actions.SendSeen, so a
// fight emote reading "kicks at your legs" tells two onlookers who are not in
// the fight that they are being kicked. The engine has no target-aware emote
// form (an emote is one fixed string; only `sayto` and its kin resolve a
// target), so combatcommands and angrycommands, which fire on a mob that is
// fighting a specific opponent, must speak of "its foe" in the third person.
//
// idlecommands are not covered: an idle emote has no opponent, it is room
// ambience that every reader may take personally ("watches you from a crack").
func TestMobFightEmotesDoNotAddressEveryoneAsYou(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	base := configs.GetFilePathsConfig().DataFiles.String()

	mobPath := base + `/mobs`
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](mobPath)
	if err != nil {
		t.Fatalf("loading mob templates from %s: %v", mobPath, err)
	}

	speciesPath := base + `/species`
	allSpecies, err := fileloader.LoadAllFlatFiles[int, *species.Species](speciesPath)
	if err != nil {
		t.Fatalf("loading species from %s: %v", speciesPath, err)
	}

	checked := 0
	check := func(list, owner string, commands []string) {
		n, bad := secondPersonEmotes(commands)
		checked += n
		for _, part := range bad {
			key := list + ` | ` + owner + ` | ` + part
			if _, ok := secondPersonEmoteAllowlist[key]; ok {
				continue
			}
			t.Errorf("%s of %s addresses every onlooker as you: %q "+
				"(an emote reaches the whole room; write the opponent as \"its foe\")",
				list, owner, part)
		}
	}

	for id, m := range templates {
		owner := m.Character.Name + ` (mob ` + strconv.Itoa(id) + `)`
		check(`combatcommands`, owner, m.CombatCommands)
		check(`angrycommands`, owner, m.AngryCommands)
	}
	for id, s := range allSpecies {
		check(`angrycommands`, s.Name+` (species `+strconv.Itoa(id)+`)`, s.AngryCommands)
	}

	if checked < 50 {
		t.Fatalf("only %d fight emotes were inspected; the loader or field names drifted and the guard is blind", checked)
	}
}

// secondPersonEmotes splits each command on `;`, counts the emote parts, and
// returns those that address the reader as you.
func secondPersonEmotes(commands []string) (checked int, bad []string) {
	for _, c := range commands {
		for _, part := range strings.Split(c, `;`) {
			part = strings.TrimSpace(part)
			if part != `emote` && !strings.HasPrefix(part, `emote `) {
				continue
			}
			checked++
			// Quoted speech inside an emote is the mob talking, as `say`
			// is, and may address its listener ("Stronger than you
			// looked."); only the narration around it is read.
			if secondPersonWord.MatchString(quotedSpeech.ReplaceAllString(part, ``)) {
				bad = append(bad, part)
			}
		}
	}
	return checked, bad
}

// behaviorEventHeld classifies every event a behaviour tree under
// behaviors/** reacts to. true: a room-wide line sent then is held to the
// rule, because the event concerns one opponent, entrant, giver, asker or
// wearer, or a fight, and every other occupant reads the line too. false:
// ambience with no one in particular addressed (mob, room and item idle),
// exempt as mob idlecommands are, where "watches you" reads right to every
// reader. A node outside any event (`""`) runs on all of them and is held.
// An event missing from this map fails the guard, so a new event is
// classified on purpose rather than skipped.
var behaviorEventHeld = map[string]bool{
	``:                       true,
	`mob_combat_round`:       true,
	`mob_hurt`:               true,
	`packmate_hurt`:          true,
	`player_attack_rejected`: true, // a player just tried to hit this mob
	`heard_callforhelp`:      true,
	`mob_die`:                true,
	`mob_flee`:               true,
	`player_enter`:           true, // one entrant; the rest of the room reads it too
	`player_give`:            true,
	`player_topic`:           true, // one player chose a talk topic; the room reads it too
	`room_command`:           true,
	`room_enter`:             true,
	`on_kill`:                true,
	`on_hit`:                 true,
	`on_block`:               true,
	`on_grapple`:             true,
	`on_spell_hit`:           true,
	`on_equip`:               true,
	`on_unequip`:             true,
	`on_hunger_feeding`:      true,
	`mob_idle`:               false,
	`room_idle`:              false,
	`item_idle`:              false,
}

// behaviorRoomLines returns the room-wide lines a behaviour node sends, each
// as an emote command so secondPersonEmotes reads them alike: `emote` and
// `mob_emote` send "emote <text>"; `send_room_text` sends its text to every
// occupant and `respond` its room_text to everyone but the asker, so both
// are read as emotes too; `command` and `command_mob` send `cmd`, and
// `command_best_of` one of `cmds` (a YAML list or, as generic_fighter writes
// it, a comma-separated string). Speech (`say`, `mob_say`, an item's
// `speak`) is quoted dialogue and may address its listeners; it is not read.
func behaviorRoomLines(n behaviortree.NodeDef) []string {
	str := func(key string) string {
		s, _ := n.Params[key].(string)
		return s
	}
	switch n.Do {
	case `emote`, `mob_emote`, `send_room_text`:
		if text := str(`text`); text != `` {
			return []string{`emote ` + text}
		}
	case `respond`:
		if text := str(`room_text`); text != `` {
			return []string{`emote ` + text}
		}
	case `command`, `command_mob`:
		if cmd := str(`cmd`); cmd != `` {
			return []string{cmd}
		}
	case `command_best_of`:
		var out []string
		switch v := n.Params[`cmds`].(type) {
		case []any:
			for _, el := range v {
				if s, ok := el.(string); ok {
					out = append(out, s)
				}
			}
		case string:
			for _, s := range strings.Split(v, `,`) {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	}
	return nil
}

// behaviorHeldLines walks a tree, carrying the nearest event down, and
// returns the room-wide lines sent under a held event. Unclassified events
// are returned so the caller can fail on them. heldNodes counts the nodes
// visited under a held event, so a blind walk is caught.
func behaviorHeldLines(root behaviortree.NodeDef) (cmds []string, unclassified []string, heldNodes int) {
	var walk func(n behaviortree.NodeDef, event string)
	walk = func(n behaviortree.NodeDef, event string) {
		if n.Event != `` {
			event = n.Event
			if _, ok := behaviorEventHeld[event]; !ok {
				unclassified = append(unclassified, event)
			}
		}
		if behaviorEventHeld[event] {
			heldNodes++
			cmds = append(cmds, behaviorRoomLines(n)...)
		}
		for _, c := range n.Children {
			walk(c, event)
		}
		if n.Child != nil {
			walk(*n.Child, event)
		}
	}
	walk(root, ``)
	return cmds, unclassified, heldNodes
}

// TestBehaviorRoomLinesDoNotAddressEveryoneAsYou extends the #515 guard to
// every behaviour tree under behaviors/** (archetypes, per-mob, room and
// item trees). They carry no combatcommands key; their room-wide lines are
// `emote`, `send_room_text`, `respond` room_text and emote commands. Under a
// held event (see behaviorEventHeld) such a line reaches every occupant, so
// it must not address the reader as you: write the one it concerns in the
// third person ("its foe", "the newcomer").
func TestBehaviorRoomLinesDoNotAddressEveryoneAsYou(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	root := filepath.Join(configs.GetFilePathsConfig().DataFiles.String(), `behaviors`)
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, `.yaml`) {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil || len(paths) < 70 {
		t.Fatalf("found %d behaviour files under %s (err %v); the path drifted and the guard is blind", len(paths), root, err)
	}

	heldNodes := 0
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		def, err := behaviortree.LoadTreeDef(p)
		if err != nil {
			t.Fatalf("parsing %s: %v", rel, err)
		}
		cmds, unclassified, n := behaviorHeldLines(def.Tree)
		heldNodes += n
		for _, ev := range unclassified {
			t.Errorf("%s reacts to event %q, which behaviorEventHeld does not classify; "+
				"add it, held unless it is ambience addressed to no one", rel, ev)
		}
		_, bad := secondPersonEmotes(cmds)
		for _, part := range bad {
			key := `behavior | ` + rel + ` | ` + part
			if _, ok := secondPersonEmoteAllowlist[key]; ok {
				continue
			}
			t.Errorf("behavior %s sends a room-wide line addressing every onlooker as you: %q "+
				"(every occupant reads it; write the one it concerns in the third person)",
				rel, part)
		}
	}
	if heldNodes < 200 {
		t.Fatalf("only %d nodes were walked under a held event; the tree shape or event names drifted and the guard is blind", heldNodes)
	}
}

// TestBehaviorRoomLineProbeCanFail proves the walk catches what it is for:
// planted lines under held events, nested in a decorator, a comma-string
// cmds, a send_room_text and a respond room_text, are found; the same line
// under mob_idle is not.
func TestBehaviorRoomLineProbeCanFail(t *testing.T) {
	const planted = `
tree:
  type: selector
  children:
    - type: sequence
      event: mob_hurt
      children:
        - type: action
          do: emote
          text: "kicks at your legs."
        - type: decorator
          mod: invert
          child:
            type: action
            do: command_best_of
            cmds: bash,emote snaps at you
    - type: action
      event: player_enter
      do: emote
      text: "glances up as you enter."
    - type: action
      event: room_enter
      do: send_room_text
      text: "The floor creaks under your weight."
    - type: action
      event: player_topic
      do: respond
      room_text: "{actor} nods at you."
    - type: action
      event: mob_die
      do: send_room_text
      text: 'The old man looks up. "Stronger than you looked," he says.'
    - type: action
      event: mob_idle
      do: emote
      text: "watches you from a crack."
`
	var def behaviortree.TreeDef
	if err := yaml.Unmarshal([]byte(planted), &def); err != nil {
		t.Fatalf("parsing the planted tree: %v", err)
	}
	cmds, unclassified, _ := behaviorHeldLines(def.Tree)
	if len(unclassified) != 0 {
		t.Fatalf("planted tree uses only classified events, got %q", unclassified)
	}
	_, bad := secondPersonEmotes(cmds)
	want := []string{`emote kicks at your legs.`, `emote snaps at you`, `emote glances up as you enter.`,
		`emote The floor creaks under your weight.`, `emote {actor} nods at you.`}
	if strings.Join(bad, `|`) != strings.Join(want, `|`) {
		t.Fatalf("probe found %q, want %q (the idle emote and the quoted speech must not be flagged)", bad, want)
	}
}

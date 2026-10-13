package main

import (
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
			if secondPersonWord.MatchString(part) {
				bad = append(bad, part)
			}
		}
	}
	return checked, bad
}

// archetypeFightEvents classifies every event an archetype tree reacts to.
// true: the mob has a specific opponent when the event fires, so an emote
// there is a fight emote and is held to the rule. false: no opponent (idle
// ambience, greetings, gifts, a call for help heard from afar), exempt as
// idlecommands are. An event missing from this map fails the guard, so a
// new event is classified on purpose rather than skipped.
var archetypeFightEvents = map[string]bool{
	`mob_combat_round`:       true,
	`mob_hurt`:               true,
	`packmate_hurt`:          true,
	`player_attack_rejected`: true, // a player just tried to hit this mob
	`mob_idle`:               false,
	`player_enter`:           false,
	`player_give`:            false,
	`heard_callforhelp`:      false,
}

// archetypeCommands returns the command strings a behaviour node sends
// through mob.Command, in the forms the actions read them: `emote` and
// `mob_emote` send "emote <text>", `command` and `command_mob` send `cmd`,
// and `command_best_of` sends one of `cmds`. `cmds` is read as a YAML list
// or, as generic_fighter writes it, a comma-separated string.
func archetypeCommands(n behaviortree.NodeDef) []string {
	str := func(key string) string {
		s, _ := n.Params[key].(string)
		return s
	}
	switch n.Do {
	case `emote`, `mob_emote`:
		if text := str(`text`); text != `` {
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

// archetypeFightCommands walks a tree, carrying the nearest event down, and
// returns the commands sent under a fight event. Unclassified events are
// returned so the caller can fail on them. fightNodes counts the nodes
// visited under a fight event, so a blind walk is caught.
func archetypeFightCommands(root behaviortree.NodeDef) (cmds []string, unclassified []string, fightNodes int) {
	var walk func(n behaviortree.NodeDef, event string)
	walk = func(n behaviortree.NodeDef, event string) {
		if n.Event != `` {
			event = n.Event
			if _, ok := archetypeFightEvents[event]; !ok {
				unclassified = append(unclassified, event)
			}
		}
		if archetypeFightEvents[event] {
			fightNodes++
			cmds = append(cmds, archetypeCommands(n)...)
		}
		for _, c := range n.Children {
			walk(c, event)
		}
		if n.Child != nil {
			walk(*n.Child, event)
		}
	}
	walk(root, ``)
	return cmds, unclassified, fightNodes
}

// TestArchetypeFightEmotesDoNotAddressEveryoneAsYou extends the #515 guard to
// the behaviour archetypes (behaviors/archetypes/*.yaml). They carry no
// combatcommands key; their fight-time speech is an `emote`, `command` or
// `command_best_of` node under a fight event, and the same rule holds: an
// emote reaches the whole room, so it must not address the reader as you.
// As of this writing no archetype emotes in a fight, so the guard pins the
// walk itself (fight nodes visited) and proves its probe on a planted tree.
func TestArchetypeFightEmotesDoNotAddressEveryoneAsYou(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	dir := filepath.Join(configs.GetFilePathsConfig().DataFiles.String(), `behaviors`, `archetypes`)
	paths, err := filepath.Glob(filepath.Join(dir, `*.yaml`))
	if err != nil || len(paths) < 20 {
		t.Fatalf("found %d archetype files under %s (err %v); the path drifted and the guard is blind", len(paths), dir, err)
	}

	fightNodes := 0
	for _, p := range paths {
		def, err := behaviortree.LoadTreeDef(p)
		if err != nil {
			t.Fatalf("parsing %s: %v", p, err)
		}
		cmds, unclassified, n := archetypeFightCommands(def.Tree)
		fightNodes += n
		for _, ev := range unclassified {
			t.Errorf("%s reacts to event %q, which archetypeFightEvents does not classify; "+
				"add it as a fight event (an opponent is present) or not", filepath.Base(p), ev)
		}
		_, bad := secondPersonEmotes(cmds)
		for _, part := range bad {
			key := `archetype | ` + filepath.Base(p) + ` | ` + part
			if _, ok := secondPersonEmoteAllowlist[key]; ok {
				continue
			}
			t.Errorf("archetype %s emotes in a fight addressing every onlooker as you: %q "+
				"(an emote reaches the whole room; write the opponent as \"its foe\")",
				filepath.Base(p), part)
		}
	}
	if fightNodes < 50 {
		t.Fatalf("only %d nodes were walked under a fight event; the tree shape or event names drifted and the guard is blind", fightNodes)
	}
}

// TestArchetypeFightEmoteProbeCanFail proves the walk catches what it is for:
// a planted fight emote nested under a fight event, in both node forms and a
// comma-string cmds, is found; the same emote under mob_idle is not.
func TestArchetypeFightEmoteProbeCanFail(t *testing.T) {
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
      event: mob_idle
      do: emote
      text: "watches you from a crack."
`
	var def behaviortree.TreeDef
	if err := yaml.Unmarshal([]byte(planted), &def); err != nil {
		t.Fatalf("parsing the planted tree: %v", err)
	}
	cmds, unclassified, _ := archetypeFightCommands(def.Tree)
	if len(unclassified) != 0 {
		t.Fatalf("planted tree uses only classified events, got %q", unclassified)
	}
	_, bad := secondPersonEmotes(cmds)
	want := []string{`emote kicks at your legs.`, `emote snaps at you`}
	if strings.Join(bad, `|`) != strings.Join(want, `|`) {
		t.Fatalf("probe found %q, want %q (the idle emote must not be flagged)", bad, want)
	}
}

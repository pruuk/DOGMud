package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// familyReads are the reads that decide a beast move by attack family
// instead of by Species.BeastMoves (spec 2026-10-16 #541 section 5).
var familyReads = map[string]bool{
	"SpeciesIsFanged": true, "SpeciesIsClawed": true, "SpeciesIsHorned": true,
	"SpeciesIsQuadrupedPredator": true, "SpeciesHasLifeDrain": true,
	"SpeciesNaturalAttack": true, "NaturalAttack": true,
}

// familyReadsIn names each family read inside a gate function of src.
func familyReadsIn(t *testing.T, src string, isGate func(string) bool) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
	require.NoError(t, err)
	var out []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !isGate(fn.Name.Name) {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && familyReads[id.Name] {
				out = append(out, fn.Name.Name+" reads "+id.Name)
			}
			return true
		})
	}
	return out
}

// Every beast-move gate reads the species list. The three sync points:
// CanUse* in ai.go, CommandIsReady, and each Execute* entry.
func TestBeastMoveGatesNeverReadTheAttackFamily(t *testing.T) {
	beastGate := func(name string) bool {
		for _, m := range []string{"Rake", "Maul", "Pounce", "Gore", "Throttle", "Hamstring", "Drain", "Smother"} {
			if name == "CanUse"+m || name == "Execute"+m {
				return true
			}
		}
		return name == "CommandIsReady"
	}
	files := []string{"internal/combat/ai.go", "internal/actions/command_readiness.go"}
	actionFiles, err := filepath.Glob("internal/actions/combat_*.go")
	require.NoError(t, err)
	require.NotEmpty(t, actionFiles)
	files = append(files, actionFiles...)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		for _, hit := range familyReadsIn(t, string(raw), beastGate) {
			t.Errorf("%s: %s; beast moves come from Species.BeastMoves", f, hit)
		}
	}
}

func TestFamilyReadsIn_CanFail(t *testing.T) {
	src := "package p\nfunc CanUseRake(c int) bool { return SpeciesIsClawed(c) }\n" +
		"func ExecuteHamstring(a int) { if sp.NaturalAttack != 1 {} }\nfunc other() { SpeciesIsFanged(1) }\n"
	got := familyReadsIn(t, src, func(n string) bool { return n == "CanUseRake" || n == "ExecuteHamstring" })
	require.Len(t, got, 2, "two gate reads and none from a non-gate: %v", got)
}

// scriptedBeastMoveFindings names each scripted beast move the issuing mob's
// species does not list: the action gate would refuse it in silence every
// time. A source with no entry in mobSpecies (a room or item tree's own
// command, or a command_mob naming a mob id no file defines) has no species
// that could list the move, so any beast move it issues is a finding too.
func scriptedBeastMoveFindings(mobCommands map[string][]string, mobSpecies map[string]int, lists map[int][]string) []string {
	beast := map[string]bool{"rake": true, "maul": true, "pounce": true, "gore": true,
		"throttle": true, "hamstring": true, "drain": true, "smother": true}
	var out []string
	for source, cmds := range mobCommands {
		for _, c := range cmds {
			words := strings.Fields(c) // an empty command is a scripted idle round
			if len(words) == 0 || !beast[words[0]] {
				continue
			}
			verb := words[0]
			speciesId, ok := mobSpecies[source]
			if !ok {
				out = append(out, source+": issues "+verb+" with no mob whose species could list it")
				continue
			}
			listed := false
			for _, m := range lists[speciesId] {
				listed = listed || m == verb
			}
			if !listed {
				out = append(out, source+": scripts "+verb+" but species does not list it")
			}
		}
	}
	sort.Strings(out)
	return out
}

// treeCommands returns the mob commands a behaviour-tree file issues: as its
// own mob (`do: command` with `cmd`, `do: command_best_of` with `cmds`) and
// to another mob by id (`do: command_mob` with `mob_id` and `cmd`). These
// are the three tree actions that run a literal command string
// (internal/behaviortree/actions_mob.go).
func treeCommands(raw []byte) (own []string, toMob map[int][]string, err error) {
	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, nil, err
	}
	toMob = map[int][]string{}
	var walk func(v any)
	walk = func(v any) {
		switch n := v.(type) {
		case map[string]any:
			switch n["do"] {
			case "command":
				if s, ok := n["cmd"].(string); ok {
					own = append(own, s)
				}
			case "command_best_of":
				list, _ := n["cmds"].([]any)
				for _, el := range list {
					if s, ok := el.(string); ok {
						own = append(own, s)
					}
				}
			case "command_mob":
				id, _ := n["mob_id"].(int)
				if s, ok := n["cmd"].(string); ok {
					toMob[id] = append(toMob[id], s)
				}
			}
			for _, child := range n {
				walk(child)
			}
		case []any:
			for _, child := range n {
				walk(child)
			}
		}
	}
	walk(doc)
	return own, toMob, nil
}

// scriptedMob is one dogmud mob file as the scripted-move guard reads it.
type scriptedMob struct {
	MobId             int      `yaml:"mobid"`
	BehaviorArchetype string   `yaml:"behavior_archetype"`
	CombatCommands    []string `yaml:"combatcommands"`
	Character         struct {
		SpeciesId int `yaml:"speciesid"`
	} `yaml:"character"`
}

// addTreeCommands files a tree's commands under each mob that runs them:
// owners are the mobs whose own commands they are (none for a room or item
// tree), and command_mob commands go to the mob with that id.
func addTreeCommands(cmds map[string][]string, species map[string]int, tree string,
	own []string, toMob map[int][]string, owners []string, mobById map[int]string, mobSpecies map[string]int) {
	if len(owners) == 0 && len(own) > 0 {
		cmds[tree] = append(cmds[tree], own...) // no mob: any beast move is a finding
	}
	for _, mobFile := range owners {
		key := tree + " (run by " + mobFile + ")"
		cmds[key], species[key] = append(cmds[key], own...), mobSpecies[mobFile]
	}
	for id, list := range toMob {
		key := fmt.Sprintf("%s (command_mob %d)", tree, id)
		cmds[key] = append(cmds[key], list...)
		if mobFile, ok := mobById[id]; ok {
			species[key] = mobSpecies[mobFile]
		}
	}
}

func TestScriptedBeastMovesAreInTheSpeciesList(t *testing.T) {
	lists := map[int][]string{}
	spFiles, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "species", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, spFiles)
	for _, f := range spFiles {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		var sp struct {
			Id    int      `yaml:"speciesid"`
			Moves []string `yaml:"beast_moves"`
		}
		require.NoError(t, yaml.Unmarshal(raw, &sp), f)
		lists[sp.Id] = sp.Moves
	}

	// Mob files: combatcommands, species, id and archetype. A file the guard
	// cannot decode fails the test: its scripted moves would go unchecked.
	cmds, species := map[string][]string{}, map[string]int{}
	mobById, byArchetype := map[int]string{}, map[string][]string{}
	require.NoError(t, filepath.WalkDir(filepath.Join("_datafiles", "world", "dogmud", "mobs"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".yaml" {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var m scriptedMob
		if err := yaml.Unmarshal(raw, &m); err != nil {
			t.Errorf("%s: cannot decode mob file, so its scripted moves go unchecked: %v", p, err)
			return nil
		}
		cmds[p], species[p] = m.CombatCommands, m.Character.SpeciesId
		mobById[m.MobId] = p
		if m.BehaviorArchetype != "" {
			byArchetype[m.BehaviorArchetype] = append(byArchetype[m.BehaviorArchetype], p)
		}
		return nil
	}))
	require.Greater(t, len(cmds), 100, "the mob walk found too few files to mean anything")
	mobSpecies := maps.Clone(species)

	// Behaviour trees: archetypes/<name>.yaml runs as every mob declaring that
	// behavior_archetype; <zone>/<mobId>-<name>.yaml runs as that mob;
	// rooms/ and items/ trees have no mob of their own. command_mob goes to
	// the mob named by its mob_id wherever it appears.
	behaviors := filepath.Join("_datafiles", "world", "dogmud", "behaviors")
	trees := 0
	require.NoError(t, filepath.WalkDir(behaviors, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".yaml" {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		own, toMob, err := treeCommands(raw)
		if err != nil {
			t.Errorf("%s: cannot decode behaviour tree, so its commands go unchecked: %v", p, err)
			return nil
		}
		trees++
		rel, err := filepath.Rel(behaviors, p)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		var owners []string
		switch {
		case parts[0] == "archetypes":
			owners = byArchetype[strings.TrimSuffix(parts[len(parts)-1], ".yaml")]
		case parts[0] == "rooms" || parts[0] == "items":
		default:
			var mobId int
			if _, err := fmt.Sscanf(parts[len(parts)-1], "%d-", &mobId); err == nil {
				if f, ok := mobById[mobId]; ok {
					owners = []string{f}
				}
			}
		}
		addTreeCommands(cmds, species, p, own, toMob, owners, mobById, mobSpecies)
		return nil
	}))
	require.Greater(t, trees, 50, "the behaviour-tree walk found too few files to mean anything")

	for _, f := range scriptedBeastMoveFindings(cmds, species, lists) {
		t.Error(f)
	}
}

func TestScriptedBeastMoveFindings_CanFail(t *testing.T) {
	got := scriptedBeastMoveFindings(map[string][]string{"wolf.yaml": {"rake", "", "charge", "emote howls"}},
		map[string]int{"wolf.yaml": 2}, map[int][]string{2: {"maul"}})
	require.Equal(t, []string{"wolf.yaml: scripts rake but species does not list it"}, got)

	// The behaviour-tree path: a nested command, a command_best_of list and a
	// command_mob are all found, and each is filed under the mob that runs it.
	tree := []byte(`tree:
  type: selector
  children:
    - type: sequence
      children:
        - type: action
          do: command
          cmd: maul
    - type: action
      do: command_best_of
      cmds: [bash, gore]
    - type: action
      do: command_mob
      mob_id: 7
      cmd: rake
    - type: action
      do: command_mob
      mob_id: 8
      cmd: hamstring
`)
	own, toMob, err := treeCommands(tree)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"maul", "bash", "gore"}, own)
	require.Equal(t, map[int][]string{7: {"rake"}, 8: {"hamstring"}}, toMob)

	mobSpecies := map[string]int{"wolf.yaml": 2, "cat.yaml": 11}
	mobById := map[int]string{7: "cat.yaml"}
	lists := map[int][]string{2: {"maul"}, 11: {"pounce"}}
	cmds, species := map[string][]string{}, map[string]int{}
	addTreeCommands(cmds, species, "pack.yaml", own, toMob, []string{"wolf.yaml"}, mobById, mobSpecies)
	addTreeCommands(cmds, species, "room.yaml", []string{"throttle"}, nil, nil, mobById, mobSpecies)
	require.Equal(t, []string{
		"pack.yaml (command_mob 7): scripts rake but species does not list it",
		"pack.yaml (command_mob 8): issues hamstring with no mob whose species could list it",
		"pack.yaml (run by wolf.yaml): scripts gore but species does not list it",
		"room.yaml: issues throttle with no mob whose species could list it",
	}, scriptedBeastMoveFindings(cmds, species, lists))
}

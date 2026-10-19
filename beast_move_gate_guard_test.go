package main

import (
	"go/ast"
	"go/parser"
	"go/token"
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

// scriptedBeastMoveFindings names each scripted combatcommands beast move
// the mob's species does not list: the action gate would refuse it in
// silence every time.
func scriptedBeastMoveFindings(mobCommands map[string][]string, mobSpecies map[string]int, lists map[int][]string) []string {
	beast := map[string]bool{"rake": true, "maul": true, "pounce": true, "gore": true,
		"throttle": true, "hamstring": true, "drain": true, "smother": true}
	var out []string
	for file, cmds := range mobCommands {
		for _, c := range cmds {
			words := strings.Fields(c) // an empty command is a scripted idle round
			if len(words) == 0 || !beast[words[0]] {
				continue
			}
			verb := words[0]
			listed := false
			for _, m := range lists[mobSpecies[file]] {
				listed = listed || m == verb
			}
			if !listed {
				out = append(out, file+": scripts "+verb+" but species does not list it")
			}
		}
	}
	sort.Strings(out)
	return out
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
	cmds, species := map[string][]string{}, map[string]int{}
	require.NoError(t, filepath.WalkDir(filepath.Join("_datafiles", "world", "dogmud", "mobs"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".yaml" {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var m struct {
			CombatCommands []string `yaml:"combatcommands"`
			Character      struct {
				SpeciesId int `yaml:"speciesid"`
			} `yaml:"character"`
		}
		if yaml.Unmarshal(raw, &m) == nil {
			cmds[p], species[p] = m.CombatCommands, m.Character.SpeciesId
		}
		return nil
	}))
	require.Greater(t, len(cmds), 100, "the mob walk found too few files to mean anything")
	for _, f := range scriptedBeastMoveFindings(cmds, species, lists) {
		t.Error(f)
	}
}

func TestScriptedBeastMoveFindings_CanFail(t *testing.T) {
	got := scriptedBeastMoveFindings(map[string][]string{"wolf.yaml": {"rake", "", "charge", "emote howls"}},
		map[string]int{"wolf.yaml": 2}, map[int][]string{2: {"maul"}})
	require.Equal(t, []string{"wolf.yaml: scripts rake but species does not list it"}, got)
}

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"gopkg.in/yaml.v2"
)

// dlgFile is one converted dialogue file and the zone folder it sits in.
type dlgFile struct {
	Folder string
	File   *dialogue.DialogueFile
}

// treeFile is one behavior tree. MobId is 0 for a room tree.
type treeFile struct {
	Path  string // relative to the data root, forward slashes
	Zone  string
	MobId int
	Def   behaviortree.TreeDef
}

// Input is everything the gate walk reads. It is built from real files by
// buildInput; tests build it from testdata fixtures the same way.
type Input struct {
	Files  []dlgFile
	Quests []quests.Quest
	Trees  []treeFile
	// Retired lists quest ids that are deliberately not startable (owner
	// ruling R17). Characters already holding their tokens must still be
	// able to finish, so the walk seeds each one with a holder at its start
	// step and keeps every other check on.
	Retired map[int]bool
	// WorldItems maps an item id to the places in the world data that
	// supply it (a mob's shop or gear, a room spawn or container, a biome,
	// a recipe output). Those sources are ASSUMED obtainable.
	WorldItems map[int][]string
	// ItemTokens maps an item id to the quest token its spec grants when a
	// player gains it (ItemSpec.QuestToken, hooks.ItemOwnership_CheckItemQuests).
	ItemTokens map[int]string
	// RoomZones maps a room id to its zone folder under rooms/.
	RoomZones map[int]string
	// CodeGrants are quest tokens Go code grants by literal, outside any
	// data file (character creation in internal/usercommands/start.go).
	CodeGrants []codeGrant
	// Plan is what the converter report says Tasks 16 and 17 will do to the
	// quest triggers and behavior trees. Nil when no report is applied.
	Plan *convPlan
}

// codeGrant is one `QuestToken: "<id>-<step>"` literal in Go source.
type codeGrant struct {
	Token string
	Where string // file:line, relative to the repository
}

var codeGrantLiteral = regexp.MustCompile(`QuestToken:\s*"(\d+-[A-Za-z0-9_]+)"`)

// readCodeGrants scans the repository's non-test Go files under internal/
// and modules/ for quest tokens granted by literal.
func readCodeGrants(repo string) ([]codeGrant, error) {
	var out []codeGrant
	for _, dir := range []string{"internal", "modules"} {
		root := filepath.Join(repo, dir)
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(repo, p)
			for i, line := range strings.Split(string(b), "\n") {
				if m := codeGrantLiteral.FindStringSubmatch(line); m != nil {
					out = append(out, codeGrant{Token: m[1], Where: fmt.Sprintf("%s:%d", filepath.ToSlash(rel), i+1)})
				}
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return out, nil
}

// repoRoot walks up from dir to the folder holding go.mod, or "".
func repoRoot(dir string) string {
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// convPlan is read from the converter's report.md.
type convPlan struct {
	// DeleteTriggers: quest id -> trigger index (counted over all of the
	// quest's triggers from 0) that Task 16 deletes.
	DeleteTriggers map[int]map[int]bool
	// Branches: behavior file path (relative to the data root) -> 1-based
	// player_ask branch number -> what Task 17 makes of it. A branch the
	// report deletes, or does not name, maps to nothing.
	Branches map[string]map[int]branchPlan
}

type branchPlan struct {
	Topics       []string // topic_is keys the branch answers after Task 17
	RemoveGrants []string // grant_quest actions Task 17 removes
}

var (
	reportDeleteLine = regexp.MustCompile("^- delete quest (\\S+) trigger (\\d+)")
	reportBranchLine = regexp.MustCompile("^- `([^`]+)` branch (\\d+)[^:]*: topic ([a-z0-9_]+)[^:]*: (.*)$")
	reportRemoveGQ   = regexp.MustCompile(`remove grant_quest (\S+?)[;,]?(\s|$)`)
	leadingDigits    = regexp.MustCompile(`^(\d+)`)
)

// parseReport reads the converter report's trigger deletions and behavior
// branch rewrites.
func parseReport(data string) (*convPlan, error) {
	p := &convPlan{DeleteTriggers: map[int]map[int]bool{}, Branches: map[string]map[int]branchPlan{}}
	for _, line := range strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n") {
		if m := reportDeleteLine.FindStringSubmatch(line); m != nil {
			d := leadingDigits.FindStringSubmatch(m[1])
			if d == nil {
				return nil, fmt.Errorf("report: quest file %q has no id prefix", m[1])
			}
			qid, _ := strconv.Atoi(d[1])
			n, _ := strconv.Atoi(m[2])
			if p.DeleteTriggers[qid] == nil {
				p.DeleteTriggers[qid] = map[int]bool{}
			}
			p.DeleteTriggers[qid][n] = true
			continue
		}
		m := reportBranchLine.FindStringSubmatch(line)
		if m == nil || !strings.Contains(m[4], "event: player_topic") {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		if p.Branches[m[1]] == nil {
			p.Branches[m[1]] = map[int]branchPlan{}
		}
		bp := p.Branches[m[1]][n]
		bp.Topics = append(bp.Topics, m[3])
		for _, g := range reportRemoveGQ.FindAllStringSubmatch(m[4], -1) {
			bp.RemoveGrants = append(bp.RemoveGrants, g[1])
		}
		p.Branches[m[1]][n] = bp
	}
	return p, nil
}

var treeIdPrefix = regexp.MustCompile(`^(\d+)(-.*)?\.ya?ml$`)

// readTrees reads every mob and room behavior tree under <data>/behaviors.
// Archetype and item trees carry no quest grants and are skipped.
func readTrees(data string) ([]treeFile, error) {
	root := filepath.Join(data, "behaviors")
	var out []treeFile
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if d.IsDir() {
			if len(parts) == 1 && (parts[0] == "archetypes" || parts[0] == "items") {
				return filepath.SkipDir
			}
			return nil
		}
		m := treeIdPrefix.FindStringSubmatch(d.Name())
		if m == nil || len(parts) < 2 {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var def behaviortree.TreeDef
		if err := yaml.Unmarshal(b, &def); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		id, _ := strconv.Atoi(m[1])
		tf := treeFile{Path: "behaviors/" + filepath.ToSlash(rel), Def: def}
		if parts[0] == "rooms" {
			if len(parts) >= 3 {
				tf.Zone = parts[1]
			}
		} else {
			tf.Zone = parts[0]
			tf.MobId = id
		}
		out = append(out, tf)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// readWorldItems collects every item id the world data hands out outside
// quests and dialogue: any `itemid` key under mobs/, rooms/,
// rooms.instances/ and biomes/ (shops, gear, carried items, spawns,
// containers, forage), and recipe outputs.
func readWorldItems(data string) (map[int][]string, error) {
	out := map[int][]string{}
	add := func(id int, src string) {
		for _, s := range out[id] {
			if s == src {
				return
			}
		}
		out[id] = append(out[id], src)
	}
	for _, dir := range []string{"mobs", "rooms", "rooms.instances", "biomes", "recipes"} {
		root := filepath.Join(data, dir)
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(p, ".yaml") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			var doc any
			if yaml.Unmarshal(b, &doc) != nil {
				return nil // not ours to judge; the boot loaders do that
			}
			rel, _ := filepath.Rel(data, p)
			src := filepath.ToSlash(rel)
			key := "itemid"
			if dir == "recipes" {
				key = "item_id"
			}
			collectInts(doc, key, func(id int) { add(id, src) })
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return out, nil
}

func collectInts(v any, key string, f func(int)) {
	switch t := v.(type) {
	case map[any]any:
		for k, val := range t {
			if ks, ok := k.(string); ok && ks == key {
				if n, ok := val.(int); ok && n > 0 {
					f(n)
				}
			}
			collectInts(val, key, f)
		}
	case []any:
		for _, e := range t {
			collectInts(e, key, f)
		}
	}
}

// readRoomZones maps a room id to its zone folder (<data>/rooms/<zone>/<id>.yaml).
func readRoomZones(data string) (map[int]string, error) {
	paths, err := filepath.Glob(filepath.Join(data, "rooms", "*", "*.yaml"))
	if err != nil {
		return nil, err
	}
	out := map[int]string{}
	for _, p := range paths {
		if id, err := strconv.Atoi(strings.TrimSuffix(filepath.Base(p), ".yaml")); err == nil {
			out[id] = filepath.Base(filepath.Dir(p))
		}
	}
	return out, nil
}

// dialogueFolders maps a mob id to its zone folder under dir.
func dialogueFolders(dir string) (map[int]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*", "*.yaml"))
	if err != nil {
		return nil, err
	}
	out := map[int]string{}
	for _, p := range paths {
		id, err := strconv.Atoi(strings.TrimSuffix(filepath.Base(p), ".yaml"))
		if err != nil {
			continue
		}
		out[id] = filepath.Base(filepath.Dir(p))
	}
	return out, nil
}

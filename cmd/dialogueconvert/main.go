// Command dialogueconvert is the one-time converter that rewrites every
// pre-topics dialogue file (patterns, tree, memory) into keyed topics, folds
// the quest item_give and dialogue triggers and the behavior-tree ask
// branches into topics, and writes a reconciliation report (quest arc #375
// phase 1, plan Task 13, spec section 6.1).
//
// It reads the data folder and writes only under -out, which must lie
// outside the repository. It never edits a repository file. It is deleted in
// Task 24 once the converted content has landed.
//
//	go run ./cmd/dialogueconvert -data _datafiles/world/dogmud -out <scratch>/convert1
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"gopkg.in/yaml.v2"
)

func main() {
	data := flag.String("data", "_datafiles/world/dogmud", "data root to read (dialogue/, quests/, behaviors/, mobs/)")
	out := flag.String("out", "", "output folder, outside the repository (required)")
	flag.Parse()

	if err := run(*data, *out); err != nil {
		fmt.Fprintln(os.Stderr, "dialogueconvert:", err)
		os.Exit(1)
	}
}

func run(data, out string) error {
	if out == "" {
		return errors.New("-out is required")
	}
	absData, err := filepath.Abs(data)
	if err != nil {
		return err
	}
	if err := checkOutDir(out, repoRoot(absData)); err != nil {
		return err
	}
	in, err := LoadInput(data)
	if err != nil {
		return err
	}
	res := Convert(in)
	if err := WriteOutput(res, out); err != nil {
		return err
	}
	r := res.Report
	fmt.Printf("%d files written to %s; grants in %d, out %d, by ruling %d; reconciled: %v\n",
		len(res.Files), out, r.GrantsInTotal(), r.GrantsOutTotal(), r.GrantsByRulingTotal(), r.Reconciled())
	return nil
}

// repoRoot walks up from dir to the folder holding go.mod. With none found
// it returns dir itself, so -out must at least lie outside the data root.
func repoRoot(dir string) string {
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return dir
		}
		d = parent
	}
}

// checkOutDir refuses an output folder inside the repository.
func checkOutDir(out, repo string) error {
	absOut, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(strings.ToLower(absRepo), strings.ToLower(absOut))
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("-out %s is inside the repository %s; write to a scratch folder outside it", absOut, absRepo)
	}
	return nil
}

// ---------------------------------------------------------------- reading

// AddDialogue parses one legacy dialogue file. Strict: a key the legacy
// format does not know would otherwise be dropped without a word.
func (in *Input) AddDialogue(folder, path string, data []byte) error {
	var lf legacyFile
	if err := yaml.UnmarshalStrict(data, &lf); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	in.Dialogues = append(in.Dialogues, dialogueSource{Folder: folder, Path: path, File: lf})
	return nil
}

// AddQuest parses one quest file.
func (in *Input) AddQuest(file string, data []byte) error {
	var q quests.Quest
	if err := yaml.Unmarshal(data, &q); err != nil {
		return fmt.Errorf("quest %s: %w", file, err)
	}
	in.Quests = append(in.Quests, questSource{File: file, Quest: q})
	return nil
}

// AddBehavior parses one mob behavior file.
func (in *Input) AddBehavior(path string, mobId int, data []byte) error {
	var bf btFile
	if err := yaml.Unmarshal(data, &bf); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	in.Behaviors = append(in.Behaviors, behaviorSource{Path: path, MobId: mobId, Tree: bf.Tree})
	return nil
}

var idPrefix = regexp.MustCompile(`^(\d+)-.*\.ya?ml$`)

// notMobTrees are the behaviors/ folders whose trees belong to rooms, items
// and archetypes, not to one mob.
var notMobTrees = map[string]bool{"archetypes": true, "rooms": true, "items": true}

// LoadInput reads dialogue/**, quests/*.yaml, behaviors/**/<mobId>-*.yaml
// (not rooms/, items/ or archetypes/) and mobs/**/<mobId>-*.yaml.
func LoadInput(root string) (*Input, error) {
	in := NewInput()

	dlgRoot := filepath.Join(root, "dialogue")
	err := filepath.WalkDir(dlgRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".yaml") {
			return err
		}
		rel, _ := filepath.Rel(dlgRoot, filepath.Dir(p))
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return in.AddDialogue(filepath.ToSlash(rel), filepath.ToSlash(p), b)
	})
	if err != nil {
		return nil, err
	}

	qfiles, err := filepath.Glob(filepath.Join(root, "quests", "*.yaml"))
	if err != nil {
		return nil, err
	}
	for _, p := range qfiles {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		if err := in.AddQuest(filepath.Base(p), b); err != nil {
			return nil, err
		}
	}

	btRoot := filepath.Join(root, "behaviors")
	err = filepath.WalkDir(btRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if filepath.Dir(p) == btRoot && notMobTrees[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		m := idPrefix.FindStringSubmatch(d.Name())
		if m == nil {
			return nil
		}
		id, _ := strconv.Atoi(m[1])
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		return in.AddBehavior(filepath.ToSlash(rel), id, b)
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	mobRoot := filepath.Join(root, "mobs")
	err = filepath.WalkDir(mobRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		m := idPrefix.FindStringSubmatch(d.Name())
		if m == nil {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var spec struct {
			MobId int    `yaml:"mobid"`
			Zone  string `yaml:"zone"`
		}
		if err := yaml.Unmarshal(b, &spec); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		id := spec.MobId
		if id == 0 {
			id, _ = strconv.Atoi(m[1])
		}
		in.MobZones[id] = spec.Zone
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return in, nil
}

// ---------------------------------------------------------------- writing

// WriteOutput writes <out>/dialogue/<folder>/<mobId>.yaml for every file and
// <out>/report.md.
func WriteOutput(res *Result, out string) error {
	for id, f := range res.Files {
		dir := filepath.Join(out, "dialogue", filepath.FromSlash(f.Folder))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		b, err := yaml.Marshal(f.Output())
		if err != nil {
			return fmt.Errorf("mob %d: %w", id, err)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.yaml", id)), b, 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(out, "report.md"), []byte(res.Report.Markdown()), 0o644)
}

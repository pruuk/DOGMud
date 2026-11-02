package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

// The Pothole Coulee dash guard (#375 phase 7, first zone pass). House style
// (dogmud-player-copy) bars em dashes, en dashes and a spaced double hyphen
// standing in for one. The global prose guard (prose_dash_guard_test.go)
// still exempts quests, dialogue and conversations until phase 7 closes, so
// this guard holds the files the Coulee pass cleared: the pass's quests,
// every dialogue file of the Coulee and the Newcomer Antechamber, every
// conversation pair with a Coulee NPC in it, and the two zones' mobs, rooms,
// behaviour trees and schedules (couleeDashWorldDirs).
//
// It reads the parsed YAML, not lines: every scalar value is checked and no
// comment is, whole-line or trailing (a comment after a value on the same
// line is still a comment, which a line scan cannot tell from prose). Keys
// are not checked; they are never player text.

// couleeDashDoubleHyphen is a double hyphen standing alone: between spaces,
// at the start or at the end of a value. A folded scalar joins its lines
// with a space, so a line that ended in " --" reads " -- " here. "--help"
// and "well-made" do not match.
var couleeDashDoubleHyphen = regexp.MustCompile(`(^|\s)--(\s|$)`)

// couleeDashQuest reports whether a quest id is in the Coulee pass: 28 (the
// antechamber), 30 to 59, 76, 88, 89 and the bridge and Shadows quests 90 to
// 101. Quest 29 is retired.
func couleeDashQuest(id int) bool {
	return id == 28 || (id >= 30 && id <= 59) || id == 76 || id == 88 || id == 89 || (id >= 90 && id <= 101)
}

// couleeDashNPC reports whether a mob id is a Coulee or antechamber NPC: 9100
// to 9169, Dewey (9491) and the four crafters 9605 to 9608.
func couleeDashNPC(id int) bool {
	return (id >= 9100 && id <= 9169) || id == 9491 || (id >= 9605 && id <= 9608)
}

// couleeDashDialogueDirs are the dialogue folders checked whole.
var couleeDashDialogueDirs = []string{"pothole_coulee", "newcomer_antechamber"}

// couleeDashWorldDirs are the other folders of the two zones read whole: the
// lines their mobs, rooms and behaviour trees send, and the schedules' idle
// lines. Every one holds at least one file today, and the floors below count
// them, so a renamed folder fails rather than passes empty.
var couleeDashWorldDirs = []string{
	"behaviors/pothole_coulee",
	"behaviors/newcomer_antechamber",
	"behaviors/rooms/pothole_coulee",
	"behaviors/rooms/newcomer_antechamber",
	"rooms/pothole_coulee",
	"rooms/newcomer_antechamber",
	"mobs/pothole_coulee",
	"mobs/newcomer_antechamber",
	"schedules/pothole_coulee",
}

// couleeDashWorldFiles lists the YAML files of couleeDashWorldDirs, failing on
// a folder that holds none.
func couleeDashWorldFiles(t *testing.T, world string) []string {
	t.Helper()
	var out []string
	for _, d := range couleeDashWorldDirs {
		fs, err := filepath.Glob(filepath.Join(world, filepath.FromSlash(d), "*.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if len(fs) == 0 {
			t.Fatalf("%s holds no YAML; the guard would check nothing there", d)
		}
		out = append(out, fs...)
	}
	return out
}

// couleeDashFiles lists the files the guard reads, by kind.
func couleeDashFiles(t *testing.T, world string) (quests, dialogue, pairs []string) {
	t.Helper()
	qs, err := filepath.Glob(filepath.Join(world, "quests", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range qs {
		idText, _, ok := strings.Cut(filepath.Base(p), "-")
		if id, err := strconv.Atoi(idText); ok && err == nil && couleeDashQuest(id) {
			quests = append(quests, p)
		}
	}
	for _, d := range couleeDashDialogueDirs {
		ds, err := filepath.Glob(filepath.Join(world, "dialogue", d, "*.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		dialogue = append(dialogue, ds...)
	}
	ps, err := filepath.Glob(filepath.Join(world, "conversations", "pairs", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		for _, part := range strings.Split(strings.TrimSuffix(filepath.Base(p), ".yaml"), "_") {
			if id, err := strconv.Atoi(part); err == nil && couleeDashNPC(id) {
				pairs = append(pairs, p)
				break
			}
		}
	}
	return quests, dialogue, pairs
}

// couleeDashFindings parses one YAML file and returns one line per scalar
// value that carries a dash, with the number of scalars read.
func couleeDashFindings(name string, data []byte) ([]string, int, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, 0, fmt.Errorf("%s: %w", name, err)
	}
	var out []string
	scalars := 0
	var walk func(n *yaml.Node, path string)
	walk = func(n *yaml.Node, path string) {
		switch n.Kind {
		case yaml.DocumentNode:
			for _, c := range n.Content {
				walk(c, path)
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				walk(n.Content[i+1], path+"."+n.Content[i].Value)
			}
		case yaml.SequenceNode:
			for i, c := range n.Content {
				walk(c, fmt.Sprintf("%s[%d]", path, i))
			}
		case yaml.ScalarNode:
			scalars++
			if emOrEnDash.MatchString(n.Value) || couleeDashDoubleHyphen.MatchString(n.Value) {
				out = append(out, fmt.Sprintf("%s:%d %s uses a dash: %q", name, n.Line, strings.TrimPrefix(path, "."), n.Value))
			}
		}
	}
	walk(&doc, "")
	return out, scalars, nil
}

func TestCouleeTextHasNoDashes(t *testing.T) {
	root := repoRoot(t)
	world := filepath.Join(root, "_datafiles", "world", "dogmud")
	quests, dialogue, pairs := couleeDashFiles(t, world)
	// Floors: before any bridge exists the pass holds 34 quest files, 30
	// dialogue files and 3 conversation pairs (spec facts table). Fewer means
	// a path or a pattern broke, not that the text is clean.
	if len(quests) < 34 || len(dialogue) < 30 || len(pairs) < 3 {
		t.Fatalf("found %d quest, %d dialogue and %d conversation files; the guard would check too little",
			len(quests), len(dialogue), len(pairs))
	}
	// 176 Coulee rooms alone today; well under that means the walk broke.
	worldFiles := couleeDashWorldFiles(t, world)
	if len(worldFiles) < 200 {
		t.Fatalf("found %d mob, room, behaviour and schedule files; the guard would check too little", len(worldFiles))
	}
	scalars := 0
	files := append(append(append(append([]string{}, quests...), dialogue...), pairs...), worldFiles...)
	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(root, p)
		found, n, err := couleeDashFindings(filepath.ToSlash(rel), data)
		if err != nil {
			t.Fatal(err)
		}
		scalars += n
		for _, f := range found {
			t.Error(f)
		}
	}
	if scalars == 0 {
		t.Fatal("read no scalar values")
	}
}

// TestCouleeTextHasNoDashes_GuardCanFail proves the guard sees a planted em
// dash, en dash and spaced double hyphen (one across a folded line), skips
// comments whole-line and trailing, passes clean text, and picks the right
// quests and NPCs.
func TestCouleeTextHasNoDashes_GuardCanFail(t *testing.T) {
	bad := []byte("# a comment may say -- this — freely\n" +
		"name: First Casting\n" +
		"description: >-\n  You let it go --\n  that is the whole of it.\n" +
		"steps:\n  - id: start\n    hint: Type cast spike mote — then wait.\n" +
		"  - id: end\n    description: The mote reknits – unharmed.\n" +
		"room: 5354  # Echoing Narrows — a trailing comment\n")
	found, n, err := couleeDashFindings("bad.yaml", bad)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 3 {
		t.Fatalf("guard saw %d of 3 planted dashes in %d scalars: %v", len(found), n, found)
	}
	good := []byte("name: First Casting\nhint: Type cast spike-mote. A --help flag is no dash.\n")
	if found, _, err := couleeDashFindings("good.yaml", good); err != nil || len(found) != 0 {
		t.Fatalf("guard flagged clean text (err %v): %v", err, found)
	}
	for id, want := range map[int]bool{28: true, 29: false, 30: true, 59: true, 60: false, 76: true, 88: true, 89: true, 90: true, 101: true, 102: false} {
		if couleeDashQuest(id) != want {
			t.Errorf("quest %d: in the pass = %v, want %v", id, !want, want)
		}
	}
	for id, want := range map[int]bool{9099: false, 9100: true, 9169: true, 9170: false, 9491: true, 9604: false, 9605: true, 9608: true, 9609: false} {
		if couleeDashNPC(id) != want {
			t.Errorf("mob %d: Coulee NPC = %v, want %v", id, !want, want)
		}
	}
}

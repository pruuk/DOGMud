package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The #375 closing step's guards. The quest loop backlog is retired: every
// loop and destination finding refuses boot, so there is no list to park a
// finding on, and nothing may bring one back.

const closingStepGuardFile = "closing_step_guard_test.go"

var backlogRevivers = regexp.MustCompile(`\b(Ratchet|ParseBacklog|FormatBacklog|LoadBacklog|BacklogPath)\b|loop-backlog\.txt`)

// matchingLines returns "line N: text" for each line of src that re matches.
func matchingLines(re *regexp.Regexp, src string) []string {
	var out []string
	for i, line := range strings.Split(src, "\n") {
		if re.MatchString(line) {
			out = append(out, fmt.Sprintf("line %d: %s", i+1, strings.TrimSpace(line)))
		}
	}
	return out
}

// repoGoFiles lists every Go file at the repo root and under internal,
// modules, cmd and tools, except this guard file. It fails if it finds too
// few to mean anything.
func repoGoFiles(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	files, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"internal", "modules", "cmd", "tools"} {
		err := filepath.WalkDir(filepath.Join(root, d), func(path string, de os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !de.IsDir() && filepath.Ext(path) == ".go" && de.Name() != closingStepGuardFile {
				files = append(files, path)
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	if len(files) < 500 {
		t.Fatalf("found only %d Go files; the guard would check almost nothing", len(files))
	}
	return files
}

func TestLoopBacklogStaysRetired(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "_datafiles", "world", "dogmud", "quests", "loop-backlog.txt")); err == nil {
		t.Error("quests/loop-backlog.txt is back; every loop finding refuses boot, so fix the content instead")
	}
	for _, path := range repoGoFiles(t) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range matchingLines(backlogRevivers, string(data)) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s %s: the loop backlog is retired (#375 closing step)", rel, m)
		}
	}
}

func TestLoopBacklogStaysRetired_GuardCanFail(t *testing.T) {
	src := "package x\n\nfunc f() { questloop.Ratchet(nil, nil) }\n// reads quests/loop-backlog.txt\nvar ok = 1\n"
	if got := matchingLines(backlogRevivers, src); len(got) != 2 {
		t.Fatalf("guard saw %d lines, want 2: %v", len(got), got)
	}
}

// room_interact was split into look_at and search_at at the #375 closing
// step. The name may appear only in the loader's refusal (the
// retiredInteractEvent constant and the lines that use it).
var roomInteractRevivers = regexp.MustCompile(`room_interact|RoomInteract`)

func roomInteractLines(src string) []string {
	var out []string
	for _, m := range matchingLines(roomInteractRevivers, src) {
		if !strings.Contains(m, "retiredInteractEvent") {
			out = append(out, m)
		}
	}
	return out
}

func TestRoomInteractStaysRetired(t *testing.T) {
	root := repoRoot(t)
	files := repoGoFiles(t)
	for _, d := range []string{
		filepath.Join(root, "_datafiles", "world"),
		filepath.Join(root, "_datafiles", "html"),
	} {
		err := filepath.WalkDir(d, func(path string, de os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ext := filepath.Ext(path); !de.IsDir() && (ext == ".yaml" || ext == ".js") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range roomInteractLines(string(data)) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s %s: use look_at or search_at (#375 closing step)", rel, m)
		}
	}
}

func TestRoomInteractStaysRetired_GuardCanFail(t *testing.T) {
	src := "  - event: room_interact\n    noun: stone\nnotifyRoomInteract(u)\nif t.Event == retiredInteractEvent {\n"
	if got := roomInteractLines(src); len(got) != 2 {
		t.Fatalf("guard saw %d lines, want 2: %v", len(got), got)
	}
}

package usercommands

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v2"
)

// TestRangedQuestTriggersMatchTheNotifyConstant pins the contract between the Go
// notify string and the quest YAML that gates on it.
//
// Nothing else connects them. The notify is HARDCODED rather than taken from
// what the player typed, because `shoot` is an alias of `fire` and still arrives
// at the same handler -- so a rename on either side leaves the other pointing at
// a command that never fires. A quest that silently stops granting produces no
// error anywhere: the player just types the right thing and nothing happens.
//
// This is the same silent-data-contract family as a mob YAML key that binds to
// no field, which is how seventeen mobs shipped carrying nothing.
func TestRangedQuestTriggersMatchTheNotifyConstant(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/usercommands/<file> -> repo root
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))

	// The ranged spoke: First Shot, Across the Canyon, Range Practice.
	for _, name := range []string{
		"50-first_shot.yaml",
		"51-across_the_canyon.yaml",
		"59-range_practice.yaml",
	} {
		path := filepath.Join(root, "_datafiles", "world", "dogmud", "quests", name)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		var q struct {
			Triggers []struct {
				Event   string `yaml:"event"`
				Command string `yaml:"command"`
			} `yaml:"triggers"`
		}
		if err := yaml.Unmarshal(raw, &q); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		// Commands these quests legitimately gate on besides the shot itself.
		// `sneak` is quest 50's ambush beat: it grants for LEARNING TO HIDE,
		// because the command event carries only a name and so cannot tell that
		// a shot was an ambush.
		allowed := map[string]bool{
			questNotifyCommand: true,
			"sneak":            true,
		}

		found := 0
		for _, tr := range q.Triggers {
			if tr.Event != "command" || tr.Command == "" {
				continue
			}
			found++
			if !allowed[tr.Command] {
				t.Errorf("%s: command trigger %q is not a command this spoke notifies; "+
					"the shot notifies %q", name, tr.Command, questNotifyCommand)
			}
		}

		// ⚠️ Without this, the test passes vacuously on a file whose triggers
		// were renamed to something the loop never inspects -- which is the
		// exact failure it exists to catch.
		if found == 0 {
			t.Errorf("%s: no command triggers found; this test would prove nothing", name)
		}
	}
}

// TestRangedQuestsNoLongerGateOnTheRetiredCommand pins that no quest waits on a
// command the player can no longer type.
//
// The `reload` beat was quest 50's second step until firing began chambering its
// own next round. A trigger left behind would make the quest uncompletable, and
// nothing would say so: the player would simply never be able to finish.
func TestRangedQuestsNoLongerGateOnTheRetiredCommand(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	questDir := filepath.Join(root, "_datafiles", "world", "dogmud", "quests")

	entries, err := os.ReadDir(questDir)
	if err != nil {
		t.Fatalf("read quest dir: %v", err)
	}

	checked := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(questDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var q struct {
			Triggers []struct {
				Event   string `yaml:"event"`
				Command string `yaml:"command"`
			} `yaml:"triggers"`
		}
		if err := yaml.Unmarshal(raw, &q); err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		checked++
		for _, tr := range q.Triggers {
			if tr.Event == "command" && tr.Command == "reload" {
				t.Errorf("%s gates a beat on `reload`, which players can no longer type; "+
					"the quest would be uncompletable", e.Name())
			}
		}
	}
	if checked == 0 {
		t.Fatal("no quest files were inspected; this test would prove nothing")
	}
}

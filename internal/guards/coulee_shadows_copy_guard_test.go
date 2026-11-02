package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// The Shadows line's text is what a first-time player has to go on. A live
// playtest (#375 phase 7) stuck on From the Dark because the hint left out
// half the action, and because a kill out in the open failed in silence. These
// hold the text to what the game does. The numbers it may not state are pinned
// in internal/actions (the odds) and in the config: StealCooldown ships at 60
// real seconds, and the Flooded Sump respawns its brute in 5 real minutes.

type shadowsQuest struct {
	Steps []struct {
		Id          string `yaml:"id"`
		Description string `yaml:"description"`
		Hint        string `yaml:"hint"`
	} `yaml:"steps"`
	Triggers []struct {
		Event      string `yaml:"event"`
		Area       string `yaml:"area"`
		Conditions struct {
			Has     []string `yaml:"has"`
			Missing []string `yaml:"missing"`
		} `yaml:"conditions"`
		Actions []struct {
			Grant string `yaml:"grant"`
			Actor string `yaml:"actor"`
		} `yaml:"actions"`
	} `yaml:"triggers"`
}

func readShadowsQuest(t *testing.T, file string) shadowsQuest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "quests", file))
	if err != nil {
		t.Fatal(err)
	}
	var q shadowsQuest
	if err := yaml.Unmarshal(data, &q); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return q
}

func (q shadowsQuest) step(t *testing.T, id string) (description, hint string) {
	t.Helper()
	for _, s := range q.Steps {
		if s.Id == id {
			return s.Description, s.Hint
		}
	}
	t.Fatalf("no step %q", id)
	return "", ""
}

var typeWord = regexp.MustCompile(`(?i)\btype\b`)

// inOneType reports what is wrong with a hint meant to name a whole action in
// order under one "type": it must hold the word once, and each word after it,
// in the order given.
func inOneType(hint string, words ...string) error {
	if n := len(typeWord.FindAllString(hint, -1)); n != 1 {
		return fmt.Errorf("%d uses of \"type\", want 1: %q", n, hint)
	}
	at := typeWord.FindStringIndex(hint)[1]
	for _, w := range words {
		i := strings.Index(hint[at:], w)
		if i < 0 {
			return fmt.Errorf("%q missing or out of order: %q", w, hint)
		}
		at += i + len(w)
	}
	return nil
}

func TestQuest100_HintNamesTheWholeActionInOrder(t *testing.T) {
	_, hint := readShadowsQuest(t, "100-from_the_dark.yaml").step(t, "hidden")
	if err := inOneType(hint, "hood", "west", "unhood", "attack brute"); err != nil {
		t.Error(err)
	}
}

func TestInOneType_GuardCanFail(t *testing.T) {
	for _, bad := range []string{
		"Go west into the Flooded Sump and type attack brute.",
		"Type hood. Then type west, unhood, attack brute.",
		"Type west, hood, unhood, attack brute.",
	} {
		if inOneType(bad, "hood", "west", "unhood", "attack brute") == nil {
			t.Errorf("should refuse %q", bad)
		}
	}
	if err := inOneType("In the drift, type hood, then west, then unhood, then attack brute.", "hood", "west", "unhood", "attack brute"); err != nil {
		t.Error(err)
	}
}

// A kill in the open moved nothing and said nothing. The line fires only on
// the step it explains, grants nothing, and says why, how to try again and how
// long a dead brute takes.
func TestQuest100_KillInTheOpenIsExplained(t *testing.T) {
	q := readShadowsQuest(t, "100-from_the_dark.yaml")
	for _, tr := range q.Triggers {
		if tr.Event != "mob_death" || tr.Area != "coulee_mine" ||
			len(tr.Conditions.Has) != 1 || tr.Conditions.Has[0] != "100-hidden" {
			continue
		}
		if len(tr.Conditions.Missing) != 1 || tr.Conditions.Missing[0] != "100-ambush" {
			t.Errorf("must fire only before the ambush: missing %v", tr.Conditions.Missing)
		}
		line := ""
		for _, a := range tr.Actions {
			if a.Grant != "" {
				t.Errorf("the line must not advance the quest: grants %s", a.Grant)
			}
			line += a.Actor
		}
		for _, want := range []string{"hiding", "five minutes", "attack brute"} {
			if !strings.Contains(line, want) {
				t.Errorf("line must name %q: %q", want, line)
			}
		}
		return
	}
	t.Error("no mob_death trigger explains a kill made before the ambush")
}

// The step's own text names the wait, for a student who finds the sump empty
// because somebody else killed the brute.
func TestQuest100_StepNamesTheEmptySump(t *testing.T) {
	description, _ := readShadowsQuest(t, "100-from_the_dark.yaml").step(t, "hidden")
	for _, want := range []string{"about five minutes", "see nothing", "flee"} {
		if !strings.Contains(description, want) {
			t.Errorf("hidden step must say %q: %q", want, description)
		}
	}
}

// StealCooldown ships at 60 real seconds and steal and plant share it.
func TestShadowsLine_TheftHintsSayHowLong(t *testing.T) {
	q99 := readShadowsQuest(t, "99-quiet_hands.yaml")
	for _, id := range []string{"hidden", "lifted"} {
		if _, hint := q99.step(t, id); !strings.Contains(hint, "about a minute") {
			t.Errorf("quest 99 step %s hint must say about how long: %q", id, hint)
		}
	}
	if d, _ := readShadowsQuest(t, "101-quick_fingers.yaml").step(t, "start"); !strings.Contains(d, "about a minute") {
		t.Errorf("quest 101 must say about how long: %q", d)
	}
}

// `steal purse dummy` reads only its first word, and "purse" is part of
// "Cutpurse Dace", so it answered "You can't steal from Cutpurse Dace."
func TestQuest99_SaysStealNamesTheMark(t *testing.T) {
	description, _ := readShadowsQuest(t, "99-quiet_hands.yaml").step(t, "hidden")
	if !strings.Contains(description, "not the thing") {
		t.Errorf("hidden step must say steal names the mark: %q", description)
	}
}

// picklock.go: a wrong pin spends one of a set's uses, wipes the lock's
// progress and closes the prompt, so the player must type the command again.
func TestQuest99_SaysWhatAWrongPinCosts(t *testing.T) {
	description, hint := readShadowsQuest(t, "99-quiet_hands.yaml").step(t, "planted")
	for _, want := range []string{"wrong pin", "Dace"} {
		if !strings.Contains(hint, want) {
			t.Errorf("planted hint must name %q: %q", want, hint)
		}
	}
	if !strings.Contains(description, "picklock lockbox again") {
		t.Errorf("planted step must say to start the command again: %q", description)
	}
}

// combat.ResolveFleeBlockers: a rank-1 fleer gets clear of a scree scavenger
// about a third of the time and of a mine crawler about a fifth, per try. A
// warning that says to flee has to say so.
func TestShadowsLine_FleeWarningsSayItCanBeBlocked(t *testing.T) {
	root := filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud")
	for _, c := range []struct{ file, near string }{
		{filepath.Join(root, "quests", "98-first_light.yaml"), "attack on sight"},
		{filepath.Join(root, "dialogue", "pothole_coulee", "9106.yaml"), "One at a"},
		{filepath.Join(root, "quests", "100-from_the_dark.yaml"), "Keep fighting until it falls"},
	} {
		data, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		i := strings.Index(text, c.near)
		if i < 0 {
			t.Fatalf("%s: lost %q", c.file, c.near)
		}
		end := i + 400
		if end > len(text) {
			end = len(text)
		}
		if !strings.Contains(strings.Join(strings.Fields(text[i:end]), " "), "block") {
			t.Errorf("%s: the warning near %q must say a flee can be blocked", c.file, c.near)
		}
	}
}

// The jetty's description must name each way out, by the exit's own room.
func TestReedJetty_DescriptionNamesBothExits(t *testing.T) {
	type room struct {
		Title       string `yaml:"title"`
		Description string `yaml:"description"`
		Exits       map[string]struct {
			RoomId int `yaml:"roomid"`
		} `yaml:"exits"`
	}
	read := func(id int) room {
		data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "rooms", "pothole_coulee", fmt.Sprintf("%d.yaml", id)))
		if err != nil {
			t.Fatal(err)
		}
		var r room
		if err := yaml.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	jetty := read(5218)
	text := strings.Join(strings.Fields(jetty.Description), " ")
	if len(jetty.Exits) != 2 {
		t.Errorf("the jetty has %d exits; this test and the text name two", len(jetty.Exits))
	}
	for dir, ex := range jetty.Exits {
		want := fmt.Sprintf("%s to %s", dir, read(ex.RoomId).Title)
		if !strings.Contains(text, want) {
			t.Errorf("description must say %q: %q", want, text)
		}
	}
}

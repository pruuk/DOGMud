package users

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
)

// A save that is valid YAML but has one bad field must NOT be silently
// rewritten with defaults. Before this fix LoadUser logged the unmarshal
// error, continued, and re-saved the defaulted record over the original.
func TestLoadUserDoesNotOverwriteCorruptSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1.yaml")
	// `training` is an int; a string here makes yaml.v2 return a TypeError
	// while still populating the rest of the document.
	original := "userid: 1\nusername: corrupt\ncharacter:\n  name: Corrupt\n  stats:\n    strength:\n      base: 120\n      training: notanumber\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadUserFromPath(path, false)
	if err == nil {
		t.Fatal("LoadUser accepted a save with a malformed field; want an error")
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != original {
		t.Errorf("save file was rewritten:\n--- before ---\n%s\n--- after ---\n%s", original, string(after))
	}
	if !strings.Contains(string(after), "training: notanumber") {
		t.Error("the offending field was rewritten — the original must be preserved for diagnosis")
	}
}

// #451 review G4: logout forces a hidden player visible, which leaves record 9
// (or 31) expired in the save. Loading must drop it silently, or the first
// prune after login tells the room "emerges from the shadows".
func TestLoadUserDropsAnEndedStealthRecord(t *testing.T) {
	dir := withUserDataDir(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {ConditionId: 9, Name: "Hidden", Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
	}))
	path := filepath.Join(dir, "users", "7.yaml")
	// An expired record saves with no triggersleft (TriggersLeftExpired, 0).
	save := "userid: 7\nusername: quitter\ncharacter:\n  name: Quitter\n  conditions:\n    list:\n      - conditionid: 9\n"
	if err := os.WriteFile(path, []byte(save), 0644); err != nil {
		t.Fatal(err)
	}

	u, err := loadUserFromPath(path, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range u.Character.Conditions.List {
		if rec.ConditionId == 9 {
			t.Fatalf("an ended record 9 survived the load: %+v", rec)
		}
	}
}

// A completely unparseable file must return an error, not panic on a nil
// Character.
func TestLoadUserTornFileReturnsErrorNotPanic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2.yaml")
	if err := os.WriteFile(path, []byte("userid: 2\nusername: torn\ncharacter:\n  stats:\n   \x00\x00 broken"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadUserFromPath(path, false); err == nil {
		t.Fatal("torn file was accepted; want an error")
	}
}

// #375: a talk topic renamed in the topic-key manifest stays heard. LoadUser
// reads the renames from dialogue/topic-keys.txt under the data folder.
func TestLoadUserRenamesHeardTopicsFromTheManifest(t *testing.T) {
	dir := withUserDataDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "dialogue"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "12.new\n94.kept\n# renamed\n12.old -> 12.new\n"
	if err := os.WriteFile(filepath.Join(dir, "dialogue", "topic-keys.txt"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	idx := NewUserIndex()
	if err := idx.Create(); err != nil {
		t.Fatal(err)
	}
	if err := idx.AddUser(12, "listener"); err != nil {
		t.Fatal(err)
	}
	save := "userid: 12\nusername: listener\ncharacter:\n  name: Listener\n  heardtopics:\n    - 12.old\n    - 94.kept\n"
	if err := os.WriteFile(filepath.Join(dir, "users", "12.yaml"), []byte(save), 0644); err != nil {
		t.Fatal(err)
	}

	u, err := LoadUser("listener", true)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(u.Character.HeardTopics, ",")
	if got != "12.new,94.kept" {
		t.Fatalf("heard topics after load = %q, want \"12.new,94.kept\" (the rename applied, the other key kept)", got)
	}
}

// #375 phase 2: a quest step renamed in the step-id manifest keeps the player
// on it. LoadUser reads the renames from quests/step-ids.txt.
func TestLoadUserRenamesQuestStepsFromTheManifest(t *testing.T) {
	dir := withUserDataDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "quests"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "# renamed\n53-firstkill -> 53-scout\n"
	if err := os.WriteFile(filepath.Join(dir, "quests", "step-ids.txt"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	idx := NewUserIndex()
	if err := idx.Create(); err != nil {
		t.Fatal(err)
	}
	if err := idx.AddUser(13, "stepper"); err != nil {
		t.Fatal(err)
	}
	save := "userid: 13\nusername: stepper\ncharacter:\n  name: Stepper\n  questprogress:\n    53: firstkill\n"
	if err := os.WriteFile(filepath.Join(dir, "users", "13.yaml"), []byte(save), 0644); err != nil {
		t.Fatal(err)
	}

	u, err := LoadUser("stepper", true)
	if err != nil {
		t.Fatal(err)
	}
	if got := u.Character.QuestProgress[53]; got != "scout" {
		t.Fatalf("quest 53 step after load = %q, want \"scout\"", got)
	}
}

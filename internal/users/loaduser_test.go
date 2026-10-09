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

package migration

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func writeUserSave(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Deliberately NOT in yaml.v2's sorted key order, so a needless rewrite
// changes the bytes and the no-op tests can see it.
const saveWithHintsOff = "username: alice\nuserid: 1\nconfigoptions:\n  tinymap: true\n  hints: false\ncharacter:\n  name: Aliceia\ntipscomplete:\n  list: true\n"

func TestRenameTipsConfigOption_MovesTheFlag(t *testing.T) {
	dir := t.TempDir()
	path := writeUserSave(t, dir, "1.yaml", saveWithHintsOff)

	if err := renameTipsConfigOptionInDir(dir, false); err != nil {
		t.Fatal(err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(readBytes(t, path), &doc); err != nil {
		t.Fatal(err)
	}
	opts := doc["configoptions"].(map[any]any)
	if v, ok := opts["tips"]; !ok || v != false {
		t.Errorf("configoptions.tips = %v (present %v), want false", v, ok)
	}
	if _, ok := opts["hints"]; ok {
		t.Error("configoptions.hints is still present")
	}
	if opts["tinymap"] != true || doc["username"] != "alice" || doc["userid"] != 1 {
		t.Errorf("other fields changed: %v", doc)
	}
	if doc["character"].(map[any]any)["name"] != "Aliceia" {
		t.Error("character data changed")
	}
	if doc["tipscomplete"].(map[any]any)["list"] != true {
		t.Error("tipscomplete, a different key, changed")
	}
}

func TestRenameTipsConfigOption_LeavesOtherSavesByteIdentical(t *testing.T) {
	dir := t.TempDir()
	body := "username: bob\nuserid: 2\nconfigoptions:\n  tinymap: false\n"
	path := writeUserSave(t, dir, "2.yaml", body)

	if err := renameTipsConfigOptionInDir(dir, false); err != nil {
		t.Fatal(err)
	}
	if got := readBytes(t, path); !bytes.Equal(got, []byte(body)) {
		t.Errorf("a save without the hints option was rewritten:\n%s", got)
	}
}

func TestRenameTipsConfigOption_SecondRunIsANoOp(t *testing.T) {
	dir := t.TempDir()
	path := writeUserSave(t, dir, "1.yaml", saveWithHintsOff)

	if err := renameTipsConfigOptionInDir(dir, false); err != nil {
		t.Fatal(err)
	}
	first := readBytes(t, path)
	if err := renameTipsConfigOptionInDir(dir, false); err != nil {
		t.Fatal(err)
	}
	if second := readBytes(t, path); !bytes.Equal(first, second) {
		t.Error("a second run rewrote an already-migrated save")
	}
}

func TestRenameTipsConfigOption_RefusesBothKeys(t *testing.T) {
	dir := t.TempDir()
	body := "username: carol\nconfigoptions:\n  hints: false\n  tips: true\n"
	path := writeUserSave(t, dir, "3.yaml", body)

	err := renameTipsConfigOptionInDir(dir, false)
	if err == nil || !strings.Contains(err.Error(), "both hints and tips") {
		t.Fatalf("err = %v, want a refusal naming both keys", err)
	}
	if got := readBytes(t, path); !bytes.Equal(got, []byte(body)) {
		t.Error("the ambiguous save was modified")
	}
}

func TestRenameTipsConfigOption_DryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	path := writeUserSave(t, dir, "1.yaml", saveWithHintsOff)

	if err := renameTipsConfigOptionInDir(dir, true); err != nil {
		t.Fatal(err)
	}
	if got := readBytes(t, path); !bytes.Equal(got, []byte(saveWithHintsOff)) {
		t.Error("dry run wrote the file")
	}
}

func TestRenameTipsConfigOption_MissingUsersDirIsFine(t *testing.T) {
	if err := renameTipsConfigOptionInDir(filepath.Join(t.TempDir(), "users"), false); err != nil {
		t.Errorf("a tree with no users directory returned %v", err)
	}
}

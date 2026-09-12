package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"gopkg.in/yaml.v3"
)

// Slice E: every flag a dogmud buff carries must be one the engine declares,
// spelled exactly. An unknown flag used to load silently and do nothing: the
// Cat's Eye Draught shipped `night-vision` for `nightvision` and gave no night
// vision for weeks, and Stone Stomach's `poison-immunity` was read by nothing
// at all. LoadDataFiles now panics on one; this fails the merge before it can
// reach a boot.
func TestEveryDogmudBuffFlagIsDeclared(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "buffs", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no buff files: %v", err)
	}
	sort.Strings(files)
	var problems []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var b struct {
			BuffId int      `yaml:"buffid"`
			Name   string   `yaml:"name"`
			Flags  []string `yaml:"flags"`
		}
		if err := yaml.Unmarshal(raw, &b); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		spec := &buffs.BuffSpec{BuffId: b.BuffId, Name: b.Name}
		for _, fl := range b.Flags {
			spec.Flags = append(spec.Flags, buffs.Flag(fl))
		}
		if err := spec.ValidateFlags(); err != nil {
			problems = append(problems, filepath.Base(f)+": "+err.Error())
		}
	}
	if len(problems) > 0 {
		t.Fatalf("%d unknown buff flags:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
}

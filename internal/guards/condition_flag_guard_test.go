package guards

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"gopkg.in/yaml.v3"
)

// Slice E: every flag a dogmud condition carries must be one the engine
// declares, spelled exactly. An unknown flag used to load silently and do
// nothing: the Cat's Eye Draught shipped `night-vision` for `nightvision`
// and gave no night vision for weeks, and Stone Stomach's `poison-immunity`
// was read by nothing at all. LoadDataFiles now panics on one; this fails
// the merge before it can reach a boot.
func TestEveryDogmudConditionFlagIsDeclared(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "conditions", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no condition files: %v", err)
	}
	sort.Strings(files)
	var problems []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var b struct {
			ConditionId int      `yaml:"conditionid"`
			Name        string   `yaml:"name"`
			Flags       []string `yaml:"flags"`
		}
		if err := yaml.Unmarshal(raw, &b); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		spec := &conditions.ConditionSpec{ConditionId: b.ConditionId, Name: b.Name}
		for _, fl := range b.Flags {
			spec.Flags = append(spec.Flags, conditions.Flag(fl))
		}
		if err := spec.ValidateFlags(); err != nil {
			problems = append(problems, filepath.Base(f)+": "+err.Error())
		}
	}
	if len(problems) > 0 {
		t.Fatalf("%d unknown condition flags:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
}

// A condition that bleeds its holder carries the `bleeding` flag. The flag is
// what names the holder "bleeding" and what reports a bleed-out death as one
// (tickCauseFor), so a bleed without it is a bleed the rest of the game cannot
// see. 115 Rending Bleed shipped without it (2026-10-17 playtest).
//
// A bleed here is a record that harms the health pool on its ticks and says
// "bleed" in its name, description or tick line.
func TestEveryDogmudBleedCarriesTheBleedingFlag(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "conditions", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no condition files: %v", err)
	}
	sort.Strings(files)
	var problems []string
	bleeds := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var b struct {
			Name              string   `yaml:"name"`
			Description       string   `yaml:"description"`
			TickPool          string   `yaml:"tick_pool"`
			TickPercent       float64  `yaml:"tick_percent"`
			TickFromMagnitude bool     `yaml:"tick_from_magnitude"`
			TriggerActee      string   `yaml:"trigger_actee"`
			Flags             []string `yaml:"flags"`
		}
		if err := yaml.Unmarshal(raw, &b); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		harmsHealth := b.TickPool == "health" && (b.TickPercent < 0 || b.TickFromMagnitude)
		text := strings.ToLower(b.Name + " " + b.Description + " " + b.TriggerActee)
		if !harmsHealth || !strings.Contains(text, "bleed") {
			continue
		}
		bleeds++
		flagged := false
		for _, fl := range b.Flags {
			if conditions.Flag(fl) == conditions.Bleeding {
				flagged = true
			}
		}
		if !flagged {
			problems = append(problems, filepath.Base(f))
		}
	}
	if bleeds == 0 {
		t.Fatal("no bleed condition matched, so this guard checked nothing")
	}
	if len(problems) > 0 {
		t.Fatalf("%d bleed conditions lack the bleeding flag:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
}

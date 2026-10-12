package gossip

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// M6 slice 3 (ledger row 25): a fact description is spliced after a lead-in
// ("Word is, ..."), so it must start lowercase. A, An and The are lowered at
// render by LowerLeadingArticle; proper nouns are allowed by name.
var factProperNouns = map[string]bool{"Thornwall": true, "Elgar": true, "Delia": true}

func capitalStarts(descs map[string]string) []string {
	var bad []string
	for id, d := range descs {
		fields := strings.Fields(d)
		if len(fields) == 0 {
			bad = append(bad, id+" (empty)")
			continue
		}
		w := fields[0]
		if w[0] < 'A' || w[0] > 'Z' {
			continue
		}
		switch w {
		case "A", "An", "The":
			continue
		}
		if factProperNouns[w] {
			continue
		}
		bad = append(bad, id+" starts with "+w)
	}
	return bad
}

func TestFactDescriptionsStartLowercase(t *testing.T) {
	if len(capitalStarts(map[string]string{"x": "Boats return."})) != 1 ||
		len(capitalStarts(map[string]string{"x": "boats return.", "y": "The mayor", "z": "Delia left"})) != 0 {
		t.Fatal("capitalStarts cannot tell a capital from a lowercase start; the guard would prove nothing")
	}
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "facts.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Facts []struct {
			Id          string `yaml:"id"`
			Description string `yaml:"description"`
		} `yaml:"facts"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Facts) < 10 {
		t.Fatalf("only %d facts parsed; the guard would check almost nothing", len(doc.Facts))
	}
	descs := map[string]string{}
	for _, f := range doc.Facts {
		descs[f.Id] = f.Description
	}
	for _, b := range capitalStarts(descs) {
		t.Errorf("fact description must start lowercase: %s", b)
	}
}

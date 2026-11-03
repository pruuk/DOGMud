package guards

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Every torn page counts toward The Torn Record, and every page can be had: it
// has exactly one home, on a seller's list or in a found-room trigger. A page
// added to one and left out of the other would make the achievement impossible.
func TestEveryTornPageIsCountedAndHasOneHome(t *testing.T) {
	const world = `_datafiles/world/dogmud/`

	raw, err := os.ReadFile(world + `achievements/the-torn-record.yaml`)
	if err != nil {
		t.Fatal(err)
	}
	var ach struct {
		Trigger struct {
			Type  string `yaml:"type"`
			Items []int  `yaml:"items"`
		} `yaml:"trigger"`
	}
	if err := yaml.Unmarshal(raw, &ach); err != nil {
		t.Fatal(err)
	}
	if ach.Trigger.Type != `items_read` || len(ach.Trigger.Items) < 25 {
		t.Fatalf(`the-torn-record: type %q with %d items`, ach.Trigger.Type, len(ach.Trigger.Items))
	}

	// The loader derives each filename from the item's name, so the pages
	// cannot share a `torn_page` filename stem; they share `namesimple`.
	files, err := filepath.Glob(world + `items/materials-40000/*.yaml`)
	if err != nil {
		t.Fatal(err)
	}
	onDisk := map[string]bool{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if regexp.MustCompile(`(?m)^namesimple: torn page\s*$`).Match(b) {
			onDisk[strings.SplitN(filepath.Base(f), `-`, 2)[0]] = true
		}
	}
	if len(onDisk) != len(ach.Trigger.Items) {
		t.Errorf(`%d page files, %d in the achievement`, len(onDisk), len(ach.Trigger.Items))
	}

	homes := map[int]int{}
	sold := map[int]bool{}
	shop := regexp.MustCompile(`(?m)^\s*- itemid: (\d+)\s*$`)
	mobFiles := []string{
		`mobs/greenford/9509-aldith_the_bookseller.yaml`,
		`mobs/the_confluence/9444-drunn_the_bookseller.yaml`,
		`mobs/new_plymouth_crafting/9332-orin_the_bookseller.yaml`,
		`mobs/marches_spur_road/250-peddler_malk.yaml`,
	}
	for _, f := range mobFiles {
		b, err := os.ReadFile(world + f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range shop.FindAllStringSubmatch(string(b), -1) {
			homes[atoi(t, m[1])]++
			sold[atoi(t, m[1])] = true
		}
	}
	q, err := os.ReadFile(world + `quests/76-the_disc.yaml`)
	if err != nil {
		t.Fatal(err)
	}
	found := regexp.MustCompile(`(?m)^\s*- give_item: (\d+)\s*$`)
	for _, m := range found.FindAllStringSubmatch(string(q), -1) {
		homes[atoi(t, m[1])]++
	}
	for _, id := range ach.Trigger.Items {
		if homes[id] != 1 {
			t.Errorf(`page %d has %d homes, want 1`, id, homes[id])
		}
		spec, err := filepath.Glob(world + `items/materials-40000/` + strconv.Itoa(id) + `-*.yaml`)
		if err != nil || len(spec) != 1 {
			t.Errorf(`page %d has no item file`, id)
			continue
		}
		text, _ := os.ReadFile(spec[0])
		// A found page is not_salable, so finding one cannot be a gold loop. A
		// page on a seller's list cannot be: TestSevenMerchantsAreRealShops
		// refuses not_salable stock, and the vendor validator then needs a
		// category on it.
		wants := []string{`type: readable`, `not_salable: true`}
		if sold[id] {
			wants = []string{`type: readable`, `vendor_categories:`}
			if strings.Contains(string(text), `not_salable`) {
				t.Errorf(`page %d is sold but not_salable`, id)
			}
		}
		for _, want := range wants {
			if !strings.Contains(string(text), want) {
				t.Errorf(`page %d: missing %q`, id, want)
			}
		}
	}
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

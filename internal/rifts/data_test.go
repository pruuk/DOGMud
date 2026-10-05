package rifts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shippedDir is the authored rift data, relative to this package.
const shippedDir = `../../_datafiles/world/dogmud/rifts`

// The shipped profiles and templates parse strictly (no unknown keys) and
// pass every structural check. The world-dependent checks (mob, item and
// condition ids) run at boot, where the world is loaded.
func TestShippedRiftData_Validates(t *testing.T) {
	loaded, err := loadFrom(shippedDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) == 0 {
		t.Fatal(`no rift profiles found under ` + shippedDir)
	}
	for id, p := range loaded {
		if err := p.validate(false); err != nil {
			t.Errorf(`%s: %v`, id, err)
		}
		for _, pool := range AllPools {
			if len(p.Templates(pool)) == 0 {
				t.Errorf(`%s: pool %s has no templates`, id, pool)
			}
		}
	}
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Starting from the shipped obelisk data, break one thing at a time and make
// sure validation names it.
func TestValidate_RejectsBrokenData(t *testing.T) {
	base := map[string]string{}
	err := filepath.Walk(shippedDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, `.yaml`) {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(shippedDir, path)
		base[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, file, old, new, want string
	}{
		{`unknown key`, `rooms/obelisk/a-narrow-facet-way.yaml`, `source: authored`, "source: authored\nbogus: 1", `bogus`},
		{`title casing`, `rooms/obelisk/a-narrow-facet-way.yaml`, `title: A Narrow Facet Way`, `title: a narrow facet way`, `title case`},
		{`bad exit name`, `rooms/obelisk/b-hall-of-still-lenses.yaml`, "  - exit: gallery\n", "  - exit: gallery_x\n", `one lowercase word`},
		{`memory with a sequence`, `rooms/obelisk/c-chamber-of-four-chimes.yaml`, `kind: memory`, "kind: memory\n  sequence: [black, red]", `no answers or sequence`},
		{`too few frequencies`, `profiles/obelisk.yaml`, `symbols: "!#$%&*+-=?^~|/:;()[]{}"`, `symbols: "!#$%&*"`, `distinct`},
		{`a letter for a frequency`, `profiles/obelisk.yaml`, `symbols: "!#$%&*+-=?^~|/:;()[]{}"`, `symbols: "!#$%&*+-=?^~|/:;()[]{}x"`, `may not be used`},
		{`no failure stages`, `profiles/obelisk.yaml`, `fails: [8, 4, 2]`, `fails: []`, `fails`},
		{`a room noun takes the table`, `rooms/obelisk/c-chamber-of-four-chimes.yaml`, "nouns:\n", "nouns:\n  table: a table\n", `lens table needs it`},
		{`easy key`, `rooms/obelisk/c-chamber-of-four-chimes.yaml`, `hard: true`, `hard: false`, `hard puzzle`},
		{`a table without a key`, `rooms/obelisk/c-chamber-of-four-chimes.yaml`, `reward: key`, `reward: ore`, `rewards a key`},
		{`missing message`, `profiles/obelisk.yaml`, `  closed: >-`, `  closedx: >-`, `closed`},
		{`no encounter`, `rooms/obelisk/d-shattered-gallery.yaml`, "encounter:\n  tier: trash\n  count: {min: 2, max: 3}\n", ``, `encounter`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{}
			for k, v := range base {
				files[k] = v
			}
			if !strings.Contains(files[c.file], c.old) {
				t.Fatalf(`fixture drifted: %q not found in %s`, c.old, c.file)
			}
			files[c.file] = strings.Replace(files[c.file], c.old, c.new, 1)
			loaded, err := loadFrom(writeTree(t, files))
			if err == nil {
				for _, p := range loaded {
					if err = p.validate(false); err != nil {
						break
					}
				}
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf(`want an error mentioning %q, got %v`, c.want, err)
			}
		})
	}
}

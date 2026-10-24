package guards

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// #370: a siphon weapon is a spectral or cursed pull on life-force, so it must
// actually steal life. An item carries life steal when it has a positive
// lifesteal_pct statmod, or names a behavior tree holding a node with
// `effect: lifesteal` (see behaviors/items/blackrazor.yaml).

type siphonItemDoc struct {
	Subtype  string             `yaml:"subtype"`
	Behavior string             `yaml:"behavior"`
	StatMods map[string]float64 `yaml:"statmods"`
}

// treeHasLifestealNode reports whether any node under v (a decoded YAML value)
// carries `effect: lifesteal`.
func treeHasLifestealNode(v interface{}) bool {
	switch n := v.(type) {
	case map[interface{}]interface{}:
		if e, ok := n["effect"].(string); ok && e == "lifesteal" {
			return true
		}
		for _, child := range n {
			if treeHasLifestealNode(child) {
				return true
			}
		}
	case []interface{}:
		for _, child := range n {
			if treeHasLifestealNode(child) {
				return true
			}
		}
	}
	return false
}

// siphonItemCarriesLifeSteal decides the rule for one item document.
// behaviorYAML returns the bytes of the named item behavior tree file, or nil
// when it does not exist.
func siphonItemCarriesLifeSteal(item siphonItemDoc, behaviorYAML func(name string) []byte) bool {
	if item.StatMods["lifesteal_pct"] > 0 {
		return true
	}
	if item.Behavior == "" {
		return false
	}
	raw := behaviorYAML(item.Behavior)
	if raw == nil {
		return false
	}
	var tree interface{}
	if err := yaml.Unmarshal(raw, &tree); err != nil {
		return false
	}
	return treeHasLifestealNode(tree)
}

func TestSiphonItemCarriesLifeSteal_Decision(t *testing.T) {
	behaviors := map[string]string{
		"leech": "tree:\n  type: selector\n  children:\n    - type: action\n      event: on_hit\n      do: proc\n      effect: lifesteal\n      ratio: 0.25\n",
		"mute":  "tree:\n  type: action\n  do: speak\n  pool: on_idle\n",
	}
	lookup := func(name string) []byte {
		if s, ok := behaviors[name]; ok {
			return []byte(s)
		}
		return nil
	}
	parse := func(src string) siphonItemDoc {
		var d siphonItemDoc
		if err := yaml.Unmarshal([]byte(src), &d); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		return d
	}

	cases := []struct {
		name string
		yaml string
		want bool
	}{
		{"statmod", "subtype: siphon\nstatmods:\n  lifesteal_pct: 5\n", true},
		{"behavior tree", "subtype: siphon\nbehavior: leech\n", true},
		{"zero statmod only", "subtype: siphon\nstatmods:\n  lifesteal_pct: 0\n", false},
		{"unrelated statmod", "subtype: siphon\nstatmods:\n  strength: 5\n", false},
		{"behavior without the effect", "subtype: siphon\nbehavior: mute\n", false},
		{"missing behavior file", "subtype: siphon\nbehavior: nowhere\n", false},
		{"nothing at all", "subtype: siphon\n", false},
	}
	for _, c := range cases {
		if got := siphonItemCarriesLifeSteal(parse(c.yaml), lookup); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// No item uses the siphon subtype yet, so this walk passes vacuously today. It
// starts to bite the first time a siphon weapon ships without life steal. The
// file-count floor keeps an empty or mistyped glob from passing the same way.
func TestEverySiphonWeaponCarriesLifeSteal(t *testing.T) {
	root := filepath.Join("_datafiles", "world", "dogmud")
	behaviorYAML := func(name string) []byte {
		raw, err := os.ReadFile(filepath.Join(root, "behaviors", "items", name+".yaml"))
		if err != nil {
			return nil
		}
		return raw
	}

	files := 0
	err := filepath.Walk(filepath.Join(root, "items"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		files++
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Errorf("read %s: %v", path, rerr)
			return nil
		}
		var item siphonItemDoc
		if uerr := yaml.Unmarshal(raw, &item); uerr != nil {
			t.Errorf("unmarshal %s: %v", path, uerr)
			return nil
		}
		if item.Subtype != "siphon" {
			return nil
		}
		if !siphonItemCarriesLifeSteal(item, behaviorYAML) {
			t.Errorf("%s: subtype siphon needs a lifesteal_pct statmod above zero or a behavior with an `effect: lifesteal` node", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk items: %v", err)
	}
	if files <= 100 {
		t.Fatalf("walked only %d item files; the glob found too little to trust", files)
	}
}

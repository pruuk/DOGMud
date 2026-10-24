package guards

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/pets"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// Every loader in this codebase ignores unknown YAML keys, so a Go tag and a
// data file that disagree load as a silent zero value. For every shipped
// file that SAYS a renamed key, this proves the key reaches a non-empty Go
// field through the real struct the loader decodes into.
type keyBinding struct {
	folder string
	key    string
	bound  func(t *testing.T, path string, data []byte) bool
}

func keyLine(key string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^\s*-?\s*` + regexp.QuoteMeta(key) + `\s*:`)
}

// emptyKeyLine matches a key written as an explicitly empty flow list
// (`conditionids: []`, six caravan mobs). An empty list decodes to an empty
// slice whether or not the tag matches, so such a line carries no value a
// binding check could prove and is not counted.
func emptyKeyLine(key string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^\s*-?\s*` + regexp.QuoteMeta(key) + `\s*:\s*\[\s*\]\s*(#.*)?\r?$`)
}

func decode[T any](t *testing.T, path string, data []byte) T {
	t.Helper()
	var v T
	require.NoError(t, yaml.Unmarshal(data, &v), "decode %s", path)
	return v
}

var conditionKeyBindings = []keyBinding{
	{"conditions", "conditionid", func(t *testing.T, p string, d []byte) bool {
		id, err := strconv.Atoi(strings.SplitN(filepath.Base(p), "-", 2)[0])
		require.NoError(t, err, "condition file %s must start with its id", p)
		return decode[conditions.ConditionSpec](t, p, d).ConditionId == id
	}},
	{"conditions", "start_remove_conditions", func(t *testing.T, p string, d []byte) bool {
		return len(decode[conditions.ConditionSpec](t, p, d).StartRemoveConditions) > 0
	}},
	{"items", "conditionids", func(t *testing.T, p string, d []byte) bool {
		return len(decode[items.ItemSpec](t, p, d).ConditionIds) > 0
	}},
	{"items", "wornconditionids", func(t *testing.T, p string, d []byte) bool {
		return len(decode[items.ItemSpec](t, p, d).WornConditionIds) > 0
	}},
	{"items", "critconditionids", func(t *testing.T, p string, d []byte) bool {
		return len(decode[items.ItemSpec](t, p, d).Damage.CritConditionIds) > 0
	}},
	{"species", "conditionids", func(t *testing.T, p string, d []byte) bool {
		return len(decode[species.Species](t, p, d).ConditionIds) > 0
	}},
	{"species", "critconditionids", func(t *testing.T, p string, d []byte) bool {
		return len(decode[species.Species](t, p, d).Damage.CritConditionIds) > 0
	}},
	{"mobs", "conditionids", func(t *testing.T, p string, d []byte) bool {
		return len(decode[mobs.Mob](t, p, d).ConditionIds) > 0
	}},
	{"mobs", "conditionid", func(t *testing.T, p string, d []byte) bool {
		for _, s := range decode[mobs.Mob](t, p, d).Character.Shop {
			if s.ConditionId > 0 {
				return true
			}
		}
		return false
	}},
	{"spells", "condition_ids", func(t *testing.T, p string, d []byte) bool {
		return len(decode[spells.SpellData](t, p, d).ConditionIds) > 0
	}},
	{"mutators", "playerconditionids", func(t *testing.T, p string, d []byte) bool {
		return len(decode[mutators.MutatorSpec](t, p, d).PlayerConditionIds) > 0
	}},
	{"mutators", "mobconditionids", func(t *testing.T, p string, d []byte) bool {
		return len(decode[mutators.MutatorSpec](t, p, d).MobConditionIds) > 0
	}},
	{"rooms", "trapconditionids", func(t *testing.T, p string, d []byte) bool {
		r := decode[rooms.Room](t, p, d)
		for _, e := range r.Exits {
			if len(e.Lock.TrapConditionIds) > 0 {
				return true
			}
		}
		for _, c := range r.Containers {
			if len(c.Lock.TrapConditionIds) > 0 {
				return true
			}
		}
		return false
	}},
	{"pets", "conditionids", func(t *testing.T, p string, d []byte) bool {
		return len(decode[pets.Pet](t, p, d).ConditionIds) > 0
	}},
	{"quests", "conditionid", func(t *testing.T, p string, d []byte) bool {
		return decode[quests.Quest](t, p, d).Rewards.ConditionId > 0
	}},
}

func TestShippedConditionKeysBind(t *testing.T) {
	repo := repoRoot(t)
	checked := 0
	for _, world := range []string{"dogmud", "default"} {
		for _, b := range conditionKeyBindings {
			root := filepath.Join(repo, "_datafiles", "world", world, b.folder)
			if _, err := os.Stat(root); os.IsNotExist(err) {
				continue
			}
			re := keyLine(b.key)
			empty := emptyKeyLine(b.key)
			require.NoError(t, filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					if os.IsNotExist(err) {
						return nil
					}
					return err
				}
				if d.IsDir() || !strings.HasSuffix(p, ".yaml") {
					return nil
				}
				data, rerr := os.ReadFile(p)
				if rerr != nil {
					return rerr
				}
				// Skip a file whose every occurrence is `[]`: an empty list decodes the same whether or not the tag matches.
				said := len(re.FindAllIndex(data, -1))
				if said == 0 || said == len(empty.FindAllIndex(data, -1)) {
					return nil
				}
				checked++
				if !b.bound(t, p, data) {
					t.Errorf("%s says `%s:` but it does not reach its Go field (tag and file disagree)", p, b.key)
				}
				return nil
			}))
		}
	}
	require.GreaterOrEqual(t, checked, 290, "checked only %d files; this floor guards against broken key patterns or folders (299 provable files on 2026-09-15)", checked)
}

// A missing colour alias drops colour silently: the renderer prints the text
// uncoloured. The category strings a condition start or end line is sent with
// must be real alias keys, and the condition colour must exist in both worlds.
func TestConditionColourAliasesExist(t *testing.T) {
	repo := repoRoot(t)
	colorsOf := func(world string) map[string]any {
		data, err := os.ReadFile(filepath.Join(repo, "_datafiles", "world", world, "ansi-aliases.yaml"))
		require.NoError(t, err)
		var doc struct {
			Colors map[string]any `yaml:"colors"`
		}
		require.NoError(t, yaml.Unmarshal(data, &doc))
		require.NotEmpty(t, doc.Colors, "%s ansi-aliases.yaml has no colors: map", world)
		return doc.Colors
	}

	dogmud := colorsOf("dogmud")
	for _, key := range []string{"condition", messaging.CategoryConditionApply.String(), messaging.CategoryConditionExpire.String()} {
		require.Contains(t, dogmud, key, "dogmud ansi-aliases.yaml must define %q", key)
	}
	require.Contains(t, colorsOf("default"), "condition", "default ansi-aliases.yaml must define condition")
}

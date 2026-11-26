package guards

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Every named exit in the shipped world fits a deck button (owner ruling 18,
// 2026-11-26, touch arc spec docs/superpowers/specs/2026-11-22-moving-by-
// touch-design.md): at most exit.MaxNameLetters letters, compass words
// exempt. The room and mutator loaders refuse a longer one at boot
// (rooms.Room.Validate, mutators.MutatorSpec.Validate); this guard finds it
// in review, before a boot. Only the dogmud world is in scope; the upstream
// sample world (_datafiles/world/default) does not boot here.
//
// Temporary exits are not in YAML. The constant ones are pinned by tests in
// their packages (ferry gangplankExitName, mobcommands mobPortalExitName,
// rooms instanceReturnExitName),
// a behaviour tree's add_temp_exit name is refused at tree load, and the
// instance portal key "<zone>-<userId>" is exempt by the same ruling.

var exitNameLengthDirs = []string{
	"_datafiles/world/dogmud/rooms",
	"_datafiles/world/dogmud/mutators",
}

// exitNameLengthFindings names each exit in one room or mutator file that is
// longer than a deck button holds.
func exitNameLengthFindings(rel string, data []byte) ([]string, error) {
	var doc struct {
		Exits map[string]any `yaml:"exits"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var found []string
	for name := range doc.Exits {
		if err := exit.CheckName(name); err != nil {
			found = append(found, fmt.Sprintf("%s: %v", rel, err))
		}
	}
	sort.Strings(found)
	return found, nil
}

func TestShippedExitNamesFitAButton(t *testing.T) {
	root := repoRoot(t)
	files := map[string]int{}
	for _, dir := range exitNameLengthDirs {
		err := util.WalkDirTolerant(filepath.Join(root, filepath.FromSlash(dir)), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".yaml") || d.Name() == "zone-config.yaml" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			found, err := exitNameLengthFindings(filepath.ToSlash(rel), data)
			if err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
			files[dir]++
			for _, f := range found {
				t.Error(f)
			}
			return nil
		})
		require.NoError(t, err)
	}
	require.Greater(t, files["_datafiles/world/dogmud/rooms"], 1000, "the room walk found too few files to mean anything")
	require.Greater(t, files["_datafiles/world/dogmud/mutators"], 0, "the mutator walk found no files")
}

// The guard can fail: a seven-letter named exit is found, a six-letter one
// and a long compass word are not.
func TestExitNameLengthFindings_CanFail(t *testing.T) {
	fixture := []byte(`roomid: 1
exits:
  northeast:
    roomid: 2
  shrine:
    roomid: 3
  outlook:
    roomid: 4
`)
	found, err := exitNameLengthFindings("rooms/x/1.yaml", fixture)
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Contains(t, found[0], `"outlook"`)
	require.Contains(t, found[0], "rooms/x/1.yaml")
}

package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// No shipped room template sets rotationseed on an exit's lock (#781, owner
// ruling 2026-11-25: every relock gets a new combination).
//
// The ASCII map judges a known combination against an unloaded room's exit
// lock without touching disk (rooms.ExitLockAt). For a room not yet loaded
// this boot it has only what the map node carries, the lock's difficulty, and
// takes rotation 0. That is the template's real rotation only while no
// template sets one on an exit. Container locks may set it (the forager
// lockboxes do); the map never reads them.

const exitRotationSeedRoomsDir = "_datafiles/world/dogmud/rooms"

// exitRotationSeedFindings names each exit in one room template whose lock
// sets rotationseed.
func exitRotationSeedFindings(rel string, data []byte) ([]string, error) {
	var room struct {
		Exits map[string]struct {
			Lock map[string]any `yaml:"lock"`
		} `yaml:"exits"`
	}
	if err := yaml.Unmarshal(data, &room); err != nil {
		return nil, err
	}
	var found []string
	for name, ex := range room.Exits {
		for key := range ex.Lock {
			if strings.EqualFold(key, "rotationseed") {
				found = append(found, fmt.Sprintf("%s: exit %q sets rotationseed on its lock", rel, name))
			}
		}
	}
	sort.Strings(found)
	return found, nil
}

func TestNoRoomTemplateSetsAnExitRotationSeed(t *testing.T) {
	root := filepath.Join(repoRoot(t), filepath.FromSlash(exitRotationSeedRoomsDir))
	files := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".yaml") || info.Name() == "zone-config.yaml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoRoot(t), path)
		found, err := exitRotationSeedFindings(filepath.ToSlash(rel), data)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		files++
		for _, f := range found {
			t.Error(f + "; the map reads an unloaded exit at rotation 0 (rooms.ExitLockAt)")
		}
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, files, 1000, "the room template walk found too few files to mean anything")
}

func TestExitRotationSeedFindings_CanFail(t *testing.T) {
	fixture := []byte(`roomid: 1
exits:
  north:
    roomid: 2
    lock:
      difficulty: 4
      rotationseed: 1
  south:
    roomid: 3
    lock:
      difficulty: 4
containers:
  lockbox:
    lock:
      difficulty: 10
      rotationseed: 1
`)
	got, err := exitRotationSeedFindings("fixture.yaml", fixture)
	require.NoError(t, err)
	require.Equal(t, []string{`fixture.yaml: exit "north" sets rotationseed on its lock`}, got,
		"the exit lock must be found and the container lock left alone")
}

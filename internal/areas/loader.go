package areas

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"gopkg.in/yaml.v3"
)

// reg is the loaded registry. It is swapped whole on a load, so readers on
// any goroutine see either the old set or the new one.
var reg atomic.Pointer[registry]

func current() *registry {
	if r := reg.Load(); r != nil {
		return r
	}
	return emptyRegistry
}

var emptyRegistry = newRegistry()

// DataPath is the directory the area files live in: <DataFiles>/areas.
func DataPath() string {
	return filepath.Join(configs.GetFilePathsConfig().DataFiles.String(), "areas")
}

// LoadDataFiles reads every *.yaml in DataPath(), resolves each area to its
// template rooms through roomZones (template room id -> zone, every loaded
// room) and installs the result. Warnings go to the log. On an error the
// previous registry stays installed and the error is returned: the boot
// refuses on it, a reload reports it.
func LoadDataFiles(roomZones map[int]string) error {
	dir := DataPath()
	r, warns, err := loadDir(dir, roomZones)
	if err != nil {
		return fmt.Errorf("areas: %s: %w", dir, err)
	}
	for _, w := range warns {
		mudlog.Warn("areas.LoadDataFiles()", "warning", w)
	}
	reg.Store(r)
	mudlog.Info("areas.LoadDataFiles()", "loadedCount", len(r.areas))
	return nil
}

// loadDir reads and resolves the area files in dir. A missing directory
// loads no areas (a world with none).
func loadDir(dir string, roomZones map[int]string) (*registry, []string, error) {
	r := newRegistry()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return r, nil, nil
		}
		return nil, nil, err
	}
	var warns []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, nil, err
		}
		var a Area
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&a); err != nil && !errors.Is(err, io.EOF) {
			return nil, nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if want := strings.TrimSuffix(e.Name(), ".yaml"); a.Key != want {
			return nil, nil, fmt.Errorf("%s: key %q is not the file name %q", e.Name(), a.Key, want)
		}
		set, w, err := resolve(a, roomZones)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		warns = append(warns, w...)
		r.add(a, set)
	}
	return r, warns, nil
}

// Get returns the area with this key.
func Get(key string) (Area, bool) {
	a, ok := current().areas[key]
	return a, ok
}

// Contains reports whether the template room is in the area. An unknown key
// contains nothing.
func Contains(key string, roomId int) bool { return current().contains(key, roomId) }

// AreasOf lists the keys of every area holding the template room, sorted.
// The slice is shared: do not change it.
func AreasOf(roomId int) []string { return current().areasOf(roomId) }

// Rooms lists the area's template room ids, sorted. Nil for an unknown key.
func Rooms(key string) []int {
	set := current().rooms[key]
	if set == nil {
		return nil
	}
	out := make([]int, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

// Keys lists every loaded area key, sorted.
func Keys() []string {
	r := current()
	out := make([]string, 0, len(r.areas))
	for k := range r.areas {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// InstallForTest installs these areas, resolved over roomZones, for another
// package's test, and returns a func that restores the previous registry. It
// panics on an area that would not load.
func InstallForTest(list []Area, roomZones map[int]string) (restore func()) {
	r := newRegistry()
	for _, a := range list {
		set, _, err := resolve(a, roomZones)
		if err != nil {
			panic(err)
		}
		r.add(a, set)
	}
	prev := reg.Swap(r)
	return func() { reg.Store(prev) }
}

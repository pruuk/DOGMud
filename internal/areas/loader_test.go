package areas

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeArea(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDirGoodFile(t *testing.T) {
	dir := t.TempDir()
	writeArea(t, dir, "grove.yaml", "key: grove\nname: the Grove\nrooms: [5313, 5311]\nzones: [thornwall]\n")
	writeArea(t, dir, "area-keys.txt", "grove\n") // not a yaml file: ignored
	r, warns, err := loadDir(dir, testRooms())
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings %v", warns)
	}
	if !r.contains("grove", 5311) || !r.contains("grove", 101) || r.contains("grove", 5312) {
		t.Errorf("grove resolved wrong: %v", r.rooms["grove"])
	}
}

func TestLoadDirRefuses(t *testing.T) {
	for _, c := range []struct {
		file, body, want string
	}{
		{"grove.yaml", "key: wood\nname: the Wood\nrooms: [5311]\n", `key "wood" is not the file name "grove"`},
		{"grove.yaml", "key: grove\nname: the Grove\nroom: [5311]\n", "field room not found"},
		{"grove.yaml", "key: grove\nname: the Grove\nrooms: [9]\n", "unknown room 9"},
	} {
		dir := t.TempDir()
		writeArea(t, dir, c.file, c.body)
		if _, _, err := loadDir(dir, testRooms()); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want %q, got %v", c.body, c.want, err)
		}
	}
}

func TestLoadDirMissingIsEmpty(t *testing.T) {
	r, _, err := loadDir(filepath.Join(t.TempDir(), "absent"), testRooms())
	if err != nil || len(r.areas) != 0 {
		t.Fatalf("a missing directory should load no areas, got %v %v", r.areas, err)
	}
}

func TestPublicAPI(t *testing.T) {
	restore := InstallForTest([]Area{
		{Key: "grove", Name: "the Grove", Rooms: []int{5313, 5311, 5312}},
		{Key: "thorns", Name: "the Thorns", Zones: []string{"thornwall"}},
	}, testRooms())
	defer restore()
	if a, ok := Get("grove"); !ok || a.Name != "the Grove" {
		t.Errorf("Get(grove) = %v %v", a, ok)
	}
	if _, ok := Get("nope"); ok {
		t.Error("Get(nope) found an area")
	}
	if !Contains("thorns", 100) || Contains("thorns", 5311) || Contains("nope", 100) {
		t.Error("Contains wrong")
	}
	if got := AreasOf(5312); !reflect.DeepEqual(got, []string{"grove"}) {
		t.Errorf("AreasOf(5312) = %v", got)
	}
	if got := Rooms("grove"); !reflect.DeepEqual(got, []int{5311, 5312, 5313}) {
		t.Errorf("Rooms(grove) = %v", got)
	}
	if got := Keys(); !reflect.DeepEqual(got, []string{"grove", "thorns"}) {
		t.Errorf("Keys() = %v", got)
	}
}

func TestZonesOfAndRoomsIn(t *testing.T) {
	restore := InstallForTest([]Area{
		{Key: "grove", Name: "the Grove", Zones: []string{"thornwall"}, Rooms: []int{5313, 5311}},
	}, testRooms())
	defer restore()
	if got := ZonesOf("grove"); !reflect.DeepEqual(got, []string{"pothole_coulee", "thornwall"}) {
		t.Errorf("ZonesOf(grove) = %v", got)
	}
	if got := RoomsIn("grove", "pothole_coulee"); !reflect.DeepEqual(got, []int{5311, 5313}) {
		t.Errorf("RoomsIn(grove, pothole_coulee) = %v", got)
	}
	if got := RoomsIn("grove", "nowhere"); got != nil {
		t.Errorf("RoomsIn on a zone the area misses = %v, want nil", got)
	}
	if ZonesOf("nope") != nil || RoomsIn("nope", "thornwall") != nil {
		t.Error("an unknown key has no zones and no rooms")
	}
}

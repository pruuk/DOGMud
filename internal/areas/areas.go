// Package areas holds the named sets of rooms that quests and the minimap
// refer to (#375 phase 3b slice 4): one YAML file per area under
// _datafiles/world/dogmud/areas/, each listing whole zones and/or single
// rooms, resolved to template room ids at boot.
package areas

import (
	"fmt"
	"sort"
)

// Area is one area file.
type Area struct {
	Key   string   `yaml:"key"`
	Name  string   `yaml:"name"` // player-facing: "the Grove"
	Zones []string `yaml:"zones,omitempty"`
	Rooms []int    `yaml:"rooms,omitempty"`
}

type registry struct {
	areas  map[string]Area
	rooms  map[string]map[int]bool     // key -> template room ids
	byRoom map[int][]string            // template room id -> area keys, sorted
	zones  map[string][]string         // key -> zones its rooms lie in, sorted (#375 phase 4)
	inZone map[string]map[string][]int // key -> zone -> its rooms there, sorted
}

func newRegistry() *registry {
	return &registry{areas: map[string]Area{}, rooms: map[string]map[int]bool{}, byRoom: map[int][]string{},
		zones: map[string][]string{}, inZone: map[string]map[string][]int{}}
}

func (r *registry) add(a Area, set map[int]bool, roomZones map[int]string) {
	r.areas[a.Key] = a
	r.rooms[a.Key] = set
	byZone := map[string][]int{}
	for id := range set {
		r.byRoom[id] = append(r.byRoom[id], a.Key)
		sort.Strings(r.byRoom[id])
		z := roomZones[id]
		byZone[z] = append(byZone[z], id)
	}
	zs := make([]string, 0, len(byZone))
	for z, ids := range byZone {
		sort.Ints(ids)
		zs = append(zs, z)
	}
	sort.Strings(zs)
	r.zones[a.Key] = zs
	r.inZone[a.Key] = byZone
}

func (r *registry) contains(key string, roomId int) bool { return r.rooms[key][roomId] }
func (r *registry) areasOf(roomId int) []string          { return r.byRoom[roomId] }

// resolve turns an area into its room set. roomZones maps every loaded
// template room to its zone. It refuses an unnamed area, an unknown zone or
// room, and an area with no rooms; it warns on a room already inside one of
// the area's zones.
func resolve(a Area, roomZones map[int]string) (map[int]bool, []string, error) {
	if a.Name == "" {
		return nil, nil, fmt.Errorf("area %q has no name", a.Key)
	}
	zones := map[string]bool{}
	for _, z := range a.Zones {
		zones[z] = true
	}
	set := map[int]bool{}
	found := map[string]bool{}
	for id, z := range roomZones {
		if zones[z] {
			set[id] = true
			found[z] = true
		}
	}
	for _, z := range a.Zones {
		if !found[z] {
			return nil, nil, fmt.Errorf("area %q: unknown zone %q", a.Key, z)
		}
	}
	var warns []string
	for _, id := range a.Rooms {
		z, ok := roomZones[id]
		if !ok {
			return nil, nil, fmt.Errorf("area %q: unknown room %d", a.Key, id)
		}
		if zones[z] {
			warns = append(warns, fmt.Sprintf("area %q: room %d is already in zone %s", a.Key, id, z))
		}
		set[id] = true
	}
	if len(set) == 0 {
		return nil, nil, fmt.Errorf("area %q resolves to no rooms", a.Key)
	}
	return set, warns, nil
}

package quests

import (
	"fmt"
	"slices"
)

// DestinationAnywhere is the scalar a step writes when it can be done
// anywhere (#375 phase 4): no marker, and no unknown-destination warning.
const DestinationAnywhere = "anywhere"

// StepDestination is where a quest step points the minimap and the quest
// log's destination line (#375 phase 4). The zero value means no block: the
// destination is inferred from the step's triggers. Shapes: anywhere; room;
// area; zone; or mob, alone or with one of room, area, zone as its fallback.
// Mobs holds one NPC or several, any one of which will do (owner ruling 16);
// on disk the key is mob, a single id or a list. Not comparable: use IsZero
// or reflect.DeepEqual. questengine.ResolveDestination resolves it.
type StepDestination struct {
	Anywhere bool   `json:"anywhere,omitempty"`
	Room     int    `json:"room,omitempty"`
	Mobs     []int  `json:"mob,omitempty"`
	Area     string `json:"area,omitempty"`
	Zone     string `json:"zone,omitempty"`
}

// destinationFields is the mapping form on disk.
type destinationFields struct {
	Room int    `yaml:"room,omitempty"`
	Mob  mobIds `yaml:"mob,omitempty"`
	Area string `yaml:"area,omitempty"`
	Zone string `yaml:"zone,omitempty"`
}

// mobIds is the mob key on disk: a single id, or a list of ids.
type mobIds []int

// UnmarshalYAML takes `mob: 94` or `mob: [9316, 9300]`. An empty list decodes
// as an empty, non-nil list so Validate refuses it rather than reading it as
// no mob at all.
func (m *mobIds) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var one int
	if err := unmarshal(&one); err == nil {
		*m = mobIds{one}
		return nil
	}
	var list []int
	if err := unmarshal(&list); err != nil {
		return fmt.Errorf("mob: want one mob id or a list of mob ids: %w", err)
	}
	*m = append(mobIds{}, list...)
	return nil
}

// MarshalYAML writes one id as a single id and several as a list, so a file
// written with `mob: 94` stays a fixed point.
func (m mobIds) MarshalYAML() (interface{}, error) {
	if len(m) == 1 {
		return m[0], nil
	}
	return []int(m), nil
}

var destinationKeys = []string{"room", "mob", "area", "zone"}

// IsZero reports whether the step declares no destination. A non-nil empty
// Mobs (`mob: []`) is a declared, invalid block, not an absent one.
func (d StepDestination) IsZero() bool {
	return !d.Anywhere && d.Room == 0 && d.Mobs == nil && d.Area == "" && d.Zone == ""
}

// UnmarshalYAML takes the scalar anywhere or a mapping of room, mob, area and
// zone; any other word or key refuses the file.
func (d *StepDestination) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var word string
	if err := unmarshal(&word); err == nil {
		if word != DestinationAnywhere {
			return fmt.Errorf("destination %q: the only word a destination takes is %q; otherwise give room, mob, area or zone", word, DestinationAnywhere)
		}
		*d = StepDestination{Anywhere: true}
		return nil
	}
	var raw map[string]interface{}
	if err := unmarshal(&raw); err != nil {
		return fmt.Errorf("destination: want %q or a mapping of room, mob, area, zone: %w", DestinationAnywhere, err)
	}
	for k := range raw {
		if !slices.Contains(destinationKeys, k) {
			return fmt.Errorf("destination: unknown key %q (use room, mob, area or zone)", k)
		}
	}
	var f destinationFields
	if err := unmarshal(&f); err != nil {
		return err
	}
	*d = StepDestination{Room: f.Room, Mobs: []int(f.Mob), Area: f.Area, Zone: f.Zone}
	return nil
}

// MarshalYAML writes anywhere as the scalar and anything else as the mapping.
func (d StepDestination) MarshalYAML() (interface{}, error) {
	if d.Anywhere {
		return DestinationAnywhere, nil
	}
	return destinationFields{Room: d.Room, Mob: mobIds(d.Mobs), Area: d.Area, Zone: d.Zone}, nil
}

// Validate checks the shape. Existence of the room, mob, area and zone is
// ValidateQuestRefs' job.
func (d StepDestination) Validate() error {
	places := 0
	for _, set := range []bool{d.Room != 0, d.Area != "", d.Zone != ""} {
		if set {
			places++
		}
	}
	switch {
	case d.Anywhere && (places > 0 || len(d.Mobs) != 0):
		return fmt.Errorf("anywhere takes no room, mob, area or zone")
	case d.Mobs != nil && len(d.Mobs) == 0:
		return fmt.Errorf("mob is an empty list; give one mob id or several")
	case d.Room < 0 || slices.ContainsFunc(d.Mobs, func(m int) bool { return m <= 0 }):
		return fmt.Errorf("room and mob must be positive ids")
	case places > 1:
		return fmt.Errorf("names %d of room, area and zone; give one (a mob may add one as its fallback)", places)
	}
	return nil
}

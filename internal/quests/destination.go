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
// questengine.ResolveDestination resolves it.
type StepDestination struct {
	Anywhere bool   `json:"anywhere,omitempty"`
	Room     int    `json:"room,omitempty"`
	Mob      int    `json:"mob,omitempty"`
	Area     string `json:"area,omitempty"`
	Zone     string `json:"zone,omitempty"`
}

// destinationFields is the mapping form on disk.
type destinationFields struct {
	Room int    `yaml:"room,omitempty"`
	Mob  int    `yaml:"mob,omitempty"`
	Area string `yaml:"area,omitempty"`
	Zone string `yaml:"zone,omitempty"`
}

var destinationKeys = []string{"room", "mob", "area", "zone"}

// IsZero reports whether the step declares no destination.
func (d StepDestination) IsZero() bool { return d == StepDestination{} }

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
	*d = StepDestination{Room: f.Room, Mob: f.Mob, Area: f.Area, Zone: f.Zone}
	return nil
}

// MarshalYAML writes anywhere as the scalar and anything else as the mapping.
func (d StepDestination) MarshalYAML() (interface{}, error) {
	if d.Anywhere {
		return DestinationAnywhere, nil
	}
	return destinationFields{Room: d.Room, Mob: d.Mob, Area: d.Area, Zone: d.Zone}, nil
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
	case d.Anywhere && (places > 0 || d.Mob != 0):
		return fmt.Errorf("anywhere takes no room, mob, area or zone")
	case d.Room < 0 || d.Mob < 0:
		return fmt.Errorf("room and mob must be positive ids")
	case places > 1:
		return fmt.Errorf("names %d of room, area and zone; give one (a mob may add one as its fallback)", places)
	}
	return nil
}

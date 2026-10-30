package rooms

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/util"
)

type ZoneConfig struct {
	Name           string               `yaml:"name,omitempty"`
	RoomId         int                  `yaml:"roomid,omitempty"`
	Mutators       mutators.MutatorList `yaml:"mutators,omitempty"`        // mutators defined here apply to entire zone
	IdleMessages   []string             `yaml:"idlemessages,omitempty"`    // list of messages that can be displayed to players in the zone, assuming a room has none defined
	MusicFile      string               `yaml:"musicfile,omitempty"`       // background music to play when in this zone
	DefaultBiome   string               `yaml:"defaultbiome,omitempty"`    // city, swamp etc. see biomes.go
	Region         string               `yaml:"region,omitempty"`          // Geographic region for world event filtering (e.g. "Thornwall Region")
	Instanced      bool                 `yaml:"instanced,omitempty"`       // true = this zone is an instance template
	DeathPolicy    string               `yaml:"death_policy,omitempty"`    // "rejoin" (default) or "ejected"
	PortalDuration string               `yaml:"portal_duration,omitempty"` // e.g. "30m", "1h"
	EntryRoom      int                  `yaml:"entry_room,omitempty"`      // room ID where portal drops players
	AllowRecall    bool                 `yaml:"allow_recall,omitempty"`    // whether recall works inside (default true)
	InstanceGold   bool                 `yaml:"instance_gold,omitempty"`   // true = mobs in a paid instance of this zone carry gold scaled by the entry (InstanceMobGold); false keeps template gold
	NonCartesian   bool                 `yaml:"non_cartesian,omitempty"`   // true = zone is intentionally non-Euclidean (toroidal/maze); skip Cartesian consistency checks and render wrap exits as edge stubs
	DefaultPlane   int                  `yaml:"default_plane,omitempty"`   // builder convenience: default plane for new rooms in this zone (rooms' own plane is authoritative)
	// AmbientMultiplier scales this zone's idle mob chatter roll and room
	// idle chance and divides the ambient budget window (#317): 1.5 is
	// livelier, 0.5 hushed. WanderMultiplier scales idle mob movement here.
	// Pointers: unset (nil) is 1.0 and 0 is never.
	AmbientMultiplier *float64         `yaml:"ambientmultiplier,omitempty"`
	WanderMultiplier  *float64         `yaml:"wandermultiplier,omitempty"`
	RoomIds           map[int]struct{} `yaml:"-"` // Does not get written. Built dyanmically when rooms are loaded.
}

func (z *ZoneConfig) Id() string {
	return z.Name
}

func (z *ZoneConfig) Validate() error {
	if z.RoomIds == nil {
		z.RoomIds = make(map[int]struct{})
	}

	if z.DeathPolicy == "" {
		z.DeathPolicy = "rejoin"
	}

	if z.Instanced && z.PortalDuration == "" {
		z.PortalDuration = "30 real minutes"
	}

	if z.AmbientMultiplier != nil && *z.AmbientMultiplier < 0 {
		return fmt.Errorf("zone %q: ambientmultiplier must be 0 or more, got %v", z.Name, *z.AmbientMultiplier)
	}
	if z.WanderMultiplier != nil && *z.WanderMultiplier < 0 {
		return fmt.Errorf("zone %q: wandermultiplier must be 0 or more, got %v", z.Name, *z.WanderMultiplier)
	}

	return nil
}

// ZoneAmbientMultiplier is zone's ambientmultiplier, 1.0 when the zone or
// the field is unset (#317). An instance clone keeps its template room's
// Zone, so a paid instance reads its template zone's value.
func ZoneAmbientMultiplier(zone string) float64 {
	if z := GetZoneConfig(zone); z != nil && z.AmbientMultiplier != nil {
		return *z.AmbientMultiplier
	}
	return 1.0
}

// ZoneWanderMultiplier is zone's wandermultiplier, 1.0 when unset (#317).
func ZoneWanderMultiplier(zone string) float64 {
	if z := GetZoneConfig(zone); z != nil && z.WanderMultiplier != nil {
		return *z.WanderMultiplier
	}
	return 1.0
}

func (z *ZoneConfig) Filename() string {
	return "zone-config.yaml"
}

func (z *ZoneConfig) Filepath() string {
	zone := ZoneNameSanitize(z.Name)
	return util.FilePath(zone, `/`, z.Filename())
}

func NewZoneConfig(zName string) *ZoneConfig {
	return &ZoneConfig{
		Name:        zName,
		RoomIds:     map[int]struct{}{},
		DeathPolicy: "rejoin",
		AllowRecall: true,
	}
}

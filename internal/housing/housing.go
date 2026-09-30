// Package housing owns persistent player housing: the authored buildings a
// player can take a room in, the living-state record of who owns which room,
// the landlord purchase, and the routing that sends each lodger through a
// building's one shared door to their own room.
//
// Two kinds of data, deliberately kept apart:
//
//   - Buildings are AUTHORED content, one YAML per building under
//     <DataFiles>/housing_buildings/. They name the door, the landlord, the
//     city faction whose standing gates a purchase, the price of each tier,
//     and the pool of blank unit rooms (ordinary authored rooms) the building
//     lets out. A bad building file panics at boot, like any authored content.
//
//   - Houses are LIVING STATE, one YAML per house under
//     <DataFiles>/housing/<buildingid>/<entryroomid>.yaml. They record who owns
//     which unit rooms. They follow the living-state contract
//     (internal/util/livingstate.go): durable atomic writes, absent is not
//     corrupt, a corrupt file is quarantined and never deleted, and every
//     change is persisted before it is published to the in-memory registry.
//
// The unit rooms themselves are real authored rooms, so the room engine
// (lighting, biome, saving the floor, the mapper) treats them like any other.
// A house is a list of room ids, so a later tier can own more than one.
package housing

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/opinions"
)

// Tier is one size of house a building offers.
type Tier struct {
	TierId string `yaml:"tier_id"`
	Name   string `yaml:"name"`  // what the landlord calls it: "a simple room"
	Price  int    `yaml:"price"` // gold, paid once
	Rooms  int    `yaml:"rooms"` // unit rooms a house of this tier occupies
}

// Building is one authored lodging house.
type Building struct {
	BuildingId string `yaml:"building_id"`
	Name       string `yaml:"name"` // "the Back Court lodgings"

	// DoorRoom and DoorExit name the one shared door. The exit is authored in
	// DoorRoom and locked so mobs and strangers cannot use it; the router
	// sends each owner through it to their own entry room.
	DoorRoom int    `yaml:"door_room"`
	DoorExit string `yaml:"door_exit"`

	// LandlordMobId is the NPC who lets the rooms (behavior-tree action
	// buy_housing). Only used for naming him in messages and validation.
	LandlordMobId int `yaml:"landlord_mob_id"`

	// Faction and MinRepTier gate a purchase on the buyer's standing with the
	// city (internal/factions). MinRepTier is one of hostile, cold, neutral,
	// warm, friendly.
	Faction    string `yaml:"faction"`
	MinRepTier string `yaml:"min_rep_tier"`

	Tiers []Tier `yaml:"tiers"`

	// UnitRooms is the pool of blank authored rooms this building lets out.
	// Each must have an exit named DoorExit leading back to DoorRoom.
	UnitRooms []int `yaml:"unit_rooms,flow"`
}

func (b Building) Id() string       { return b.BuildingId }
func (b Building) Filepath() string { return b.BuildingId + `.yaml` }

// Validate covers intrinsic checks. World checks (rooms, mob and faction
// exist) run in LoadBuildings once those are loaded.
func (b Building) Validate() error {
	if b.BuildingId == `` {
		return fmt.Errorf(`housing building missing building_id`)
	}
	if b.Name == `` {
		return fmt.Errorf(`housing building %s: missing name`, b.BuildingId)
	}
	if b.DoorRoom <= 0 || b.DoorExit == `` {
		return fmt.Errorf(`housing building %s: door_room and door_exit are required`, b.BuildingId)
	}
	if b.LandlordMobId <= 0 {
		return fmt.Errorf(`housing building %s: landlord_mob_id is required`, b.BuildingId)
	}
	if b.Faction == `` {
		return fmt.Errorf(`housing building %s: faction is required`, b.BuildingId)
	}
	if _, ok := ParseRepTier(b.MinRepTier); !ok {
		return fmt.Errorf(`housing building %s: min_rep_tier %q is not one of hostile, cold, neutral, warm, friendly`, b.BuildingId, b.MinRepTier)
	}
	if len(b.Tiers) == 0 {
		return fmt.Errorf(`housing building %s: at least one tier is required`, b.BuildingId)
	}
	seenTier := map[string]bool{}
	for _, t := range b.Tiers {
		if t.TierId == `` || t.Name == `` {
			return fmt.Errorf(`housing building %s: every tier needs tier_id and name`, b.BuildingId)
		}
		if seenTier[t.TierId] {
			return fmt.Errorf(`housing building %s: duplicate tier %s`, b.BuildingId, t.TierId)
		}
		seenTier[t.TierId] = true
		if t.Price <= 0 {
			return fmt.Errorf(`housing building %s: tier %s price must be positive`, b.BuildingId, t.TierId)
		}
		if t.Rooms < 1 {
			return fmt.Errorf(`housing building %s: tier %s must occupy at least one room`, b.BuildingId, t.TierId)
		}
	}
	if len(b.UnitRooms) == 0 {
		return fmt.Errorf(`housing building %s: unit_rooms is empty`, b.BuildingId)
	}
	seenRoom := map[int]bool{}
	for _, id := range b.UnitRooms {
		if id <= 0 || seenRoom[id] {
			return fmt.Errorf(`housing building %s: unit room %d is invalid or listed twice`, b.BuildingId, id)
		}
		if id == b.DoorRoom {
			return fmt.Errorf(`housing building %s: the door room cannot also be a unit`, b.BuildingId)
		}
		seenRoom[id] = true
	}
	return nil
}

// Tier returns the named tier.
func (b Building) Tier(tierId string) (Tier, bool) {
	for _, t := range b.Tiers {
		if t.TierId == tierId {
			return t, true
		}
	}
	return Tier{}, false
}

// MinTier returns the parsed minimum standing. Validate guarantees it parses.
func (b Building) MinTier() opinions.Tier {
	t, _ := ParseRepTier(b.MinRepTier)
	return t
}

// ParseRepTier maps a tier name to an opinions.Tier.
func ParseRepTier(name string) (opinions.Tier, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case `hostile`:
		return opinions.TierHostile, true
	case `cold`:
		return opinions.TierCold, true
	case `neutral`:
		return opinions.TierNeutral, true
	case `warm`:
		return opinions.TierWarm, true
	case `friendly`:
		return opinions.TierFriendly, true
	}
	return opinions.TierNeutral, false
}

// House is one player's home in one building. It is living state.
type House struct {
	BuildingId string `yaml:"building_id"`
	// OwnerUserId is the ACCOUNT (UserRecord) id, so a house is shared by an
	// account's alts, exactly like the bank.
	OwnerUserId int `yaml:"owner_user_id"`
	// OwnerName is the character that bought it. For staff reading the file;
	// never used to decide access.
	OwnerName string `yaml:"owner_name"`
	TierId    string `yaml:"tier_id"`
	// RoomIds are the unit rooms this house occupies. RoomIds[0] is the entry
	// room the door leads to. Later tiers append rooms here.
	RoomIds     []int     `yaml:"room_ids,flow"`
	PricePaid   int       `yaml:"price_paid"`
	PurchasedAt time.Time `yaml:"purchased_at"`
}

// EntryRoom is the room the building door leads this owner to.
func (h House) EntryRoom() int {
	if len(h.RoomIds) == 0 {
		return 0
	}
	return h.RoomIds[0]
}

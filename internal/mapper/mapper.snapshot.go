package mapper

import "github.com/GoMudEngine/GoMud/internal/rooms"

// SnapshotExit is one classified spatial edge for the web map renderer.
type SnapshotExit struct {
	ToRoomId int      `json:"to"`
	DX       int      `json:"dx"`
	DY       int      `json:"dy"`
	DZ       int      `json:"dz"`
	Kind     ExitKind `json:"kind"`
	Locked   bool     `json:"locked,omitempty"`
	Secret   bool     `json:"secret,omitempty"`
	OneWay   bool     `json:"oneway,omitempty"`
	Gate     bool     `json:"gate,omitempty"`
	Stub     bool     `json:"stub,omitempty"`   // destination not a visited same-zone room
	ToZone   string   `json:"tozone,omitempty"` // set when the destination is in a different zone
}

// SnapshotRoom is one room placed in the zone coordinate space.
type SnapshotRoom struct {
	RoomId int            `json:"num"`
	X      int            `json:"x"`
	Y      int            `json:"y"`
	Z      int            `json:"z"`
	Plane  int            `json:"plane"` // coordinate-space id; the builder filters/renders one plane at a time
	Symbol string         `json:"symbol"`
	Biome  string         `json:"biome"`
	Name   string         `json:"name"`           // room title (client tooltip / identify)
	Tags   []string       `json:"tags,omitempty"` // service tags: bank | storage | trainer | shop
	Exits  []SnapshotExit `json:"exits"`
	// Builder-only: set for rooms that belong to a DIFFERENT zone than the one
	// being edited, but sit on the same plane near its edge — drawn as dimmed
	// context so the builder can see (and not collide with) a neighbour.
	Foreign   bool   `json:"foreign,omitempty"`
	OwnerZone string `json:"ozone,omitempty"`
}

// Snapshot returns the visited rooms of this zone with classified exits. Exits
// to visited same-zone rooms are full edges; exits to unvisited/uncrawled rooms
// are emitted with Stub=true (a fog-of-war hint toward the unexplored region),
// and ToZone is set when the destination is in a different zone. The exit Kind
// drives client rendering (normal/long/wrap/vertical).
//
// Undiscovered secret exits are omitted entirely, matching GetLimitedMap's
// behaviour for the ASCII map. See the comment at the gate below.
func (r *mapper) Snapshot(visited map[int]struct{}) []SnapshotRoom {
	out := make([]SnapshotRoom, 0, len(visited))

	// Iteration order is nondeterministic; that's fine — the client places rooms
	// by (x,y,z), not by slice order.
	for id, n := range r.crawledRooms {
		if _, ok := visited[id]; !ok {
			continue
		}
		// Ephemeral/instance rooms (dungeon instances, the Mending Hut copy,
		// etc.) are not part of the static zone map — a crawl that stepped into
		// one must not leak its billion-range id into the client snapshot
		// (mirrors mapper.consistency's roomCrawlable exclusion).
		if rooms.IsEphemeralRoomId(id) {
			continue
		}

		sym := n.Symbol
		if sym == 0 {
			sym = defaultMapSymbol
		}
		sr := SnapshotRoom{
			RoomId: id,
			X:      n.Pos.x,
			Y:      n.Pos.y,
			Z:      n.Pos.z,
			Plane:  n.Plane,
			Symbol: string(sym),
		}
		// Biome name comes from the room's biome (not n.Legend, which may hold a
		// per-room MapLegend override like "Townsquare"); the client uses it for tinting.
		//
		// NEVER load a room here. This runs on the event loop for every room
		// change a web-client player makes, over every visited room the crawl
		// reached (790 to 1,209 on the overworld). A room idle unloading has
		// dropped used to be read and parsed from disk per call: ~0.17 ms each,
		// ~150 ms for 960 cold rooms locally and far more on the droplet, the
		// "lag when moving area to area" the owner saw.
		//
		// A loaded room is read live, exactly as before. An unloaded room reads
		// the template fields its mapNode copied at crawl time; it would have
		// come back from disk with those same values (all are instance:"skip"),
		// and with no mobs, because unloading destroys them and LoadRoom does
		// not respawn them, so it never carried the live-only "shop" tag either.
		srcZone := ""
		if rooms.IsRoomLoaded(id) {
			if room := rooms.LoadRoom(id); room != nil { // memory hit, no disk
				sr.Name = room.Title
				srcZone = room.Zone
				if b := room.GetBiome(); b != nil {
					sr.Biome = b.Name
				}
				// Service tags so the client can highlight important rooms.
				if room.IsBank {
					sr.Tags = append(sr.Tags, "bank")
				}
				if room.IsStorage {
					sr.Tags = append(sr.Tags, "storage")
				}
				if len(room.GetMobs(rooms.FindMerchant)) > 0 {
					sr.Tags = append(sr.Tags, "shop")
				}
			}
		} else {
			sr.Name = n.Title
			srcZone = n.Zone
			// A throwaway Room so the biome resolves through Room.GetBiome
			// itself (zone default when unset, then the "default" biome),
			// without a copy of that rule here.
			if b := (&rooms.Room{Biome: n.Biome, Zone: n.Zone}).GetBiome(); b != nil {
				sr.Biome = b.Name
			}
			if n.IsBank {
				sr.Tags = append(sr.Tags, "bank")
			}
			if n.IsStorage {
				sr.Tags = append(sr.Tags, "storage")
			}
		}

		for _, e := range n.Exits {
			// Don't render an exit stub toward an ephemeral/instance room.
			if rooms.IsEphemeralRoomId(e.RoomId) {
				continue
			}
			// A secret exit is not disclosed until the player has been through
			// it. GetLimitedMap does exactly this for the ASCII map (see the
			// exitInfo.Secret block there): it skips the exit entirely unless
			// the room on the far side has been visited. Without the same gate
			// the web map revealed secret exits the ASCII map hides, which is a
			// real advantage rather than a cosmetic difference. Skipping the
			// whole exit (rather than just clearing the Secret flag) is
			// deliberate: emitting an unflagged stub would still tell the
			// player a passage is there.
			if e.Secret {
				if _, seen := visited[e.RoomId]; !seen {
					continue
				}
			}
			se := SnapshotExit{
				ToRoomId: e.RoomId,
				DX:       e.Direction.x,
				DY:       e.Direction.y,
				DZ:       e.Direction.z,
				Locked:   e.LockDifficulty > 0,
				Secret:   e.Secret,
				OneWay:   e.OneWay,
				Gate:     e.Gate,
			}
			dst, crawled := r.crawledRooms[e.RoomId]
			_, vis := visited[e.RoomId]
			if crawled && vis {
				actual := positionDelta{x: dst.Pos.x - n.Pos.x, y: dst.Pos.y - n.Pos.y, z: dst.Pos.z - n.Pos.z}
				se.Kind = classifyKind(e.Direction, actual)
			} else {
				se.Stub = true
				se.Kind = classifyKind(e.Direction, e.Direction) // nominal placement
			}
			if dz := r.snapshotZone(e.RoomId); dz != "" && dz != srcZone {
				se.ToZone = dz
			}
			sr.Exits = append(sr.Exits, se)
		}
		out = append(out, sr)
	}
	return out
}

// snapshotZone is a room's zone for an exit's ToZone, without loading it: the
// live room when it is in memory, else the zone its mapNode copied at crawl
// time. The crawl follows every exit of every node it records, so an exit's
// destination is either crawled or a room that failed to load at crawl time
// (which LoadRoom would also have returned nil for); either way "" is right.
func (r *mapper) snapshotZone(roomId int) string {
	if rooms.IsRoomLoaded(roomId) {
		if room := rooms.LoadRoom(roomId); room != nil { // memory hit, no disk
			return room.Zone
		}
	}
	if n, ok := r.crawledRooms[roomId]; ok {
		return n.Zone
	}
	return ""
}

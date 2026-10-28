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
//
// This is one zone's map only; the play client's Zone.Map draws WebMap, which
// adds the explored rooms of the zones this one joins.
func (r *mapper) Snapshot(visited map[int]struct{}) []SnapshotRoom {
	return r.snapshotInto(make([]SnapshotRoom, 0, len(visited)), visited, nil, nil)
}

// WebMap is the room set the play client's web map draws from one zone's map:
// that map plus the map of every zone its border exits reach, directly or
// through other zones. Before zone maps stopped at their border (#375 phase 4,
// ruling 18) one map crawled every room a compass exit reached, across zones,
// and the web map showed the explored rooms of all of them (the owner's
// 2026-07-30 rule: a room the player has explored stays on the map whatever
// zone it is in). WebMap gathers the same rooms from their own zones' maps.
// It builds no map: a zone whose map is not built yet (PreCacheMaps builds
// them all at boot) is left out.
type WebMap struct {
	root  *mapper
	maps  []*mapper        // the maps drawn: root first, then each joined map once
	drawn map[*mapper]bool // the maps above
}

// WebMap returns this map's web map view. A map at crawl positions (no clean
// authored layout) or an instance's copy has a frame of its own, so its view
// is the map alone, and a joined map at crawl positions is left out for the
// same reason.
func (r *mapper) WebMap() *WebMap {
	w := &WebMap{root: r, maps: []*mapper{r}, drawn: map[*mapper]bool{r: true}}
	if !r.authored || rooms.IsEphemeralRoomId(r.rootRoomId) {
		return w
	}
	// Breadth-first over maps, as the old crawl went breadth-first over rooms:
	// a joined map at crawl positions is walked through but not drawn.
	walked := map[*mapper]bool{r: true}
	queue := []*mapper{r}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, id := range cur.borderRooms {
			m := GetMapperIfExists(id)
			if m == nil || walked[m] {
				continue
			}
			walked[m] = true
			queue = append(queue, m)
			if m.authored && !rooms.IsEphemeralRoomId(m.rootRoomId) {
				w.drawn[m] = true
				w.maps = append(w.maps, m)
			}
		}
	}
	return w
}

// HasRoom reports whether the view draws roomId once visited: the root map
// holds it, or the room's own zone's map is one the view draws.
func (w *WebMap) HasRoom(roomId int) bool {
	if w.root.HasRoom(roomId) {
		return true
	}
	m := GetMapperIfExists(roomId)
	return m != nil && w.drawn[m] && m.HasRoom(roomId)
}

// RoomIds is every room the view draws, each once.
func (w *WebMap) RoomIds() []int {
	seen := map[int]bool{}
	var out []int
	for _, m := range w.maps {
		for id := range m.crawledRooms {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// Snapshot is the visited rooms of every map in the view, each from its own
// map, with each exit a full edge when its far room is a visited room the
// view draws, as the one map's Snapshot made it when that map held them all.
// A room two gathered maps hold (a stale map a forced rebuild left behind) is
// drawn once, from the first.
func (w *WebMap) Snapshot(visited map[int]struct{}) []SnapshotRoom {
	out := make([]SnapshotRoom, 0, len(visited))
	drawnFrom := make(map[int]*mapper, len(visited))
	for _, m := range w.maps {
		for id := range m.crawledRooms {
			if _, ok := visited[id]; !ok {
				continue
			}
			if _, dup := drawnFrom[id]; !dup {
				drawnFrom[id] = m
			}
		}
	}
	far := func(id int) *mapNode {
		if m, ok := drawnFrom[id]; ok {
			return m.crawledRooms[id]
		}
		for _, m := range w.maps {
			if n, ok := m.crawledRooms[id]; ok {
				return n
			}
		}
		return nil
	}
	for _, m := range w.maps {
		m := m
		out = m.snapshotInto(out, visited, far, func(id int) bool { return drawnFrom[id] == m })
	}
	return out
}

// snapshotInto appends this map's visited rooms to out. far looks up an exit's
// far room outside this map (nil: none), and own (nil: every room) picks which
// of this map's rooms to draw.
func (r *mapper) snapshotInto(out []SnapshotRoom, visited map[int]struct{}, far func(int) *mapNode, own func(int) bool) []SnapshotRoom {
	// Iteration order is nondeterministic; that's fine — the client places rooms
	// by (x,y,z), not by slice order.
	for id, n := range r.crawledRooms {
		if _, ok := visited[id]; !ok {
			continue
		}
		if own != nil && !own(id) {
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
			if !crawled && far != nil {
				dst = far(e.RoomId)
				crawled = dst != nil
			}
			_, vis := visited[e.RoomId]
			if crawled && vis {
				actual := positionDelta{x: dst.Pos.x - n.Pos.x, y: dst.Pos.y - n.Pos.y, z: dst.Pos.z - n.Pos.z}
				se.Kind = classifyKind(e.Direction, actual)
			} else {
				se.Stub = true
				se.Kind = classifyKind(e.Direction, e.Direction) // nominal placement
			}
			if dz := r.snapshotZone(e); dz != "" && dz != srcZone {
				se.ToZone = dz
			}
			sr.Exits = append(sr.Exits, se)
		}
		out = append(out, sr)
	}
	return out
}

// snapshotZone is the zone an exit leads to, for its ToZone, without loading
// the far room: the live room when it is in memory, else the zone its mapNode
// copied at crawl time, else the zone the exit recorded at crawl time. A
// zone's map stops at its border, so a border exit's far room is in another
// zone's map and only the exit's own record names its zone; an exit to a
// room that does not exist recorded "".
func (r *mapper) snapshotZone(e nodeExit) string {
	if rooms.IsRoomLoaded(e.RoomId) {
		if room := rooms.LoadRoom(e.RoomId); room != nil { // memory hit, no disk
			return room.Zone
		}
	}
	if n, ok := r.crawledRooms[e.RoomId]; ok {
		return n.Zone
	}
	return e.ToZone
}

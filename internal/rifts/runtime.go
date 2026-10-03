package rifts

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/forager"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// runtime.go: live runs, building rooms, and the per-round sweep that tears
// them down. Everything here runs on the game loop under the world lock
// (commands, event listeners, NewRound), like the rest of the room layer.

// Run is one rift: the rooms built for it so far, who is inside, and the
// portal it was entered through.
type Run struct {
	Id           int
	Profile      *Profile
	OriginRoomId int // the overworld room the portal opened in
	EntryRoomId  int // the first room of the rift
	PortalOpen   bool
	PortalUntil  time.Time
	Entered      bool
	Members      map[int]bool // user ids currently inside
	Rooms        map[int]*RiftRoom

	chunk          *rooms.OwnedChunk
	usedTemplates  map[string]bool
	placedWritings map[int]bool
	hunts          map[int]*Hunt     // by hunted user id (hunter.go)
	claims         map[int]time.Time // players committed at the portal, until they arrive (instance.go)
}

// RiftRoom is the run's knowledge of one of its rooms.
type RiftRoom struct {
	RoomId       int
	Pool         Pool
	Template     *Template
	Depth        int
	ParentRoomId int    // 0 for the entry room
	ViaDoor      string // the parent's door that leads here
	Doors        map[string]*Door
	DoorOrder    []string

	Expanded bool // the rooms behind its doors have been built
	Entered  bool // a player has stood in it

	BossMobId      int  // E: the boss's mob id
	KeyAwarded     bool // E: the boss's key has dropped; C: the puzzle's key was given
	Cleared        bool // D/E: the "doors open" line has been said
	PuzzleSolved   bool
	SeqProgress    int
	Memory         *MemoryBoard // C: the lens table, when the puzzle is a memory one
	LoreToken      string       // non-empty when the room holds a lore object; unique per placement
	Writing        int          // index into Profile.Writings, or -1
	Rubble         bool         // the room holds a rubble pile
	RubbleSearched bool         // its find has been claimed
	trapChecked    map[int]bool // user ids already tested against the trap

	// present is who the RoomChange handler has seen arrive and not yet
	// seen leave. A room is kept while anyone is in it by this count as well
	// as by the room's own player list, so a sweep that runs between a move
	// and its queued RoomChange event cannot tear the room down under the
	// handler (which needs it to spend keys and take them away on leaving).
	present map[int]bool
}

// Door is one way out of a rift room.
type Door struct {
	Exit       string
	Pool       Pool // the pool of the room behind it
	Locked     bool // needs a Facet Key (every door into an exit room)
	Unlocked   bool // a key has been spent on it; it stays open
	Sealed     bool // held shut by the room's puzzle until solved
	DestRoomId int  // 0 until built
}

var (
	runs      = map[int]*Run{}
	runByRoom = map[int]*Run{}
	nextRunId = 1

	// rng is util.Rand; tests may replace it.
	rng = util.Rand
	// now is time.Now; tests may replace it.
	now = time.Now
)

// RunForRoom returns the run that owns roomId, or nil.
func RunForRoom(roomId int) *Run { return runByRoom[roomId] }

// RoomInfo returns the run and rift room for roomId, or nils.
func RoomInfo(roomId int) (*Run, *RiftRoom) {
	run := runByRoom[roomId]
	if run == nil {
		return nil, nil
	}
	return run, run.Rooms[roomId]
}

// Runs returns every live run, by id.
func Runs() []*Run {
	out := make([]*Run, 0, len(runs))
	for _, r := range runs {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Id < out[j].Id })
	return out
}

// runOfMember returns the run userId is a member of, or nil.
func runOfMember(userId int) *Run {
	for _, run := range runs {
		if run.Members[userId] {
			return run
		}
	}
	return nil
}

// frontierRoomId is a room of the run a member is standing in (the lowest
// user id's, so the answer is stable), or 0. Late joiners are sent there, so
// a party that has moved on from the entry room is still joined.
func (run *Run) frontierRoomId() int {
	ids := make([]int, 0, len(run.Members))
	for uid := range run.Members {
		ids = append(ids, uid)
	}
	sort.Ints(ids)
	for _, uid := range ids {
		u := users.GetByUserId(uid)
		if u == nil {
			continue
		}
		if rr := run.Rooms[u.Character.RoomId]; rr != nil && rooms.LoadRoom(rr.RoomId) != nil {
			return rr.RoomId
		}
	}
	return 0
}

// IsRiftRoom reports whether roomId belongs to a live run.
func IsRiftRoom(roomId int) bool { return runByRoom[roomId] != nil }

// newRun reserves a chunk and builds the entry room. It does not open a
// portal; OpenPortal does that.
func newRun(p *Profile, originRoomId int) (*Run, error) {
	chunk, err := rooms.ReserveOwnedChunk(p.Zone)
	if err != nil {
		return nil, err
	}
	run := &Run{
		Id:             nextRunId,
		Profile:        p,
		OriginRoomId:   originRoomId,
		Members:        map[int]bool{},
		Rooms:          map[int]*RiftRoom{},
		chunk:          chunk,
		usedTemplates:  map[string]bool{},
		placedWritings: map[int]bool{},
		hunts:          map[int]*Hunt{},
		claims:         map[int]time.Time{},
	}
	entry, err := run.buildRoom(p.StartPool, 0, nil, ``)
	if err != nil {
		chunk.Release()
		return nil, err
	}
	run.EntryRoomId = entry.RoomId
	runs[run.Id] = run
	nextRunId++
	return run, nil
}

// pickTemplate chooses a template of pool, preferring one this run has not
// used yet.
func (run *Run) pickTemplate(pool Pool) *Template {
	all := run.Profile.Templates(pool)
	if len(all) == 0 {
		return nil
	}
	fresh := make([]*Template, 0, len(all))
	for _, t := range all {
		if !run.usedTemplates[t.Id] {
			fresh = append(fresh, t)
		}
	}
	if len(fresh) == 0 {
		fresh = all
	}
	// Generated rooms grow without the authored ones shrinking: when both
	// are on offer, half the picks come from the authored rooms.
	var authored, generated []*Template
	for _, t := range fresh {
		if t.Source == `generated` {
			generated = append(generated, t)
		} else {
			authored = append(authored, t)
		}
	}
	if len(authored) > 0 && len(generated) > 0 {
		if rng(2) == 0 {
			fresh = authored
		} else {
			fresh = generated
		}
	}
	t := fresh[rng(len(fresh))]
	run.usedTemplates[t.Id] = true
	return t
}

// buildRoom builds one room of pool at depth and loads it. Its doors are
// planned (their pools rolled) but the rooms behind them are not built until
// a player enters (expand). Until then each door exit points back at the room
// itself; the router refuses it as "unsettled".
func (run *Run) buildRoom(pool Pool, depth int, parent *RiftRoom, viaDoor string) (*RiftRoom, error) {
	p := run.Profile
	t := run.pickTemplate(pool)
	if t == nil {
		return nil, fmt.Errorf(`rifts: profile %q has no %s template`, p.Id, pool)
	}
	// Now and then a new room of the pool is written in the background for
	// later rooms (gen.go); this one uses the bank as it is.
	run.maybeGenerate(pool)

	room := &rooms.Room{
		Zone:        p.Zone,
		Title:       t.Title,
		Description: t.Description,
		Biome:       p.Biome,
		Nouns:       map[string]string{},
		Exits:       map[string]exit.RoomExit{},
		ExitsTemp:   map[string]exit.TemporaryRoomExit{},
	}
	if p.Lamp != nil {
		lamp := *p.Lamp
		room.Lamp = &lamp
	}
	for k, v := range t.Nouns {
		room.Nouns[k] = v
	}
	room.IdleMessages = append(append([]string{}, t.IdleMessages...), p.IdleMessages...)

	rr := &RiftRoom{
		Pool:        pool,
		Template:    t,
		Depth:       depth,
		ViaDoor:     viaDoor,
		Doors:       map[string]*Door{},
		trapChecked: map[int]bool{},
	}
	if parent != nil {
		rr.ParentRoomId = parent.RoomId
	}

	// Doors: how many, which of the template's, and where each leads.
	n := RollRange(p.Doors[pool], rng)
	chosen := ChooseDoors(t.Doors, n, rng)
	if t.Puzzle != nil {
		chosen = ensureDoor(chosen, t.Doors, t.Puzzle.SealedDoor)
	}
	pools := PlanDoors(p, pool, depth, len(chosen), rng)
	if t.Puzzle != nil {
		pools = keepAWayOpen(pools, chosen, t.Puzzle.SealedDoor)
	}
	for i, d := range chosen {
		door := &Door{Exit: d.Exit, Pool: pools[i], Locked: pools[i] == PoolExit}
		if t.Puzzle != nil && d.Exit == t.Puzzle.SealedDoor {
			door.Sealed = true
		}
		rr.Doors[d.Exit] = door
		rr.DoorOrder = append(rr.DoorOrder, d.Exit)
		// `look <door>` reads the door's description as a noun.
		if _, ok := room.Nouns[d.Exit]; !ok {
			room.Nouns[d.Exit] = d.Description
		}
	}

	// Monsters, placed as soon as the room exists (below), so they are there
	// when the first player walks in. Never respawned, never wandering, gone
	// with the room. Placed directly rather than through SpawnInfo: Prepare's
	// orphan check reattaches a second slot of the same mob id to the first
	// slot's mob, so three of one kind would come out as one.
	var spawns []plannedSpawn
	switch pool {
	case PoolMonster:
		tier, ids := t.Encounter.Tier, p.Mobs.Trash
		if tier == `elite` && len(p.Mobs.Elite) > 0 {
			ids = p.Mobs.Elite
		} else if len(ids) == 0 {
			tier, ids = `elite`, p.Mobs.Elite
		}
		for i, count := 0, RollRange(t.Encounter.Count, rng); i < count; i++ {
			spawns = append(spawns, plannedSpawn{ids[rng(len(ids))], StatPool(p, tier, depth)})
		}
	case PoolBoss:
		rr.BossMobId = p.Mobs.Boss[rng(len(p.Mobs.Boss))]
		spawns = append(spawns, plannedSpawn{rr.BossMobId, StatPool(p, `boss`, depth)})
	}
	// A lurker: no pool is always safe (a Glint Stalker in a passage).
	if c := p.Lurkers.Chance[pool]; c > 0 && len(p.Lurkers.Mobs) > 0 && rng(100) < c {
		spawns = append(spawns, plannedSpawn{p.Lurkers.Mobs[rng(len(p.Lurkers.Mobs))], StatPool(p, `trash`, depth)})
	}

	// Ore: a seam the room's foragers can work.
	if c := p.OreChance[pool]; c > 0 && rng(100) < c && len(p.OreItems) > 0 {
		ore := p.OreItems[rng(len(p.OreItems))]
		extra := make([]int, p.OreWeight)
		for i := range extra {
			extra[i] = ore
		}
		room.SetTempData(forager.RoomExtraYieldsKey, extra)
		noun, look := p.DefaultOreNoun, p.DefaultOreLook
		if t.Ore != nil {
			noun, look = t.Ore.Noun, t.Ore.Look
		}
		if noun != `` {
			if _, taken := room.Nouns[noun]; !taken {
				room.Nouns[noun] = look
				room.Description += ` ` + oreSentence(noun)
			}
		}
	}

	// The way out of an exit room. Routed: see Router.
	if pool == PoolExit {
		room.Exits[p.ExitName] = exit.RoomExit{RoomId: run.OriginRoomId}
	}

	// A light left on the floor: nobody tends a torch here, so it is the
	// profile's own (the Obelisk's glowing crystal).
	if c := p.LightChance[pool]; c > 0 && p.LightItemId != 0 && rng(100) < c {
		if itm := items.New(p.LightItemId); itm.IsValid() {
			room.AddItem(itm, false)
		}
	}

	// Lore and writings, each a noun the player studies with `look`
	// (OnLook does the rest).
	if c := p.Lore.Chance[pool]; c > 0 && rng(100) < c {
		if _, taken := room.Nouns[p.Lore.Noun]; !taken {
			room.Nouns[p.Lore.Noun] = p.Lore.Look
			room.Description += ` ` + strings.TrimSpace(p.Lore.Mention)
			rr.LoreToken = fmt.Sprintf(`%s-%d-%d`, p.Id, now().UnixNano(), rng(1_000_000))
		}
	}
	rr.Writing = -1
	if c := p.WritingChance[pool]; c > 0 && len(p.Writings) > 0 && rng(100) < c {
		idx := run.pickWriting()
		if w := p.Writings[idx]; room.Nouns[w.Noun] == `` {
			room.Nouns[w.Noun] = w.Look
			room.Description += ` ` + strings.TrimSpace(w.Mention)
			rr.Writing = idx
			run.placedWritings[idx] = true
		}
	}

	// A rubble pile to search, the rift's chest. Its find is claimed by the
	// first search (ClaimRubble).
	if c := p.Rubble.Chance[pool]; c > 0 && rng(100) < c {
		if _, taken := room.Nouns[p.Rubble.Noun]; !taken {
			room.Nouns[p.Rubble.Noun] = p.Rubble.Look
			room.Description += ` ` + strings.TrimSpace(p.Rubble.Mention)
			rr.Rubble = true
		}
	}

	// The lens table: a fresh board every time the room is built, so no
	// solution carries over (memory.go).
	if t.Puzzle != nil && t.Puzzle.Kind == `memory` && p.Memory != nil {
		rr.Memory = newMemoryBoard(p.Memory)
		room.Nouns[p.Memory.Noun] = p.Memory.Look
		room.Description += ` ` + strings.TrimSpace(p.Memory.Mention)
	}

	room.SetTempData(`allow_recall`, false)
	room.SetTempData(`rift_run`, run.Id)

	roomId, err := run.chunk.AddRoom(room)
	if err != nil {
		return nil, err
	}
	rr.RoomId = roomId
	for _, name := range rr.DoorOrder {
		room.Exits[name] = exit.RoomExit{RoomId: roomId}
	}
	run.Rooms[roomId] = rr
	runByRoom[roomId] = run
	if placed := rooms.LoadRoom(roomId); placed != nil {
		for _, s := range spawns {
			spawnMob(placed, s.mobId, s.statPool)
		}
	}
	return rr, nil
}

// plannedSpawn is a mob a room is built with.
type plannedSpawn struct {
	mobId    int
	statPool int
}

// spawnMob places a fresh instance of mobId in room with statPool. Its own
// file decides whether it attacks on sight (hostile) or waits (an ambusher).
func spawnMob(room *rooms.Room, mobId int, statPool int) *mobs.Mob {
	mob := mobs.NewMobByIdFresh(mobs.MobId(mobId), room.RoomId, statPool)
	if mob == nil {
		mudlog.Error(`rifts.spawnMob`, `mobId`, mobId, `room`, room.RoomId, `error`, `no such mob`)
		return nil
	}
	mob.Character.Zone = room.Zone
	mob.Validate()
	room.AddMob(mob.InstanceId)
	return mob
}

// pickWriting chooses which writing (story fragment) a room gets, preferring
// one not yet placed in this run.
func (run *Run) pickWriting() int {
	n := len(run.Profile.Writings)
	fresh := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if !run.placedWritings[i] {
			fresh = append(fresh, i)
		}
	}
	if len(fresh) == 0 {
		return rng(n)
	}
	return fresh[rng(len(fresh))]
}

// oreSentence is the line added to an ore room's description so the seam can
// be noticed without reading every noun.
func oreSentence(noun string) string {
	return fmt.Sprintf(`In one wall a %s of thicker, darker crystal catches your eye.`, noun)
}

// ensureDoor makes sure the door named want is among chosen, swapping it in
// for the last chosen door when it is missing.
func ensureDoor(chosen []DoorSpec, all []DoorSpec, want string) []DoorSpec {
	for _, d := range chosen {
		if d.Exit == want {
			return chosen
		}
	}
	for _, d := range all {
		if d.Exit == want {
			if len(chosen) == 0 {
				return []DoorSpec{d}
			}
			out := append([]DoorSpec(nil), chosen...)
			out[len(out)-1] = d
			return out
		}
	}
	return chosen
}

// expand builds the room behind every door of rr and points the doors at
// them. Called when a player first enters rr.
func (run *Run) expand(rr *RiftRoom) {
	if rr.Expanded {
		return
	}
	room := rooms.LoadRoom(rr.RoomId)
	if room == nil {
		return
	}
	for _, name := range rr.DoorOrder {
		door := rr.Doors[name]
		child, err := run.buildRoom(door.Pool, rr.Depth+1, rr, name)
		if err != nil {
			mudlog.Error(`rifts.expand`, `run`, run.Id, `room`, rr.RoomId, `door`, name, `error`, err)
			continue
		}
		door.DestRoomId = child.RoomId
		info := room.Exits[name]
		info.RoomId = child.RoomId
		room.Exits[name] = info
	}
	rr.Expanded = true
}

// removable reports whether a room can be torn down now: nobody is in it,
// and either somebody has been in it (there is no going back) or the room
// that led to it is gone (nobody can reach it any more). The entry room is
// kept while its portal is open and nobody has gone in.
func (run *Run) removable(rr *RiftRoom) bool {
	room := rooms.LoadRoom(rr.RoomId)
	if room != nil && len(room.GetPlayers()) > 0 {
		return false
	}
	if len(rr.present) > 0 {
		return false
	}
	if rr.RoomId == run.EntryRoomId && run.PortalOpen && !run.Entered {
		return false
	}
	if rr.Entered {
		return true
	}
	if rr.ParentRoomId == 0 {
		return !run.PortalOpen
	}
	_, parentAlive := run.Rooms[rr.ParentRoomId]
	return !parentAlive
}

// Sweep tears down what can be torn down: rooms behind the players, whole
// runs nobody is in any more, portals that have timed out. It runs every
// round from NewRound, never from inside a RoomChange listener (the rest of
// that listener chain still loads the room being left).
func Sweep() {
	sweepSites()
	for _, run := range Runs() {
		run.sweep()
	}
}

func (run *Run) sweep() {
	if run.PortalOpen {
		switch {
		case run.Entered && now().After(run.PortalUntil):
			run.closePortal() // the party's join window is over
		case !run.Entered && !hasPlayers(run.OriginRoomId) && len(run.company()) == 0:
			run.closePortal() // nobody is waiting at the site any more
		default:
			run.keepPortal()
		}
	}
	run.checkMembers()
	run.sweepHunts()

	// Remove rooms until nothing more is removable: a removed parent can make
	// its unentered children removable in the same pass.
	for changed := true; changed; {
		changed = false
		for id, rr := range run.Rooms {
			if !run.removable(rr) {
				continue
			}
			if !run.chunk.RemoveRoom(id) {
				continue
			}
			delete(run.Rooms, id)
			delete(runByRoom, id)
			changed = true
			if id == run.EntryRoomId && run.PortalOpen && !run.Entered {
				run.closePortal()
			}
		}
	}

	if run.over() {
		run.end()
	}
}

// checkMembers is the backstop for departures the RoomChange handler never
// saw: a member who is offline, or who stands outside the run while no room
// of it still expects their RoomChange, leaves the run and loses their keys
// and rift lights. Stale arrival marks of offline players are dropped too.
func (run *Run) checkMembers() {
	for _, rr := range run.Rooms {
		for uid := range rr.present {
			if users.GetByUserId(uid) == nil {
				delete(rr.present, uid)
			}
		}
	}
	for uid := range run.Members {
		u := users.GetByUserId(uid)
		if u == nil {
			delete(run.Members, uid)
			continue
		}
		if run.Rooms[u.Character.RoomId] != nil {
			continue
		}
		pending := false
		for _, rr := range run.Rooms {
			if rr.present[uid] {
				pending = true
				break
			}
		}
		if pending {
			continue // their RoomChange is still queued; it will see to them
		}
		delete(run.Members, uid)
		run.endHunt(uid)
		PurgeKeys(u)
		clearOrigin(u)
	}
}

// over reports whether the run has nothing left to wait for: nobody inside
// and no portal anyone could still walk through.
func (run *Run) over() bool {
	return len(run.Members) == 0 && !run.PortalOpen && (run.Entered || len(run.Rooms) <= 1)
}

// end releases everything the run still holds. A player still standing in a
// rift room (an admin who teleported in, a member whose departure was never
// seen) is moved to the origin first so nobody is stranded in a room that is
// about to vanish.
func (run *Run) end() {
	for uid := range run.hunts {
		run.endHunt(uid)
	}
	for id, rr := range run.Rooms {
		rr.present = nil // the handler will no longer find this run
		if room := rooms.LoadRoom(id); room != nil {
			for _, uid := range room.GetPlayers() {
				run.evict(uid)
				if err := rooms.MoveToRoom(uid, run.safeRoomId()); err != nil {
					mudlog.Error(`rifts.end`, `run`, run.Id, `userId`, uid, `error`, err)
				}
			}
		}
	}
	if stranded := run.chunk.Release(); stranded > 0 {
		return // try again next round
	}
	for id := range run.Rooms {
		delete(runByRoom, id)
	}
	run.Rooms = map[int]*RiftRoom{}
	delete(runs, run.Id)
	mudlog.Info(`rifts`, `run`, run.Id, `ended`, true)
}

// ForceEnd closes the run's portal, moves everyone still inside back to the
// world (taking their keys) and releases the run (admin).
func (run *Run) ForceEnd() {
	run.closePortal()
	for id := range run.Rooms {
		if room := rooms.LoadRoom(id); room != nil {
			for _, uid := range room.GetPlayers() {
				run.evict(uid)
				if err := rooms.MoveToRoom(uid, run.safeRoomId()); err != nil {
					mudlog.Error(`rifts.ForceEnd`, `run`, run.Id, `userId`, uid, `error`, err)
				}
			}
		}
	}
	run.Members = map[int]bool{}
	run.end()
}

// evict is what leaving does, for a player the run itself moves out: by the
// time their RoomChange is handled the run is gone, so it cannot do it.
func (run *Run) evict(userId int) {
	delete(run.Members, userId)
	run.endHunt(userId)
	if u := users.GetByUserId(userId); u != nil {
		PurgeKeys(u)
		clearOrigin(u)
	}
}

// GetRun returns a live run by id, or nil.
func GetRun(id int) *Run { return runs[id] }

// safeRoomId is where a player leaving the rift lands: the portal's room if
// it still exists, else the start room.
func (run *Run) safeRoomId() int {
	if run.OriginRoomId > 0 && rooms.LoadRoom(run.OriginRoomId) != nil {
		return run.OriginRoomId
	}
	return rooms.StartRoomIdAlias
}

// hasPlayers reports whether anyone is standing in roomId.
func hasPlayers(roomId int) bool {
	room := rooms.LoadRoom(roomId)
	return room != nil && len(room.GetPlayers()) > 0
}

// hostilesIn reports whether a living, uncharmed hostile other than except
// is in room: one of the profile's own creatures (a Glint Stalker waiting in
// hiding counts, though it does not attack on sight), anything that attacks
// on sight, or anything in a fight. Rift doors stay sealed while one is. The
// profile's hunter never counts: it holds its quarry only.
func hostilesIn(p *Profile, room *rooms.Room, except int) bool {
	for _, id := range room.GetMobs() {
		if id == except {
			continue
		}
		m := mobs.GetInstance(id)
		if m == nil || m.Character.Health <= 0 || m.Character.IsCharmed() {
			continue
		}
		// The hunter holds only its quarry (hunterHolds), never the room.
		if p != nil && p.Mobs.Hunter != 0 && int(m.MobId) == p.Mobs.Hunter {
			continue
		}
		if m.AutoAggro || m.Character.IsInCombat() || (p != nil && p.ownsMob(int(m.MobId))) {
			return true
		}
	}
	return false
}

// keepAWayOpen holds a puzzle room to the no-strand rule: a puzzle can be
// failed for good (a lens table goes dark), so besides its sealed door the
// room keeps at least one door that needs no key. If every other door was
// planned into an exit (F) room, the first of them becomes a passage.
func keepAWayOpen(pools []Pool, doors []DoorSpec, sealed string) []Pool {
	first := -1
	for i, d := range doors {
		if d.Exit == sealed {
			continue
		}
		if pools[i] != PoolExit {
			return pools
		}
		if first < 0 {
			first = i
		}
	}
	if first >= 0 {
		pools[first] = PoolPassage
	}
	return pools
}

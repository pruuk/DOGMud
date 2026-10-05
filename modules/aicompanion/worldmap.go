package aicompanion

import (
	"container/heap"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// The companion's own map (F11.1 to F11.9). It is built only from rooms the
// companion has stood in and exits it has walked (or, for the way back, a
// guess from the opposite direction, clearly marked). It is never filled
// from the world's room graph, so the companion cannot know a room it has
// not been to, and it cannot know where an exit leads until it has used
// it. mapper.GetPath is deliberately not used: it searches the whole world.

// RoomRecord is what the companion remembers about one room.
type RoomRecord struct {
	Title      string                 `yaml:"title"`
	Zone       string                 `yaml:"zone,omitempty"`
	FirstUnix  int64                  `yaml:"first,omitempty"`
	LastUnix   int64                  `yaml:"last,omitempty"`
	Visits     int                    `yaml:"visits,omitempty"`
	Exits      map[string]*ExitRecord `yaml:"exits,omitempty"`
	Features   []string               `yaml:"features,omitempty"` // fixed things: merchants, fixtures, containers
	People     []string               `yaml:"people,omitempty"`   // who was here last time (they move)
	Danger     int                    `yaml:"danger,omitempty"`   // fights and falls here
	DangerUnix int64                  `yaml:"danger_t,omitempty"`
}

// ExitRecord is one way out of a known room.
type ExitRecord struct {
	To      int   `yaml:"to,omitempty"`      // 0 until walked (or guessed)
	Guessed bool  `yaml:"guessed,omitempty"` // To inferred from the opposite exit, not yet walked
	Door    bool  `yaml:"door,omitempty"`
	Fails   int   `yaml:"fails,omitempty"`
	Used    int64 `yaml:"used,omitempty"`
}

// PlaceTip is hearsay about a place: directions or a landmark someone told
// the companion about (F11.9). Confirmed is the room it turned out to be.
type PlaceTip struct {
	Unix      int64  `yaml:"t"`
	From      string `yaml:"from"`
	Text      string `yaml:"text"`
	Confirmed int    `yaml:"confirmed,omitempty"`
}

var oppositeExit = map[string]string{
	`north`: `south`, `south`: `north`, `east`: `west`, `west`: `east`,
	`up`: `down`, `down`: `up`, `in`: `out`, `out`: `in`,
	`northeast`: `southwest`, `southwest`: `northeast`,
	`northwest`: `southeast`, `southeast`: `northwest`,
}

// safeExitName rejects exit names the mob `go` command would treat as
// something other than an exit: a number is a room id (and would move the
// companion straight there), and "home" runs the mob's own pathing.
func safeExitName(name string) bool {
	if name == `` || name == `home` {
		return false
	}
	if _, err := strconv.Atoi(name); err == nil {
		return false
	}
	return true
}

func (m *Mind) roomRecord(roomId int) *RoomRecord {
	if m.Map == nil {
		m.Map = map[int]*RoomRecord{}
	}
	r, ok := m.Map[roomId]
	if !ok {
		r = &RoomRecord{Exits: map[string]*ExitRecord{}}
		m.Map[roomId] = r
	}
	if r.Exits == nil {
		r.Exits = map[string]*ExitRecord{}
	}
	return r
}

// recordRoom updates the map for the room the companion is standing in. It
// reads only what the companion can see: the title, visible exits, whether
// they have doors, and what the scene showed. prevRoomId, when non-zero, is
// the room it just came from; the exit it used is recorded as walked.
func (m *Mind) recordRoom(room *rooms.Room, sc *scene, prevRoomId int, nowUnix int64, maxRooms int) {
	rec := m.roomRecord(room.RoomId)
	if rec.FirstUnix == 0 {
		rec.FirstUnix = nowUnix
	}
	rec.LastUnix = nowUnix
	rec.Visits++
	rec.Title = strings.TrimSpace(room.Title)
	rec.Zone = room.Zone

	for name, ex := range room.Exits {
		if ex.Secret || !safeExitName(name) {
			continue
		}
		er, ok := rec.Exits[name]
		if !ok {
			er = &ExitRecord{}
			rec.Exits[name] = er
		}
		er.Door = ex.HasLock()
	}

	if sc != nil && !sc.Dark {
		var features, people []string
		for _, t := range sc.Things {
			switch {
			case t.Kind == `npc` && t.Class == `a merchant`:
				features = append(features, t.Name+` (merchant)`)
			case t.Kind == `fixture` || t.Kind == `container`:
				features = append(features, t.Name)
			case t.Kind == `npc`:
				people = append(people, t.Name)
			}
		}
		// A place to work (a cooking fire, a forge) is worth remembering:
		// it is where she can make what she has the makings for.
		if st := strings.TrimSpace(room.Station); st != `` {
			features = append([]string{stationWords(st)}, features...)
		}
		rec.Features = capStrings(features, 12)
		rec.People = capStrings(people, 8)
	}

	// The exit just used: the companion knows which way it went, as a
	// player who walked or followed through it would.
	if prevRoomId > 0 && prevRoomId != room.RoomId {
		if prevRoom := rooms.LoadRoom(prevRoomId); prevRoom != nil {
			prevRec := m.roomRecord(prevRoomId)
			for name, ex := range prevRoom.Exits {
				if ex.Secret || ex.RoomId != room.RoomId || !safeExitName(name) {
					continue
				}
				er, ok := prevRec.Exits[name]
				if !ok {
					er = &ExitRecord{}
					prevRec.Exits[name] = er
				}
				er.To, er.Guessed, er.Fails, er.Used = room.RoomId, false, 0, nowUnix

				// Guess the way back from the opposite direction, if this
				// room has it and it has not been walked yet.
				if back, ok := oppositeExit[name]; ok {
					if ber, ok := rec.Exits[back]; ok && (ber.To == 0 || ber.Guessed) {
						ber.To, ber.Guessed = prevRoomId, true
					}
				}
				break
			}
		}
	}

	if maxRooms > 0 && len(m.Map) > maxRooms {
		m.forgetOldestRooms(maxRooms)
	}
}

// forgetOldestRooms drops the least recently seen rooms beyond max.
func (m *Mind) forgetOldestRooms(max int) {
	type aged struct {
		id   int
		last int64
	}
	all := make([]aged, 0, len(m.Map))
	for id, r := range m.Map {
		all = append(all, aged{id, r.LastUnix})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].last < all[j].last })
	for i := 0; i < len(all)-max; i++ {
		delete(m.Map, all[i].id)
	}
}

func capStrings(in []string, max int) []string {
	sort.Strings(in)
	if len(in) > max {
		in = in[:max]
	}
	return in
}

// markDanger notes a fight or a fall in a room, at most once per five
// minutes for fights.
func (m *Mind) markDanger(roomId int, amount int, nowUnix int64, rateLimit bool) {
	if roomId <= 0 {
		return
	}
	rec := m.roomRecord(roomId)
	if rateLimit && nowUnix-rec.DangerUnix < 300 {
		return
	}
	rec.Danger += amount
	if rec.Danger > 20 {
		rec.Danger = 20
	}
	rec.DangerUnix = nowUnix
}

// step is one move along a route.
type step struct {
	Exit string
	To   int
}

// pathItem and pathQueue are the Dijkstra priority queue.
type pathItem struct {
	room int
	cost float64
	idx  int
}

type pathQueue []*pathItem

func (q pathQueue) Len() int           { return len(q) }
func (q pathQueue) Less(i, j int) bool { return q[i].cost < q[j].cost }
func (q pathQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i]; q[i].idx = i; q[j].idx = j }
func (q *pathQueue) Push(x any)        { it := x.(*pathItem); it.idx = len(*q); *q = append(*q, it) }
func (q *pathQueue) Pop() any          { old := *q; n := len(old); it := old[n-1]; *q = old[:n-1]; return it }

func edgeCost(to *RoomRecord, er *ExitRecord) float64 {
	c := 1.0
	if to != nil {
		d := to.Danger
		if d > 6 {
			d = 6
		}
		c += 0.5 * float64(d)
	}
	if er.Door {
		c += 2
	}
	if er.Guessed {
		c += 0.5
	}
	return c
}

// findPath returns the cheapest known route (F11.5): distance, weighted
// against remembered danger, doors and guessed exits. Exits that failed
// three times are not used. maxSteps bounds the route length.
func findPath(mp map[int]*RoomRecord, from int, to int, maxSteps int) ([]step, bool) {
	if from == to {
		return nil, true
	}
	if _, ok := mp[from]; !ok {
		return nil, false
	}
	dist := map[int]float64{from: 0}
	hops := map[int]int{from: 0}
	prev := map[int]step{}
	prevRoom := map[int]int{}
	q := &pathQueue{{room: from}}
	heap.Init(q)

	for q.Len() > 0 {
		cur := heap.Pop(q).(*pathItem)
		if cur.cost > dist[cur.room] {
			continue
		}
		if cur.room == to {
			break
		}
		rec := mp[cur.room]
		if rec == nil || hops[cur.room] >= maxSteps {
			continue
		}
		names := make([]string, 0, len(rec.Exits))
		for n := range rec.Exits {
			names = append(names, n)
		}
		sort.Strings(names) // deterministic ties
		for _, name := range names {
			er := rec.Exits[name]
			if er.To == 0 || er.Fails >= 3 {
				continue
			}
			nc := cur.cost + edgeCost(mp[er.To], er)
			if old, seen := dist[er.To]; seen && old <= nc {
				continue
			}
			dist[er.To] = nc
			hops[er.To] = hops[cur.room] + 1
			prev[er.To] = step{Exit: name, To: er.To}
			prevRoom[er.To] = cur.room
			heap.Push(q, &pathItem{room: er.To, cost: nc})
		}
	}

	if _, ok := dist[to]; !ok {
		return nil, false
	}
	var out []step
	for at := to; at != from; at = prevRoom[at] {
		out = append(out, prev[at])
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, true
}

// reachable returns every known room within maxSteps and its step count.
func reachable(mp map[int]*RoomRecord, from int, maxSteps int) map[int]int {
	out := map[int]int{from: 0}
	frontier := []int{from}
	for d := 1; d <= maxSteps && len(frontier) > 0; d++ {
		var next []int
		for _, id := range frontier {
			rec := mp[id]
			if rec == nil {
				continue
			}
			for _, er := range rec.Exits {
				if er.To == 0 || er.Fails >= 3 {
					continue
				}
				if _, seen := out[er.To]; seen {
					continue
				}
				out[er.To] = d
				next = append(next, er.To)
			}
		}
		frontier = next
	}
	return out
}

// placeRef and parsePlaceRef convert between room ids and the "r123" refs
// the model uses.
func placeRef(roomId int) string { return fmt.Sprintf(`r%d`, roomId) }

func parsePlaceRef(ref string) (int, bool) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if !strings.HasPrefix(ref, `r`) {
		return 0, false
	}
	id, err := strconv.Atoi(ref[1:])
	return id, err == nil && id > 0
}

// describePlace renders one known room for the prompt, with its age.
func describePlace(id int, rec *RoomRecord, steps int, nowUnix int64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s", placeRef(id), rec.Title)
	switch {
	case steps < 0:
		b.WriteString(` (you know no way there from here)`)
	case steps == 0:
		b.WriteString(` (here)`)
	case steps == 1:
		b.WriteString(` (one step away)`)
	default:
		fmt.Fprintf(&b, ` (%d steps away)`, steps)
	}
	if len(rec.Features) > 0 {
		fmt.Fprintf(&b, `; %s`, strings.Join(rec.Features, `, `))
	}
	if len(rec.People) > 0 {
		fmt.Fprintf(&b, `; last time you saw %s there`, strings.Join(rec.People, `, `))
	}
	if rec.Danger >= 3 {
		b.WriteString(`; you have had trouble there`)
	}
	fmt.Fprintf(&b, `; last seen %s ago`, humanizeElapsed(nowUnix-rec.LastUnix))
	return b.String()
}

// nearbyPlaces lists the closest known rooms that have something in them.
func nearbyPlaces(mp map[int]*RoomRecord, from int, max int, nowUnix int64) []string {
	dist := reachable(mp, from, 15)
	type cand struct {
		id    int
		steps int
	}
	var cands []cand
	for id, d := range dist {
		if id == from {
			continue
		}
		if rec := mp[id]; rec != nil && len(rec.Features) > 0 {
			cands = append(cands, cand{id, d})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].steps != cands[j].steps {
			return cands[i].steps < cands[j].steps
		}
		return cands[i].id < cands[j].id
	})
	var out []string
	for i, c := range cands {
		if i >= max {
			break
		}
		out = append(out, describePlace(c.id, mp[c.id], c.steps, nowUnix))
	}
	return out
}

// unexploredExits lists the exits of a known room the companion has not
// walked.
func unexploredExits(rec *RoomRecord) []string {
	if rec == nil {
		return nil
	}
	var out []string
	for name, er := range rec.Exits {
		if (er.To == 0 || er.Guessed) && er.Fails < 3 {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// searchPlaces finds known rooms matching a query by words in their title,
// features and people, nearest first (F11.3). Hearsay that matches is
// returned as well.
func searchPlaces(mind *Mind, from int, query string, max int, nowUnix int64) []string {
	words := keywordsOf(query)
	if len(words) == 0 {
		return nil
	}
	dist := reachable(mind.Map, from, 60)
	type hit struct {
		id    int
		score int
		steps int
	}
	var hits []hit
	for id, rec := range mind.Map {
		text := rec.Title + ` ` + strings.Join(rec.Features, ` `) + ` ` + strings.Join(rec.People, ` `)
		n := 0
		for w := range keywordsOf(text) {
			if words[w] || words[strings.TrimSuffix(w, `s`)] {
				n++
			}
		}
		if n == 0 {
			continue
		}
		d, ok := dist[id]
		if !ok {
			d = -1
		}
		hits = append(hits, hit{id, n, d})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		di, dj := hits[i].steps, hits[j].steps
		if di < 0 {
			di = 1 << 20
		}
		if dj < 0 {
			dj = 1 << 20
		}
		if di != dj {
			return di < dj
		}
		return hits[i].id < hits[j].id
	})
	var out []string
	for i, h := range hits {
		if i >= max {
			break
		}
		out = append(out, describePlace(h.id, mind.Map[h.id], h.steps, nowUnix))
	}
	for _, tip := range mind.Hearsay {
		n := 0
		for w := range keywordsOf(tip.Text) {
			if words[w] {
				n++
			}
		}
		if n > 0 {
			out = append(out, fmt.Sprintf(`Hearsay from %s (%s ago): %s`, tip.From, humanizeElapsed(nowUnix-tip.Unix), tip.Text))
		}
	}
	return out
}

// addHearsay stores a tip about a place, ignoring duplicates.
func (m *Mind) addHearsay(from string, text string, nowUnix int64) bool {
	text = capRunes(text)
	if text == `` {
		return false
	}
	for _, t := range m.Hearsay {
		if strings.EqualFold(t.Text, text) {
			return false
		}
	}
	m.Hearsay = append(m.Hearsay, PlaceTip{Unix: nowUnix, From: from, Text: text})
	if len(m.Hearsay) > 30 {
		m.Hearsay = append([]PlaceTip(nil), m.Hearsay[len(m.Hearsay)-30:]...)
	}
	return true
}

// confirmHearsay marks tips that describe the room just entered: two or
// more shared content words with its title and features.
func (m *Mind) confirmHearsay(roomId int) {
	rec := m.Map[roomId]
	if rec == nil {
		return
	}
	roomWords := keywordsOf(rec.Title + ` ` + strings.Join(rec.Features, ` `))
	for i := range m.Hearsay {
		if m.Hearsay[i].Confirmed != 0 {
			continue
		}
		n := 0
		for w := range keywordsOf(m.Hearsay[i].Text) {
			if roomWords[w] {
				n++
			}
		}
		if n >= 2 {
			m.Hearsay[i].Confirmed = roomId
		}
	}
}

// frontierPlaces lists nearby known rooms that still have ways out the
// companion has never taken, preferring rooms where it has had no trouble
// (F11.8: explore outward from known, safe ground).
func frontierPlaces(mp map[int]*RoomRecord, from int, max int) []string {
	dist := reachable(mp, from, 8)
	type cand struct {
		id    int
		steps int
		ways  []string
	}
	var cands []cand
	for id, d := range dist {
		if id == from {
			continue
		}
		rec := mp[id]
		if rec == nil || rec.Danger >= 3 {
			continue
		}
		if ways := unexploredExits(rec); len(ways) > 0 {
			cands = append(cands, cand{id, d, ways})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].steps != cands[j].steps {
			return cands[i].steps < cands[j].steps
		}
		return cands[i].id < cands[j].id
	})
	var out []string
	for i, c := range cands {
		if i >= max {
			break
		}
		away := fmt.Sprintf(`%d steps away`, c.steps)
		if c.steps == 1 {
			away = `one step away`
		}
		out = append(out, fmt.Sprintf(`[%s] %s (%s): never taken %s`,
			placeRef(c.id), mp[c.id].Title, away, strings.Join(c.ways, `, `)))
	}
	return out
}

// stationWords names a room's crafting station the way she would think of
// it: "a cooking fire", "a forge".
func stationWords(station string) string {
	return `a ` + strings.ReplaceAll(strings.TrimSpace(station), `_`, ` `)
}

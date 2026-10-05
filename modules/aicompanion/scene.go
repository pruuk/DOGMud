package aicompanion

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The scene (F6.1, F6.3, F7.1) is the structured version of what the
// companion perceives: every visible thing it could look at or deal with,
// classified, scored locally, and given a short reference ("t3", "p1") the
// model uses to name it in an action. Perception rules are the same as
// describeSituation: nothing hidden, nothing in a dark room beyond its own
// pack, no container contents, no numbers.

// thing is one entry in the scene.
type thing struct {
	Ref   string // t1.. in the room, p1.. carried, w1.. worn
	Key   string // stable identity for interaction memory
	Kind  string // item, gold, corpse, container, fixture, npc, player, carried, worn
	Name  string
	Class string // short classification words for the prompt
	Score float64

	// What the command builder and inspectors need. Only ids, never names
	// used for lookups.
	Item          items.Item
	HasItem       bool
	MobInstanceId int
	MobId         int
	UserId        int
	FixtureDesc   string
	CorpseRef     string // identity for companion-loot
	Price         int    // wares: remembered price
}

// scene is the whole picture for one companion at one moment.
type scene struct {
	RoomId  int
	Dark    bool
	Things  []thing
	byRef   map[string]*thing
	Carried []thing
	Worn    []thing
	Wares   []thing // s1.. what merchants here sell, as last browsed
}

func (s *scene) get(ref string) *thing {
	if s == nil || s.byRef == nil {
		return nil
	}
	return s.byRef[strings.ToLower(strings.TrimSpace(ref))]
}

var ansiTagRE = regexp.MustCompile(`</?ansi[^>]*>`)

// plainText removes <ansi> markup and collapses whitespace.
func plainText(s string) string {
	return strings.Join(strings.Fields(ansiTagRE.ReplaceAllString(s, ``)), ` `)
}

// itemClass describes an item's kind in words.
func itemClass(it *items.Item) string {
	spec := it.GetSpec()
	switch spec.Type {
	case items.Weapon:
		return `a weapon`
	case items.Offhand:
		return `a shield or off-hand piece`
	case items.Head, items.Neck, items.Body, items.Belt, items.Gloves, items.Ring,
		items.Wrist, items.Back, items.Shoulders, items.Legs, items.Feet:
		return `something to wear`
	case items.Food:
		return `food`
	case items.Drink:
		return `drink`
	case items.Potion:
		return `a potion`
	case items.Botanical:
		return `a plant or herb`
	case items.Gemstone:
		return `a gem`
	case items.Ammo:
		return `ammunition`
	case items.Readable, items.Scroll:
		return `something with writing`
	case items.Junk:
		return `junk`
	}
	return `an object`
}

// isWearable reports whether an item can be equipped.
func isWearable(it *items.Item) bool {
	switch it.GetSpec().Type {
	case items.Weapon, items.Offhand, items.Head, items.Neck, items.Body, items.Belt,
		items.Gloves, items.Ring, items.Wrist, items.Back, items.Shoulders, items.Legs,
		items.Feet:
		return true
	}
	return false
}

// matchesInterest reports whether a name touches one of the profile's
// interests.
func matchesInterest(name string, interests []string) bool {
	l := strings.ToLower(name)
	for _, in := range interests {
		if in = strings.ToLower(strings.TrimSpace(in)); in != `` && strings.Contains(l, in) {
			return true
		}
	}
	return false
}

// buildScene reads the companion's room and pack. Runs under the mud lock.
func buildScene(mob *mobs.Mob, owner *users.UserRecord, p *Profile, mind *Mind, nowUnix int64) *scene {
	sc := &scene{byRef: map[string]*thing{}}
	if mob == nil {
		return sc
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return sc
	}
	sc.RoomId = room.RoomId
	sc.Dark = cannotSee(mob, room)

	add := func(t thing) {
		t.Score += noveltyBonus(mind, t.Key, nowUnix)
		sc.Things = append(sc.Things, t)
	}

	if !sc.Dark {
		// Loose items.
		for _, it := range room.Items {
			if it.ItemId < 1 {
				continue
			}
			item := it
			t := thing{
				Kind: `item`, Name: item.ModelName(), Class: `lying here`,
				Key:  fmt.Sprintf(`item:%d@%d`, item.ItemId, room.RoomId),
				Item: item, HasItem: true, Score: 0.3,
			}
			// A player sees only the name of a thing on the floor. What
			// kind of thing it is (and it is never told its value) comes
			// from looking at it, so the class is shown only once it has.
			if looked, _ := mind.lastInteraction(t.Key); looked > 0 {
				t.Class = itemClass(&item)
			}
			// Interest is judged by the name alone, as a person would.
			if matchesInterest(t.Name, p.Interests) {
				t.Score += 0.3
			}
			t.Score += p.Archetype.gearFit(t.Name)
			add(t)
		}
		if room.Gold > 0 {
			add(thing{Kind: `gold`, Name: `some coins`, Class: `loose coin`, Key: fmt.Sprintf(`gold@%d`, room.RoomId), Score: 0.5})
		}

		// Bodies. The companion cannot loot them yet, but it notices them.
		for i := range room.Corpses {
			c := &room.Corpses[i]
			if c.Prunable {
				continue
			}
			name := c.CorpseName
			if name == `` {
				name = `the body of ` + c.Character.Name
			}
			class := `a body`
			if !c.HasLoot() {
				class = `a body, picked clean` // nothing to loot; it could be butchered
			}
			add(thing{Kind: `corpse`, Name: name, Class: class, CorpseRef: corpseRef(c),
				Key: `corpse:` + corpseRef(c), Score: 0.35})
		}

		// Containers, unhidden.
		cnames := make([]string, 0, len(room.Containers))
		for name, ct := range room.Containers {
			if !ct.Hidden {
				cnames = append(cnames, name)
			}
		}
		sort.Strings(cnames)
		for _, name := range cnames {
			add(thing{Kind: `container`, Name: name, Class: `a container`, Key: fmt.Sprintf(`container:%s@%d`, name, room.RoomId), Score: 0.25})
		}

		// Fixtures: the room's visible nouns.
		nouns := make([]string, 0, len(room.Nouns))
		for n := range room.Nouns {
			nouns = append(nouns, n)
		}
		sort.Strings(nouns)
		for _, n := range nouns {
			desc := room.Nouns[n]
			if strings.HasPrefix(desc, `:`) {
				desc = room.Nouns[strings.TrimPrefix(desc, `:`)]
			}
			add(thing{Kind: `fixture`, Name: n, Class: `part of this place`, Key: fmt.Sprintf(`fixture:%s@%d`, n, room.RoomId),
				FixtureDesc: plainText(desc), Score: 0.15})
		}

		// NPCs. Only what a player sees in the room listing is used: who is
		// fighting, whose companion a creature is, and the visible tags on a
		// name (shop, poisoned, lit, dead). Whether a creature will attack
		// on sight is not shown to players, so it is not used here either;
		// the companion learns it the hard way, and its map remembers.
		fighting := map[int]bool{}
		for _, id := range room.GetMobs(rooms.FindFightingPlayer) {
			fighting[id] = true
		}
		for _, id := range room.GetMobs(rooms.FindFightingMob) {
			fighting[id] = true
		}
		for _, id := range room.GetMobs() {
			if id == mob.InstanceId {
				continue
			}
			m := mobs.GetInstance(id)
			if m == nil || !mob.Character.Perceives(&m.Character) {
				continue
			}
			t := thing{Kind: `npc`, Name: m.Character.Name, MobInstanceId: id, MobId: int(m.MobId),
				Key: fmt.Sprintf(`npc:%d`, int(m.MobId)), Score: 0.2}
			switch {
			case fighting[id]:
				t.Class, t.Score = `fighting`, 0.8
			case m.Character.IsCharmed():
				t.Class, t.Score = `someone's companion`, 0.1
			case m.HasShop():
				t.Class, t.Score = `a merchant`, 0.4
			default:
				t.Class = `a creature or stranger`
			}
			if tags := visibleTags(&m.Character); tags != `` {
				t.Class += ` (` + tags + `)`
			}
			if imp, ok := mind.NPCs[t.MobId]; ok && imp.Feeling != `` && imp.Feeling != `neutral` {
				t.Class += `; you ` + feelingVerb(imp.Feeling) + ` them`
			}
			add(t)
		}

		// Other players (the owner is always known and not listed here).
		for _, uid := range room.GetPlayers() {
			if owner != nil && uid == owner.UserId {
				continue
			}
			u := users.GetByUserId(uid)
			if u == nil || u.Character == nil || !mob.Character.Perceives(u.Character) {
				continue
			}
			add(thing{Kind: `player`, Name: u.Character.Name, Class: `a traveller`, UserId: uid,
				Key: fmt.Sprintf(`player:%d`, uid), Score: 0.1})
		}
	}

	// Rank the room things and give them refs in rank order.
	sort.SliceStable(sc.Things, func(i, j int) bool { return sc.Things[i].Score > sc.Things[j].Score })
	for i := range sc.Things {
		sc.Things[i].Ref = fmt.Sprintf(`t%d`, i+1)
	}

	// The companion's own pack and gear, always known.
	for i, it := range mob.Character.Items {
		if it.ItemId < 1 {
			continue
		}
		item := it
		sc.Carried = append(sc.Carried, thing{Ref: fmt.Sprintf(`p%d`, i+1), Kind: `carried`, Name: item.ModelName(),
			Class: itemClass(&item), Item: item, HasItem: true, Key: fmt.Sprintf(`own:%d`, item.ItemId)})
	}
	wi := 0
	for _, it := range mob.Character.Equipment.GetAllItems() {
		if it.ItemId < 1 {
			continue
		}
		wi++
		item := it
		sc.Worn = append(sc.Worn, thing{Ref: fmt.Sprintf(`w%d`, wi), Kind: `worn`, Name: item.ModelName(),
			Class: itemClass(&item), Item: item, HasItem: true, Key: fmt.Sprintf(`worn:%d`, item.ItemId)})
	}

	// Wares, from the companion's memory of each merchant here, if it has
	// browsed there in the last half hour.
	if !sc.Dark {
		si := 0
		for _, t := range sc.Things {
			if t.Kind != `npc` || t.Class != `a merchant` {
				continue
			}
			rec, ok := mind.Shops[t.MobId]
			if !ok || nowUnix-rec.SeenUnix > 1800 {
				continue
			}
			ids := make([]int, 0, len(rec.Wares))
			for id := range rec.Wares {
				ids = append(ids, id)
			}
			sort.Ints(ids)
			for _, id := range ids {
				w := rec.Wares[id]
				if w.Qty <= 0 || si >= 20 {
					continue
				}
				si++
				item := items.New(id)
				sc.Wares = append(sc.Wares, thing{Ref: fmt.Sprintf(`s%d`, si), Kind: `ware`, Name: w.Name,
					Class: fmt.Sprintf(`%d gold from %s`, w.Price, t.Name), Item: item, HasItem: true,
					MobId: t.MobId, MobInstanceId: t.MobInstanceId, Price: w.Price,
					Key: fmt.Sprintf(`ware:%d:%d`, t.MobId, id)})
			}
		}
	}

	for i := range sc.Things {
		sc.byRef[sc.Things[i].Ref] = &sc.Things[i]
	}
	for i := range sc.Wares {
		sc.byRef[sc.Wares[i].Ref] = &sc.Wares[i]
	}
	for i := range sc.Carried {
		sc.byRef[sc.Carried[i].Ref] = &sc.Carried[i]
	}
	for i := range sc.Worn {
		sc.byRef[sc.Worn[i].Ref] = &sc.Worn[i]
	}
	return sc
}

// noveltyBonus favours things the companion has not dealt with lately and
// penalises things it has just handled or failed at (F7.7, F8.11).
func noveltyBonus(mind *Mind, key string, nowUnix int64) float64 {
	last, fails := mind.lastInteraction(key)
	switch {
	case fails >= 2:
		return -1
	case last == 0:
		return 0.2
	case nowUnix-last < 1800:
		return -0.4
	}
	return 0
}

// notable returns the room things worth mentioning, best first.
func (s *scene) notable(threshold float64, max int) []thing {
	var out []thing
	for _, t := range s.Things {
		if t.Score >= threshold {
			out = append(out, t)
		}
		if len(out) >= max {
			break
		}
	}
	return out
}

// describeOptions renders the scene for the prompt with refs.
func (s *scene) describeOptions(max int) string {
	if s == nil {
		return ``
	}
	var b strings.Builder
	n := 0
	for _, t := range s.Things {
		if n >= max {
			break
		}
		fmt.Fprintf(&b, "[%s] %s (%s)\n", t.Ref, t.Name, t.Class)
		n++
	}
	for _, t := range s.Carried {
		fmt.Fprintf(&b, "[%s] %s (%s, in your pack)\n", t.Ref, t.Name, t.Class)
	}
	for _, t := range s.Worn {
		fmt.Fprintf(&b, "[%s] %s (%s, you are wearing or wielding it)\n", t.Ref, t.Name, t.Class)
	}
	for _, t := range s.Wares {
		fmt.Fprintf(&b, "[%s] %s (for sale: %s)\n", t.Ref, t.Name, t.Class)
	}
	return strings.TrimRight(b.String(), "\n")
}

// keysAbove returns the keys of room things at or above a score.
func (s *scene) keysAbove(threshold float64) map[string]bool {
	out := map[string]bool{}
	for _, t := range s.Things {
		if t.Score >= threshold {
			out[t.Key] = true
		}
	}
	return out
}

func feelingVerb(f string) string {
	switch f {
	case `like`:
		return `like`
	case `dislike`:
		return `dislike`
	case `wary`:
		return `are wary of`
	case `trust`:
		return `trust`
	case `distrust`:
		return `distrust`
	}
	return `have no strong view of`
}

// visibleTags are the adjectives a player sees on a character's name in the
// room listing, minus the ones already covered by the class.
func visibleTags(ch *characters.Character) string {
	var out []string
	for _, a := range ch.GetAdjectives() {
		if a == `shop` || a == `hidden` {
			continue
		}
		out = append(out, a)
	}
	return strings.Join(out, `, `)
}

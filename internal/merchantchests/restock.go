package merchantchests

import (
	"fmt"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// restockKeyPrefix prefixes the room long-term data key that records the
// round a chest was last restocked: "merchantchest:<name>:restocked". It is
// saved with the room instance, so the restock clock survives a reboot.
const restockKeyPrefix = `merchantchest:`

// tickEveryRounds throttles Tick. Restock and relock intervals are game
// hours to days, so looking once a minute (15 rounds of 4 seconds) is ample.
const tickEveryRounds = 15

// chestClock is what Tick knows about one chest without loading its room.
type chestClock struct {
	last     uint64 // round it was last restocked; 0 = never
	due      uint64 // round it is next due; 0 = at the first Tick
	openedAt uint64 // round it was picked open; 0 = shut
}

var (
	clockMu sync.Mutex
	clocks  = map[string]*chestClock{} // "<roomId>/<name>"
)

func restockKey(name string) string {
	return restockKeyPrefix + name + `:restocked`
}

func clockKey(roomId int, name string) string {
	return fmt.Sprintf(`%d/%s`, roomId, name)
}

// clockFor returns the chest's clock, creating it. Callers hold clockMu.
func clockFor(roomId int, name string) *chestClock {
	k := clockKey(roomId, name)
	c, ok := clocks[k]
	if !ok {
		c = &chestClock{}
		clocks[k] = c
	}
	return c
}

// lastRestocked reads a chest's last restock round from the room. A value
// read back from a saved instance arrives as whatever integer type the YAML
// decoder chose, so every integer kind is accepted.
func lastRestocked(room *rooms.Room, name string) uint64 {
	switch v := room.GetLongTermData(restockKey(name)).(type) {
	case uint64:
		return v
	case int:
		if v > 0 {
			return uint64(v)
		}
	case int64:
		if v > 0 {
			return uint64(v)
		}
	case uint:
		return uint64(v)
	case float64:
		if v > 0 {
			return uint64(v)
		}
	}
	return 0
}

// dueRound is the round a chest restocked at last becomes due again; 0
// (due at once) for one never restocked.
func dueRound(s Settings, last uint64) uint64 {
	if last == 0 {
		return 0
	}
	return gametime.GetDate(last).AddPeriod(s.RestockInterval)
}

// isDue reports whether a chest must restock at round now: never stocked,
// its time has come, or its record is from the future (the round counter
// was reset), which would otherwise leave it empty for as long again.
func isDue(c chestClock, now uint64) bool {
	return c.last == 0 || c.due == 0 || now >= c.due || c.last > now
}

// OwnerIn returns the chest's merchant when it is in the room, nil when it
// is not (out on its schedule, or dead).
func OwnerIn(room *rooms.Room, ch Chest) *mobs.Mob {
	if room == nil {
		return nil
	}
	for _, instId := range room.GetMobs() {
		if m := mobs.GetInstance(instId); m != nil && int(m.MobId) == ch.MobId {
			return m
		}
	}
	return nil
}

// ownerName is the name goods from this chest are marked with.
func ownerName(ch Chest) string {
	if spec := mobs.GetMobSpec(mobs.MobId(ch.MobId)); spec != nil {
		return spec.Character.Name
	}
	return fmt.Sprintf(`merchant %d`, ch.MobId)
}

// syncContainer places the chest in the room if it is missing and brings its
// lock and description in line with the catalog, leaving its contents and
// its locked or unlocked state alone. The catalog wins over a saved room
// instance, so retuning a lock only needs the catalog edited.
func syncContainer(room *rooms.Room, ch Chest, s Settings) rooms.Container {
	if room.Containers == nil {
		room.Containers = map[string]rooms.Container{}
	}
	c := room.Containers[ch.Name]
	pins := ch.Difficulty
	if pins == 0 {
		pins = LockDifficulty(s, AverageStockValue(ch.MobId))
	}
	c.Lock.Difficulty = uint8(pins)
	c.Lock.RelockInterval = s.RelockInterval
	c.Description = ch.Description
	c.Hidden = false
	c.DespawnRound = 0
	room.Containers[ch.Name] = c
	return c
}

// Restock empties the chest and fills it: gold from the merchant's average
// stock value (GoldFor), and ItemsMin to ItemsMax goods drawn without
// repeats from its inventory list, each marked as the merchant's property.
// The lock is then locked afresh, which also rotates its combination, so a
// thief's remembered solution no longer opens it.
func Restock(room *rooms.Room, ch Chest, now uint64) {
	s := Current()
	c := syncContainer(room, ch, s)
	avg := AverageStockValue(ch.MobId)
	ids := StockItemIds(ch.MobId)

	c.Items = nil
	c.Gold = GoldFor(s, avg, float64(util.Rand(1001))/1000)

	n := s.ItemsMin
	if s.ItemsMax > s.ItemsMin {
		n += util.Rand(s.ItemsMax - s.ItemsMin + 1)
	}
	pool := append([]int(nil), ids...)
	owner := ownerName(ch)
	for i := 0; i < n && len(pool) > 0; i++ {
		j := util.Rand(len(pool))
		itm := items.New(pool[j])
		pool = append(pool[:j], pool[j+1:]...)
		if itm.ItemId == 0 {
			continue
		}
		itm.StolenFrom = owner
		itm.StolenFromMob = ch.MobId
		c.AddItem(itm)
	}

	c.Lock.SetLocked()
	room.Containers[ch.Name] = c
	room.SetLongTermData(restockKey(ch.Name), now)

	clockMu.Lock()
	cl := clockFor(ch.RoomId, ch.Name)
	cl.last, cl.due, cl.openedAt = now, dueRound(s, now), 0
	clockMu.Unlock()
}

// MarkOpened records that a merchant chest was picked open at round now, so
// Tick locks it again once Settings.RelockInterval has passed. (A picked
// lock does not relock by itself: gamelock.Lock.IsLocked measures its relock
// period from the current round, which never comes due. The chest does it
// here instead.) A container that is not a merchant chest is ignored.
func MarkOpened(roomId int, containerName string, now uint64) {
	if _, ok := ChestAt(roomId, containerName); !ok {
		return
	}
	clockMu.Lock()
	clockFor(roomId, containerName).openedAt = now
	clockMu.Unlock()
}

// EnsureAll places every catalog chest in its room and syncs its lock and
// description, and loads each chest's restock record. It restocks nothing:
// at boot it runs before the saved round counter is restored, so Tick, on
// the live round, stocks new chests and restocks due ones at its first look.
// Run at boot and on a data reload, after rooms, mobs and items are loaded.
func EnsureAll() {
	s := Current()
	fresh := map[string]*chestClock{}
	placed := 0
	for _, ch := range All() {
		room := rooms.LoadRoom(ch.RoomId)
		if room == nil {
			continue
		}
		syncContainer(room, ch, s)
		last := lastRestocked(room, ch.Name)
		fresh[clockKey(ch.RoomId, ch.Name)] = &chestClock{last: last, due: dueRound(s, last)}
		placed++
	}
	clockMu.Lock()
	for k, c := range fresh {
		if old, ok := clocks[k]; ok {
			c.openedAt = old.openedAt // a reload keeps a picked chest's relock timer
		}
	}
	clocks = fresh
	clockMu.Unlock()
	mudlog.Info("merchantchests.EnsureAll()", "chests", placed)
}

// Tick restocks every chest that is due and relocks every picked chest whose
// relock interval has passed. Called each round; it only looks every
// tickEveryRounds rounds, and only loads the rooms that need work.
func Tick(roundNumber uint64) {
	if roundNumber%tickEveryRounds != 0 {
		return
	}
	s := Current()
	var restock, relock []Chest
	clockMu.Lock()
	for _, ch := range All() {
		cl, ok := clocks[clockKey(ch.RoomId, ch.Name)]
		if !ok {
			continue // not placed (EnsureAll could not load its room)
		}
		if isDue(*cl, roundNumber) {
			restock = append(restock, ch)
		} else if cl.openedAt > 0 && roundNumber >= gametime.GetDate(cl.openedAt).AddPeriod(s.RelockInterval) {
			relock = append(relock, ch)
		}
	}
	clockMu.Unlock()

	for _, ch := range restock {
		if room := rooms.LoadRoom(ch.RoomId); room != nil {
			Restock(room, ch, roundNumber)
		}
	}
	for _, ch := range relock {
		if room := rooms.LoadRoom(ch.RoomId); room != nil {
			if c, ok := room.Containers[ch.Name]; ok {
				c.Lock.SetLocked() // rotates the combination too
				room.Containers[ch.Name] = c
			}
		}
		clockMu.Lock()
		clockFor(ch.RoomId, ch.Name).openedAt = 0
		clockMu.Unlock()
	}
	if len(restock)+len(relock) > 0 {
		mudlog.Debug("merchantchests.Tick()", "restocked", len(restock), "relocked", len(relock))
	}
}

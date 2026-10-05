package aicompanion

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Finds: what a companion turns up searching, the baubles a share of her
// searches may roll for, and butchering what she and her owner have killed.
//
// Searching is the engine's own search (internal/mobcommands/search.go).
// What it turns up comes back here (onSearched): she is told, through a
// stimulus, so she can say "there's a loose stone here", and her action is
// judged by it. A share of her searches (CompanionBaubleChance) also rolls
// for a bauble, on her owner's behalf and under every rule a player's roll
// has, but only for an owner with a model to name it and react
// (baubleSearchFor). The find goes into her pack (onBaubleFound).
//
// Butchering is the engine's corpse salvage, aimed at one body
// (`salvage <mobId>:<round>`), and only a body her owner could loot that
// has nothing left on it: salvage destroys the corpse.

// baubleSearchFor decides, as a companion searches, whether this search
// also rolls for a bauble and for whom: her owner, online, agreed to the
// model and with one to use (their own key when RequirePlayerKey), on
// CompanionBaubleChance percent of her searches.
func (m *AICompanionModule) baubleSearchFor(mobInstanceId int) int {
	if !m.cfg.Enabled || m.cfg.CompanionBaubleChance <= 0 {
		return 0
	}
	c := m.controllerForInstance(mobInstanceId)
	if c == nil || c.paused {
		return 0
	}
	owner := c.ownerUserId
	if u := users.GetByUserId(owner); u == nil || u.Character == nil {
		return 0
	}
	if !m.consented(owner) || !m.modelReady(owner) {
		return 0
	}
	if m.cfg.RequirePlayerKey && !m.hasOwnKey(owner) {
		return 0
	}
	if util.Rand(100) >= m.cfg.CompanionBaubleChance {
		return 0
	}
	return owner
}

// onSearched hears what her search turned up. It is kept for judging the
// search (verifyPending), and anything found is put to her, so she can
// point it out.
func (m *AICompanionModule) onSearched(mobInstanceId int, found []string) {
	c := m.controllerForInstance(mobInstanceId)
	if c == nil {
		return
	}
	c.lastSearchFound = append([]string(nil), found...)
	if len(found) == 0 || !m.mayRemember(c) {
		return
	}
	// The same hidden way out found again in the same room (it was never
	// hidden from her twice) is not news: no call for it.
	text := strings.Join(found, `; `)
	roomId := 0
	if mob := mobs.GetInstance(mobInstanceId); mob != nil {
		roomId = mob.Character.RoomId
	}
	if c.searchSaid[roomId] == text {
		return
	}
	if !m.takeNotice(c) {
		return
	}
	if c.searchSaid == nil {
		c.searchSaid = map[int]string{}
	}
	c.searchSaid[roomId] = text
	c.push(stimulus{Kind: `searched`, Text: text})
}

// onBaubleFound hears that a find of hers was worked free: in her pack, or
// left on the ground when she could not carry it. name is the model-safe
// name (never the finder's own view of it). Her pack is copied to her
// owner's record straight away, so a find delivered just before a shutdown
// or copyover is not lost.
func (m *AICompanionModule) onBaubleFound(mobInstanceId int, name string, pocketed bool) {
	c := m.controllerForInstance(mobInstanceId)
	if c == nil {
		return
	}
	if m.mayRemember(c) {
		text := `You turned up ` + name + ` while searching, and pocketed it.`
		stim := name
		if !pocketed {
			text = `You turned up ` + name + ` while searching, but were carrying too much, and left it on the ground.`
			stim = name + ` (left on the ground: you could not carry it)`
		}
		c.mind.addLine(Line{Kind: `event`, Text: text}, m.cfg.WorkingMemoryLines)
		if m.takeNotice(c) {
			c.push(stimulus{Kind: `found`, Text: stim})
		}
	}
	c.dirty = true
	if pocketed {
		c.snapshotDue = true
		companionai.Snapshot(c.ownerUserId)
	}
}

// butcherable reports whether she may butcher this body: one her owner
// could loot, with nothing left on it, of a kind that gives something.
// meat restricts it to game (the butcher pastime); otherwise any body the
// engine can salvage.
func butcherable(c *rooms.Corpse, ownerUserId int, meat bool) bool {
	if c == nil || c.Prunable || c.MobId <= 0 || c.HasLoot() {
		return false
	}
	if !c.LootAllowed(ownerUserId, util.GetRoundCount()) {
		return false
	}
	spec := mobs.GetMobSpec(mobs.MobId(c.MobId))
	if spec == nil {
		return false
	}
	returns := crafting.LookupCorpseSalvageForMob(spec.Groups, spec.Character.SpeciesId)
	if len(returns) == 0 {
		return false
	}
	if !meat {
		return true
	}
	// Game is any body whose salvage yields meat. Reading the returns rather
	// than the group tags keeps this in step with the species fallback: a
	// steppe wolf grouped `canine` is game even though it is not `animal`.
	for _, r := range returns {
		if r.ItemTag == `raw-meat` || r.ItemTag == `wild-hare-meat` {
			return true
		}
	}
	return false
}

// salvageHits reports whether the engine's salvage, aimed at this body by
// its mob and round (salvageCommand), would land on this very body. Two of
// the same creature killed in the same round share that key, and the
// engine takes the first; a twin further down the list is never offered,
// since salvaging it would destroy the other one, which may still carry
// loot or be someone else's kill.
func salvageHits(room *rooms.Room, c *rooms.Corpse) bool {
	if room == nil || c == nil {
		return false
	}
	for i := range room.Corpses {
		o := &room.Corpses[i]
		if o.Prunable || o.MobId <= 0 || o.MobId != c.MobId || o.RoundCreated != c.RoundCreated {
			continue
		}
		return o == c
	}
	return false
}

// salvageCommand is the mob command that butchers exactly this body.
func salvageCommand(c *rooms.Corpse) string {
	return fmt.Sprintf(`salvage %d:%d`, c.MobId, c.RoundCreated)
}

// butcherHere picks a body in the room she would butcher idle: game her
// owner could loot, picked clean.
func butcherHere(room *rooms.Room, ownerUserId int) *rooms.Corpse {
	if room == nil {
		return nil
	}
	for i := range room.Corpses {
		if butcherable(&room.Corpses[i], ownerUserId, true) && salvageHits(room, &room.Corpses[i]) {
			return &room.Corpses[i]
		}
	}
	return nil
}

// butcherKey is how a butchering is remembered, per room.
func butcherKey(roomId int) string {
	return fmt.Sprintf(`butcher@%d`, roomId)
}

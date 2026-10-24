package questworld

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/forager"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// itemSource is one place an item comes from. owners are the quests whose
// own grant it is (a quest's give_item, spawn_item or reward, a topic of or
// gated on that quest, a mob only that quest spawns); none for the world at
// large (a room, a shop, a forage table, a recipe, a room-spawned mob).
type itemSource struct {
	where  string
	owners []int
}

// Sources indexes, once, where each item can come from and which mobs are
// placed in the world, from the loaded registries (spec 7.2). A mob's items,
// equipment, loot pool and shop stock count only when that mob is spawned.
func Sources() questloop.Sources {
	itemSrc := map[int][]itemSource{}
	add := func(id int, where string, owners ...int) {
		if id > 0 {
			itemSrc[id] = append(itemSrc[id], itemSource{where: where, owners: owners})
		}
	}
	// spawnedBy holds, per mob, who places it: 0 for a room's spawninfo, a
	// quest id for that quest's spawn_mob action.
	spawnedBy := map[int]map[int]bool{}
	spawn := func(mobId, by int) {
		if mobId <= 0 {
			return
		}
		if spawnedBy[mobId] == nil {
			spawnedBy[mobId] = map[int]bool{}
		}
		spawnedBy[mobId][by] = true
	}

	for _, roomId := range rooms.GetAllRoomIds() {
		r := rooms.LoadRoomTemplate(roomId)
		if r == nil {
			continue
		}
		for _, si := range r.SpawnInfo {
			spawn(si.MobId, 0)
			add(si.ItemId, fmt.Sprintf("room %d spawn", roomId))
		}
		for _, it := range r.Items {
			add(it.ItemId, fmt.Sprintf("room %d floor", roomId))
		}
		for name, c := range r.Containers {
			for _, it := range c.Items {
				add(it.ItemId, fmt.Sprintf("room %d %s", roomId, name))
			}
			for made := range c.Recipes {
				add(made, fmt.Sprintf("room %d %s recipe", roomId, name))
			}
		}
	}

	for _, q := range quests.GetAllQuests() {
		walkActions(q, func(a quests.ActionDef) {
			add(a.GiveItem, fmt.Sprintf("quest %d give_item", q.QuestId), q.QuestId)
			if a.SpawnItem != nil {
				add(a.SpawnItem.Id, fmt.Sprintf("quest %d spawn_item", q.QuestId), q.QuestId)
			}
			if a.SpawnMob != nil {
				spawn(a.SpawnMob.Id, q.QuestId)
			}
		})
		add(q.Rewards.ItemId, fmt.Sprintf("quest %d reward", q.QuestId), q.QuestId)
		for _, g := range quests.ParseItemGrants(q.Rewards.ItemInfo) {
			add(g.ItemId, fmt.Sprintf("quest %d reward", q.QuestId), q.QuestId)
		}
	}

	for _, m := range mobs.AllMobTemplates() {
		if m == nil {
			continue
		}
		// One source per placer, so a mob a room spawns is the world's and
		// a mob only quest q spawns is q's own.
		for by := range spawnedBy[int(m.MobId)] {
			var owners []int
			if by != 0 {
				owners = []int{by}
			}
			who := fmt.Sprintf("mob %d", m.MobId)
			for _, it := range m.Character.Items {
				add(it.ItemId, who+" carried", owners...)
			}
			for _, it := range m.Character.Equipment.GetAllItems() {
				add(it.ItemId, who+" worn", owners...)
			}
			for _, id := range m.LootPool {
				add(id, who+" loot", owners...)
			}
			for _, s := range m.Character.Shop {
				add(s.ItemId, fmt.Sprintf("shop %d", m.MobId), owners...)
			}
		}
	}

	files, _ := dialogue.LoadAllFiles()
	for _, df := range files {
		if df == nil {
			continue
		}
		for _, t := range df.Topics {
			add(t.GivesItem, fmt.Sprintf("mob %d topic %s", df.MobId, t.Key), topicOwners(t)...)
		}
	}

	for _, table := range []map[string][]int{forager.ForageYields, forager.NightForageYields,
		forager.ZoneForageYields, forager.StormForageYields} {
		for place, ids := range table {
			for _, id := range ids {
				add(id, "forage "+place)
			}
		}
	}
	for id, r := range crafting.GetAll() {
		if r != nil {
			add(r.Output.ItemId, "recipe "+id)
		}
	}

	names := map[int][]string{}
	for id, ss := range itemSrc {
		for _, s := range ss {
			names[id] = append(names[id], s.where)
		}
		sort.Strings(names[id])
	}
	return questloop.Sources{
		ItemSources: func(id int) []string { return names[id] },
		MobSpawned:  func(id int) bool { return len(spawnedBy[id]) > 0 },
		ItemOutside: func(id, questId int) bool {
			for _, s := range itemSrc[id] {
				if !slices.Contains(s.owners, questId) {
					return true
				}
			}
			return false
		},
	}
}

// topicOwners is the quests a topic's item grant belongs to: its own quest,
// and every quest its questRequired gate names (a "lost the page?" topic
// gated on 20-vella_journal is quest 20's own replacement).
func topicOwners(t dialogue.Topic) []int {
	var out []int
	if t.Quest > 0 {
		out = append(out, t.Quest)
	}
	for _, tok := range t.QuestRequired {
		idStr, _, _ := strings.Cut(tok, quests.QuestTokenSeparator)
		if id, err := strconv.Atoi(idStr); err == nil && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// walkActions visits every action of q's triggers, sequence on_complete
// actions included.
func walkActions(q quests.Quest, fn func(quests.ActionDef)) {
	var walk func([]quests.ActionDef)
	walk = func(as []quests.ActionDef) {
		for _, a := range as {
			fn(a)
			if a.Sequence != nil {
				walk(a.Sequence.OnComplete)
			}
		}
	}
	for _, t := range q.Triggers {
		walk(t.Actions)
	}
}

package questworld

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/forager"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Sources indexes, once, where each item can come from and which mobs are
// placed in the world, from the loaded registries (spec 7.2). A mob's items,
// equipment, loot pool and shop stock count only when that mob is spawned.
func Sources() questloop.Sources {
	itemSrc := map[int][]string{}
	add := func(id int, where string) {
		if id > 0 {
			itemSrc[id] = append(itemSrc[id], where)
		}
	}
	spawned := map[int]bool{}

	for _, roomId := range rooms.GetAllRoomIds() {
		r := rooms.LoadRoomTemplate(roomId)
		if r == nil {
			continue
		}
		for _, si := range r.SpawnInfo {
			if si.MobId > 0 {
				spawned[si.MobId] = true
			}
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
			add(a.GiveItem, fmt.Sprintf("quest %d give_item", q.QuestId))
			if a.SpawnItem != nil {
				add(a.SpawnItem.Id, fmt.Sprintf("quest %d spawn_item", q.QuestId))
			}
			if a.SpawnMob != nil && a.SpawnMob.Id > 0 {
				spawned[a.SpawnMob.Id] = true
			}
		})
		add(q.Rewards.ItemId, fmt.Sprintf("quest %d reward", q.QuestId))
		for _, g := range quests.ParseItemGrants(q.Rewards.ItemInfo) {
			add(g.ItemId, fmt.Sprintf("quest %d reward", q.QuestId))
		}
	}

	for _, m := range mobs.AllMobTemplates() {
		if m == nil || !spawned[int(m.MobId)] {
			continue
		}
		who := fmt.Sprintf("mob %d", m.MobId)
		for _, it := range m.Character.Items {
			add(it.ItemId, who+" carried")
		}
		for _, it := range m.Character.Equipment.GetAllItems() {
			add(it.ItemId, who+" worn")
		}
		for _, id := range m.LootPool {
			add(id, who+" loot")
		}
		for _, s := range m.Character.Shop {
			add(s.ItemId, fmt.Sprintf("shop %d", m.MobId))
		}
	}

	files, _ := dialogue.LoadAllFiles()
	for _, df := range files {
		if df == nil {
			continue
		}
		for _, t := range df.Topics {
			add(t.GivesItem, fmt.Sprintf("mob %d topic %s", df.MobId, t.Key))
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

	for id := range itemSrc {
		sort.Strings(itemSrc[id])
	}
	return questloop.Sources{
		ItemSources: func(id int) []string { return itemSrc[id] },
		MobSpawned:  func(id int) bool { return spawned[id] },
	}
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

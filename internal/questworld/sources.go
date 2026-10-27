package questworld

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/forager"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// itemSource is one place an item comes from. owners are the quests whose
// own grant it is (a quest's give_item, spawn_item or reward, a topic of or
// gated on that quest, a mob only that quest spawns); none for the world at
// large (a room, a shop, a forage table, a recipe, a room-spawned mob).
type itemSource struct {
	where  string
	owners []owner
}

// owner is one quest an item source belongs to. gates are that quest's
// tokens the source is gated on (a topic's questRequired): once a player
// holds them all, the source is open to them at that step. No gates: the
// source opens only through the quest's own grants.
type owner struct {
	quest int
	gates []string
}

// questOwners is owners for quest ids with no gates.
func questOwners(ids ...int) []owner {
	out := make([]owner, 0, len(ids))
	for _, id := range ids {
		out = append(out, owner{quest: id})
	}
	return out
}

// Sources indexes, once, where each item can come from and which mobs are
// placed in the world, from the loaded registries (spec 7.2). A mob's items,
// equipment, loot pool and shop stock count only when that mob is spawned.
func Sources() questloop.Sources {
	itemSrc := map[int][]itemSource{}
	add := func(id int, where string, owners ...owner) {
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
			add(a.GiveItem, fmt.Sprintf("quest %d give_item", q.QuestId), questOwners(q.QuestId)...)
			if a.SpawnItem != nil {
				add(a.SpawnItem.Id, fmt.Sprintf("quest %d spawn_item", q.QuestId), questOwners(q.QuestId)...)
			}
			if a.SpawnMob != nil {
				spawn(a.SpawnMob.Id, q.QuestId)
			}
		})
		add(q.Rewards.ItemId, fmt.Sprintf("quest %d reward", q.QuestId), questOwners(q.QuestId)...)
		for _, g := range quests.ParseItemGrants(q.Rewards.ItemInfo) {
			add(g.ItemId, fmt.Sprintf("quest %d reward", q.QuestId), questOwners(q.QuestId)...)
		}
	}

	for _, m := range mobs.AllMobTemplates() {
		if m == nil {
			continue
		}
		// One source per placer, so a mob a room spawns is the world's and
		// a mob only quest q spawns is q's own.
		for by := range spawnedBy[int(m.MobId)] {
			var owners []owner
			if by != 0 {
				owners = questOwners(by)
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

	heard := topicHeardGrants(quests.GetAllQuests())
	files, _ := dialogue.LoadAllFiles()
	for _, df := range files {
		if df == nil {
			continue
		}
		for _, t := range df.Topics {
			add(t.GivesItem, fmt.Sprintf("mob %d topic %s", df.MobId, t.Key),
				topicOwners(t, heard[mobTopic{mob: df.MobId, key: t.Key}]...)...)
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
		ItemAvailable: func(id, questId int, has func(string) bool) bool {
			return available(itemSrc[id], questId, has)
		},
		SpellEffect: func(id string) string {
			if s := spells.GetSpell(id); s != nil {
				return s.EffectType
			}
			return ""
		},
		StarterSpells: slices.Clone(characters.StarterSpells),
	}
}

// externalGrants is World.External (Registry): the grants outside triggers
// and topics, sorted by
// Where: every grant_quest action in a behaviour tree file (mob, room,
// archetype and item trees), with the topics its branch answers, and every
// item whose questtoken grants a token. A tree that grants one token from
// two branches gives one grant with both branches' topics.
func externalGrants() []questloop.ExternalGrant {
	byWhere := map[string]*questloop.ExternalGrant{}
	open := map[string]bool{} // a branch keys on no topic: the grant is taken as is
	addTree := func(who, path string, mobId int) {
		gs, err := behaviortree.QuestGrantsInFile(path)
		if err != nil {
			return // a tree that does not parse is the behaviour validator's finding
		}
		for _, g := range gs {
			where := fmt.Sprintf("%s tree grant_quest %s", who, g.Token)
			x := byWhere[where]
			if x == nil {
				x = &questloop.ExternalGrant{Where: where, Token: g.Token, MobId: mobId}
				byWhere[where] = x
			}
			if len(g.Topics) == 0 {
				open[where] = true
			}
			for _, k := range g.Topics {
				if !slices.Contains(x.Topics, k) {
					x.Topics = append(x.Topics, k)
				}
			}
		}
	}
	for _, row := range behaviortree.ListTreeFiles() {
		switch row.Kind {
		case "mob":
			addTree(fmt.Sprintf("mob %d", row.MobId), row.Path, row.MobId)
		case "room":
			addTree(fmt.Sprintf("room %d", row.RoomId), row.Path, 0)
		default:
			addTree(row.Kind+" "+row.Name, row.Path, 0)
		}
	}
	for _, spec := range items.GetAllItemSpecs() {
		if tok := spec.QuestToken; tok != "" {
			where := fmt.Sprintf("item %d questtoken", spec.ItemId)
			byWhere[where] = &questloop.ExternalGrant{Where: where, Token: tok, ItemId: spec.ItemId}
		}
	}
	out := make([]questloop.ExternalGrant, 0, len(byWhere))
	for where, g := range byWhere {
		if open[where] {
			g.Topics = nil
		}
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Where < out[j].Where })
	return out
}

// available reports whether one of ss is open to a player on quest questId
// whose progress is has: a source questId does not own, or one it owns
// through gates the player already holds.
func available(ss []itemSource, questId int, has func(string) bool) bool {
	for _, s := range ss {
		i := slices.IndexFunc(s.owners, func(o owner) bool { return o.quest == questId })
		if i < 0 {
			return true
		}
		if g := s.owners[i].gates; len(g) > 0 && has != nil && !slices.ContainsFunc(g, func(tok string) bool { return !has(tok) }) {
			return true
		}
	}
	return false
}

// mobTopic names one NPC's topic: the mob template id and the topic key.
type mobTopic struct {
	mob int
	key string
}

// topicHeardGrants is, per NPC topic, the tokens the topic_heard triggers on
// it grant (#375 phase 3b moved mid-quest grantsQuest onto those triggers).
// Revokes (a leading "-") are left out.
func topicHeardGrants(qs []quests.Quest) map[mobTopic][]string {
	out := map[mobTopic][]string{}
	for _, q := range qs {
		for _, t := range q.Triggers {
			if t.Event != "topic_heard" || t.Mob <= 0 || t.Topic == "" {
				continue
			}
			k := mobTopic{mob: t.Mob, key: t.Topic}
			var walk func([]quests.ActionDef)
			walk = func(as []quests.ActionDef) {
				for _, a := range as {
					if g := a.Grant; g != "" && !strings.HasPrefix(g, "-") && !slices.Contains(out[k], g) {
						out[k] = append(out[k], g)
					}
					if a.Sequence != nil {
						walk(a.Sequence.OnComplete)
					}
				}
			}
			walk(t.Actions)
		}
	}
	return out
}

// topicOwners is the quests a topic's item grant belongs to: its own quest,
// every quest its questRequired gate names and the quest of the token it
// grants, each with the tokens of that quest the gate names or it grants. A
// "lost the page?" topic gated on 20-vella_journal is quest 20's own
// replacement, open from that step on; a topic of quest 70 gated on 63-start
// is open to quest 63 from start on. heard is the tokens topic_heard triggers
// on the topic grant; they stand in for GrantsQuest when the topic carries
// none.
func topicOwners(t dialogue.Topic, heard ...string) []owner {
	var out []owner
	at := func(id int) int {
		i := slices.IndexFunc(out, func(o owner) bool { return o.quest == id })
		if i < 0 {
			out = append(out, owner{quest: id})
			i = len(out) - 1
		}
		return i
	}
	if t.Quest > 0 {
		at(t.Quest)
	}
	gate := func(tok string) {
		idStr, _, _ := strings.Cut(tok, quests.QuestTokenSeparator)
		if id, err := strconv.Atoi(idStr); err == nil {
			i := at(id)
			out[i].gates = append(out[i].gates, tok)
		}
	}
	for _, tok := range t.QuestRequired {
		gate(tok)
	}
	// A topic that grants a step hands its item over with that step (quest
	// 20's vellahandoff grants 20-vella_journal and the journal together), so
	// for that quest it is a source only from that step on.
	// A topic whose grant moved onto a topic_heard trigger does the same.
	if g := t.GrantsQuest; g != "" && !strings.HasPrefix(g, "-") {
		gate(g)
	} else if g == "" {
		for _, tok := range heard {
			gate(tok)
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

package gmcp

// Quest and dialogue reference checks for Build.Room.Delete and
// Build.Item.Delete (#375 phase 2). The quest loop's refs rule and the
// dialogue door refuse at boot a quest or a dialogue file that names a
// missing room or item, so a builder delete that leaves one naming the
// deleted id would stop the next boot. These run the doors' own rules over
// the world with the id gone, and report only what the delete adds: a
// finding the world already has is not the delete's doing. No separate walker: ValidateQuestRefs already visits every
// room and item field a quest carries.

import (
	"sort"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/questworld"
)

// questRefsWithout is the refs findings w gains when gone replaces its
// reference checks, as each finding's "quest <id> refs: <where>: <what>".
func questRefsWithout(w questloop.World, gone quests.QuestValidators) []string {
	if w.Refs == nil {
		return nil
	}
	before := map[string]bool{}
	for _, f := range questloop.Validate(w) {
		if f.Rule == "refs" {
			before[f.Key()] = true
		}
	}
	w.Refs = &gone
	var out []string
	for _, f := range questloop.Validate(w) {
		if f.Rule == "refs" && !before[f.Key()] {
			out = append(out, f.Where)
		}
	}
	return out
}

func questRefsWithoutRoom(w questloop.World, roomId int) []string {
	if w.Refs == nil {
		return nil
	}
	gone, exists := *w.Refs, w.Refs.RoomExists
	gone.RoomExists = func(id int) bool { return id != roomId && exists(id) }
	return questRefsWithout(w, gone)
}

func questRefsWithoutItem(w questloop.World, itemId int) []string {
	if w.Refs == nil {
		return nil
	}
	gone, exists := *w.Refs, w.Refs.ItemExists
	gone.ItemExists = func(id int) bool { return id != itemId && exists(id) }
	return questRefsWithout(w, gone)
}

func questRefsWithoutMob(w questloop.World, mobId int) []string {
	if w.Refs == nil {
		return nil
	}
	gone, exists := *w.Refs, w.Refs.MobExists
	gone.MobExists = func(id int) bool { return id != mobId && exists(id) }
	return questRefsWithout(w, gone)
}

// mobQuestRefEntries is the mob delete's refusal rows for those findings.
func mobQuestRefEntries(wheres []string) []mobRefEntry {
	var out []mobRefEntry
	for _, s := range wheres {
		out = append(out, mobRefEntry{Kind: "quest", Id: s})
	}
	return out
}

// mobQuestRefs is mobRefIterators.questRefs over the live registries.
func mobQuestRefs(mobId int) []mobRefEntry {
	w := questworld.Registry()
	return mobQuestRefEntries(questRefsWithoutMob(w, mobId))
}

// dialogueRefsWithoutItem is the dialogue problems w gains when itemId is
// gone: a topic gate or effect (requiresItem, itemExcluded, carriesItem,
// givesItem) that names it.
func dialogueRefsWithoutItem(w dialogue.World, itemId int) []string {
	before := map[string]bool{}
	for _, p := range dialogue.ValidateWorld(w) {
		before[p.String()] = true
	}
	exists := w.ItemExists
	w.ItemExists = func(id int) bool { return id != itemId && exists != nil && exists(id) }
	var out []string
	for _, p := range dialogue.ValidateWorld(w) {
		if s := p.String(); !before[s] {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// roomQuestRefs is buildDeps.questRefs over the live registries.
func roomQuestRefs(roomId int) []string {
	w := questworld.Registry()
	return questRefsWithoutRoom(w, roomId)
}

// itemQuestRefs is refIterators.questRefs over the live registries.
func itemQuestRefs(itemId int) []itemRef {
	w := questworld.Registry()
	var out []itemRef
	for _, s := range questRefsWithoutItem(w, itemId) {
		out = append(out, itemRef{Kind: "quest", Id: s})
	}
	dw, _ := dialogue.RegistryWorld(nil)
	for _, s := range dialogueRefsWithoutItem(dw, itemId) {
		out = append(out, itemRef{Kind: "dialogue", Id: s})
	}
	return out
}

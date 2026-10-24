package questloop

import (
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// Way is one thing in the world that grants a quest token, as the rules see
// it: a trigger, a topic, or a completion reward. Where is its stable backlog
// name. The walkthrough (questengine.Walk) drives each Way.
type Way struct {
	Where   string
	Trigger *quests.TriggerDef // set for a trigger
	Owner   int                // the quest whose file holds the trigger
	Nested  bool               // granted inside a sequence's on_complete
	Topic   *dialogue.Topic    // set for a topic
	MobId   int                // the topic's NPC
	File    *dialogue.DialogueFile
	Reward  bool // rewards.questid
}

// WaysInto maps each token ("53-firstkill") to every Way that grants it, the
// same index every rule reads.
func WaysInto(w World) map[string][]Way {
	out := map[string][]Way{}
	files := map[int]*dialogue.DialogueFile{}
	for _, df := range w.Dialogue {
		if df != nil {
			files[df.MobId] = df
		}
	}
	for tok, ws := range indexGrants(w) {
		for _, x := range ws {
			out[tok] = append(out[tok], Way{
				Where: x.where, Trigger: x.trigger, Owner: x.owner, Nested: x.nested,
				Topic: x.topic, MobId: x.mobId, File: files[x.mobId], Reward: x.reward,
			})
		}
	}
	return out
}

// Sources answers the walkthrough's world questions. Each func may be nil,
// which skips that check (a fixture world that only tests step order).
type Sources struct {
	// ItemSources names where an item can come from ("mob 9110 loot",
	// "shop 9123", "room 5203 floor", "forage marsh", "recipe iron-dagger",
	// "quest 32 give_item", "mob 9106 topic blood_offer"). Empty: no source.
	ItemSources func(itemId int) []string
	// MobSpawned reports whether a mob template is placed by some room's
	// spawninfo or a quest spawn_mob action.
	MobSpawned func(mobId int) bool
	// ItemOutside reports whether an item has a source that is not one of
	// quest questId's own grants: its give_item, spawn_item and reward
	// actions, a topic of that quest or gated on its steps, a mob only that
	// quest spawns. The walk's skip probe gives a later Way an item only when
	// this holds, since an item only the quest's own steps hand out cannot be
	// held before those steps. Nil: every item counts as outside.
	ItemOutside func(itemId, questId int) bool
}

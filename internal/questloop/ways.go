package questloop

import (
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// Way is one thing in the world that grants a quest token, as the rules see
// it: a trigger, a topic, or a completion reward (WaysInto), or for the
// walkthrough also an external grant (ExternalWays). Where is its stable
// backlog name. The walkthrough (questengine.Walk) drives each Way.
type Way struct {
	Where    string
	Trigger  *quests.TriggerDef // set for a trigger
	Owner    int                // the quest whose file holds the trigger
	Nested   bool               // granted inside a sequence's on_complete
	Topic    *dialogue.Topic    // set for a topic
	MobId    int                // the topic's NPC
	File     *dialogue.DialogueFile
	Reward   bool           // rewards.questid
	External *ExternalGrant // set for a behaviour-tree grant or an item questtoken
}

// ExternalWays adds a Way for each external grant to ways, keyed by token.
func ExternalWays(ways map[string][]Way, ext []ExternalGrant) {
	for i := range ext {
		g := &ext[i]
		ways[g.Token] = append(ways[g.Token], Way{Where: g.Where, MobId: g.MobId, External: g})
	}
}

// ExternalGrant is a grant outside the trigger engine and the dialogue
// topics, which Validate does not read and the walkthrough does: a
// behaviour tree's grant_quest action, or an item's questtoken (holding the
// item grants the token).
type ExternalGrant struct {
	// Where is the stable backlog name: "mob 9100 tree grant_quest 30-rite",
	// "room 407 tree grant_quest 4-investigate", "item 40035 questtoken".
	Where string
	Token string
	// MobId is the tree's NPC (a mob tree); 0 for a room, archetype or item.
	MobId int
	// Topics are the topic_is keys the tree branch checks before the grant;
	// empty when it keys on no topic.
	Topics []string
	// ItemId is set for an item questtoken.
	ItemId int
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
	// ItemAvailable reports whether a player on quest questId, whose quest
	// progress is has, can obtain an item without doing the quest's later
	// steps: the item has a source that is not one of the quest's own grants
	// (its give_item, spawn_item and reward actions, a topic of that quest, a
	// mob only that quest spawns), or a source gated on steps of the quest
	// that has already holds (a topic gated on questId-start is open from
	// start on). The walk's skip probe gives a later Way an item only when
	// the walk player already holds it or this holds. Nil: every item counts
	// as available.
	ItemAvailable func(itemId, questId int, has func(token string) bool) bool
	// External lists the grants behaviour trees and item questtokens make.
	// The walk adds them to the Ways into their tokens.
	External []ExternalGrant
}

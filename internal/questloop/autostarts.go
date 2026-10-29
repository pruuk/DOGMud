package questloop

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// AutoStarts maps each quest whose -start token is granted automatically to
// the Wheres of those grants, sorted (owner ruling 6, #375 phase 5): a quest
// trigger's grant action (a sequence's on_complete included) or a behaviour
// tree's grant_quest. Such a grant would take a shelved quest back up
// without the player choosing to, so the quest cannot be abandoned. An NPC
// offer, a completion reward (rewards.questid) and an item questtoken do not
// count. It reads the same index as every rule (WaysInto).
func AutoStarts(w World) map[int][]string {
	out := map[int][]string{}
	for tok, ways := range indexGrants(w) {
		qid, step := quests.TokenToParts(tok)
		if qid <= 0 || step != `start` {
			continue
		}
		for _, x := range ways {
			tree := x.external != nil && x.external.ItemId == 0
			if x.trigger != nil || tree {
				out[qid] = append(out[qid], x.where)
			}
		}
	}
	for q := range out {
		slices.Sort(out[q])
	}
	return out
}

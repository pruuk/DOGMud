package dialogue

import (
	"slices"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// The offer index (#375 phase 5): which NPCs offer each quest, that is, have a
// topic granting its -start. A quest may be abandoned only when some NPC
// offers it (owner ruling 3). Dialogue loads lazily per NPC, so the index is
// built once at boot from every file and kept current by SaveDialogueFile.
var (
	offerMu     sync.RWMutex
	offersByMob = map[int][]int{} // mob id -> quest ids it offers
	offerers    = map[int][]int{} // quest id -> mob ids, sorted
)

func offeredQuests(df *DialogueFile) []int {
	var out []int
	for _, t := range df.Topics {
		if t.GrantsQuest == "" {
			continue
		}
		qid, step := quests.TokenToParts(t.GrantsQuest)
		if qid > 0 && step == `start` && !slices.Contains(out, qid) {
			out = append(out, qid)
		}
	}
	return out
}

func rebuildOfferersLocked() {
	offerers = map[int][]int{}
	for mob, qs := range offersByMob {
		for _, q := range qs {
			offerers[q] = append(offerers[q], mob)
		}
	}
	for q := range offerers {
		slices.Sort(offerers[q])
	}
}

// BuildOfferIndex replaces the index from files.
func BuildOfferIndex(files []*DialogueFile) {
	offerMu.Lock()
	defer offerMu.Unlock()
	offersByMob = map[int][]int{}
	for _, df := range files {
		if df != nil {
			offersByMob[df.MobId] = offeredQuests(df)
		}
	}
	rebuildOfferersLocked()
}

// UpdateOfferIndex replaces one file's offers.
func UpdateOfferIndex(df *DialogueFile) {
	if df == nil {
		return
	}
	offerMu.Lock()
	defer offerMu.Unlock()
	offersByMob[df.MobId] = offeredQuests(df)
	rebuildOfferersLocked()
}

// Offerers is the sorted mob ids that offer questId (shared; do not modify).
func Offerers(questId int) []int {
	offerMu.RLock()
	defer offerMu.RUnlock()
	return offerers[questId]
}

// SetOfferIndexForTest replaces the index and returns a restore func.
func SetOfferIndexForTest(byQuest map[int][]int) func() {
	offerMu.Lock()
	prevMob, prevQ := offersByMob, offerers
	offersByMob = map[int][]int{}
	offerers = map[int][]int{}
	for q, mobs := range byQuest {
		for _, m := range mobs {
			offersByMob[m] = append(offersByMob[m], q)
		}
	}
	rebuildOfferersLocked()
	offerMu.Unlock()
	return func() {
		offerMu.Lock()
		offersByMob, offerers = prevMob, prevQ
		offerMu.Unlock()
	}
}

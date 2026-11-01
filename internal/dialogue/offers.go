package dialogue

import (
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// The offer index (#375 phase 5): which NPCs offer each quest, that is, have a
// topic granting its -start. A quest may be abandoned only when some NPC
// offers it (owner ruling 3) through a topic that can show again while it is
// shelved (a resumer, see resumes). Dialogue loads lazily per NPC, so the
// index is built once at boot from every file and kept current by
// SaveDialogueFile.
var (
	offerMu     sync.RWMutex
	offersByMob = map[int][]offer{} // mob id -> quests it offers
	offerers    = map[int][]int{}   // quest id -> mob ids, sorted
	resumers    = map[int][]int{}   // quest id -> mob ids with a resuming offer, sorted
)

// offer is one quest a mob offers, and whether any of its offering topics
// can resume that quest from the shelf.
type offer struct {
	questId int
	resumes bool
}

// gateQuestId is the quest a gate token ("31-start") or flag key
// ("11-branch") belongs to, or 0 when it names none.
func gateQuestId(s string) int {
	head, _, _ := strings.Cut(s, quests.QuestTokenSeparator)
	id, err := strconv.Atoi(head)
	if err != nil {
		return 0
	}
	return id
}

// resumes reports whether t, an offer of questId, can show again to a player
// who shelved questId. Its own quest's tokens and flags are fine: a shelved
// quest is not held. A questExcluded token or questFlagExcluded key of
// another quest is not: once the player gains it (quest 88's road_toke
// excludes 31-start) the offer hides for good.
func resumes(t *Topic, questId int) bool {
	for _, tok := range t.QuestExcluded {
		if gateQuestId(tok) != questId {
			return false
		}
	}
	for key := range t.QuestFlagExcluded {
		if gateQuestId(key) != questId {
			return false
		}
	}
	return true
}

func offeredQuests(df *DialogueFile) []offer {
	var out []offer
	for i := range df.Topics {
		t := &df.Topics[i]
		if t.GrantsQuest == "" {
			continue
		}
		qid, step := quests.TokenToParts(t.GrantsQuest)
		if qid <= 0 || step != `start` {
			continue
		}
		r := resumes(t, qid)
		if j := slices.IndexFunc(out, func(o offer) bool { return o.questId == qid }); j >= 0 {
			out[j].resumes = out[j].resumes || r
			continue
		}
		out = append(out, offer{questId: qid, resumes: r})
	}
	return out
}

func rebuildOfferersLocked() {
	offerers = map[int][]int{}
	resumers = map[int][]int{}
	for mob, os := range offersByMob {
		for _, o := range os {
			offerers[o.questId] = append(offerers[o.questId], mob)
			if o.resumes {
				resumers[o.questId] = append(resumers[o.questId], mob)
			}
		}
	}
	for q := range offerers {
		slices.Sort(offerers[q])
	}
	for q := range resumers {
		slices.Sort(resumers[q])
	}
}

// BuildOfferIndex replaces the index from files.
func BuildOfferIndex(files []*DialogueFile) {
	offerMu.Lock()
	defer offerMu.Unlock()
	offersByMob = map[int][]offer{}
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

// Resumers is the sorted mob ids with an offer of questId that can show again
// while it is shelved (shared; do not modify).
func Resumers(questId int) []int {
	offerMu.RLock()
	defer offerMu.RUnlock()
	return resumers[questId]
}

// Resumable reports whether some NPC offers questId through a topic that can
// take it back off the shelf, the condition for abandoning it.
func Resumable(questId int) bool {
	return len(Resumers(questId)) > 0
}

// Unresumable is the sorted quest ids that some NPC offers but none can
// resume: offered, yet not abandonable, by design.
func Unresumable() []int {
	offerMu.RLock()
	defer offerMu.RUnlock()
	var out []int
	for q := range offerers {
		if len(resumers[q]) == 0 {
			out = append(out, q)
		}
	}
	slices.Sort(out)
	return out
}

// SetOfferIndexForTest replaces the index and returns a restore func. Every
// offer it sets can resume.
func SetOfferIndexForTest(byQuest map[int][]int) func() {
	offerMu.Lock()
	prevMob, prevQ, prevR := offersByMob, offerers, resumers
	offersByMob = map[int][]offer{}
	for q, mobs := range byQuest {
		for _, m := range mobs {
			offersByMob[m] = append(offersByMob[m], offer{questId: q, resumes: true})
		}
	}
	rebuildOfferersLocked()
	offerMu.Unlock()
	return func() {
		offerMu.Lock()
		offersByMob, offerers, resumers = prevMob, prevQ, prevR
		offerMu.Unlock()
	}
}

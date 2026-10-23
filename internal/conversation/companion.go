package conversation

import (
	"strings"
	"unicode"

	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The aicompanion module reaches AskForOwner through companionai, so modules
// need not import internal/conversation (R13).
func init() {
	companionai.SetNpcAsker(AskForOwner)
}

// CompanionTopic resolves an owner's `companion-ask <who> about <what>`
// words to one of the NPC's eligible info topics (R13): the key itself, or
// the one label containing every word typed. Offers and hand-ins are never
// candidates: accepting a quest or handing over an item is the owner's own
// act. labels lists every eligible info label, for the refusal line.
func CompanionTopic(user *users.UserRecord, mob *mobs.Mob, what string) (key string, labels []string, ok bool) {
	if user == nil || user.Character == nil || mob == nil || mob.Character.IsCharmed() {
		return "", nil, false
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	if df == nil {
		return "", nil, false
	}
	ps := PlayerState(user)
	what = strings.TrimSpace(what)
	words := splitWords(what)
	var hits []string
	for i := range df.Topics {
		t := &df.Topics[i]
		// A behavior topic can move the owner (a ferry, a stake prompt), which
		// a companion never does on the owner's behalf.
		if t.Kind != dialogue.KindInfo || t.Behavior || !df.Eligible(t, ps) {
			continue
		}
		labels = append(labels, t.Label)
		if strings.EqualFold(t.Key, what) {
			key = t.Key
		}
		if labelHasWords(t.Label, words) {
			hits = append(hits, t.Key)
		}
	}
	if key != "" {
		return key, labels, true
	}
	if len(hits) == 1 {
		return hits[0], labels, true
	}
	return "", labels, false
}

// splitWords lowercases s and splits it on anything that is not a letter or
// a digit, so punctuation and underscores never glue words together.
func splitWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// labelHasWords reports whether every typed word is a whole word of label or
// the start (three letters or more) of one. "ore" is not in "more".
func labelHasWords(label string, typed []string) bool {
	if len(typed) == 0 {
		return false
	}
	lw := splitWords(label)
	for _, w := range typed {
		found := false
		for _, l := range lw {
			if w == l || (len(w) >= 3 && strings.HasPrefix(l, w)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// AskForOwner plays an info topic a companion was authorized to ask about,
// exactly as the owner choosing it, minus the owner's own line (the
// companion already spoke aloud) and minus the menu (the owner opened none).
// key comes from CompanionTopic at command time, never from the model's
// words. Installed as the companion NPC asker at init.
func AskForOwner(ownerUserId int, mobInstanceId int, key string, authorized bool) bool {
	user := users.GetByUserId(ownerUserId)
	if user == nil || user.Character == nil || !authorized {
		return false
	}
	// The owner must be able to hold the conversation: the talk menu's own
	// gate (awake, idle, not in a fight or held), the NPC in the room, seen
	// at full sight and perceived. An open prompt does not matter: no menu
	// opens here.
	if reason, _ := user.MenuBlockedReason(); reason != menu.CloseNone {
		return false
	}
	mob, room, ok := present(user, mobInstanceId)
	if !ok || mob.Character.IsCharmed() {
		return false
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	buf := &lineBuffer{user: user}
	ps := talkState(user, buf)
	t := df.FindTopic(key)
	if t == nil || t.Kind != dialogue.KindInfo || t.Behavior || !df.Eligible(t, ps) {
		mob.Command(`emote shrugs.`)
		return true
	}
	if applyWithHandover(user, room, mob, df, t, ps, buf) {
		npcSay(mob, room, t.Text)
		sendHints(user, t.Hints)
		after(user, room, mob, df, t)
	}
	return true
}

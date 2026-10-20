package conversation

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

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
	words := strings.Fields(strings.ToLower(what))
	var hits []string
	for i := range df.Topics {
		t := &df.Topics[i]
		if t.Kind != dialogue.KindInfo || !df.Eligible(t, ps) {
			continue
		}
		labels = append(labels, t.Label)
		if strings.EqualFold(t.Key, what) {
			key = t.Key
		}
		label := strings.ToLower(t.Label)
		all := len(words) > 0
		for _, w := range words {
			if !strings.Contains(label, w) {
				all = false
				break
			}
		}
		if all {
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

// AskForOwner plays an info topic a companion was authorized to ask about,
// exactly as the owner choosing it, minus the owner's own line (the
// companion already spoke aloud) and minus the menu (the owner opened none).
// key comes from CompanionTopic at command time, never from the model's
// words. Installed as the companion NPC asker in Task 19.
func AskForOwner(ownerUserId int, mobInstanceId int, key string, authorized bool) bool {
	user := users.GetByUserId(ownerUserId)
	mob := mobs.GetInstance(mobInstanceId)
	if user == nil || user.Character == nil || mob == nil || !authorized {
		return false
	}
	if mob.Character.IsCharmed() || mob.Character.RoomId != user.Character.RoomId {
		return false
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return false
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	buf := &lineBuffer{user: user}
	ps := talkState(user, buf)
	t := df.FindTopic(key)
	if t == nil || t.Kind != dialogue.KindInfo || !df.Eligible(t, ps) {
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

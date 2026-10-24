package conversation

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// nextOffers are the keys of df's offers unlocked by finalToken (P1: a
// chain is an offer whose questRequired names the finished quest's final
// token), eligible for ps, in authored order.
func nextOffers(df *dialogue.DialogueFile, finalToken string, ps *dialogue.PlayerState) []string {
	if df == nil || finalToken == "" {
		return nil
	}
	var out []string
	for i := range df.Topics {
		t := &df.Topics[i]
		if t.Kind != dialogue.KindOffer || !slices.Contains(t.QuestRequired, finalToken) {
			continue
		}
		if df.Eligible(t, ps) {
			out = append(out, t.Key)
		}
	}
	return out
}

// queueNextOffer runs after a hand-in that granted finalToken: when exactly
// one next offer opened on this NPC, it queues that offer's pitch to run
// after the completion lines. It reports whether it queued one; when it did
// not, the caller re-shows the menu as before.
func queueNextOffer(user *users.UserRecord, mobInstanceId int, df *dialogue.DialogueFile, finalToken string) bool {
	keys := nextOffers(df, finalToken, heldAlso(PlayerState(user), finalToken))
	if len(keys) != 1 {
		return false
	}
	events.AddToQueue(events.TalkNextOffer{
		UserId:        user.UserId,
		MobInstanceId: mobInstanceId,
		TopicKey:      keys[0],
	}, events.TalkNextOfferPriority)
	return true
}

// HandleTalkNextOffer pitches the queued next offer as if the player had
// chosen it: their line, the pitch, then the card. If the NPC is gone or the
// offer no longer applies, the talk menu is re-shown (or nothing, as
// openMenu decides). While a menu gate holds or a prompt is open the card
// could not open, so nothing is said at all: the player never speaks a line
// they did not choose, and the offer waits in the talk menu with its [!].
func HandleTalkNextOffer(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.TalkNextOffer)
	if !ok {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}
	if reason, promptOpen := user.MenuBlockedReason(); reason != menu.CloseNone || promptOpen {
		return events.Continue
	}
	choose(user, evt.MobInstanceId, evt.TopicKey)
	return events.Continue
}

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

// queueRedraw brings the talk back after a choice that granted token, on the
// event queue at TalkNextOfferPriority. GiveQuest only queues the token: the
// quest engine lands it, prints the banner and rewards and runs any
// quest_granted trigger (a trainer's own offer skips the arrival talk, #940)
// at priority 0 in the same pass, so what follows is built from the state
// those leave. A hand-in queues TalkNextOffer, which pitches the one next
// offer its final token opened or re-shows the menu; any other grant queues
// TalkMenuAgain.
func queueRedraw(user *users.UserRecord, mobInstanceId int, token string, handIn bool) {
	if handIn {
		events.AddToQueue(events.TalkNextOffer{
			UserId:        user.UserId,
			MobInstanceId: mobInstanceId,
			FinalToken:    token,
		}, events.TalkNextOfferPriority)
		return
	}
	events.AddToQueue(events.TalkMenuAgain{UserId: user.UserId, MobInstanceId: mobInstanceId}, events.TalkNextOfferPriority)
}

// talkBlocked reports whether a menu gate holds or a prompt is open, when a
// queued redraw opens nothing.
func talkBlocked(user *users.UserRecord) bool {
	reason, promptOpen := user.MenuBlockedReason()
	return reason != menu.CloseNone || promptOpen
}

// HandleTalkNextOffer runs after a hand-in, once its final token has landed.
// When exactly one offer on the NPC is gated on that token and eligible, it
// pitches it as if the player had chosen it: their line, the pitch, then the
// card. Several next offers, or none, re-show the talk menu (the offers wait
// with their [!]); nothing opens when the NPC is gone. While a menu gate holds
// or a prompt is open the card could not open, so nothing is said at all: the
// player never speaks a line they did not choose, and the offer waits in the
// talk menu with its [!].
func HandleTalkNextOffer(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.TalkNextOffer)
	if !ok {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil || talkBlocked(user) {
		return events.Continue
	}
	mob, _, ok := present(user, evt.MobInstanceId)
	if !ok {
		return events.Continue
	}
	df := dialogue.Load(int(mob.MobId), mob.Zone)
	if keys := nextOffers(df, evt.FinalToken, PlayerState(user)); len(keys) == 1 {
		choose(user, evt.MobInstanceId, keys[0])
		return events.Continue
	}
	openMenu(user, evt.MobInstanceId)
	return events.Continue
}

// HandleTalkMenuAgain re-shows the talk menu queued by a choice that granted
// a quest token (an accepted offer), after the grant and its step-start chain
// have run (#940: a quest_granted trigger may move the step before the menu
// should be drawn). While a menu gate holds or a prompt is open nothing opens,
// and the hand-in waits in the talk menu with its [?].
func HandleTalkMenuAgain(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.TalkMenuAgain)
	if !ok {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil || talkBlocked(user) {
		return events.Continue
	}
	openMenu(user, evt.MobInstanceId)
	return events.Continue
}

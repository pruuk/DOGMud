package quests

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/textutil"
)

func TestActionNarrationSendTextIsActorAndRoomTextIsObserver(t *testing.T) {
	v := ActionDef{SendText: "You pocket the disc."}.Narration()
	if len(v.Actor) != 1 || v.Actor[0] != "You pocket the disc." || len(v.Observer) != 0 {
		t.Fatalf("actor: %+v", v)
	}
	v = ActionDef{RoomText: "{actor} pockets a disc."}.Narration()
	if len(v.Observer) != 1 || v.Observer[0] != "{actor} pockets a disc." || len(v.Actor) != 0 {
		t.Fatalf("observer: %+v", v)
	}
	if v := (ActionDef{Grant: "1-end"}).Narration(); v.Len() != 0 {
		t.Fatalf("a non-text action narrates nothing, got %+v", v)
	}
}

func TestActionNarrateSubstitutesThePlayer(t *testing.T) {
	roles := ActionDef{RoomText: "{actor} pockets a disc."}.Narrate(textutil.TokenContext{ActorName: "Aliceia"})
	if roles.Observer != "Aliceia pockets a disc." || roles.Actor != "" {
		t.Fatalf("roles: %+v", roles)
	}
}

func TestRewardNarration(t *testing.T) {
	r := QuestReward{PlayerMessage: "The clerk thanks you.", RoomMessage: "The clerk thanks {actor}."}
	roles := r.Narrate(textutil.TokenContext{ActorName: "Aliceia"})
	if roles.Actor != "The clerk thanks you." || roles.Observer != "The clerk thanks Aliceia." {
		t.Fatalf("roles: %+v", roles)
	}
}

// validQuest is the smallest quest Validate accepts, with one text action.
func validQuest(a ActionDef) *Quest {
	return &Quest{
		QuestId: 9001, Name: "Probe",
		Steps:    []QuestStep{{Id: "start"}},
		Triggers: []TriggerDef{{Event: "command", Actions: []ActionDef{a}}},
	}
}

// The dialogue event fired from the old `ask` chain, which is gone (#375
// Task 19): a trigger on it could never fire, so Validate refuses it.
// Control: the same quest on a live event passes.
func TestValidateRefusesTheRetiredDialogueEvent(t *testing.T) {
	q := validQuest(ActionDef{Grant: "9001-start"})
	if err := q.Validate(); err != nil {
		t.Fatalf("control quest refused: %v", err)
	}
	q.Triggers[0].Event = "dialogue"
	err := q.Validate()
	if err == nil || !strings.Contains(err.Error(), `invalid event "dialogue"`) {
		t.Fatalf("a dialogue trigger must be refused, got %v", err)
	}
}

// #375 closing step: the room feature event was split into look_at and
// search_at, so the event names the verb family. The old event and any verb
// key are refused with a message naming both new events; the new events
// load with no verb and, like the old one, take no count.
func TestValidateRetiredInteract(t *testing.T) {
	namesBoth := func(err error) bool {
		return err != nil && strings.Contains(err.Error(), EventLookAt) && strings.Contains(err.Error(), EventSearchAt)
	}

	q := validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event = retiredInteractEvent
	if err := q.Validate(); !namesBoth(err) || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("the retired event must be refused naming look_at and search_at, got %v", err)
	}

	for _, ev := range []string{EventLookAt, EventSearchAt, "command"} {
		q = validQuest(ActionDef{Grant: "9001-start"})
		q.Triggers[0].Event, q.Triggers[0].RetiredVerb = ev, "search"
		if err := q.Validate(); !namesBoth(err) || !strings.Contains(err.Error(), `verb "search"`) {
			t.Fatalf("a verb on a %s trigger must be refused naming look_at and search_at, got %v", ev, err)
		}
	}

	for _, ev := range []string{EventLookAt, EventSearchAt} {
		q = validQuest(ActionDef{Grant: "9001-start"})
		q.Triggers[0].Event = ev
		if err := q.Validate(); err != nil {
			t.Fatalf("%s with no verb refused: %v", ev, err)
		}
		q.Triggers[0].Count = 2
		if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "count on a "+ev+" trigger") {
			t.Fatalf("count on %s must be refused, got %v", ev, err)
		}
	}
}

func TestValidateRefusesAnActionThatSetsBothTexts(t *testing.T) {
	err := validQuest(ActionDef{SendText: "You see it.", RoomText: "{actor} sees it."}).Validate()
	if err == nil || !strings.Contains(err.Error(), "both actor and observer") {
		t.Fatalf("expected the both-set refusal, got %v", err)
	}
}

func TestValidateRefusesWhitespaceOnlyQuestText(t *testing.T) {
	err := validQuest(ActionDef{SendText: "  "}).Validate()
	// The full "action <j> actor:" prefix, not a bare "actor": the canonical
	// key is a common word and several other refusals mention {actor}, so a
	// substring that loose would pass on the wrong error.
	if err == nil || !strings.Contains(err.Error(), "action 0 actor:") {
		t.Fatalf("expected an actor-line refusal, got %v", err)
	}
	q := validQuest(ActionDef{SendText: "You see it."})
	q.Rewards.RoomMessage = " "
	err = q.Validate()
	if err == nil || !strings.Contains(err.Error(), "rewards") {
		t.Fatalf("expected a rewards refusal, got %v", err)
	}
	if err := validQuest(ActionDef{SendText: "You see it."}).Validate(); err != nil {
		t.Fatalf("ordinary text must validate, got %v", err)
	}
}

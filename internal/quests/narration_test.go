package quests

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/textutil"
)

func TestActionNarrationSendTextIsActorAndRoomTextIsObserver(t *testing.T) {
	v := ActionDef{SendText: "You pocket the disc."}.Narration()
	if len(v.Actor) != 1 || v.Actor[0] != "You pocket the disc." || len(v.Observer) != 0 {
		t.Fatalf("send_text: %+v", v)
	}
	v = ActionDef{RoomText: "{source} pockets a disc."}.Narration()
	if len(v.Observer) != 1 || v.Observer[0] != "{source} pockets a disc." || len(v.Actor) != 0 {
		t.Fatalf("room_text: %+v", v)
	}
	if v := (ActionDef{Grant: "1-end"}).Narration(); v.Len() != 0 {
		t.Fatalf("a non-text action narrates nothing, got %+v", v)
	}
}

func TestActionNarrateSubstitutesThePlayer(t *testing.T) {
	roles := ActionDef{RoomText: "{source} pockets a disc."}.Narrate(textutil.TokenContext{SourceName: "Aliceia"})
	if roles.Observer != "Aliceia pockets a disc." || roles.Actor != "" {
		t.Fatalf("roles: %+v", roles)
	}
}

func TestRewardNarration(t *testing.T) {
	r := QuestReward{PlayerMessage: "The clerk thanks you.", RoomMessage: "The clerk thanks {source}."}
	roles := r.Narrate(textutil.TokenContext{SourceName: "Aliceia"})
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

func TestValidateRefusesAnActionThatSetsBothTexts(t *testing.T) {
	err := validQuest(ActionDef{SendText: "You see it.", RoomText: "{source} sees it."}).Validate()
	if err == nil || !strings.Contains(err.Error(), "both send_text and room_text") {
		t.Fatalf("expected the both-set refusal, got %v", err)
	}
}

func TestValidateRefusesWhitespaceOnlyQuestText(t *testing.T) {
	err := validQuest(ActionDef{SendText: "  "}).Validate()
	if err == nil || !strings.Contains(err.Error(), "send_text") {
		t.Fatalf("expected a send_text refusal, got %v", err)
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

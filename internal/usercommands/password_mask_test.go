package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// syncLikeProcessInput stands in for processInput's deferred sync.
func syncLikeProcessInput(u *users.UserRecord) { u.SyncTextMask() }

// A wrong current password: masked while asked, unmasked (TEXTMASK:false)
// once the refusal clears the prompt.
func TestPassword_WrongCurrentPasswordMaskThenUnmask(t *testing.T) {
	sent, restore := users.CaptureTextMaskForTest(true)
	defer restore()
	user := users.NewTestUser(502, "pwplayer2", "Pw2", 9902) // bcrypt password
	room := &rooms.Room{RoomId: 502, Zone: "TestZone"}

	Password("", user, room, 0)
	syncLikeProcessInput(user)
	if !user.InputMasked() || len(*sent) != 1 || (*sent)[0] != "TEXTMASK:true" {
		t.Fatalf("asking: masked=%v sent=%v", user.InputMasked(), *sent)
	}

	q := user.GetPrompt().GetNextQuestion()
	q.Response, q.Done = "definitely-wrong", true
	Password("", user, room, 0) // refuses and clears the prompt
	syncLikeProcessInput(user)
	if user.InputMasked() || len(*sent) != 2 || (*sent)[1] != "TEXTMASK:false" {
		t.Fatalf("after refusal: masked=%v sent=%v", user.InputMasked(), *sent)
	}
}

// A legacy plaintext password skips the current-password question: the first
// question is the new password, and it is masked from the start.
func TestPassword_PlaintextPasswordStartsAtNewPasswordMasked(t *testing.T) {
	sent, restore := users.CaptureTextMaskForTest(true)
	defer restore()
	user := users.NewTestUser(503, "pwplayer3", "Pw3", 9903)
	user.Password = "plain"
	room := &rooms.Room{RoomId: 503, Zone: "TestZone"}

	Password("", user, room, 0)
	syncLikeProcessInput(user)
	q := user.GetPrompt().GetNextQuestion()
	if q == nil || q.Question != `What new password would you like?` || !user.InputMasked() {
		t.Fatalf("first question %+v masked=%v", q, user.InputMasked())
	}
	if len(*sent) != 1 || (*sent)[0] != "TEXTMASK:true" {
		t.Fatalf("sent %v", *sent)
	}
}

// TestPassword_QuestionsAreMasked: every question the password command asks
// is a secret, so the echo, history and web field are masked (#868).
func TestPassword_QuestionsAreMasked(t *testing.T) {
	if err := configs.AddOverlayOverrides(map[string]any{
		"Validation.PasswordSizeMin": 4,
		"Validation.PasswordSizeMax": 64,
	}); err != nil {
		t.Fatalf("AddOverlayOverrides: %v", err)
	}
	user := users.NewTestUser(501, "pwplayer", "Pw", 9900)
	if err := user.SetPassword("oldsecret1"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	room := &rooms.Room{RoomId: 501, Zone: "TestZone"}

	if _, err := Password("", user, room, 0); err != nil {
		t.Fatal(err)
	}
	q := user.GetPrompt().GetNextQuestion()
	if q == nil || !q.Masked() {
		t.Fatalf("current-password question not masked: %+v", q)
	}

	q.Response, q.Done = "oldsecret1", true
	if _, err := Password("", user, room, 0); err != nil {
		t.Fatal(err)
	}
	q = user.GetPrompt().GetNextQuestion()
	if q == nil || q.Question != `What new password would you like?` || !q.Masked() {
		t.Fatalf("new-password question not masked: %+v", q)
	}

	q.Response, q.Done = "newsecret1", true
	if _, err := Password("", user, room, 0); err != nil {
		t.Fatal(err)
	}
	q = user.GetPrompt().GetNextQuestion()
	if q == nil || !q.Masked() {
		t.Fatalf("confirm question not masked: %+v", q)
	}
}

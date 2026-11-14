package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

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

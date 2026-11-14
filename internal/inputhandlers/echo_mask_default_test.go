package inputhandlers

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
)

// The default inputMasked seam finds the user by connection id and returns
// the user's InputMasked (#868).
func TestDefaultInputMaskedResolvesUserByConnection(t *testing.T) {
	_, restore := users.CaptureTextMaskForTest(false)
	defer restore()

	u := users.NewTestUser(801, "maskdefault", "Maskd", 8801)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{u.UserId: u})()

	if inputMasked(8801) {
		t.Fatal("no prompt: not masked")
	}
	p, _ := u.StartPrompt("password", "")
	p.Ask("secret?", []string{}).MaskInput()
	u.SyncTextMask()
	if !inputMasked(8801) {
		t.Fatal("masked question: the default seam must report masked")
	}
	if inputMasked(9999) {
		t.Fatal("unknown connection: not masked")
	}
}

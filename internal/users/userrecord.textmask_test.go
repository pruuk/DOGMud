package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
)

// captureTextMask replaces the TEXTMASK send seams for one test.
func captureTextMask(t *testing.T, websocket bool) *[]string {
	t.Helper()
	var sent []string
	oldWS, oldSend := textMaskIsWebsocket, textMaskSend
	textMaskIsWebsocket = func(connections.ConnectionId) bool { return websocket }
	textMaskSend = func(b []byte, _ ...connections.ConnectionId) { sent = append(sent, string(b)) }
	t.Cleanup(func() { textMaskIsWebsocket, textMaskSend = oldWS, oldSend })
	return &sent
}

func TestInputMasked(t *testing.T) {
	u := NewTestUser(701, "maskuser", "Mask", 7001)
	if u.InputMasked() {
		t.Fatal("no prompt: not masked")
	}
	p, _ := u.StartPrompt("password", "")
	p.Ask("plain?", []string{})
	if u.InputMasked() {
		t.Fatal("plain question: not masked")
	}
	p.Ask("secret?", []string{}).MaskInput()
	if !u.InputMasked() {
		t.Fatal("masked question: masked")
	}
	r := &rec{}
	u.sessionMu.Lock()
	u.activeMenu = &openMenu{m: testMenu(), s: r.session()}
	u.sessionMu.Unlock()
	if u.InputMasked() {
		t.Fatal("a menu open over the prompt: not masked")
	}
}

func TestSyncTextMask_SendsOnlyOnChange(t *testing.T) {
	sent := captureTextMask(t, true)
	u := NewTestUser(702, "maskuser2", "Mask2", 7002)

	u.SyncTextMask()
	if len(*sent) != 0 {
		t.Fatalf("unmasked start must send nothing, sent %v", *sent)
	}
	p, _ := u.StartPrompt("password", "")
	p.Ask("secret?", []string{}).MaskInput()
	u.SyncTextMask()
	u.SyncTextMask()
	if len(*sent) != 1 || (*sent)[0] != "TEXTMASK:true" {
		t.Fatalf("want one TEXTMASK:true, sent %v", *sent)
	}
	u.ClearPrompt() // syncs
	if len(*sent) != 2 || (*sent)[1] != "TEXTMASK:false" {
		t.Fatalf("want TEXTMASK:false after ClearPrompt, sent %v", *sent)
	}
	u.SyncTextMask()
	if len(*sent) != 2 {
		t.Fatalf("no change, no send: %v", *sent)
	}
}

func TestSyncTextMask_NotWebsocketSendsNothing(t *testing.T) {
	sent := captureTextMask(t, false)
	u := NewTestUser(703, "maskuser3", "Mask3", 7003)
	p, _ := u.StartPrompt("password", "")
	p.Ask("secret?", []string{}).MaskInput()
	u.SyncTextMask()
	if len(*sent) != 0 {
		t.Fatalf("telnet connection must get no TEXTMASK, sent %v", *sent)
	}
}

func TestSyncTextMask_NewConnectionResets(t *testing.T) {
	sent := captureTextMask(t, true)
	u := NewTestUser(704, "maskuser4", "Mask4", 7004)
	p, _ := u.StartPrompt("password", "")
	p.Ask("secret?", []string{}).MaskInput()
	u.SyncTextMask()
	if len(*sent) != 1 {
		t.Fatalf("first sync: %v", *sent)
	}
	u.connectionId = 7005 // reconnect: the client starts unmasked
	u.SyncTextMask()
	if len(*sent) != 2 || (*sent)[1] != "TEXTMASK:true" {
		t.Fatalf("a new connection must be told again, sent %v", *sent)
	}
}

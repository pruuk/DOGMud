package menu_test

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/ansitags"
)

var sgrFg = regexp.MustCompile("\x1b\\[38;5;(\\d+)m")

// fgBefore returns the 256-colour foreground in effect where label starts in
// the telnet bytes.
func fgBefore(t *testing.T, out, label string) string {
	t.Helper()
	i := strings.Index(out, label)
	if i < 0 {
		t.Fatalf("label %q not in output %q", label, out)
	}
	codes := sgrFg.FindAllStringSubmatch(out[:i], -1)
	if len(codes) == 0 {
		t.Fatalf("no foreground colour before %q in %q", label, out)
	}
	return codes[len(codes)-1][1]
}

// TestRender_DimLineIsDimmerOnTheTelnetPort renders a menu the way a player
// receives it (the category's colour wrap, then the dogmud aliases turned
// into bytes) for every category menus ride. A heard line must come out in a
// different, darker colour than an unheard one. The playtest found both in
// the screen colour (grey), so Dim showed nothing (#375 phase 1).
func TestRender_DimLineIsDimmerOnTheTelnetPort(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	aliases := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "ansi-aliases.yaml")
	if err := ansitags.LoadAliases(aliases); err != nil {
		t.Fatalf("load aliases: %v", err)
	}
	m := menu.Menu{
		Heading: "Talking with Kael",
		Entries: []menu.Entry{
			{Id: "feud", Marker: menu.MarkerQuest, Label: "The Feud at the Well", Detail: "Do you want help settling the feud?"},
			{Id: "pass", Label: "How do I reach the high pass?"},
			{Id: "carve", Label: "What do you carve?", Dim: true},
		},
		Leave: "Leave",
	}
	for _, cat := range []messaging.Category{messaging.CategoryScreen, messaging.CategoryAdmin} {
		tagged := messaging.RenderForRecipient(messaging.RenderInput{
			Category: cat, Text: menu.Render(m), Channel: messaging.ChannelAudio, LineWidth: 80,
		})
		out := ansitags.Parse(tagged)
		dim := fgBefore(t, out, "What do you carve?")
		for _, label := range []string{"How do I reach the high pass?", "The Feud at the Well", "Do you want help settling", "Leave"} {
			if got := fgBefore(t, out, label); got == dim {
				t.Errorf("%s: %q renders in colour %s, the same as the heard line; Dim shows nothing", cat, label, got)
			}
		}
		if dim != "8" {
			t.Errorf("%s: heard line colour %s, want the grey 8", cat, dim)
		}
	}
}

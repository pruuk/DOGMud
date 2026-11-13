package inputhandlers

import (
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/language"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	textlang "golang.org/x/text/language"
)

// The live world's data files, from this package's folder.
const loginTestDataFiles = "../../_datafiles/world/dogmud"

// loginRenderSetup renders the live world's templates with its English
// labels, as the server does.
func loginRenderSetup(t *testing.T) {
	t.Helper()
	language.InitTranslation(language.BundleCfg{
		DefaultLanguage: textlang.English,
		Language:        textlang.English,
		LanguagePaths:   []string{"../../_datafiles/localize", loginTestDataFiles + "/localize"},
	})
	prev := configs.GetFilePathsConfig().DataFiles.String()
	if err := configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": loginTestDataFiles}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": prev}) })
	templates.SetFSForTest(t, os.DirFS(loginTestDataFiles).(fs.ReadFileFS))
}

// loginTestConns registers a web client that reported 43 columns (an
// upright phone), one that reported none, and a telnet client whose terminal
// reported 40.
func loginTestConns(t *testing.T) (phone, bareWeb, telnet connections.ConnectionId) {
	t.Helper()
	phone, bareWeb, telnet = 97301, 97302, 97303
	connections.RegisterTestWebsocketConnection(phone)
	connections.RegisterTestWebsocketConnection(bareWeb)
	connections.RegisterTestConnection(telnet)
	t.Cleanup(func() {
		for _, c := range []connections.ConnectionId{phone, bareWeb, telnet} {
			connections.UnregisterTestConnection(c)
		}
	})
	for id, w := range map[connections.ConnectionId]uint32{phone: 43, telnet: 40} {
		cs := connections.GetClientSettings(id)
		cs.Display.ScreenWidth = w
		connections.OverwriteClientSettings(id, cs)
	}
	return phone, bareWeb, telnet
}

var loginTags = regexp.MustCompile(`</?ansi[^>]*>`)

// loginWords is the words of s, tags and line breaks not counted.
func loginWords(s string) string {
	return strings.Join(strings.Fields(loginTags.ReplaceAllString(s, "")), " ")
}

// loginPromptCases are every prompt the login sends, with the data
// login.go gives it, the yes or no questions at their longest (a name of
// NameSizeMax, 28).
func loginPromptCases() map[string][2]any {
	yn := func(prompt string) map[string]any {
		return map[string]any{"prompt": prompt, "options": []string{"y", "n"}, "default": "n"}
	}
	return map[string][2]any{
		"username":          {"login/username.prompt", map[string]any{}},
		"password":          {"login/password.prompt", map[string]any{}},
		"username-new":      {"login/username-new.prompt", map[string]any{}},
		"password-new":      {"login/password-new.prompt", map[string]any{}},
		"password-verify":   {"login/password-new-verify.prompt", map[string]any{}},
		"email optional":    {"login/email-new.prompt", map[string]any{"emailIsOptional": true}},
		"email required":    {"login/email-new.prompt", map[string]any{"emailIsOptional": false}},
		"kick":              {"generic/prompt.yn", yn("User is already connected. Kick them?")},
		"screen reader":     {"generic/prompt.yn", yn("Are you using a screen reader?")},
		"confirm long name": {"generic/prompt.yn", yn(language.T("Login.CreateUser", map[any]any{"Username": "Bartholomew_Thistlewood_Iron"}))},
	}
}

// A telnet client gets every login prompt exactly as templates.Process
// rendered it before login text had a width, whatever its terminal reports.
// A web client gets it laid out for its stored width: every line fits, the
// last line ends in the prompt's ": " (the cursor goes after it), and every
// word is kept. A web client that reported no width is laid out for 80.
func TestLoginPromptsAreLaidOutForTheConnection(t *testing.T) {
	loginRenderSetup(t)
	phone, bareWeb, telnet := loginTestConns(t)
	for label, c := range loginPromptCases() {
		name, data := c[0].(string), c[1]
		before, err := templates.Process(name, data)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}

		got, err := renderLoginTemplate(telnet, name, data)
		if err != nil || got != before {
			t.Errorf("%s: telnet changed (err %v):\nbefore %q\nafter  %q", label, err, before, got)
		}

		for id, width := range map[connections.ConnectionId]int{phone: 43, bareWeb: 80} {
			got, err := renderLoginTemplate(id, name, data)
			if err != nil {
				t.Fatalf("%s at %d: %v", label, width, err)
			}
			if w := templates.WidestLine(got); w > width {
				t.Errorf("%s at %d has a line %d wide:\n%s", label, width, w, got)
			}
			lines := strings.Split(got, "\n")
			last := loginTags.ReplaceAllString(lines[len(lines)-1], "")
			if !strings.HasSuffix(last, ": ") || strings.TrimSpace(last) == ":" {
				t.Errorf("%s at %d: the last line %q does not end in its prompt and \": \"", label, width, last)
			}
			if loginWords(got) != loginWords(before) {
				t.Errorf("%s at %d lost or changed words:\n%s\n%s", label, width, loginWords(before), loginWords(got))
			}
		}
	}
}

// The text sent before login that is not a template (a refusal, a
// validation error) is laid out the same way: wrapped for a web client's
// width, unchanged for telnet.
func TestLoginTextIsLaidOutForTheConnection(t *testing.T) {
	phone, bareWeb, telnet := loginTestConns(t)
	reason := "Your connection has been banned. Reason: repeated harassment of other players in the Chrysalis after two warnings from staff."
	for _, text := range []string{reason, "password must be between 4 and 16 characters long", "\r\nToo many mistakes.\r\n\r\n"} {
		if got := loginTextFor(telnet, text); got != text {
			t.Errorf("telnet changed %q to %q", text, got)
		}
		for id, width := range map[connections.ConnectionId]int{phone: 43, bareWeb: 80} {
			got := loginTextFor(id, text)
			for _, line := range strings.Split(got, "\n") {
				if w := util.VisibleWidth(strings.TrimSuffix(line, "\r")); w > width {
					t.Errorf("at %d %q has a line %d wide: %q", width, text, w, line)
				}
			}
			if loginWords(got) != loginWords(text) {
				t.Errorf("at %d the words of %q changed: %q", width, text, got)
			}
		}
	}
	if got := loginTextFor(phone, "\r\nToo many mistakes.\r\n\r\n"); got != "\r\nToo many mistakes.\r\n\r\n" {
		t.Errorf("a short message that fits changed: %q", got)
	}
}

// The goodbye on /quit or a duplicate login keeps its full art wherever the
// art fits (it is 50 columns) and is the words-only form only on a narrower
// screen; a logged-in player's set linewidth counts (BAR 2026-11-13: a width
// band gave every screen under 80 the words-only form).
func TestGoodbyeIsTheWidestFormThatFits(t *testing.T) {
	loginRenderSetup(t)
	phone, _, telnet := loginTestConns(t)
	const sixty, sixtyNarrowed connections.ConnectionId = 97304, 97305
	for _, id := range []connections.ConnectionId{sixty, sixtyNarrowed} {
		connections.RegisterTestWebsocketConnection(id)
		cs := connections.GetClientSettings(id)
		cs.Display.ScreenWidth = 60
		connections.OverwriteClientSettings(id, cs)
	}
	t.Cleanup(func() {
		connections.UnregisterTestConnection(sixty)
		connections.UnregisterTestConnection(sixtyNarrowed)
	})
	player := users.NewTestUser(9731, "leaver", "Leaver", sixtyNarrowed)
	player.LineWidth = 40
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{player.UserId: player}))

	isArt := func(s string) bool { return strings.Contains(s, "⣿") && !strings.Contains(s, "Goodbye.") }
	for _, tt := range []struct {
		name  string
		id    connections.ConnectionId
		width int
		art   bool
	}{
		{"a web client at 60 columns", sixty, 60, true},
		{"a phone at 43 columns", phone, 43, false},
		{"telnet before login", telnet, 80, true},
		{"a web player at 60 with set linewidth 40", sixtyNarrowed, 40, false},
	} {
		got := goodbyeFor(tt.id)
		if isArt(got) != tt.art {
			t.Errorf("%s: art %v, want %v: %q", tt.name, isArt(got), tt.art, got)
		}
		if w := templates.WidestLine(got); w > tt.width {
			t.Errorf("%s: a line %d wide, wider than %d", tt.name, w, tt.width)
		}
	}
}

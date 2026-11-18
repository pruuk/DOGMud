package web

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The page templates are text/template, which encodes nothing, so a value that
// came from the request or from a player must be encoded where the template
// prints it.
const hostilePageValue = `x"><b id=h>'&`

func renderPublicPage(t *testing.T, page string, data map[string]any) string {
	t.Helper()
	root := diagramsRepoRoot(t)
	public := filepath.Join(root, "_datafiles", "html", "public")
	tmpl, err := template.New(filepath.Base(page)).Funcs(funcMap).ParseFiles(
		filepath.Join(public, "_header.html"),
		filepath.Join(public, "_footer.html"),
		page,
	)
	if err != nil {
		t.Fatalf("parse %s: %v", page, err)
	}
	data["REQUEST"] = httptest.NewRequest("GET", "/", nil)
	data["CONFIG"] = configs.GetConfig()
	if _, given := data["STATS"]; !given {
		data["STATS"] = GetStats()
	}
	data["NAV"] = []WebNav{}
	data["ASSET_VERSION"] = "v"
	data["COMPANION_RELAY_ORIGIN_JSON"] = `""`
	data["ITEM_ICON_BASE_JSON"] = `"/static/images/items"`
	var out strings.Builder
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatalf("execute %s: %v", page, err)
	}
	return out.String()
}

func requireEncoded(t *testing.T, out, where string) {
	t.Helper()
	if strings.Contains(out, hostilePageValue) || strings.Contains(out, `<b id=h>`) {
		t.Errorf("%s printed a request or player value unencoded", where)
	}
	if !strings.Contains(out, `x&#34;&gt;&lt;b id=h&gt;&#39;&amp;`) {
		t.Errorf("%s did not print the encoded value at all, so this test proves nothing", where)
	}
}

func TestPageHeaderEncodesTheRequestPath(t *testing.T) {
	root := diagramsRepoRoot(t)
	page := filepath.Join(root, "_datafiles", "html", "public", "404.html")
	out := renderPublicPage(t, page, map[string]any{"PATH": hostilePageValue})
	requireEncoded(t, out, "the page header")
}

func TestHelpDetailsEncodesTheSearchTerm(t *testing.T) {
	root := diagramsRepoRoot(t)
	page := filepath.Join(root, "modules", "webhelp", "files", "datafiles", "html", "public", "help-details.html")
	out := renderPublicPage(t, page, map[string]any{
		"PATH": "/help-details", "topic": hostilePageValue, "error": "", "contents": "text",
	})
	requireEncoded(t, out, "the help topic")

	out = renderPublicPage(t, page, map[string]any{
		"PATH": "/help-details", "topic": "t", "error": hostilePageValue,
	})
	requireEncoded(t, out, "the help error")
}

// Character names reach two public pages that need no login. The name rules in
// config keep markup out of a name today, but the pages must not depend on a
// config value for their encoding.
func TestOnlinePageEncodesPlayerValues(t *testing.T) {
	root := diagramsRepoRoot(t)
	page := filepath.Join(root, "_datafiles", "html", "public", "online.html")

	fields := map[string]func(*users.OnlineInfo){
		"character name": func(u *users.OnlineInfo) { u.CharacterName = hostilePageValue },
		"title":          func(u *users.OnlineInfo) { u.Title = hostilePageValue },
		"role":           func(u *users.OnlineInfo) { u.Role = hostilePageValue },
	}
	for name, set := range fields {
		u := users.OnlineInfo{CharacterName: "Someone", Title: "Novice", Role: "user", OnlineTimeStr: "5m"}
		set(&u)
		out := renderPublicPage(t, page, map[string]any{
			"PATH":  "/online",
			"STATS": Stats{OnlineUsers: []users.OnlineInfo{u}},
		})
		requireEncoded(t, out, "the online page's "+name)
	}
}

// The leaderboards module page prints the same player-chosen name.
func TestLeaderboardsPageEncodesPlayerValues(t *testing.T) {
	root := diagramsRepoRoot(t)
	page := filepath.Join(root, "modules", "leaderboards", "files", "datafiles", "html", "public", "leaderboards.html")

	for _, field := range []string{"CharacterName", "CharacterTitle"} {
		entry := map[string]any{"CharacterName": "Someone", "CharacterTitle": "Novice", "ScoreValue": 5}
		entry[field] = hostilePageValue
		out := renderPublicPage(t, page, map[string]any{
			"PATH": "/leaderboards",
			"leaderboards": []map[string]any{
				{"Name": "Power", "Top": []map[string]any{entry}},
			},
		})
		requireEncoded(t, out, "the leaderboards page's "+field)
	}
}

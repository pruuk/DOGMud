package web

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// The page templates are text/template, which encodes nothing, so a value that
// came from the request must be encoded where the template prints it.
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
	data["STATS"] = GetStats()
	data["NAV"] = []WebNav{}
	data["ASSET_VERSION"] = "v"
	data["COMPANION_RELAY_ORIGIN_JSON"] = `""`
	var out strings.Builder
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatalf("execute %s: %v", page, err)
	}
	return out.String()
}

func requireEncoded(t *testing.T, out, where string) {
	t.Helper()
	if strings.Contains(out, hostilePageValue) || strings.Contains(out, `<b id=h>`) {
		t.Errorf("%s printed a request value unencoded", where)
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

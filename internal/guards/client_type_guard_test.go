package guards

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The web client and the site use one font, two text sizes, no italics and one
// bold (spec 2026-11-09, rulings 4 to 8a). Sizes come from tokens in gomud.css
// and nothing else. The builder, the admin pages and the architecture diagrams
// keep their own styling and are not in this list.

const typePublic = "_datafiles/html/public/"
const typeHelp = "modules/webhelp/files/datafiles/html/public/"

var typeScopeFiles = []string{
	typePublic + "static/css/gomud.css",
	typePublic + "static/css/dashboard.css",
	typePublic + "webclient-pure.html",
	typePublic + "webclient.html",
	typePublic + "_header.html",
	typePublic + "_footer.html",
	typePublic + "index.html",
	typePublic + "online.html",
	typePublic + "viewconfig.html",
	typePublic + "404.html",
	typePublic + "architecture.html",
	typeHelp + "help.html",
	typeHelp + "help-details.html",
	typePublic + "static/js/dashboard.js",
	"modules/achievements/files/datafiles/html/public/achievements.html",
	"modules/leaderboards/files/datafiles/html/public/leaderboards.html",
}

// typeTokenHome is the one file allowed to define a type token.
const typeTokenHome = typePublic + "static/css/gomud.css"

// gmcp.js draws the map art, whose lettering scales with the map, so its sizes
// are exempt (spec section 3). Italics and old font names are not.
const typeMapArtFile = typePublic + "static/js/gmcp.js"

// The map buttons are injected by gmcp.js and are not art.
const typeMapButtonRule = "font-weight:400;font-size:var(--text);"

// Scripts that draw art; scanned for italics and old fonts only.
var typeScriptFiles = []string{typeMapArtFile}

// typeExemptLines maps "file|trimmed line" to why that exact line may break a
// rule. Changing the line drops the exemption.
var typeExemptLines = map[string]string{
	typePublic + "static/css/dashboard.css|font-size: 0;":                                            "hides a character; draws nothing",
	typePublic + "webclient-pure.html|'font-size:1px;line-height:1;font-family:monospace;';":         "probe for the browser's minimum font size, never shown",
	typePublic + "webclient-pure.html|console.log(\"%cterminal commands:\", \"font-weight:bold;\");": "developer console only",
	typePublic + "webclient-pure.html|console.log(\"%cconsole commands:\", \"font-weight:bold;\");":  "developer console only",
	typePublic + "webclient-pure.html|el.style.fontFamily = o.fontFamily;":                           "xterm's own cell measure; plan 5 removes it",
	typePublic + "webclient-pure.html|el.style.fontSize = o.fontSize + 'px';":                        "xterm's own cell measure; plan 5 removes it",
	typePublic + "static/css/gomud.css|font-family: \"JetBrains Mono\";":                             "the @font-face names the family",
	typePublic + "static/css/gomud.css|font-style: normal; font-weight: 600; font-display: swap;":    "the @font-face of the one bold",
}

var (
	typeComment   = regexp.MustCompile(`(?s)/\*.*?\*/`)
	typeSizeDecl  = regexp.MustCompile(`font-size\s*:\s*([^;}]+)`)
	typeSizeOK    = regexp.MustCompile(`^var\(--(text|title|cmd|nav-text|foot-text)\)$`)
	typeShort     = regexp.MustCompile(`(^|[\s;{"'])font\s*:\s*[^;]`)
	typeItalic    = regexp.MustCompile(`font-style['"]?\s*[:=]\s*['"]?(italic|oblique)|font\s*:\s*['"]?italic`)
	typeStyleProp = regexp.MustCompile(`\.style\.font(Size|Weight|Style|Family)\b`)
	typeTokenDef  = regexp.MustCompile(`(^|[\s;{])--(text|title|cmd|nav-text|foot-text|bold)\s*:`)
	typeFamily    = regexp.MustCompile(`font-family\s*:\s*([^;}]+)`)
	typeFamilyOK  = regexp.MustCompile(`^var\(--font-(body|heading|mono)\)$`)
	typeWeight    = regexp.MustCompile(`font-weight\s*:\s*([^;}]+)`)
	typeWeightOK  = regexp.MustCompile(`^(400|normal|var\(--bold\))$`)
	typeOldFamily = regexp.MustCompile(`Source (Sans|Serif|Code)`)
)

// typeLineFaults names every rule the line breaks.
func typeLineFaults(line string) []string {
	var faults []string
	check := func(re, ok *regexp.Regexp, what string) {
		for _, m := range re.FindAllStringSubmatch(line, -1) {
			value := strings.TrimRight(strings.TrimSpace(m[1]), `"'`)
			if !ok.MatchString(value) {
				faults = append(faults, what+" "+value)
			}
		}
	}
	check(typeSizeDecl, typeSizeOK, "font-size")
	check(typeFamily, typeFamilyOK, "font-family")
	check(typeWeight, typeWeightOK, "font-weight")
	if typeShort.MatchString(line) {
		faults = append(faults, "font shorthand")
	}
	if typeItalic.MatchString(line) {
		faults = append(faults, "italic")
	}
	if typeOldFamily.MatchString(line) {
		faults = append(faults, "retired font name")
	}
	if typeStyleProp.MatchString(line) {
		faults = append(faults, "font set from script")
	}
	return faults
}

// typeReadWithoutComments blanks /* */ comments and keeps the line numbers.
func typeReadWithoutComments(t *testing.T, rel string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	text := typeComment.ReplaceAllStringFunc(string(data), func(c string) string {
		return strings.Repeat("\n", strings.Count(c, "\n"))
	})
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}

func TestClientTypeRulesCanFail(t *testing.T) {
	for line, want := range map[string]string{
		`  font-size: 13px;`:                     "font-size 13px",
		`el.style.cssText = "font-size:0.8em;";`: "font-size 0.8em",
		`  font: italic 11px var(--font-body);`:  "font shorthand",
		`  font-style: italic;`:                  "italic",
		`"font-style": "italic", fill: L.title`:  "italic",
		`  font-family: Arial, sans-serif;`:      "font-family Arial, sans-serif",
		`  font-weight: 700;`:                    "font-weight 700",
		`font-weight: bold;`:                     "font-weight bold",
		`--font-body: "Source Sans 3";`:          "retired font name",
		`<text font-style="italic">`:             "italic",
		`el.style.fontSize = '9px';`:             "font set from script",
	} {
		if got := strings.Join(typeLineFaults(line), ","); !strings.Contains(got, want) {
			t.Errorf("probe %q: got %q, want it to contain %q", line, got, want)
		}
	}
	for _, line := range []string{
		`  font-size: var(--text);`, `font-size: var(--title);`, `font-family: var(--font-mono);`,
		`font-weight: var(--bold);`, `font-weight: 400;`, `.qonmap { font-style:normal; }`,
		`el.style.cssText = "font-size:var(--text)";`,
	} {
		if got := typeLineFaults(line); len(got) != 0 {
			t.Errorf("probe %q should pass, got %v", line, got)
		}
	}
	if !typeTokenDef.MatchString(`  --text: 12px;`) || typeTokenDef.MatchString(`--text-primary: #fff;`) || typeTokenDef.MatchString(`--title-gold: #fff;`) {
		t.Error("probe: the token definition pattern is wrong")
	}
}

func TestClientAndSiteTypeUsesTheTokens(t *testing.T) {
	used := map[string]bool{}
	scanned := 0
	for _, rel := range typeScopeFiles {
		for no, line := range typeReadWithoutComments(t, rel) {
			scanned++
			if rel != typeTokenHome && typeTokenDef.MatchString(line) {
				t.Errorf("%s:%d: a type token is defined outside gomud.css: %s", rel, no+1, strings.TrimSpace(line))
			}
			key := rel + "|" + strings.TrimSpace(line)
			faults := typeLineFaults(line)
			if len(faults) == 0 {
				continue
			}
			if _, ok := typeExemptLines[key]; ok {
				used[key] = true
				continue
			}
			t.Errorf("%s:%d: %s: %s", rel, no+1, strings.Join(faults, ", "), strings.TrimSpace(line))
		}
	}
	if scanned < 7000 {
		t.Fatalf("scanned only %d lines; the walk is not reaching the files", scanned)
	}
	for key := range typeExemptLines {
		if !used[key] {
			t.Errorf("exemption matches no violation any more, remove it: %s", key)
		}
	}
}

// The map art may keep its own sizes, but not italics or the old fonts.
func TestMapArtHasNoItalicsOrOldFonts(t *testing.T) {
	for _, rel := range typeScriptFiles {
		for no, line := range typeReadWithoutComments(t, rel) {
			if typeItalic.MatchString(line) || typeOldFamily.MatchString(line) {
				t.Errorf("%s:%d: %s", rel, no+1, strings.TrimSpace(line))
			}
		}
	}
	lines := strings.Join(typeReadWithoutComments(t, typeMapArtFile), "\n")
	if !strings.Contains(lines, typeMapButtonRule) {
		t.Errorf("%s no longer holds the map buttons' type rule %q", typeMapArtFile, typeMapButtonRule)
	}
}

// The sizes are locked rulings: 16/20, 14/18 on upright phones, command box 16,
// bars 20/16 and 16/14 on phones.
func TestTypeTokensHoldTheRuledSizes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(typePublic+"static/css/gomud.css")))
	if err != nil {
		t.Fatal(err)
	}
	css := strings.ReplaceAll(string(data), "\r\n", "\n")
	for _, want := range []string{
		"--text: 16px;",
		"--title: 20px;",
		"--cmd: 16px;",
		"--nav-text: 20px;",
		"--foot-text: 16px;",
		"--bold: 600;",
		":root.upright-phone { --text: 14px; --title: 18px; }",
		"@media (max-width: 767px), (max-height: 500px) { :root { --nav-text: 16px; --foot-text: 14px; } }",
		`--font-body: "JetBrains Mono", ui-monospace, Menlo, Consolas, monospace;`,
		`--font-heading: "JetBrains Mono", ui-monospace, Menlo, Consolas, monospace;`,
		`--font-mono: "JetBrains Mono", ui-monospace, Menlo, Consolas, monospace;`,
		":root { font-variant-ligatures: none; }",
		"input, select, textarea { font-size: var(--cmd); }",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("gomud.css lost %q", want)
		}
	}
}

// The official files, unmodified. The Google Fonts cut has no box-drawing
// characters and misaligned the boxes in a mock, so the hashes also hold the
// complete font in place.
func TestFontFolderIsTheCompleteFont(t *testing.T) {
	dir := filepath.Join(repoRoot(t), filepath.FromSlash(typePublic+"static/fonts"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	want := []string{"JetBrainsMono-LICENSE.txt", "JetBrainsMono-Regular.woff2", "JetBrainsMono-SemiBold.woff2"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("static/fonts holds %v, want %v", names, want)
	}
	for name, hash := range map[string]string{
		"JetBrainsMono-Regular.woff2":  "a9cb1cd82332b23a47e3a1239d25d13c86d16c4220695e34b243effa999f45f2",
		"JetBrainsMono-SemiBold.woff2": "918edad542a1da608fd2ba8daebaff9ac802309103fe760eed465b8b4e47faf1",
	} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != hash {
			t.Errorf("%s has sha256 %s, want the v2.304 release's %s", name, got, hash)
		}
	}
}

var typeRoomTitleAlias = regexp.MustCompile(`(?m)^\s*room-title:\s*(\d+)\s*$`)

// The web client bolds room titles by their colour (roomtitlebold.js), so that
// colour and the alias must stay the same number.
func TestRoomTitleBoldMatchesTheAlias(t *testing.T) {
	root := repoRoot(t)
	aliases, err := os.ReadFile(filepath.Join(root, filepath.FromSlash("_datafiles/world/dogmud/ansi-aliases.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	m := typeRoomTitleAlias.FindSubmatch(aliases)
	if m == nil {
		t.Fatal("no room-title alias found in ansi-aliases.yaml")
	}
	script, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(typePublic+"static/js/roomtitlebold.js")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "var ROOM_TITLE_COLOUR = "+string(m[1])+";") {
		t.Errorf("roomtitlebold.js does not hold room-title's colour %s", m[1])
	}
}

// The site menu is in the top bar only (owner 2026-11-10). A second copy in the
// bottom bar was clutter, and the game page never had one.
func TestSiteMenuIsInTheTopBarOnly(t *testing.T) {
	read := func(rel string) string {
		data, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(typePublic+rel)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if !strings.Contains(read("_header.html"), ".NAV") {
		t.Fatal("probe: _header.html no longer ranges over .NAV, so this check proves nothing")
	}
	if strings.Contains(read("_footer.html"), ".NAV") {
		t.Error("_footer.html lists the site menu; it belongs in the top bar only")
	}
}

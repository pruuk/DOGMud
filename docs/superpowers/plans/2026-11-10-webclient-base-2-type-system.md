# Web Client Base, Plan 2: Type System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One font (JetBrains Mono, ligatures off), two text sizes, no italics and one bold across the web client and the site pages, held by a guard, with the game window's font swapped but its fit left alone.

**Architecture:** Size, weight and family tokens live in `gomud.css`, and every declaration in the client and the site reads them. A one-time sweep script rewrites the 109 existing size declarations from an asserted line list, so nothing is judged by hand. `viewportfit.js` sets an `upright-phone` class from the screen, so the keyboard cannot change the size. A new Go guard fails any size, family, weight or italic outside the tokens.

**Tech Stack:** CSS custom properties, self-hosted WOFF2, xterm 4.19, plain-node JS tests (`tools/jstest`), Go guards (`internal/guards`).

**Spec:** `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md`, Design section 3, rulings 4 to 8a, the Plans table (plan 2), Testing and guards. Part of #783.

**Needs:** nothing. Plan 1 is merged, but this plan does not use it.

---

## Facts verified against source

Read 2026-11-10 from master at `2644b3731`. Everything below was applied in a detached worktree of that commit and run on 2026-11-10: the guard went red on master and green after, every `tools/jstest` file passed, `go test . ./internal/... ./modules/...` had no failing package, and the server booted with `Server Ready` 1 and 0 panics. Pages were screenshotted in headless Chrome at 1280x800, 1366x650, 1024x768, 900x700, and under real phone emulation (`screen` and touch included) at 412x915, 360x740, 915x412 and 844x390, where the class and tokens read back as 14/18 upright and 16/20 sideways. Nobody was logged in for those screenshots, so a populated Health, Quests or Inventory panel has not been seen yet; Task 7 covers that. `WP` is `_datafiles/html/public/webclient-pure.html`, `PUB` is `_datafiles/html/public`.

| # | Fact | Where |
|---|---|---|
| 1 | `--font-body`, `--font-heading` are Source Sans 3, `--font-mono` is Source Code Pro | `PUB/static/css/gomud.css:163-165` |
| 2 | `--bar-top` is already a **colour** (`#281602`, and `#eadbb8` in Light), used in three places. The spec's size token of that name would collide | `gomud.css:8`, `:224`, `:529`; `dashboard.css:56`, `:801` |
| 3 | `--text`, `--title`, `--cmd`, `--nav-text`, `--foot-text`, `--bold` are unused in the files in scope. `--text` exists only inside the diagrams, which do not load `gomud.css` | grep, which did find `--text-primary` and the diagrams' `--text`, so it could match |
| 4 | Lines with a `font-size:` declaration: `gomud.css` 16, `dashboard.css` 56, `WP` 28 (one is an SVG attribute at `:297`, one is the 1px probe at `:506`), `architecture.html` 3, `help.html` 5, `help-details.html` 7 | grep |
| 5 | `help.html` and `help-details.html` use `{{template "header" .}}`, so they carry the site bars, and each sets its own sizes and one bold | `modules/webhelp/files/datafiles/html/public/` |
| 6 | The builder pages (`build.html`, `build-help.html`), the admin pages and the diagrams do not load `gomud.css`. Only `_header.html` and `WP` link it | grep |
| 7 | `gmcp.js` is loaded only by `WP`. It draws the map art with `L.font` (`:1469`), six `font-style: italic` attributes (`:708`, `:735`, `:957`, `:1016`, `:1064`, `:1147`), and injects the map button style (`:1354-1355`) | `PUB/static/js/gmcp.js`; grep of `*.html` |
| 8 | `architecture.html` uses `--font-serif`, which no file defines | `architecture.html:6`, `:66`; grep |
| 9 | Five `font:` shorthands in `dashboard.css` carry a pixel size | `dashboard.css:353`, `:367`, `:372`, `:376`, `:1638` |
| 10 | Form controls do not inherit: buttons such as `#settings-close` (`WP:80`) set no family | `WP:80-91` |
| 11 | WinBox's stylesheet gives `.wb-title` Arial 14px; `.winbox.dash-winbox .wb-title` already overrides the family | `winbox.bundle.min.js`; `dashboard.css:805` |
| 12 | xterm is built with `fontSize: 20` and no `fontWeightBold`. The font-load promise names Source Code Pro at weights 400 and 700 | `WP:435`, `:556-561` |
| 13 | The mock used weight 600 for panel titles, quest names and your own name, and 700 for one mock room title. It loaded 400, 600 and 700 | `_archive/notes/reports/full-client-mock/static/css/mock-overrides.css:87-95`, `:151`; `index.html:2` |
| 14 | **`ansitags` has no bold.** `bold=` is matched by the property regexp and then ignored by `extractProperties`, and `ansiProperties` has no bold field | `ansitags@v1.1.0/ansiproperties.go:36`, `:80-86`, `:165-200` |
| 15 | `room-title` is colour 170. A top-level title renders as `\x1b[38;5;170m\x1b[49mTitle\x1b[0m`, and the close is a full reset. A tag nested in another colour closes by restoring that colour, so bold would last until the outer close. No current caller leaks past a table cell | `_datafiles/world/dogmud/ansi-aliases.yaml:120`; printed from `ansitags.Parse`; `tables/generic.template` |
| 16 | xterm's bold brightening applies only to colours under 8, so bold 170 keeps its colour | `xterm.4.19.0.js` (`drawBoldTextInBrightColors&&e.isBold()&&h<8`) |
| 17 | Server text reaches the terminal at one line, through `MotdRestyle` | `WP:3043` |
| 18 | `isPhone` is the shorter screen side under 600. The mock's upright phone is `isPhone && !landscape`, landscape read from `screen.orientation.type`, else `screen.width >= screen.height` | `PUB/static/js/viewportfit.js:17`; mock `index.html:641-656` |
| 19 | The site bars' phone size in the mock is under 768px wide or 500px tall | `mock-overrides.css:67-68` |
| 20 | `viewportfit.js` is loaded by `WP:215` and in the body of `webclient.html:3`, and not by other site pages. `theme.js` shows the pattern for a head script | files named |
| 21 | Official JetBrains Mono v2.304 release zip: 5,622,857 bytes, sha256 `6f6376c6ed2960ea8a963cd7387ec9d76e3f629125bc33d1fdcd7eb7012f7bbf`. `Regular.woff2` 92,164 bytes, sha256 `a9cb1cd8...45f2`; `SemiBold.woff2` 94,472 bytes, sha256 `918edad5...aaf1` | downloaded and hashed |
| 22 | Both WOFF2 files hold all 128 box-drawing characters (U+2500 to U+257F) and all 32 block elements (U+2580 to U+259F). Advance 600, the same as Source Code Pro, so the column fit does not change | `fontTools` on the files |
| 23 | Five characters used in the repo are in Source Code Pro and not in JetBrains Mono: U+263A, U+2660, U+2663, U+2665, U+2666 (the smile and the card suits, as biome map symbols and name markers). They fall back to a system font, as 251 other symbols already do. The grid does not move | `fontTools` over 8,990 files |
| 24 | Line height: JetBrains Mono 1.320 of the font size, Source Code Pro 1.257. With xterm's `lineHeight: 1.2` a row is 5% taller, so a Game window shows about 5% fewer rows | `hhea` tables; `WP:438` |
| 25 | The nav has seven links with the plugins: Home, Who's Online, Web Client, Architecture, Achievements, Leaderboards, Help | `internal/web/web.go:183-188`, `:227-231`; screenshot |
| 26 | At 20px the links fit one row at 1280 wide and take two rows at 1024. The bar is 80px tall at 1280 and 84px at 1024, because the 56px logo already sets its height | screenshots |
| 27 | Repo files check out CRLF on this machine (`core.autocrlf=true`) and are LF in git. The sweep reads either and writes what it found | `tr -cd '\r'` on working tree and `git show` |
| 28 | Guards reach the repo through `repoRoot(t)` | `internal/guards/testmain_test.go:38` |
| 29 | CI runs every `tools/jstest/*.test.js` | `.github/workflows/validate.yml:90` |

## Where this plan departs from the spec, and why

1. **Bar token names.** The spec says `--bar-top` and `--bar-foot`. Fact 2: `--bar-top` is a colour. They are `--nav-text` and `--foot-text`.
2. **Room titles are bolded in the web client, by colour.** The spec assumes the server can send bold; fact 14 says it cannot. A new small script adds bold to colour 170 on the way to the terminal, and a guard keeps that number equal to the alias. Telnet and Mudlet see nothing new. Colour 170 is also reached through the alias by `scan.go:173`, `keyring.go:30-50` and `roominfo.template:7` (room names and a key label), which turn bold too, and directly by two lines of block art in `mutation_reveal.template`, where bold does not change a solid block.
3. **Equipped slot labels go off here, not in plan 7.** They are 8px today, which the two sizes forbid, and a 46px column fits about four letters at the text size. The mock's fix was to hide them, and the spec accepted that. Plan 7 keeps the rest of Equipped.
4. **The Help pages are included.** Rule 8a names five pages; the two Help pages carry the same site bars and would otherwise be the only ones left in another style (fact 5).
5. **One bold is weight 600,** the mock's panel weight (fact 13). Only Regular and SemiBold ship.
6. **Icon glyphs use the text size too,** as the mock did (`*:not(svg)` at the text size). The spec allowed them an exception; nothing needs it, so the guard exempts only a `font-size: 0` hide and an invisible 1px probe.
7. **Small additions the larger type needs:** `#panel-trig` keeps at least 200px in the right column, because at 1280x800 its list was squeezed to nothing. The Config table's cells may wrap, because long key names pushed the Value column off a phone. The phone's tab name strip is 36px instead of 30 (`mock-overrides.css:326`), and long quest names may wrap (`:242`).

Accepted limits, not changed here: on site pages on a phone held sideways under about 880px wide, the seven links take two rows (seen at 844x390, one link on the second row, a 75px bar). The game page hides that bar on phones. The Game window keeps `lineHeight: 1.2` (fact 24) until plan 5. Lettering in the map art keeps its bold (`gmcp.js:973`, `:1023`, `:1184`, `:1302`). The companion relay pop-ups (`modules/aicompanion/relayweb/`) are separate tool windows and keep their own type, like the builder.

## Files

| File | Change |
|---|---|
| Create: `internal/guards/client_type_guard_test.go` | The guard |
| Create: `PUB/static/js/roomtitlebold.js`, `tools/jstest/roomtitlebold.test.js` | Room title bold |
| Create: `PUB/static/fonts/JetBrainsMono-Regular.woff2`, `JetBrainsMono-SemiBold.woff2`, `JetBrainsMono-LICENSE.txt` | The font |
| Delete: the 9 `Source*` files in `PUB/static/fonts/` | Retired |
| Modify: `PUB/static/css/gomud.css`, `dashboard.css`, `WP`, `architecture.html`, `static/js/gmcp.js`, the two Help pages | The sweep |
| Modify: `PUB/static/js/viewportfit.js`, `tools/jstest/viewportfit.test.js`, `PUB/_header.html`, `PUB/webclient.html` | Upright phone class |
| Modify: `docs/guides/BRAND_STANDARDS.md`, `docs/PATCH_NOTES.md`, `docs/README.md`, `internal/guards/context.md` | Docs |

Every repo edit uses the Edit or Write tool, or is written to the session scratchpad and copied in with `cp`. `$S` below means the full path of the session scratchpad; shell variables do not persist between Bash calls, so set it at the start of each one. Never use `sed -i`, a `>` redirect or Python `open(path,'w')` on a repo file. These rules are enforced by a hook.

---

### Task 0: Branch

- [ ] **Step 1: Create the worktree**

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git rev-parse origin/master
git worktree add C:/tmp/dogmud-type-system -b feature/webclient-base-2-type-system origin/master
```

Expected: `git rev-parse` prints `2644b3731...` or a later commit. If later, run:

```bash
git diff --stat 2644b3731 origin/master -- _datafiles/html/public/static/css _datafiles/html/public/webclient-pure.html _datafiles/html/public/architecture.html _datafiles/html/public/static/js/gmcp.js modules/webhelp
```

If that prints anything, stop and re-verify the sweep script's rows against the new master before going on. Task 5 sweeps the commit this branch starts from, so a row left stale there would overwrite the newer work. If it prints nothing, the rows still hold.

All later commands run in `C:/tmp/dogmud-type-system`.

---

### Task 1: The guard, red

**Files:** Create `internal/guards/client_type_guard_test.go`

- [ ] **Step 1: Write the guard**

````go
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

// Scripts that draw text; scanned for italics and old fonts only.
var typeScriptFiles = []string{typeMapArtFile, typePublic + "static/js/dashboard.js"}

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
	typeItalic    = regexp.MustCompile(`font-style['"]?\s*[:=]\s*['"]?(italic|oblique)`)
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
			if !ok.MatchString(strings.TrimSpace(m[1])) {
				faults = append(faults, what+" "+strings.TrimSpace(m[1]))
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
		"font-variant-ligatures: none;",
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
````

- [ ] **Step 2: Run it**

```bash
go test ./internal/guards/ -run 'TestClientTypeRulesCanFail|TestClientAndSiteType|TestMapArt|TestTypeTokens|TestFontFolder|TestRoomTitleBold' -count=1 2>&1 | cut -c1-160 | tail -30
```

Expected: `TestClientTypeRulesCanFail` passes (it is not listed as failing) and the other five fail. `TestClientAndSiteTypeUsesTheTokens` names more than 150 lines. Do not commit yet: the guard lands with the sweep in Task 5.

---

### Task 2: The font files

Download and check only. The files land in Task 5 Step 3, with the CSS that names them: a `git rm` here would be swept into the next commit.

- [ ] **Step 1: Download into a new, empty folder and check the hash**

```bash
S="<the session scratchpad, full path>"
mkdir -p "$S/jbm-dl" && cd "$S/jbm-dl"
curl -sSL -o jbm.zip https://github.com/JetBrains/JetBrainsMono/releases/download/v2.304/JetBrainsMono-2.304.zip
sha256sum jbm.zip
unzip -q -o jbm.zip -d x
sha256sum x/fonts/webfonts/JetBrainsMono-Regular.woff2 x/fonts/webfonts/JetBrainsMono-SemiBold.woff2
```

Expected: the zip is `6f6376c6ed2960ea8a963cd7387ec9d76e3f629125bc33d1fdcd7eb7012f7bbf`, Regular is `a9cb1cd82332b23a47e3a1239d25d13c86d16c4220695e34b243effa999f45f2`, SemiBold is `918edad542a1da608fd2ba8daebaff9ac802309103fe760eed465b8b4e47faf1`. Stop on any difference.

---

### Task 3: The upright phone class

The class decides 14/18 against 16/20. It comes from the screen because the frame's own shape changes when the keyboard opens.

**Files:** Modify `tools/jstest/viewportfit.test.js`, `PUB/static/js/viewportfit.js`, `PUB/_header.html`, `PUB/webclient.html`

- [ ] **Step 1: Write the failing tests**

Edit `tools/jstest/viewportfit.test.js`. Replace

```js
check('a phone opens full screen', VF.wantFullScreen(null, true), true);
```

with

```js
check('upright phone', VF.isUprightPhone(390, 844, 'portrait-primary'), true);
check('sideways phone', VF.isUprightPhone(844, 390, 'landscape-primary'), false);
check('iOS sizes do not swap: the orientation wins', VF.isUprightPhone(390, 844, 'landscape-secondary'), false);
check('no orientation API, upright', VF.isUprightPhone(390, 844, undefined), true);
check('no orientation API, sideways', VF.isUprightPhone(844, 390, undefined), false);
check('an upright tablet is not an upright phone', VF.isUprightPhone(768, 1024, 'portrait-primary'), false);
(function () {
    var classes = {}, handlers = {};
    var win = {
        screen: { width: 390, height: 844, orientation: { type: 'portrait-primary' } },
        document: { documentElement: { classList: { toggle: function (n, on) { classes[n] = on; } } } },
        addEventListener: function (n, f) { handlers[n] = f; }
    };
    VF.watchUprightPhone(win);
    check('the class is set at once', classes['upright-phone'], true);
    win.screen.orientation.type = 'landscape-primary';
    handlers.orientationchange();
    check('turning the phone clears it', classes['upright-phone'], false);
    win.screen.orientation.type = 'portrait-primary';
    handlers.resize();
    check('a resize re-reads it', classes['upright-phone'], true);
}());

check('a phone opens full screen', VF.wantFullScreen(null, true), true);
```

And replace

```js
check('the outer page loads the module',
    outer.indexOf('/static/js/viewportfit.js?v={{ .ASSET_VERSION }}"') !== -1, true);
```

with

```js
check('every site page loads the module, before it paints',
    header.indexOf('/static/js/viewportfit.js?v={{ .ASSET_VERSION }}"') !== -1 &&
    header.indexOf('/static/js/viewportfit.js') < header.indexOf('/static/css/gomud.css'), true);
```

- [ ] **Step 2: Run it**

Run: `node tools/jstest/viewportfit.test.js 2>&1 | tail -5`
Expected: an exception, `VF.isUprightPhone is not a function`.

- [ ] **Step 3: Implement**

Edit `PUB/static/js/viewportfit.js`. Replace

```js
        root.ViewportFit = api;    // browser
```

with

```js
        root.ViewportFit = api;    // browser
        api.watchUprightPhone(root);
```

Replace

```js
    // pref is the remembered choice: '1', '0', or null for none yet.
```

with

```js
    // An upright phone reads 14/18 instead of 16/20 (gomud.css). Judged from the
    // screen, like isPhone. iOS does not swap screen.width and height when the
    // phone turns, so the orientation API comes first.
    function isUprightPhone(screenW, screenH, orientationType) {
        var landscape = orientationType ? orientationType.indexOf('landscape') === 0 : screenW >= screenH;
        return isPhone(screenW, screenH) && !landscape;
    }

    function markUprightPhone(win) {
        var s = win.screen;
        win.document.documentElement.classList.toggle('upright-phone',
            isUprightPhone(s.width, s.height, s.orientation && s.orientation.type));
    }

    function watchUprightPhone(win) {
        var mark = function () { markUprightPhone(win); };
        mark();
        win.addEventListener('resize', mark);
        win.addEventListener('orientationchange', mark);
    }

    // pref is the remembered choice: '1', '0', or null for none yet.
```

Replace

```js
        isPhone: isPhone,
```

with

```js
        isPhone: isPhone,
        isUprightPhone: isUprightPhone,
        watchUprightPhone: watchUprightPhone,
```

Edit `PUB/_header.html`. After the line loading `theme.js`, add:

```html
  <script src="{{ .CONFIG.FilePaths.WebCDNLocation }}/static/js/viewportfit.js?v={{ .ASSET_VERSION }}"></script>
```

Edit `PUB/webclient.html`. Delete the line that loads `viewportfit.js` (line 3): `_header.html`, which this page includes first, now loads it.

- [ ] **Step 4: Run it**

Run: `node tools/jstest/viewportfit.test.js 2>&1 | grep -c FAIL`
Expected: `0` (`grep -c` exits 1 on zero, so run it alone).

- [ ] **Step 5: Commit**

```bash
git add tools/jstest/viewportfit.test.js _datafiles/html/public/static/js/viewportfit.js _datafiles/html/public/_header.html _datafiles/html/public/webclient.html
git commit -F - <<'EOF'
feat(webclient): mark upright phones from the screen on every site page

The text size on an upright phone depends on this class. It is read from the
screen because the game frame changes shape when the keyboard opens.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
git show --stat HEAD | tail -6
```

Expected: four files, nothing else.

---

### Task 4: Room titles in bold

**Files:** Create `PUB/static/js/roomtitlebold.js`, `tools/jstest/roomtitlebold.test.js`. Modify `WP`.

- [ ] **Step 1: Write the failing test**

Write `tools/jstest/roomtitlebold.test.js`:

````js
// roomtitlebold.test.js: text in the room title's colour turns bold in the web
// client's game window, and nothing else does.
//
// Run:  node tools/jstest/roomtitlebold.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var PUBLIC = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var Bold = require(path.join(PUBLIC, 'static', 'js', 'roomtitlebold.js'));

var failures = 0;
function check(name, got, want) {
    if (got === want) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want));
}

// Captured from ansitags.Parse of room-title.template's tags.
var title = '\x1b[38;5;8m\x1b[49m.:\x1b[0m \x1b[38;5;170m\x1b[49mThe Square\x1b[0m \x1b[38;5;167m\x1b[49m[Town]\x1b[0m';
var bold = '\x1b[38;5;8m\x1b[49m.:\x1b[0m \x1b[1;38;5;170m\x1b[49mThe Square\x1b[0m \x1b[38;5;167m\x1b[49m[Town]\x1b[0m';
check('the room title turns bold', Bold.apply(title), bold);
check('the close still resets everything', Bold.apply(title).indexOf('The Square\x1b[0m') !== -1, true);
check('two titles in one message', Bold.apply('\x1b[38;5;170mA\x1b[0m\x1b[38;5;170mB\x1b[0m'), '\x1b[1;38;5;170mA\x1b[0m\x1b[1;38;5;170mB\x1b[0m');
check('other colours are left alone', Bold.apply('\x1b[38;5;17m\x1b[49mx\x1b[0m'), '\x1b[38;5;17m\x1b[49mx\x1b[0m');
check('a background of that colour is left alone', Bold.apply('\x1b[48;5;170mx'), '\x1b[48;5;170mx');
check('plain text is unchanged', Bold.apply('You see a lantern.\r\n'), 'You see a lantern.\r\n');
check('a non-string is unchanged', Bold.apply(null), null);

var game = fs.readFileSync(path.join(PUBLIC, 'webclient-pure.html'), 'utf8');
check('the game page loads it',
    game.indexOf('/static/js/roomtitlebold.js?v={{ .ASSET_VERSION }}"') !== -1, true);
check('server text goes through it on the way to the terminal',
    /term\.write\(window\.RoomTitleBold \? window\.RoomTitleBold\.apply\(shown\) : shown\);/.test(game), true);

if (failures) { console.log(failures + ' failed'); process.exit(1); }
console.log('all passed');
````

- [ ] **Step 2: Run it**

Run: `node tools/jstest/roomtitlebold.test.js 2>&1 | tail -3`
Expected: `Cannot find module`.

- [ ] **Step 3: Implement**

Write `PUB/static/js/roomtitlebold.js`:

````js
// roomtitlebold.js: draws room titles in the one bold in the game window.
//
// Room titles are bold (rule 7 of the web client base spec). The game's colour
// tags have no working bold (ansitags reads bold= and ignores it), so the web
// client finds the room title's colour and adds bold to it. Anything else
// drawn in that colour turns bold too: a few lines reuse the alias.
//
// A top-level tag closes with a full reset, which ends the bold. A room-title
// tag nested inside another colour would stay bold until the outer tag closes,
// so keep this colour at the top level. Telnet players are not affected.
// internal/guards/client_type_guard_test.go holds this number equal to
// room-title in ansi-aliases.yaml.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.RoomTitleBold = api;  // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var ROOM_TITLE_COLOUR = 170;
    var PLAIN = '\x1b[38;5;' + ROOM_TITLE_COLOUR + 'm';
    var BOLD = '\x1b[1;38;5;' + ROOM_TITLE_COLOUR + 'm';

    function apply(data) {
        if (typeof data !== 'string' || data.indexOf(PLAIN) === -1) return data;
        return data.split(PLAIN).join(BOLD);
    }

    return { apply: apply };
}));
````

Edit `WP`. After the line loading `motdrestyle.js`, add:

```html
    <script src="{{ .CONFIG.FilePaths.WebCDNLocation }}/static/js/roomtitlebold.js?v={{ .ASSET_VERSION }}"></script>
```

Replace

```js
                term.write(window.MotdRestyle ? window.MotdRestyle.restyle(event.data) : event.data);
```

with

```js
                var shown = window.MotdRestyle ? window.MotdRestyle.restyle(event.data) : event.data;
                term.write(window.RoomTitleBold ? window.RoomTitleBold.apply(shown) : shown);
```

The next lines read `event.data`, so the trigger path is unchanged.

- [ ] **Step 4: Run it**

Run: `node tools/jstest/roomtitlebold.test.js 2>&1 | tail -1`
Expected: `all passed`.

- [ ] **Step 5: Commit**

```bash
git add tools/jstest/roomtitlebold.test.js _datafiles/html/public/static/js/roomtitlebold.js _datafiles/html/public/webclient-pure.html
git commit -F - <<'EOF'
feat(webclient): draw room titles in bold

The colour tags have no working bold, so the client adds it to the room
title's colour. Telnet is unchanged.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
git show --stat HEAD | tail -5
```

---

### Task 5: The sweep

One script makes every size, italic and weight change, then the token, layout and font-face edits, from a table whose every row it asserts. It writes to the scratchpad only; the output is copied in with `cp`. Task 4 has already changed `WP`, so the script's line numbers are read against the commit this branch started from, not the working tree.

**Files:** Modify `gomud.css`, `dashboard.css`, `WP`, `architecture.html`, `gmcp.js`, both Help pages.

- [ ] **Step 1: Write the script** to `$S/apply_type.py`:

````python
#!/usr/bin/env python3
"""Slice 2 sweep: rewrite every font size, italic and weight in the client and
site files. Reads the repo at argv[1] and writes each changed file under
argv[2], same relative path. It never writes into the repo; copy the output in
with cp. Line numbers are the base commit's, and every row is asserted."""
import os
import re
import sys

ROOT, OUT = sys.argv[1], sys.argv[2]
PUB = "_datafiles/html/public/"
HELP = "modules/webhelp/files/datafiles/html/public/"

SIZE = re.compile(r"font-size\s*:\s*[^;}\"']+")
SHORTHAND = re.compile(r"font:\s*(italic\s+)?\d+px\s+var\(--font-body\)")
ITALIC_DECL = re.compile(r"font-style:\s*italic;\s*")
ITALIC_ATTR = re.compile(r"""['"]font-style['"]:\s*['"]italic['"],\s*""")

# path -> sizes {line: token} (anything not named is --text), keep (lines left
# alone), drop (lines removed), expect (size rewrites), weights {line: (old,
# new)}, subs [(line, old, new)], italics (expected removals).
FILES = {
    PUB + "static/css/gomud.css": dict(
        sizes={330: "nav-text", 381: "nav-text", 386: "nav-text",
               496: "foot-text", 514: "foot-text", 571: "foot-text",
               449: "title", 555: "title"},
        keep={591, 601, 611},  # the old @font-face rules, replaced below
        expect=16, italics=0,
        weights={283: ("bold", "400"), 341: ("600", "400"), 447: ("bold", "var(--bold)")},
    ),
    PUB + "static/css/dashboard.css": dict(
        sizes={125: "title", 1422: "title"},
        keep={736},
        drop=set(range(127, 132)),
        expect=54, italics=16,
        weights={62: ("bold", "var(--bold)"), 322: ("600", "400"),
                 361: ("600", "var(--bold)"), 387: ("600", "var(--bold)"),
                 533: ("600", "400"), 1149: ("600", "400"), 1575: ("700", "400")},
        subs=[(365, "smaller italic, muted", "muted"),
              (1380, "Dimmed and italic: shape and brightness carry", "Dimmed: brightness carries")],
    ),
    PUB + "webclient-pure.html": dict(
        sizes={77: "title", 173: "cmd", 3840: "title"},
        keep={506},
        expect=26, italics=1,
        weights={3650: ("bold", "var(--bold)"), 4567: ("bold", "400")},
        subs=[(435, "fontSize: 20,", "fontSize: 20,\n            // The one bold (--bold in gomud.css); room titles use it.\n            fontWeightBold: '600',"),
              (556, "Source Code Pro", "JetBrains Mono"),
              (560, "400 16px \"Source Code Pro\"", "400 16px \"JetBrains Mono\""),
              (561, "700 16px \"Source Code Pro\"", "600 16px \"JetBrains Mono\""),
              (4308, "dimmed and italic", "dimmed")],
    ),
    PUB + "architecture.html": dict(
        sizes={67: "title"}, expect=3, italics=1,
        subs=[(6, "--font-serif", "--font-heading"), (66, "--font-serif", "--font-heading"),
              (35, "italic line 4.62. A lighter fill fails the italic line.",
                   "why line 4.62. A lighter fill fails the why line.")],
    ),
    HELP + "help.html": dict(expect=4, italics=0, weights={25: ("bold", "400")}),
    HELP + "help-details.html": dict(expect=6, italics=0, weights={28: ("bold", "400")}),
    PUB + "static/js/gmcp.js": dict(
        size_scan=False, attr_italics=6, expect=0, italics=0,
        subs=[(1355, "font-weight:bold;font-size:12px;", "font-weight:400;font-size:var(--text);"),
              (1469, "\"'Source Sans 3', system-ui, sans-serif\",   // the map's lettering; system-ui where gomud.css is not loaded",
                     "\"'JetBrains Mono', ui-monospace, Menlo, Consolas, monospace\",   // the map's lettering"),
              (1052, "brass pin + italic name", "brass pin + name")],
    ),
}

FONT = '"JetBrains Mono", ui-monospace, Menlo, Consolas, monospace'
GOMUD = PUB + "static/css/gomud.css"
DASH = PUB + "static/css/dashboard.css"
GAME = PUB + "webclient-pure.html"

# Whole blocks: (start marker, end marker, replacement), end included.
BLOCKS = {
    GOMUD: [
        ("  /* ── Fonts ── */\n", '--font-mono: "Source Code Pro", ui-monospace, Menlo, Consolas, monospace;',
         "  /* ── Type (spec 2026-11-09, rulings 4 to 8a) ──\n"
         "     One font, two text sizes, one bold. Nothing else sets a size in the\n"
         "     client or the site pages: internal/guards/client_type_guard_test.go\n"
         "     holds that. The three font tokens stay separate names so a later\n"
         "     change can part them again. */\n"
         f"  --font-body: {FONT};\n"
         f"  --font-heading: {FONT};\n"
         f"  --font-mono: {FONT};\n"
         "  --text: 16px;\n"
         "  --title: 20px;\n"
         "  --cmd: 16px;        /* the command box: iOS zooms the page on anything smaller */\n"
         "  --nav-text: 20px;   /* the site's top bar */\n"
         "  --foot-text: 16px;  /* the site's footer */\n"
         "  --bold: 600;        /* the one bold: titles, quest names, your own name, room titles */"),
        ("/* ── Fonts: the Source family", 'src: url("../fonts/SourceCodePro-Italic.woff2") format("woff2");\n}',
         "/* ── Fonts: JetBrains Mono, shipped with the site so every device shows the\n"
         "   same type. The official v2.304 web fonts, unmodified: the Google Fonts\n"
         "   version has no box-drawing characters. Licence beside them in\n"
         "   static/fonts/ (SIL Open Font License 1.1). ── */\n"
         "@font-face {\n"
         '  font-family: "JetBrains Mono";\n'
         "  font-style: normal; font-weight: 400; font-display: swap;\n"
         '  src: url("../fonts/JetBrainsMono-Regular.woff2") format("woff2");\n'
         "}\n"
         "@font-face {\n"
         '  font-family: "JetBrains Mono";\n'
         "  font-style: normal; font-weight: 600; font-display: swap;\n"
         '  src: url("../fonts/JetBrainsMono-SemiBold.woff2") format("woff2");\n'
         "}"),
    ],
}

# Exact replacements after the sweep; each old text must occur exactly once.
EXACT = {
    GOMUD: [
        ("/* ── Light theme (website pages only, 2026-11-01) ──",
         ":root { font-variant-ligatures: none; }\n"
         "/* Upright phones. The class comes from the screen (viewportfit.js), so the\n"
         "   on-screen keyboard cannot change the size. */\n"
         ":root.upright-phone { --text: 14px; --title: 18px; }\n"
         "/* The site bars on phones: under 768px wide or 500px tall. */\n"
         "@media (max-width: 767px), (max-height: 500px) { :root { --nav-text: 16px; --foot-text: 14px; } }\n"
         "\n"
         "/* ── Light theme (website pages only, 2026-11-01) ──"),
        ("  font-family: var(--font-body);\n  background-color: var(--bg-base);",
         "  font-family: var(--font-body);\n  font-size: var(--text);\n  background-color: var(--bg-base);"),
        ("a { text-decoration: none; color: inherit; }\n",
         "a { text-decoration: none; color: inherit; }\n"
         "/* Browsers give form controls their own type, ligatures included. */\n"
         "button, input, select, textarea {\n"
         "  font-family: var(--font-body);\n"
         "  font-size: var(--text);\n"
         "  font-weight: 400;\n"
         "  font-variant-ligatures: none;\n"
         "}\n"
         "/* Regular weight for body text; the one bold is for titles. */\n"
         "b, strong { font-weight: 400; }\n"
         "/* No italics, including what browsers italicise by default. */\n"
         "em, i, cite, dfn, var, address { font-style: normal; }\n"),
        ("  text-transform: uppercase;\n  letter-spacing: 1px;\n  padding: 0.7em;",
         "  font-weight: var(--bold);\n  text-transform: uppercase;\n  letter-spacing: 1px;\n  padding: 0.7em;"),
        ("table.config td { font-family: var(--font-mono); font-size: var(--text); }",
         "table.config td { font-family: var(--font-mono); font-size: var(--text); overflow-wrap: anywhere; }"),
        ("h1, h2, h3, h4 { font-family: var(--font-heading); }",
         "h1, h2, h3, h4 { font-family: var(--font-heading); font-size: var(--title); font-weight: var(--bold); }"),
        ("/* Tablets: tighter links so the logo and all seven fit on one row. */",
         "/* Tablets: tighter links. At 20px the seven links take two rows below about\n   1125px wide. */"),
    ],
    DASH: [
        (".dash-panel-head .ph-title {\n  font-family: var(--font-heading);\n  font-size: var(--title); color: var(--vellum);\n}",
         ".dash-panel-head .ph-title {\n  font-family: var(--font-heading);\n  font-size: var(--title); font-weight: var(--bold); color: var(--vellum);\n"
         "  min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;\n}"),
        ("  --tabname-h: 30px;  /* the name strip, shown while a panel is open */",
         "  --tabname-h: 36px;  /* the name strip, shown while a panel is open; fits the pill at the text size */"),
        ("and a 30px strip above it for the open tab's name.", "and a 36px strip above it for the open tab's name."),
        (".qrow .qname { flex:1; font-family:var(--font-body);", ".qrow .qname { flex:1; min-width:0; overflow-wrap:anywhere; font-family:var(--font-body);"),
        ("#dash-col-right > #panel-chat { min-height: 240px; }\n",
         "#dash-col-right > #panel-chat { min-height: 240px; }\n"
         "/* Automation's tabs take two rows at the text size; this keeps its list in\n"
         "   view on a laptop until layout D replaces these columns. */\n"
         "#dash-col-right > #panel-trig { min-height: 200px; }\n"),
        (".dash-panel-head .ph-btns { margin-left: auto; display: flex; gap: 4px; }",
         ".dash-panel-head .ph-btns { margin-left: auto; display: flex; flex: none; gap: 4px; }\n"
         "/* Room for the Map's title beside its two buttons at the title size. */\n"
         "#panel-map .dash-panel-head { padding-left: 8px; padding-right: 4px; gap: 2px; }\n"
         "#panel-map .dash-panel-head .ph-btns { gap: 2px; }"),
        ("#map-controls .rgsvg-brass { min-width: 32px; height: 32px;", "#map-controls .rgsvg-brass { min-width: 36px; height: 32px;"),
        (".qrow .qdot { flex:0 0 auto; width:13px;", ".qrow .qdot { flex:0 0 auto; width:1.2em;"),
        (".qbar { flex:0 0 auto; align-self:center; width:42px;", ".qbar { flex:0 0 auto; align-self:center; width:48px;"),
        (".inv-cell {\n  display: flex;", ".inv-cell {\n  min-width: 0;\n  display: flex;"),
        ("/* Slot label below the tile */\n.inv-slab {\n  font-size: var(--text);\n",
         "/* Slot label below the tile. Off: a 46px column fits about four letters at\n   the text size (spec, Equipped panel). */\n.inv-slab {\n  display: none;\n"),
        ("  border-radius: 2px;\n  padding: 0 2px;\n  pointer-events: none;\n  line-height: 1.3;",
         "  border-radius: 2px;\n  padding: 0 3px;\n  pointer-events: none;\n  line-height: 1.3;"),
        ("  padding: 2px 9px; border-radius: 12px;\n  border: 1px solid var(--panel-border); background: var(--chip-bg);",
         "  padding: 2px 10px; border-radius: 12px;\n  border: 1px solid var(--panel-border); background: var(--chip-bg);"),
        ("  width: 18px;\n  height: 18px;\n  line-height: 18px;\n  text-align: center;\n  border-radius: 50%;\n  background: var(--madder);",
         "  width: 24px;\n  height: 24px;\n  line-height: 24px;\n  text-align: center;\n  border-radius: 50%;\n  background: var(--madder);"),
        (".winbox.dash-winbox .wb-title {\n  font-family: var(--font-heading);\n",
         ".winbox.dash-winbox .wb-title {\n  font-family: var(--font-heading);\n  font-size: var(--title);\n  font-weight: var(--bold);\n"),
        (".auto-modal-title {\n  font-family: var(--font-heading);\n  font-size: var(--title);\n",
         ".auto-modal-title {\n  font-family: var(--font-heading);\n  font-size: var(--title);\n  font-weight: var(--bold);\n"),
    ],
    GAME: [
        ("#vitals-bars {\n            height: 100%;\n            padding: 2px;\n            display: flex;\n            flex-direction: column;\n            gap: 1px;",
         "#vitals-bars {\n            height: auto;\n            padding: 2px;\n            display: flex;\n            flex-direction: column;\n            gap: 2px;"),
        (".vitals-row {\n            display: flex;\n            align-items: center;\n            height: 20px;",
         ".vitals-row {\n            display: flex;\n            align-items: center;\n            height: 26px;"),
        (".vitals-name {\n            width: 70px;\n            min-width: 70px;",
         ".vitals-name {\n            width: 9ch;\n            min-width: 9ch;"),
        (".vitals-composite {\n            flex: 1;\n            display: flex;\n            height: 14px;",
         ".vitals-composite {\n            flex: 1;\n            display: flex;\n            height: 22px;"),
        ("            font-size: var(--text);\n            padding: 2px 8px;\n            border-radius: 12px;",
         "            font-size: var(--text);\n            padding: 2px 10px;\n            border-radius: 12px;"),
        ("#settings-title {\n            font-family: var(--font-heading);\n            font-size: var(--title);\n",
         "#settings-title {\n            font-family: var(--font-heading);\n            font-size: var(--title);\n            font-weight: var(--bold);\n"),
        ("font-size: var(--title); font-family: var(--font-heading); margin: 6px 0 8px; }",
         "font-size: var(--title); font-weight: var(--bold); font-family: var(--font-heading); margin: 6px 0 8px; }"),
    ],
}

for rel, spec in FILES.items():
    with open(os.path.join(ROOT, rel), encoding="utf-8", newline="") as fh:
        raw = fh.read()
    crlf = "\r\n" in raw  # a Windows checkout; the output keeps what it found
    lines = raw.replace("\r\n", "\n").split("\n")
    size_scan = spec.get("size_scan", True)
    keep, drop = spec.get("keep", set()), spec.get("drop", set())
    sizes, weights = spec.get("sizes", {}), spec.get("weights", {})
    subs = {}
    for ln, old, new in spec.get("subs", []):
        subs.setdefault(ln, []).append((old, new))
    done_sizes = done_italics = done_attrs = 0
    used_weights, used_subs, used_tokens = set(), set(), set()
    out = []
    for no, line in enumerate(lines, 1):
        if no in drop:
            continue
        for old, new in subs.get(no, []):
            assert old in line, f"{rel}:{no}: sub text not found: {old!r} in {line!r}"
            line = line.replace(old, new)
            used_subs.add(no)
        if no in weights:
            old, new = weights[no]
            pat = re.compile(r"font-weight\s*:\s*" + re.escape(old) + r"\b")
            assert pat.search(line), f"{rel}:{no}: weight {old} not found in {line!r}"
            line = pat.sub("font-weight: " + new, line)
            used_weights.add(no)
        if size_scan and no not in keep:
            m = SHORTHAND.search(line)
            if m:
                if m.group(1):
                    done_italics += 1
                line = SHORTHAND.sub("font-family: var(--font-body); font-size: var(--text)", line)
            elif SIZE.search(line):
                token = sizes.get(no, "text")
                used_tokens.add(no)
                line, n = SIZE.subn(f"font-size: var(--{token})", line)
                assert n == 1, f"{rel}:{no}: {n} sizes on one line"
                done_sizes += 1
            if ITALIC_DECL.search(line):
                stripped = ITALIC_DECL.sub("", line)
                done_italics += 1
                if stripped.strip() == "":
                    continue
                line = stripped
        if spec.get("attr_italics") and ITALIC_ATTR.search(line):
            line = ITALIC_ATTR.sub("", line)
            done_attrs += 1
        out.append(line)
    assert done_sizes == spec["expect"], f"{rel}: {done_sizes} sizes, expected {spec['expect']}"
    assert done_italics == spec["italics"], f"{rel}: {done_italics} italics, expected {spec['italics']}"
    assert done_attrs == spec.get("attr_italics", 0), f"{rel}: {done_attrs} attr italics"
    assert used_weights == set(weights), f"{rel}: weights unused {set(weights) - used_weights}"
    assert used_subs == set(subs), f"{rel}: subs unused {set(subs) - used_subs}"
    missed = set(sizes) - used_tokens
    assert not missed, f"{rel}: named size lines not rewritten: {missed}"
    text = "\n".join(out)
    for old, new in EXACT.get(rel, []):
        assert text.count(old) == 1, f"{rel}: {text.count(old)} matches for {old!r}"
        text = text.replace(old, new)
    for start, end, new in BLOCKS.get(rel, []):
        a, b = text.index(start), text.index(end, text.index(start)) + len(end)
        text = text[:a] + new + text[b:]
    target = os.path.join(OUT, rel)
    os.makedirs(os.path.dirname(target), exist_ok=True)
    with open(target, "w", encoding="utf-8", newline="") as fh:
        fh.write(text.replace("\n", "\r\n") if crlf else text)
    print(f"{rel}: sizes {done_sizes}, italics {done_italics + done_attrs}, weights {len(used_weights)}, text edits {len(used_subs)}, exact {len(EXACT.get(rel, []))}")
````

- [ ] **Step 2: Run it on the base commit**

```bash
S="<the session scratchpad, full path>"
B=$(git merge-base HEAD origin/master)
rm -rf "$S/base" "$S/type-out"; mkdir -p "$S/type-out"
for f in _datafiles/html/public/static/css/gomud.css _datafiles/html/public/static/css/dashboard.css _datafiles/html/public/webclient-pure.html _datafiles/html/public/architecture.html _datafiles/html/public/static/js/gmcp.js modules/webhelp/files/datafiles/html/public/help.html modules/webhelp/files/datafiles/html/public/help-details.html; do
  mkdir -p "$S/base/$(dirname "$f")"
  MSYS_NO_PATHCONV=1 git show "$B:$f" > "$S/base/$f"
done
python -I "$S/apply_type.py" "$S/base" "$S/type-out"
```

Expected, exactly:

```
_datafiles/html/public/static/css/gomud.css: sizes 16, italics 0, weights 3, text edits 0, exact 7
_datafiles/html/public/static/css/dashboard.css: sizes 54, italics 16, weights 7, text edits 2, exact 16
_datafiles/html/public/webclient-pure.html: sizes 26, italics 1, weights 2, text edits 5, exact 7
_datafiles/html/public/architecture.html: sizes 3, italics 1, weights 0, text edits 3, exact 0
modules/webhelp/files/datafiles/html/public/help.html: sizes 4, italics 0, weights 1, text edits 0, exact 0
modules/webhelp/files/datafiles/html/public/help-details.html: sizes 6, italics 0, weights 1, text edits 0, exact 0
_datafiles/html/public/static/js/gmcp.js: sizes 0, italics 6, weights 0, text edits 3, exact 0
```

If an assertion fails, fix that row against the file. Do not relax an assertion.

- [ ] **Step 3: Carry Task 4's two edits into the sweep output, then land everything**

The output of `webclient-pure.html` comes from the branch point, so it lacks Task 4's script tag and write line. Edit `$S/type-out/_datafiles/html/public/webclient-pure.html` with the Edit tool and make the same two replacements as Task 4 Step 3. Then:

```bash
S="<the session scratchpad, full path>"
F=_datafiles/html/public/static/fonts
cp -r "$S/type-out"/* .
git rm -q $F/SourceCodePro-Italic.woff2 $F/SourceCodePro-LICENSE.txt $F/SourceCodePro-Upright.woff2 $F/SourceSans3-Italic.woff2 $F/SourceSans3-LICENSE.txt $F/SourceSans3-Upright.woff2 $F/SourceSerif4-Italic.woff2 $F/SourceSerif4-LICENSE.txt $F/SourceSerif4-Roman.woff2
cp "$S/jbm-dl/x/fonts/webfonts/JetBrainsMono-Regular.woff2" "$S/jbm-dl/x/fonts/webfonts/JetBrainsMono-SemiBold.woff2" $F/
cp "$S/jbm-dl/x/OFL.txt" $F/JetBrainsMono-LICENSE.txt
ls $F
git status --short
```

Expected: `ls` shows exactly the three font files. `git status` lists the seven swept files as modified, the guard and three font files as untracked, and the nine `Source*` files as staged deletions. Nothing else.

- [ ] **Step 4: Run the guard and the JS tests**

```bash
go test ./internal/guards/ -run 'TestClientTypeRulesCanFail|TestClientAndSiteType|TestMapArt|TestTypeTokens|TestFontFolder|TestRoomTitleBold' -count=1 2>&1 | tail -3
for t in tools/jstest/*.test.js; do node "$t" > /dev/null 2>&1 || echo "RED $t"; done; echo done
```

Expected: `ok`, then only `done`.

- [ ] **Step 5: Commit**

```bash
F=_datafiles/html/public/static/fonts
git add internal/guards/client_type_guard_test.go $F/JetBrainsMono-Regular.woff2 $F/JetBrainsMono-SemiBold.woff2 $F/JetBrainsMono-LICENSE.txt _datafiles/html/public/static/css/gomud.css _datafiles/html/public/static/css/dashboard.css _datafiles/html/public/webclient-pure.html _datafiles/html/public/architecture.html _datafiles/html/public/static/js/gmcp.js modules/webhelp/files/datafiles/html/public/help.html modules/webhelp/files/datafiles/html/public/help-details.html
git diff --cached --stat | tail -22
git commit -F - <<'EOF'
feat(webclient): one font, two sizes, no italics, one bold

JetBrains Mono replaces the Source family across the client and the site.
Every size reads a token in gomud.css, and a guard fails anything else.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
git show --stat HEAD | tail -3
```

Expected: the staged list has the nine deletions, three fonts, the guard and the seven swept files (20 files).

---

### Task 6: Brand standard, patch notes, index

**Files:** Modify `docs/guides/BRAND_STANDARDS.md`, `docs/PATCH_NOTES.md`, `docs/README.md`, `internal/guards/context.md`. Copy in this plan.

The owner and Malia approved the type rulings together (spec, 2026-11-09), which meets the agreement that `BRAND_STANDARDS.md` line 6 asks for.

- [ ] **Step 1: Rewrite section 3 of `docs/guides/BRAND_STANDARDS.md`.** Replace the four bullets under `## 3. Type` with:

```markdown
- **JetBrains Mono** for everything: game text, panels, menus, buttons,
  headings and the site pages (`--font-body`, `--font-heading` and
  `--font-mono` all name it). Ligatures off.
- Self-hosted (`static/fonts/`): the official v2.304 web files, unmodified.
  The Google Fonts version has no box-drawing characters, which misaligns
  every box in the game.
- **Two sizes:** text 16px and titles 20px (`--text`, `--title`), 14px and
  18px on upright phones. The command box is always 16px (`--cmd`): iOS zooms
  the page on anything smaller.
- **Site bars:** top bar 20px and footer 16px (`--nav-text`, `--foot-text`),
  16px and 14px on phones (under 768px wide or 500px tall).
- **No italics.** Regular weight for body text and every button. One bold
  (`--bold`, 600) for titles, quest names, your own character's name and room
  titles. No glow or text shadow on brass labels.
- Lettering in the map art scales with the map. The builder, the admin pages
  and the architecture diagrams keep their own styling.
- `internal/guards/client_type_guard_test.go` holds the font, sizes, weights
  and italics.
```

- [ ] **Step 2: Add to the top of `docs/PATCH_NOTES.md`**, under the title, dated the day it ships:

```markdown
## <ship date, YYYY-MM-DD>: One look for the web client

- The web client and the website now share one clear typeface, in one text
  size and one title size, with no italics.
- Room titles, quest names and your own name stand out in bold.
- Text on the side panels is larger and easier to read.
- The game window's text still changes size with your window for now.
```

- [ ] **Step 3: Update `docs/README.md`.** Edit this worktree's copy with the Edit tool. Do not copy the file from the main checkout, which holds another session's uncommitted row.

Replace the row for `static/fonts/` with:

```markdown
| [`../_datafiles/html/public/static/fonts/`](../_datafiles/html/public/static/fonts/) | The site's self-hosted font, so every device shows the same type: JetBrains Mono v2.304, Regular and SemiBold, the official web files unmodified, with its SIL Open Font License beside them. Wired up by the `@font-face` rules and the type tokens in `static/css/gomud.css`, and held by `internal/guards/client_type_guard_test.go` |
```

After the row for `2026-11-09-webclient-base-1-server-wrap-width.md`, add:

```markdown
| [`superpowers/plans/2026-11-10-webclient-base-2-type-system.md`](superpowers/plans/2026-11-10-webclient-base-2-type-system.md) | Plan 2 of the web client base: JetBrains Mono, the size tokens, no italics, one bold, the site bars and pages, with the guard |
| [`../internal/guards/client_type_guard_test.go`](../internal/guards/client_type_guard_test.go) | Rulings 4 to 8a of the web client base: every size, family and weight in the client and site reads a token, no italics, the ruled sizes, the official font files, and the room title colour the client bolds |
| [`../_datafiles/html/public/static/js/roomtitlebold.js`](../_datafiles/html/public/static/js/roomtitlebold.js) | Draws room titles in bold in the web client, by their colour, because the game's colour tags have no working bold. Tested by `tools/jstest/roomtitlebold.test.js` |
```

- [ ] **Step 4: Update `internal/guards/context.md`.** Replace

```markdown
  `LineWidth`), the `*_wrapper_guard_test.go` files
```

with

```markdown
  `LineWidth`), `client_type_guard_test.go` (the web client's and site's fonts,
  sizes and weights read the tokens in `gomud.css`), the
  `*_wrapper_guard_test.go` files
```

- [ ] **Step 5: Copy this plan in and commit**

```bash
M="/c/Users/Calabe Davis/workspace/DOGMud"
cp "$M/docs/superpowers/plans/2026-11-10-webclient-base-2-type-system.md" docs/superpowers/plans/
git add docs/guides/BRAND_STANDARDS.md docs/PATCH_NOTES.md docs/README.md internal/guards/context.md docs/superpowers/plans/2026-11-10-webclient-base-2-type-system.md
git diff --cached --stat -- docs/README.md
git commit -F - <<'EOF'
docs(webclient): the type rules in the brand standard, notes and index

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: the `docs/README.md` diff is 4 lines added and 1 removed.

---

### Task 7: Verify and ship

- [ ] **Step 1: Prove the guard can fail on real files.** Each sabotage is an Edit of one line, undone by the opposite Edit.

1. In `dashboard.css`, add `font-size: 13px;` inside `.chat-hint`. Run the guard. Expected: `TestClientAndSiteTypeUsesTheTokens` fails naming that line. Undo.
2. In `gomud.css`, change `--text: 16px;` to `--text: 15px;`. Expected: `TestTypeTokensHoldTheRuledSizes` fails. Undo.
3. In `gmcp.js`, add `"font-style": "italic",` to the map legend text call at `"font-size": 8, fill: L.ink`. Expected: `TestMapArtHasNoItalicsOrOldFonts` fails. Undo.
4. In `ansi-aliases.yaml`, change `room-title: 170` to `room-title: 171`. Expected: `TestRoomTitleBoldMatchesTheAlias` fails. Undo.

Then `git status --short` must print nothing.

- [ ] **Step 2: Pre-push gates**

```bash
gofmt -l internal/ modules/
go build ./...
go test . ./internal/... ./modules/... -count=1 2>&1 | grep -vE '^ok|no test files' | tail -20
for t in tools/jstest/*.test.js; do node "$t" > /dev/null 2>&1 || echo "RED $t"; done; echo done
golangci-lint run --new-from-merge-base=origin/master
```

Expected: `gofmt` prints nothing, the test filter prints nothing, the JS loop prints only `done`, lint reports `0 issues`.

- [ ] **Step 3: Boot on private ports, and take the checklist screenshots.** Never touch the owner's server: these ports are private and the process is killed by PID.

Run this with the Bash tool's `run_in_background`, so `timeout` ends the server by itself. Never kill a server by name or port.

```bash
S="<the session scratchpad, full path>"
go build -o boot-check.exe .
printf 'Network:\n  TelnetPort: [43333]\n  LocalPort: 19999\n  HttpPort: 18090\n  AIPort: 45555\n' > "$S/boot-override.yaml"
CONFIG_PATH="$S/boot-override.yaml" timeout 1200 ./boot-check.exe > "$S/boot.log" 2>&1
```

Wait for `Server Ready` in `$S/boot.log`. Confirm `grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$S/boot.log"` prints `0`.

Desktop sizes use `"/c/Program Files/Google/Chrome/Application/chrome.exe" --headless=new --disable-gpu --hide-scrollbars --user-data-dir="$S/chrome-prof" --window-size=W,H --virtual-time-budget=6000 --screenshot="$S/x.png" URL`. Phone sizes need real emulation, because an iframe or a small window still reports the desktop's `window.screen`, and the 14/18 class would never be set. Write `$S/cdp-shot.js`:

````js
// cdp-shot.js: screenshot a page as a real phone, so window.screen, touch and
// the upright-phone class are true. Needs Chrome started with
// --headless=new --remote-debugging-port=9333.
// Usage: node cdp-shot.js <url> <width> <height> <out.png> [js to run first]
'use strict';
const fs = require('fs');
const [url, w, h, out, js] = process.argv.slice(2);
const width = Number(w), height = Number(h);
(async () => {
  const target = await (await fetch('http://127.0.0.1:9333/json/new?about:blank', { method: 'PUT' })).json();
  const ws = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((ok, no) => { ws.onopen = ok; ws.onerror = no; });
  let id = 0;
  const waiting = new Map();
  ws.onmessage = (m) => {
    const d = JSON.parse(m.data);
    if (d.id && waiting.has(d.id)) { waiting.get(d.id)(d.result || d.error); waiting.delete(d.id); }
  };
  const send = (method, params) => new Promise((ok) => {
    id += 1; waiting.set(id, ok); ws.send(JSON.stringify({ id, method, params }));
  });
  const wait = (ms) => new Promise((ok) => setTimeout(ok, ms));
  await send('Emulation.setDeviceMetricsOverride', {
    width, height, deviceScaleFactor: 1, mobile: true,
    screenWidth: width, screenHeight: height,
    screenOrientation: width < height ? { type: 'portraitPrimary', angle: 0 } : { type: 'landscapePrimary', angle: 90 },
  });
  await send('Emulation.setTouchEmulationEnabled', { enabled: true });
  await send('Page.enable');
  await send('Runtime.enable');
  await send('Page.navigate', { url });
  await wait(5000);
  if (js) { await send('Runtime.evaluate', { expression: js }); await wait(800); }
  const probe = await send('Runtime.evaluate', { returnByValue: true, expression:
    `JSON.stringify({cls: document.documentElement.className, text: getComputedStyle(document.documentElement).getPropertyValue('--text'),
      title: getComputedStyle(document.documentElement).getPropertyValue('--title'), sideways: document.documentElement.scrollWidth > innerWidth})` });
  console.log(out.split(/[\\/]/).pop(), probe.result.value);
  const shot = await send('Page.captureScreenshot', { format: 'png' });
  fs.writeFileSync(out, Buffer.from(shot.data, 'base64'));
  await fetch('http://127.0.0.1:9333/json/close/' + target.id);
  ws.close();
})().catch((e) => { console.error(e); process.exit(1); });
````

Start Chrome with the Bash tool's `run_in_background`, noting its PID:

```bash
"/c/Program Files/Google/Chrome/Application/chrome.exe" --headless=new --disable-gpu --remote-debugging-port=9333 --user-data-dir="$S/chrome-cdp" about:blank
```

Run, for example, `node "$S/cdp-shot.js" http://localhost:18090/webclient-pure.html 412 915 "$S/p-chat.png" "document.querySelectorAll('.gtabs-phone .gtab')[4].click()"`. It prints the page's classes and tokens. Stop that Chrome by its PID afterwards. Check each:

| Where | Size | Must hold |
|---|---|---|
| Home | 1280x800, 1024x768 (two rows of links is expected), 412x915 | JetBrains Mono, 20px links, 16px on the phone, no sideways page scroll |
| Who's Online, Config, Architecture, Help | 1280x800, 412x915 | one font, titles larger than text, no italics, Config's Value column visible |
| Web Client page | 1280x800 | one row of links, Automation shows its list |
| Game page, logged in | 1280x800, 1366x650, 900x700 | no panel title overruns its buttons, Health bars and labels readable, Inventory tiles intact, a right-click menu, Settings, the Automation editor, a pop-out window title |
| Game page, phone | 412x915, 360x740 | the probe prints `upright-phone`, `14px`, `18px`; each of the seven tabs opens with its name pill clear of the panel title |
| Game page, phone sideways | 915x412, 844x390 | the probe prints no `upright-phone`, `16px`, `20px`; a tab opens and the command box stays visible |
| Game page, logged in | any | a room title is bold, and the following line is not |
| Light mode | Settings > Appearance | same type, colours unchanged |

Also confirm in the browser's Network tab that both `JetBrainsMono-*.woff2` files load with status 200 and no `Source*` font is requested. The logged-in rows need a character, so give them to the owner in Step 4 if you cannot log in yourself.

Delete `boot-check.exe` after the server has exited.

- [ ] **Step 4: Owner and Malia check.** Start the test server from Step 3 (it is on a private port) and give the owner and Malia its address. Do not ask anyone to open the worktree folder.

  - **Web frontend, the owner and Malia both** (owner ruling 2026-11-10), including the real devices the spec makes a gate before a client phase merges. Use the logged-in rows of Step 3's table.
  - **Telnet, a spot check by the owner** on the private telnet port: room titles, a `say`, and a box screen look as they did before, and room titles are not bold.
  - **Missing symbols, on the web client:** say a line holding `☺ ♠ ♣ ♥ ♦`, and open a forest area's map. They come from a fallback font because JetBrains Mono lacks them. The line must stay on its grid. A small colour-emoji look on a phone is a finding to report, not a blocker.

  Wait for the owner's go-ahead before merging. Do not merge on green CI alone.

- [ ] **Step 5: Push, PR, merge when told**

```bash
git push -u origin feature/webclient-base-2-type-system
gh pr create --repo pruuk/DOGMud --base master --head feature/webclient-base-2-type-system --title "Web client base 2: one font, two sizes" --body-file "$S/pr-body.md"
gh pr checks <n> --repo pruuk/DOGMud --watch
```

Write `$S/pr-body.md` with the Write tool: what players see, the seven departures above, and what was tested. Say "Part of #783". Do not use a closing keyword with any issue number, even in a negative. End it with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`. Read the URL `gh` prints and confirm it says `pruuk/DOGMud`. Merge with `gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch` only when the owner says so. The owner deploys. Nothing in this plan needs a deploy step or a migration.

---

## Changed after the whole-branch review

The review of the finished branch found these, and they are fixed in the commit after the docs. Every field a player types in reads `--cmd`, so iOS does not zoom the page on an upright phone (the Automation editor, Help search, `select` and `textarea`). The map's quest plates are sized for a monospaced font. Right-click menu headers are capped in characters instead of pixels. `viewportfit.js` also reads `window.orientation` for iOS before 16.4. The guard scans `dashboard.js` in full, pins all three family tokens and the ligature rule, and accepts a value that ends a quoted string. The guard does not read SVG attributes such as `font-size="7"`, which only the map legend's icons use, an accepted limit.

## Self-review against the spec

| Spec item | Task |
|---|---|
| Rule 4, one font, ligatures off, licence beside it, the complete font | 2, 5 (`:root`, form controls), guard hashes |
| Rule 5, two sizes, 14/18 on upright phones | 3, 5, guard |
| Rule 6, command box 16px | 5 (`--cmd`), guard |
| Rule 7, no italics, regular buttons, one bold | 4, 5, guard |
| Rule 8, bars 20/16 and 16/14 | 5, guard |
| Rule 8a, Home, Who's Online, Architecture, Config, 404; Source Sans and Serif retired; logo stays an image; builder, admin and diagrams kept out | 5, guard scope |
| Section 3, tokens, `BRAND_STANDARDS.md` section 3 with its 15px rule, map art and icon exceptions | 5, 6 |
| Plans table: "Game text still changes size with the window" | No change to `termfit.js` or `fontSize` |
| Testing: type guard, WCAG contrast at the new sizes | Guard; colours do not change, and contrast does not get worse as text gets larger |
| Testing: real devices | 7 Step 4 |

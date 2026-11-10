package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/language"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/templates"
	textlang "golang.org/x/text/language"
)

// Every template works down to 30 columns (web client base spec, ruling 3),
// apart from maps and wide tables. This guard knows every template, in the
// live world and in every module, and why it is safe at 30. It renders the
// ones that must fit at 30, 42 and 80.
//
// The classes are the state of the work, not a permanent exemption list:
// plan 3b empties "plan 3b", plan 4 does "help/". When both are done, only
// "wide" and the classes that never reach a narrow screen unwrapped remain.

// templateFolders are every place a template is read from: the live world,
// and each module's own files (modules/<name>/files/datafiles/templates).
func templateFolders(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	folders := []string{filepath.Join(root, "_datafiles", "world", "dogmud", "templates")}
	modules, err := filepath.Glob(filepath.Join(root, "modules", "*", "files", "datafiles", "templates"))
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) == 0 {
		t.Fatal("found no module template folders; the glob is not reaching them")
	}
	return append(folders, modules...)
}

// templateWidths are the widths every fitting template is rendered at: the
// narrowest screen, an upright phone, and a desktop.
var templateWidths = []int{30, 42, 80}

// templateClassNames lists the templates in each exact-name class.
var templateClassNames = map[string][]string{
	// Maps and wide tables may scroll sideways (ruling 3).
	"wide": {"maps/map", "tables/lockpicking"},
	// Sent outside the pipeline, before a reader has a width or as they
	// leave; the terminal soft-wraps them.
	"raw": {"goodbye", "login/connect-splash", "login/email-new.prompt",
		"login/password-new-verify.prompt", "login/password-new.prompt",
		"login/password.mask", "login/password.prompt", "login/username-new.prompt",
		"login/username.prompt", "generic/prompt.yn"},
	// Wrapped to the reader by the caller (actions/roster.go) or the
	// pipeline (CategoryLogout).
	"wrapped": {"descriptions/who", "descriptions/ontheground", "player-despawn"},
	// Narrow forms come in plan 3b.
	"plan 3b": {"character/conditions", "character/cooldowns",
		"character/description", "character/description-corpse",
		"character/inventory", "character/inventory-look", "character/pet",
		"character/quests", "character/skills", "character/skillup",
		"character/statup", "character/status", "character/storage",
		"character/title", "descriptions/biome", "descriptions/identify",
		"descriptions/insidecontainer", "descriptions/track",
		"descriptions/exits", "descriptions/fixtures", "descriptions/room",
		"descriptions/room-title", "descriptions/rune", "descriptions/sign",
		"mail/message", "auctions/auction-bid", "auctions/auction-end",
		"auctions/auction-start", "auctions/auction-update"},
	// No production code renders these (survey 2026-11-10).
	"no caller": {"character/experience", "character/jobs",
		"character/permadeath", "descriptions/train", "login/character-name.prompt",
		"login/newuser.prompt", "maps/secret",
		"generic/moon_eye_full", "generic/moon_eye_new", "generic/moon_swiftmoon_full",
		"generic/moon_swiftmoon_new", "generic/moon_wanderer_full",
		"generic/moon_wanderer_new", "generic/sunrise", "generic/sunset"},
}

// templateClassPrefixes are the classes held by a folder.
var templateClassPrefixes = map[string]string{
	"generic/splash/": "caption", // narrow readers get the scene's words (Splash_Deliver)
	"help/":           "help",    // plan 4
	"admincommands/":  "admin",   // out of scope (ruling 25)
}

// templateFixture is data that makes a template as wide as it gets in play.
// wrapped says the template goes out on a category the pipeline wraps to the
// reader, so the guard wraps it the same way before measuring.
type templateFixture struct {
	data    func() any
	wrapped bool
}

// templateFits are the templates that must fit at every width.
var templateFits = map[string]templateFixture{
	"tables/generic": {data: func() any {
		return templates.GetTable("Lost and found",
			[]string{"Item", "Where", "Since"},
			[][]string{
				{"the Ancient Lantern of the Drowned Lighthouse", "the Thornwall Temple District", "two days"},
				{"a rope", "Main Street", "today"},
			})
	}},
	"tables/shoplist": {data: func() any {
		return templates.GetTable("Items available by Grumbleguts the Provisioner",
			[]string{"Qty", "Name", "Type", "Price"},
			[][]string{
				{"3", "a flask of clear river water", "drinkable", "12 gold"},
				{"1", "a lantern", "light", "40 gold"},
			})
	}},
	"character/questup": {data: func() any { return "You have found a new quest: The Long Road Home." }, wrapped: true},
}

// templateClasses names every class that holds name.
func templateClasses(name string) []string {
	var hits []string
	if _, ok := templateFits[name]; ok {
		hits = append(hits, "fits")
	}
	for class, names := range templateClassNames {
		for _, n := range names {
			if n == name {
				hits = append(hits, class)
			}
		}
	}
	for prefix, class := range templateClassPrefixes {
		if strings.HasPrefix(name, prefix) {
			hits = append(hits, class)
		}
	}
	return hits
}

var templateFormFile = regexp.MustCompile(`^(.+)\.w(\d+)\.template$`)

// templateBases walks every template folder and returns every base name (a
// form or a screen-reader file belongs to its base) and the widths of every
// form file, per base.
func templateBases(t *testing.T) (bases map[string]bool, forms map[string][]int) {
	t.Helper()
	bases, forms = map[string]bool{}, map[string][]int{}
	for _, root := range templateFolders(t) {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(os.PathSeparator)))
			if m := templateFormFile.FindStringSubmatch(rel); m != nil {
				n, _ := strconv.Atoi(m[2])
				forms[m[1]] = append(forms[m[1]], n)
				return nil
			}
			name := strings.TrimSuffix(strings.TrimSuffix(rel, ".template"), ".md")
			bases[strings.TrimSuffix(name, ".screenreader")] = true
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(bases) < 500 {
		t.Fatalf("found only %d templates; the walk is not reaching the folders", len(bases))
	}
	return bases, forms
}

func TestEveryTemplateHasOneWidthClass(t *testing.T) {
	bases, _ := templateBases(t)
	for name := range bases {
		switch hits := templateClasses(name); len(hits) {
		case 1:
		case 0:
			t.Errorf("%s: no width class. Make it fit at 30 columns and add a fixture to templateFits, or name why it is safe", name)
		default:
			t.Errorf("%s: in more than one class: %v", name, hits)
		}
	}
	listed := []string{}
	for name := range templateFits {
		listed = append(listed, name)
	}
	for _, names := range templateClassNames {
		listed = append(listed, names...)
	}
	for _, name := range listed {
		if !bases[name] {
			t.Errorf("%s is classified but no such template exists", name)
		}
	}
}

func TestTemplateFormsAreNamedForAKnownWidth(t *testing.T) {
	bases, forms := templateBases(t)
	known := map[int]bool{}
	for _, w := range templates.FormWidths {
		known[w] = true
	}
	for base, widths := range forms {
		for _, w := range widths {
			if !known[w] {
				t.Errorf("%s.w%d.template: %d is not in templates.FormWidths %v, so it is never read", base, w, w, templates.FormWidths)
			}
			if !bases[base] {
				t.Errorf("%s.w%d.template has no base template", base, w)
			}
		}
	}
}

// templateRenderSetup renders the live world's templates with its English
// labels, as the server does.
func templateRenderSetup(t *testing.T) {
	t.Helper()
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	language.InitTranslation(language.BundleCfg{
		DefaultLanguage: textlang.English,
		Language:        textlang.English,
		LanguagePaths:   []string{"_datafiles/localize", "_datafiles/world/dogmud/localize"},
	})
	prev := configs.GetFilePathsConfig().DataFiles.String()
	if err := configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": dogmudDataFilesRoot}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": prev}) })
	templates.SetFSForTest(t, os.DirFS(dogmudDataFilesRoot).(fs.ReadFileFS))
}

func TestFittingTemplatesFitAtEveryWidth(t *testing.T) {
	templateRenderSetup(t)
	names := make([]string, 0, len(templateFits))
	for name := range templateFits {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, width := range templateWidths {
			fixture := templateFits[name]
			out, err := templates.ProcessAtWidth(name, fixture.data(), width)
			if err != nil {
				t.Errorf("%s at %d: %v", name, width, err)
				continue
			}
			if fixture.wrapped {
				out = messaging.WrapAnsi(out, width)
			}
			if strings.TrimSpace(out) == "" {
				t.Errorf("%s at %d rendered nothing; the fixture proves nothing", name, width)
				continue
			}
			if got := templates.WidestLine(out); got > width {
				t.Errorf("%s at %d columns has a line %d wide:\n%s", name, width, got, out)
			}
		}
	}
}

// A desktop sees what it always did: the width-aware banners draw their old
// 79-column rules at 80 and above, and a table that fits keeps its borders.
func TestFittingTemplatesKeepTheirFullWidthLook(t *testing.T) {
	templateRenderSetup(t)
	looks := map[string]any{
		"character/questup": "You have found a new quest.",
		"character/skillup": struct {
			SkillName  string
			SkillLevel int
		}{"brawling", 2},
		"character/statup": "Your strength has grown.",
		"tables/generic": templates.GetTable("Spells",
			[]string{"SpellId", "Name", "Target", "Cost"},
			[][]string{{"aurora", "Aurora of the Slumbering Deep", "group", "12"}}),
		"tables/shoplist": templateFits["tables/shoplist"].data(),
	}
	render := func(name string, width int) string {
		out, err := templates.ProcessAtWidth(name, looks[name], width)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return out
	}
	stars := strings.Repeat("*", 79)
	for _, name := range []string{"character/questup", "character/skillup", "character/statup"} {
		for _, width := range []int{80, 120} {
			out := render(name, width)
			if !strings.Contains(out, stars) || strings.Contains(out, stars+"*") {
				t.Errorf("%s at %d does not draw its 79-column rule", name, width)
			}
		}
	}
	for _, name := range []string{"tables/generic", "tables/shoplist"} {
		if !strings.Contains(render(name, 80), "╒") {
			t.Errorf("%s at 80 lost its border", name)
		}
	}
}

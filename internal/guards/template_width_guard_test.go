package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
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
// plan 3b-3 gave every template screen it held a narrow form and removed its
// class; plan 4 does "help/". When that is done, only "wide" and the classes
// that never reach a narrow screen unwrapped remain.

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
	// Fit at every width, held by internal/usercommands' own test
	// (TestNarrowTemplateScreensFitEveryWidth): the data its commands pass
	// is of types private to that package.
	"fits in usercommands": {"character/conditions", "character/quests", "character/title"},
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
// setup, when set, seeds the registries the data reads, restored when the
// test ends. more are further data the template is rendered with too, such
// as a second mode of the same screen. wrapped says the template goes out on
// a category the pipeline wraps to the reader, so the guard wraps it the same
// way before measuring.
type templateFixture struct {
	setup   func(t *testing.T)
	data    func() any
	more    []func() any
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
	}, more: []func() any{tableLeadingWithLaterColumn}},
	"tables/shoplist": {data: func() any {
		return templates.GetTable("Items available by Grumbleguts the Provisioner",
			[]string{"Qty", "Name", "Type", "Price"},
			[][]string{
				{"3", "a flask of clear river water", "drinkable", "12 gold"},
				{"1", "a lantern", "light", "40 gold"},
			}).WithLead(1)
	}, more: []func() any{func() any {
		// The secondhand shelf leads with its first column, the Name.
		return templates.GetTable("Secondhand goods by Grumbleguts the Provisioner",
			[]string{"Name", "Type", "Price"},
			[][]string{{"A Salt-Stiff Page of a Sailor's Telling", "readable", "1240"}})
	}}},
	"character/questup": {data: func() any { return "You have found a new quest: The Long Road Home." }, more: []func() any{questupCaptured, questupWide}, wrapped: true},
	// The banners (notice_screens_test.go). skillup and statup go out on
	// CategorySkillProgress, which the pipeline does not wrap.
	"character/skillup": {data: skillupWide, more: []func() any{skillupCaptured}},
	"character/statup":  {data: statupWide, more: []func() any{statupCaptured, statupLong}},
	// The character screens (character_screens_test.go).
	"character/status":    {setup: seedCharacterScreenRegistries, data: statusScreenFixture, more: []func() any{statusScreenLongName}},
	"character/inventory": {data: inventoryScreenFixture(false), more: []func() any{inventoryScreenFixture(true)}},
	"character/skills":    {data: skillsScreenFixture},
	"character/cooldowns": {data: cooldownsScreenFixture},
	"character/storage":   {data: storageScreenFixture},
	// The looking-at-things screens (looking_screens_test.go).
	"character/description":        {setup: seedLookingScreenRegistries, data: descriptionWide, more: []func() any{descriptionCaptured, shippedMobLooked(rissMobFile), shippedMobLooked(odellMobFile)}},
	"character/description-corpse": {data: corpseWide, more: []func() any{corpseCaptured, shippedMobLooked(rissMobFile), shippedMobLooked(odellMobFile)}},
	"character/inventory-look":     {data: inventoryLookFixture(true, false), more: []func() any{inventoryLookFixture(false, false), inventoryLookFixture(false, true)}},
	"character/pet":                {data: petFixture(true), more: []func() any{petFixture(false), petNamedAfterMob(rissMobFile), petNamedAfterMob(odellMobFile)}},
	"descriptions/biome":           {data: biomeStreetLamp, more: []func() any{biomeOutdoorCaptured, biomeInteriorCaptured}},
	"descriptions/identify":        {setup: seedLookingScreenRegistries, data: identifyWide, more: []func() any{identifyCaptured}},
	"descriptions/insidecontainer": {data: containerWide, more: []func() any{containerCaptured, containerEmpty}},
	"descriptions/track":           {data: trackWide, more: []func() any{trackCaptured}},
	"descriptions/room":            {data: roomViewFixture(true), more: []func() any{roomViewFixture(false), trackingAfterMob(rissMobFile), trackingAfterMob(odellMobFile)}},
	"descriptions/room-title":      {data: roomTitleWide, more: []func() any{roomTitleCaptured}},
	"descriptions/exits":           {data: exitsFixture(true), more: []func() any{exitsFixture(false), exitsMixed}},
	"descriptions/fixtures":        {data: fixturesWide, more: []func() any{fixturesShort}},
	"descriptions/rune":            {data: signWide, more: []func() any{signShort, signLongWord}},
	"descriptions/sign":            {data: signWide, more: []func() any{signShort, signLongWord}},
	// The notice screens (notice_screens_test.go).
	"mail/message": {data: mailWide, more: []func() any{mailCaptured, mailPlain, mailRead, mailLongWord}},
	// The auction broadcasts, read from the module's own templates.
	"auctions/auction-bid":    {setup: withAuctionTemplates, data: auctionWide, more: auctionFixtures},
	"auctions/auction-end":    {setup: withAuctionTemplates, data: auctionWide, more: auctionFixtures},
	"auctions/auction-start":  {setup: withAuctionTemplates, data: auctionWide, more: auctionFixtures},
	"auctions/auction-update": {setup: withAuctionTemplates, data: auctionWide, more: auctionFixtures},
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

// Every narrow form closes each colour tag it opens, so no colour runs on
// into the lines after it. A form is checked as written, tags in template
// strings included, so a tag opened in one action and closed in another
// still balances.
func TestTemplateFormsBalanceTheirColourTags(t *testing.T) {
	opens, closes := regexp.MustCompile(`<ansi[\s>]`), regexp.MustCompile(`</ansi>`)
	checked := 0
	for _, root := range templateFolders(t) {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !templateFormFile.MatchString(filepath.ToSlash(path)) {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			checked++
			o, c := len(opens.FindAllIndex(raw, -1)), len(closes.FindAllIndex(raw, -1))
			if o != c {
				t.Errorf("%s opens %d <ansi> tags and closes %d", path, o, c)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if checked < 20 {
		t.Fatalf("checked only %d form files; the walk is not reaching them", checked)
	}
}

// The "fits in usercommands" class is held, not just named: each template in
// it has a 42 and a 30 form, and internal/usercommands' narrow_forms_test.go
// lists it in narrowTemplateScreens, which TestNarrowTemplateScreensFitEveryWidth
// renders at every width in narrowWidths, which must hold 30 and 42. The second part is read from that file's source,
// since a test in another package cannot be called from here.
func TestFitsInUsercommandsClassIsHeld(t *testing.T) {
	_, forms := templateBases(t)
	held := templateClassNames["fits in usercommands"]
	if len(held) == 0 {
		t.Fatal(`the "fits in usercommands" class is empty; the check proves nothing`)
	}
	for _, name := range held {
		have := map[int]bool{}
		for _, w := range forms[name] {
			have[w] = true
		}
		for _, w := range []int{42, 30} {
			if !have[w] {
				t.Errorf("%s is in \"fits in usercommands\" but has no .w%d.template", name, w)
			}
		}
	}

	path := filepath.Join(repoRoot(t), "internal", "usercommands", "narrow_forms_test.go")
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	widths := map[string]bool{}
	renders, atWidths := false, false
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ValueSpec:
			if len(x.Names) == 1 && x.Names[0].Name == "narrowWidths" && len(x.Values) == 1 {
				if lit, ok := x.Values[0].(*ast.CompositeLit); ok {
					for _, el := range lit.Elts {
						if b, ok := el.(*ast.BasicLit); ok {
							widths[b.Value] = true
						}
					}
				}
			}
			if len(x.Names) == 1 && x.Names[0].Name == "narrowTemplateScreens" && len(x.Values) == 1 {
				if lit, ok := x.Values[0].(*ast.CompositeLit); ok {
					for _, el := range lit.Elts {
						if kv, ok := el.(*ast.KeyValueExpr); ok {
							if key, ok := kv.Key.(*ast.BasicLit); ok {
								listed[strings.Trim(key.Value, "`\"")] = true
							}
						}
					}
				}
			}
		case *ast.FuncDecl:
			if x.Name.Name == "TestNarrowTemplateScreensFitEveryWidth" {
				ast.Inspect(x.Body, func(m ast.Node) bool {
					if r, ok := m.(*ast.RangeStmt); ok {
						if id, ok := r.X.(*ast.Ident); ok && id.Name == "narrowTemplateScreens" {
							renders = true
						}
						if id, ok := r.X.(*ast.Ident); ok && id.Name == "narrowWidths" {
							atWidths = true
						}
					}
					return true
				})
			}
		}
		return true
	})
	if !renders {
		t.Errorf("%s: TestNarrowTemplateScreensFitEveryWidth no longer ranges over narrowTemplateScreens", path)
	}
	if !atWidths {
		t.Errorf("%s: TestNarrowTemplateScreensFitEveryWidth no longer ranges over narrowWidths", path)
	}
	for _, w := range []string{"30", "42"} {
		if !widths[w] {
			t.Errorf("%s: narrowWidths does not hold %s, so the fit test never renders the forms there", path, w)
		}
	}
	for _, name := range held {
		if !listed[name] {
			t.Errorf("%s is in \"fits in usercommands\" but %s's narrowTemplateScreens does not render it", name, path)
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
		fixture := templateFits[name]
		t.Run(name, func(t *testing.T) {
			if fixture.setup != nil {
				fixture.setup(t)
			}
			for i, data := range append([]func() any{fixture.data}, fixture.more...) {
				for _, width := range templateWidths {
					out, err := templates.ProcessAtWidth(name, data(), width)
					if err != nil {
						t.Errorf("%s (data %d) at %d: %v", name, i, width, err)
						continue
					}
					if fixture.wrapped {
						out = messaging.WrapAnsi(out, width)
					}
					if strings.TrimSpace(out) == "" {
						t.Errorf("%s (data %d) at %d rendered nothing; the fixture proves nothing", name, i, width)
						continue
					}
					if got := templates.WidestLine(out); got > width {
						t.Errorf("%s (data %d) at %d columns has a line %d wide:\n%s", name, i, width, got, out)
					}
				}
			}
		})
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

// tableLeadingWithLaterColumn is a table that names a later column as its
// lead (plan 3b-3, ruling 6), as keyring does, with the longest shipped room
// title and a lock sequence of difficulty 30 not yet learned.
func tableLeadingWithLaterColumn() any {
	return templates.GetTable("Your Keyring:",
		[]string{"Type", "Location", "Where", "Sequence"},
		[][]string{
			{"Lockpick", "#60557 The Crossing, Great Hall Over the Waters", "north", "U D " + strings.Repeat("? ", 28)},
			{"Key", "#60557 Town Square", "lockbox", "-"},
		}).WithLead(2)
}

package main

import (
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditionrename"
)

// Slice 2 of the conditions unification renamed every Go identifier and every
// player- or admin-facing template field reference that said buff to
// condition (docs/superpowers/specs/completed/2026-09-14-conditions-unification-slice-2-rename-design.md).
// Slice 3 renamed every on-disk, wire and content spelling
// (docs/superpowers/specs/completed/2026-09-15-conditions-unification-slice-3-disk-wire-design.md).
// These guards keep the word from coming back.

const identifierGuardSpecPath = "docs/superpowers/specs/completed/2026-09-15-conditions-unification-slice-3-disk-wire-design.md"

// identifierGuardWordPattern matches "buff" in any case.
// identifierGuardBufferSubstring strips every case-sensitive "buffer",
// "Buffer" or "BUFFER" run out of an identifier before it is tested for
// "buff" again, so an identifier about bytes.Buffer / ring buffers / io
// buffering reads as exempt (spec: "not buff at all: identifiers containing
// Buffer/buffer") while one that merely CONTAINS the word "buffer" only
// case-insensitively (e.g. "buffErr", which lowercases to "bufferr") still
// reads as a real leftover "buff" identifier once that substring fails to
// strip. A plain `(?i)buffer` exemption let "buffErr" through undetected;
// this two-step check is the fix.
var (
	identifierGuardWordPattern     = regexp.MustCompile(`(?i)buff`)
	identifierGuardBufferSubstring = regexp.MustCompile(`[Bb]uffer|BUFFER`)
	// Every buff-spelled Go identifier is gone (slice 3 deleted the last
	// three, dead config knobs). Kept empty so a future exception is a
	// reviewed one-line addition.
	identifierGuardAllowedNames = map[string]bool{}
)

// identifierGuardSkipDir reports whether a directory should never be
// descended into: version control, vendored/generated trees, and the world
// data files (which carry the frozen `buffid`/`buffids`/`permabuff` disk
// keys in YAML, not Go identifiers).
func identifierGuardSkipDir(name string) bool {
	if name == ".git" || name == "_datafiles" || name == "node_modules" || name == "vendor" {
		return true
	}
	return strings.HasPrefix(name, ".")
}

// TestNoIdentifierSaysBuff tokenizes every .go file in the repo with
// go/scanner and fails on any token.IDENT containing "buff" outside the
// buffer/config-field exemptions. Tokenizing rather than grepping means
// comments, string literals and struct tags (which carry frozen disk/wire
// keys such as `buffid` and `permabuff-...`) are never IDENT tokens and never
// reach this check; only real declared or referenced Go names do.
func TestNoIdentifierSaysBuff(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed; cannot anchor the repo root")
	}
	root := filepath.Dir(here)
	guardFile := filepath.Clean(here)

	type offense struct {
		file string
		line int
		name string
	}
	var offenses []offense
	scanned := 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				// behaviortree tests write temp crate files under gitignored
				// internal/**/_datafiles/, which can vanish mid-walk while
				// packages test in parallel (spec: "flake seen 2026-09-14").
				return nil
			}
			return err
		}
		if d.IsDir() {
			if path != root && identifierGuardSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if filepath.Clean(path) == guardFile {
			// This file necessarily names the word it checks for (its own
			// test names are pinned by the plan: TestNoIdentifierSaysBuff,
			// TestNoTemplateReadsABuffField, and the helpers around them), so
			// it cannot check itself without a special case that would grow
			// without bound. Scanning every OTHER .go file in the repo is
			// the actual guarantee; this one file is exempt by construction.
			return nil
		}

		src, rerr := os.ReadFile(path)
		if rerr != nil {
			if os.IsNotExist(rerr) {
				return nil
			}
			return rerr
		}
		scanned++

		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)

		fset := token.NewFileSet()
		tf := fset.AddFile(path, fset.Base(), len(src))
		var sc scanner.Scanner
		// No ScanComments flag: comments never surface as tokens at all, so
		// they cannot be mistaken for identifiers.
		sc.Init(tf, src, nil, 0)
		for {
			pos, tok, lit := sc.Scan()
			if tok == token.EOF {
				break
			}
			if tok != token.IDENT {
				continue
			}
			if lit == "_" || !identifierGuardWordPattern.MatchString(lit) {
				continue
			}
			remainder := identifierGuardBufferSubstring.ReplaceAllString(lit, "")
			if identifierGuardAllowedNames[lit] || !identifierGuardWordPattern.MatchString(remainder) {
				continue
			}
			offenses = append(offenses, offense{rel, fset.Position(pos).Line, lit})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if scanned == 0 {
		t.Fatalf("scanned zero .go files under %s; the guard would pass vacuously", root)
	}

	sort.Slice(offenses, func(i, j int) bool {
		if offenses[i].file != offenses[j].file {
			return offenses[i].file < offenses[j].file
		}
		return offenses[i].line < offenses[j].line
	})
	for _, o := range offenses {
		t.Errorf("%s:%d: identifier %s still says buff; the conditions unification renamed these (%s)",
			o.file, o.line, o.name, identifierGuardSpecPath)
	}
}

// templateBuffFieldAllowlist pardons a buff-spelled dotted reference found
// OUTSIDE a `{{ ... }}` template action (JS, HTML attribute text) that is
// verified wire: a GMCP JSON field name or an on-disk save key, neither of
// which slice 2 may touch (owner disk/wire rule). Keyed "relpath|match".
// A hit INSIDE a template action is always a live Go field or map-key
// reference the template engine resolves at render time with no compile-time
// check, so it is never allowlisted here; it is a real rename instead.
var templateBuffFieldAllowlist = map[string]string{}

// templateActionPattern matches a whole `{{ ... }}` template action,
// including trim markers (`{{-`/`-}}`), non-greedily and across lines so a
// multi-line action is still recognised as one span.
var templateActionPattern = regexp.MustCompile(`(?s)\{\{.*?\}\}`)

// templateBuffFieldPattern is the field/method reference shape a Go template
// resolves at runtime: a dot immediately followed by a name containing buff.
var templateBuffFieldPattern = regexp.MustCompile(`\.[A-Za-z_]*[Bb]uff[A-Za-z_]*`)

type byteSpan struct{ start, end int }

func (s byteSpan) contains(offset int) bool { return offset >= s.start && offset < s.end }

// jsHasTemplateActions reports whether a .js file is itself run through the
// Go template engine (embedded in an .html page's <script>, or a standalone
// .js served through templates.Process). Only such files can carry a live Go
// field reference; a plain vendored or hand-written .js file cannot, so it is
// never scanned (spec: "or .js under _datafiles/html only if it contains Go
// template actions; otherwise skip JS").
func jsHasTemplateActions(src []byte) bool {
	return templateActionPattern.Match(src)
}

// TestNoTemplateReadsABuffField scans every shipped template and admin HTML
// page for a dotted reference to a buff-named Go field or map key. A hit
// inside a `{{ ... }}` template action is a live reference the template
// engine resolves at render time with no compile-time check, so it always
// fails; a hit outside one (JS, HTML attribute text) is allowed only when
// templateBuffFieldAllowlist records it as verified wire.
func TestNoTemplateReadsABuffField(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed; cannot anchor the repo root")
	}
	root := filepath.Dir(here)

	type offense struct {
		file string
		line int
		name string
	}
	var offenses []offense
	scanned := 0
	seenAllowlist := map[string]bool{}

	scanFile := func(path string) error {
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			if os.IsNotExist(rerr) {
				return nil
			}
			return rerr
		}

		isJS := strings.HasSuffix(path, ".js")
		if isJS && !jsHasTemplateActions(src) {
			return nil
		}
		scanned++

		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)

		var actions []byteSpan
		for _, loc := range templateActionPattern.FindAllIndex(src, -1) {
			actions = append(actions, byteSpan{loc[0], loc[1]})
		}
		inAction := func(offset int) bool {
			for _, a := range actions {
				if a.contains(offset) {
					return true
				}
			}
			return false
		}

		for _, loc := range templateBuffFieldPattern.FindAllIndex(src, -1) {
			match := string(src[loc[0]:loc[1]])
			line := 1 + strings.Count(string(src[:loc[0]]), "\n")
			if inAction(loc[0]) {
				offenses = append(offenses, offense{rel, line, match})
				continue
			}
			key := rel + "|" + match
			if _, ok := templateBuffFieldAllowlist[key]; ok {
				seenAllowlist[key] = true
				continue
			}
			offenses = append(offenses, offense{rel, line, match})
		}
		return nil
	}

	// _datafiles/world/*/templates/** : every shipped world's templates.
	worldTemplateRoots, gerr := filepath.Glob(filepath.Join(root, "_datafiles", "world", "*", "templates"))
	if gerr != nil {
		t.Fatalf("glob world template roots: %v", gerr)
	}
	if len(worldTemplateRoots) == 0 {
		t.Fatalf("no _datafiles/world/*/templates directories found under %s", root)
	}
	for _, tplRoot := range worldTemplateRoots {
		err := filepath.WalkDir(tplRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".template") {
				return nil
			}
			return scanFile(path)
		})
		if err != nil {
			t.Fatalf("walk %s: %v", tplRoot, err)
		}
	}

	// _datafiles/html/** : the admin panel and the web/Mudlet client pages.
	htmlRoot := filepath.Join(root, "_datafiles", "html")
	err := filepath.WalkDir(htmlRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".html") && !strings.HasSuffix(path, ".js") {
			return nil
		}
		return scanFile(path)
	})
	if err != nil {
		t.Fatalf("walk %s: %v", htmlRoot, err)
	}

	if scanned == 0 {
		t.Fatalf("scanned zero .template/.html/.js files under %s; the guard would pass vacuously", root)
	}

	sort.Slice(offenses, func(i, j int) bool {
		if offenses[i].file != offenses[j].file {
			return offenses[i].file < offenses[j].file
		}
		return offenses[i].line < offenses[j].line
	})
	for _, o := range offenses {
		t.Errorf("%s:%d: template field %s still says buff; the conditions unification renamed these (%s)",
			o.file, o.line, o.name, identifierGuardSpecPath)
	}

	// A stale allowlist entry pardons a match that has moved or is gone,
	// which pardons whatever happens to sit at that key now instead (the
	// same staleness check condition_apply_path_guard_test.go runs on
	// conditionApplyPathAllowlist).
	var stale []string
	for key := range templateBuffFieldAllowlist {
		if !seenAllowlist[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	for _, key := range stale {
		t.Errorf("templateBuffFieldAllowlist entry %q matched nothing; find where it moved (or was fixed) and update or remove it", key)
	}
}

// stringDataAllowlist pardons a buff spelling that is deliberate. Keyed
// "relpath|exact line content (trimmed)"; each entry must still match.
// Dewey's typed `buff` keyword (9491.yaml) left with the ask engine (#375):
// topics are chosen from a menu, so no player word needs pardoning.
var stringDataAllowlist = map[string]string{}

// stringGuardNamesOldSpellings are Go files whose job is to name the old
// spellings: tests that assert they no longer bind or resolve, and the
// migration that renames them in saves.
var stringGuardNamesOldSpellings = map[string]bool{
	"internal/guards/wire_freeze_test.go":                   true,
	"internal/keywords/keywords_setcondition_alias_test.go": true,
	"internal/migration/0.17.0.go":                          true,
	"internal/migration/0.17.0_test.go":                     true,
}

var stringDataExts = map[string]bool{
	".yaml": true, ".yml": true, ".template": true, ".html": true, ".js": true, ".css": true, ".md": true, ".golden": true,
}

// TestNoStringOrDataSaysBuff is the slice 3 half of this guard: Go string
// literals and struct tags (go/scanner STRING tokens), and every tracked data,
// template, web, golden and doc file under _datafiles/, docs/schemas/ and
// internal/**/testdata, must not spell buff outside the protected words
// conditionrename.ContainsOldSpelling ignores.
func TestNoStringOrDataSaysBuff(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require := func(cond bool, format string, args ...any) {
		t.Helper()
		if !cond {
			t.Fatalf(format, args...)
		}
	}
	require(ok, "runtime.Caller(0) failed")
	root := filepath.Dir(here)
	guardFile := filepath.Base(here)

	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	require(err == nil, "git ls-files: %v", err)

	seen := map[string]bool{}
	scannedGo, scannedData := 0, 0
	for _, rel := range strings.Split(string(out), "\x00") {
		if rel == "" {
			continue
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "vendor/") || strings.HasPrefix(rel, "docs/superpowers/") ||
			strings.HasPrefix(rel, "tools/playtest/") || strings.HasPrefix(rel, "tools/_archive/") ||
			rel == "docs/PATCH_NOTES.md" || rel == guardFile || strings.HasPrefix(rel, "internal/conditionrename/") ||
			stringGuardNamesOldSpellings[rel] {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			if os.IsNotExist(rerr) {
				continue
			}
			t.Fatalf("read %s: %v", rel, rerr)
		}

		if strings.HasSuffix(rel, ".go") {
			scannedGo++
			fset := token.NewFileSet()
			tf := fset.AddFile(path, fset.Base(), len(src))
			var sc scanner.Scanner
			sc.Init(tf, src, nil, 0)
			for {
				pos, tok, lit := sc.Scan()
				if tok == token.EOF {
					break
				}
				if tok == token.STRING && conditionrename.ContainsOldSpelling(lit) {
					t.Errorf("%s:%d: string literal %s still says buff (%s)", rel, fset.Position(pos).Line, lit, identifierGuardSpecPath)
				}
			}
			continue
		}

		inScope := strings.HasPrefix(rel, "_datafiles/") || strings.HasPrefix(rel, "docs/schemas/") ||
			(strings.HasPrefix(rel, "internal/") && strings.Contains(rel, "/testdata/"))
		if !inScope || !stringDataExts[filepath.Ext(rel)] {
			continue
		}
		scannedData++
		for i, line := range strings.Split(string(src), "\n") {
			if !conditionrename.ContainsOldSpelling(line) {
				continue
			}
			key := rel + "|" + strings.TrimSpace(line)
			if _, ok := stringDataAllowlist[key]; ok {
				seen[key] = true
				continue
			}
			t.Errorf("%s:%d: still says buff: %s (%s)", rel, i+1, strings.TrimSpace(line), identifierGuardSpecPath)
		}
	}
	require(scannedGo > 500, "scanned only %d Go files", scannedGo)
	require(scannedData > 3000, "scanned only %d data files", scannedData)
	for key := range stringDataAllowlist {
		if !seen[key] {
			t.Errorf("stringDataAllowlist entry %q matched nothing; update or remove it", key)
		}
	}
}

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
)

// KNOWN LIMITS of the template check (each is a way a real leak can pass):
//   - Reflection on the Go side (reflect.ValueOf(c).MethodByName("AllConfigData"))
//     is invisible to the AST scan, which only matches a literal
//     *ast.SelectorExpr by name.
//   - A template func other than getconfig that returns configs.Config or
//     reaches Modules would evade every pattern here, since only "getconfig"
//     and the ".CONFIG" field name are matched by name. Checked
//     internal/web/template_func.go's funcMap on 2026-09-28: getconfig is
//     the only entry returning configs.Config; none returns Modules
//     directly. This limit stays live if one is added later.
//   - Not a limit, named because it looks like one: template scope
//     rebinding ({{ with .CONFIG }}{{ .Modules }}{{ end }}) is caught,
//     because "with .CONFIG" puts ".CONFIG" before a space rather than a
//     field selector, which the bare ".CONFIG" rule flags.
//   - Not a real route, named for completeness: a typed sub-section cannot
//     reach Modules by way of a captured variable ($s := .CONFIG.Server;
//     $s.Modules) because Modules is a field of Config itself, not of any
//     typed sub-struct; there is no field to reach that way.
//
// allConfigDataCallers lists repo-relative files outside internal/configs
// allowed to call a raw config reader, each with its reason. AllConfigData,
// DotPaths and GetOverrides return secrets raw; they exist because
// internal/configs builds the key and type lookups and the override file
// from them. Anything a person reads (logs, command output, web pages) goes
// through DisplayConfigData. A caller that needs only a key's TYPE may be
// added here with its reason. Empty on master: every outside caller was a
// display.
var allConfigDataCallers = map[string]string{}

// rawConfigSelectors are the Go selectors that return config values raw.
var rawConfigSelectors = map[string]bool{
	"AllConfigData": true,
	"DotPaths":      true,
	"GetOverrides":  true,
}

// rawConfigInTemplate matches a page template reaching raw config values by
// name: the three raw readers wherever they appear (including as
// `.CONFIG.AllConfigData`, since the selector name is a literal substring of
// that path), `.CONFIG.Modules` explicitly, and the getconfig template func
// (internal/web/template_func.go) anywhere at all, not only when it is
// chained straight into `.Modules` in the same action: `{{ $c := getconfig
// }}{{ $c.Modules }}` reaches the same map through a template variable.
var rawConfigInTemplate = regexp.MustCompile(`AllConfigData|DotPaths|GetOverrides|\.CONFIG\.Modules\b|\bgetconfig\b`)

// dotConfigSelector finds every `.CONFIG` reference and captures the field
// it immediately reaches, if any. `.CONFIG.Server` (group 1 = ".Server")
// reaches one typed field and is left to the caller; a bare `.CONFIG` (group
// 1 empty) reaches the whole Config value, which is how a template dumps
// Modules raw without naming it: printed directly (`{{ .CONFIG }}`) or
// captured into a variable for later use (`{{ $c := .CONFIG }}`).
var dotConfigSelector = regexp.MustCompile(`\.CONFIG\b(\.[A-Za-z_][A-Za-z0-9_]*)?`)

// findRawConfigInTemplate returns one marker string per raw-config read it
// finds in a template body: every rawConfigInTemplate hit by name, plus a
// bare ".CONFIG" for each reference that reaches the whole Config value
// instead of one typed field.
func findRawConfigInTemplate(body string) []string {
	found := rawConfigInTemplate.FindAllString(body, -1)
	for _, sm := range dotConfigSelector.FindAllStringSubmatch(body, -1) {
		if sm[1] == "" {
			found = append(found, ".CONFIG")
		}
	}
	return found
}

// TestFindRawConfigInTemplateMatcher is the matcher's own unit test, table
// driven so a probe string can be added and proven red before the matcher
// is fixed to catch it.
func TestFindRawConfigInTemplateMatcher(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantFound bool
	}{
		{"modules via dot config", `{{ .CONFIG.Modules }}`, true},
		{"getconfig modules", `{{ (getconfig).Modules }}`, true},
		{"bare getconfig", `{{ getconfig }}`, true},
		{"bare dot config struct print", `{{ .CONFIG }}`, true},
		{"dot config assigned to a template var", `{{ $c := .CONFIG }}{{ $c.Modules }}`, true},
		{"legit server mudname", `{{ .CONFIG.Server.MudName }}`, false},
		{"legit filepaths webcdn", `{{ .CONFIG.FilePaths.WebCDNLocation }}`, false},
		{"legit network telnetport", `{{ .CONFIG.Network.TelnetPort }}`, false},
		{"legit displayconfigdata", `{{ range $n, $v := (.CONFIG.DisplayConfigData "modules*") }}{{ end }}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findRawConfigInTemplate(tc.body)
			if tc.wantFound && len(got) == 0 {
				t.Errorf("findRawConfigInTemplate(%q) = %v, want at least one match", tc.body, got)
			}
			if !tc.wantFound && len(got) != 0 {
				t.Errorf("findRawConfigInTemplate(%q) = %v, want no match", tc.body, got)
			}
		})
	}
}

// TestNoDisplayReadsRawConfig fails when a Go file outside internal/configs,
// or any page template, reads the raw config. Slice M found four displays
// (boot log, server set listing, server config menu, /viewconfig) printing
// the companion's APIKey raw because each read the raw view.
func TestNoDisplayReadsRawConfig(t *testing.T) {
	root := repoRoot(t)
	var err error

	var offenders []string
	scanned := map[string]bool{}
	fset := token.NewFileSet()

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			// Dot-dirs include agent worktrees under .claude/, a second copy of the tree.
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			switch d.Name() {
			case "vendor", "node_modules", "docs", "testdata":
				return filepath.SkipDir
			}
			if rel == "internal/configs" {
				return filepath.SkipDir
			}
			return nil
		}
		if _, ok := allConfigDataCallers[rel]; ok {
			return nil
		}

		switch {
		case strings.HasSuffix(rel, ".go"):
			if strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if perr != nil {
				// A syntax error is the compiler's to report.
				return nil
			}
			scanned[rel] = true
			ast.Inspect(file, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok && rawConfigSelectors[sel.Sel.Name] {
					offenders = append(offenders, rel+":"+strconv.Itoa(fset.Position(sel.Pos()).Line)+" "+sel.Sel.Name)
				}
				return true
			})
		case strings.HasSuffix(rel, ".html"), strings.HasSuffix(rel, ".template"), strings.HasSuffix(rel, ".tmpl"):
			body, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			scanned[rel] = true
			for _, m := range findRawConfigInTemplate(string(body)) {
				offenders = append(offenders, rel+" "+m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}

	// Verify the negative: the walk must have read the files that held the
	// four displays, or an empty offender list proves nothing.
	for _, must := range []string{"main.go", "boot_config_log.go",
		"internal/usercommands/admin.server.go", "_datafiles/html/public/viewconfig.html"} {
		if !scanned[must] {
			t.Errorf("guard never scanned %s; the walk cannot see what it guards", must)
		}
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("%d raw config read(s) outside internal/configs. AllConfigData, DotPaths, GetOverrides\n"+
			"and the Modules map return secrets raw.\n"+
			"Use DisplayConfigData for anything a person reads, or add the file to\n"+
			"allConfigDataCallers in this file with the reason it needs only key types.\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}
